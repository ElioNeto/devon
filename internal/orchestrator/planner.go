package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ElioNeto/devon/internal/llm"
)

const (
	userTaskPlaceholder = "[[USER_TASK]]"
	planResultLabel     = "PLANNED_TASKS:"
)

// Task representa uma subtask no DAG de execução.
type Task struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	DependsOn   []string `json:"depends_on"`
	Priority    int      `json:"priority"`
}

// Plan é um conjunto de Tasks organizadas em DAG.
type Plan struct {
	Tasks     []Task `json:"tasks"`
	RootTasks []Task `json:"root_tasks"`
}

// Planner usa LLM para decompor uma task em subtasks.
type Planner struct {
	client     llm.Streamer
	promptTemp string
}

// NewPlanner cria um novo Planner.
func NewPlanner(client llm.Streamer) *Planner {
	return &Planner{
		client:     client,
		promptTemp: defaultSystemPrompt,
	}
}

// Plan decompõe uma task complexa em subtasks.
func (p *Planner) Plan(ctx context.Context, userTask string, maxTasks int) (*Plan, error) {
	prompt := fmt.Sprintf(p.promptTemp, userTask, maxTasks)

	// Chama o LLM para gerar o plano
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: llm.TextContent(defaultSystemPrompt)},
		{Role: llm.RoleUser, Content: llm.TextContent(prompt)},
	}

	stream, err := p.client.Stream(ctx, messages, []llm.ToolDef{})
	if err != nil {
		return nil, fmt.Errorf("llm stream: %w", err)
	}

	var textBuf strings.Builder
	for ev := range stream {
		switch ev.Type {
		case "text":
			textBuf.WriteString(ev.Text)
		case "error":
			return nil, ev.Err
		case "done":
			break
		}
	}

	text := textBuf.String()
	return parsePlan(text, userTask)
}

// parsePlan extrai JSON de texto livre.
func parsePlan(text, userTask string) (*Plan, error) {
	text = strings.TrimSpace(text)

	// Procura por PLANNED_TASKS: ou tenta parsear direto
	start := 0
	if idx := strings.Index(text, planResultLabel); idx >= 0 {
		start = idx + len(planResultLabel)
	}

	// Procura primeiro {
	jsonStart := strings.Index(text[start:], "{")
	if jsonStart == -1 {
		return pFallback(userTask), errors.New("no JSON found in response")
	}
	jsonStart += start

	// Conta chaves para fechar corretamente
	bracketCount := 0
	jsonEnd := -1
	for i := jsonStart; i < len(text); i++ {
		switch text[i] {
		case '{':
			bracketCount++
		case '}':
			bracketCount--
			if bracketCount == 0 {
				jsonEnd = i + 1
				break
			}
		}
	}

	if jsonEnd == -1 {
		jsonEnd = len(text)
	}

	jsonStr := text[jsonStart:jsonEnd]

	var plan Plan
	if err := json.Unmarshal([]byte(jsonStr), &plan); err != nil {
		return pFallback(userTask), err
	}

	if len(plan.Tasks) == 0 || (len(plan.RootTasks) == 0 && len(plan.Tasks) > 0) {
		// Se não tem rootTasks, calcula automaticamente
		hasDep := make(map[string]bool)
		for _, t := range plan.Tasks {
			for _, d := range t.DependsOn {
				hasDep[d] = true
			}
		}
		for _, t := range plan.Tasks {
			if !hasDep[t.ID] {
				plan.RootTasks = append(plan.RootTasks, t)
			}
		}
	}

	return &plan, nil
}

// PlanSimple retorna plano fallback sem LLM.
func (p *Planner) PlanSimple(userTask string) *Plan {
	return pFallback(userTask)
}

func pFallback(userTask string) *Plan {
	return &Plan{
		Tasks: []Task{
			{ID: "1", Description: userTask, Priority: 1},
		},
		RootTasks: []Task{{ID: "1", Description: userTask, Priority: 1}},
	}
}

const defaultSystemPrompt = `%s

Task Plan (JSON array with objects containing: id, description, depends_on array, priority 1-5).
Max %d tasks.
`
