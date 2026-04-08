package orchestrator

package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ElioNeto/devon/internal/llm"
)

type jsonHelper struct{}

func (j jsonHelper) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

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
	RootTasks []Task `json:"root_tasks"` // tasks sem dependências
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

//分解 decompõe uma task complexa em subtasks.
func (p *Planner) Plan(ctx context.Context, userTask string, maxTasks int) (*Plan, error) {
	prompt := fmt.Sprintf("%s\n\nUser Task: %s\n\nReturn JSON with tasks array (max %d), where each task has: id, description, depends_on (array of task IDs), and priority (1-5).",
		p.promptTemp, userTask, maxTasks)

	// Chama o LLM para gerar o plano
	resp, err := p.client.Generate(ctx, prompt, []llm.ToolDef{})
	if err != nil {
		return nil, fmt.Errorf("failed to generate plan: %w", err)
	}

	// Parse da resposta
	plan, err := parsePlan(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse plan: %w", err)
	}

	return plan, nil
}

// Decompor manual - usa heurística simples se LLM não disponível
func (p *Planner) PlanSimple(userTask string) *Plan {
	// Detecção simples baseada em palavras-chave
	tasks := []Task{
		{
			ID:          "1",
			Description: userTask,
			Priority:    1,
		},
	}

	rootTasks := []Task{tasks[0]}

	return &Plan{
		Tasks:     tasks,
		RootTasks: rootTasks,
	}
}

// parsePlan extrai JSON de texto livre.
func parsePlan(text string) (*Plan, error) {
	// Tenta encontrar blocos JSON
	text = strings.Trim(text)
	if !strings.HasPrefix(text, "{") {
		// Procura primeiro {
		start := strings.Index(text, "{")
		if start == -1 {
			return nil, fmt.Errorf("no JSON found in response")
		}
		text = text[start:]
	}

	var plan Plan
	if err := json.Unmarshal([]byte(text), &plan); err != nil {
		// Se falhou, retorna plano simples
		return &Plan{
			Tasks: []Task{
				{ID: "1", Description: userTaskOverride, Priority: 1},
			},
			RootTasks: []Task{{ID: "1", Description: userTaskOverride, Priority: 1}},
		}, nil
	}

	return &plan, nil
}

const defaultSystemPrompt = `You are a task planner. Decompose complex requests into sequential or parallel subtasks.
Each task should have:
- id: unique identifier (e.g., "1", "2", "3")
- description: what this task should accomplish
- depends_on: array of task IDs this task depends on (empty if none)
- priority: 1-5 (lower = more important)`

const userTaskOverride = "Complete the user's original task"
