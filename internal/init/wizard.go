package init

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
)

// InteractiveQuestion represents a question to ask the user.
type InteractiveQuestion struct {
	Question      string
	Default       string
	ValidatorFunc func(string) error
}

// Wizard handles interactive user input for initialization.
type Wizard struct {
	detector    *Detector
	reader      *bufio.Reader
	info        ProjectInfo
	questions   []InteractiveQuestion
	currentStep int
}

// NewWizard creates a new interactive wizard.
func NewWizard(detector *Detector) *Wizard {
	return &Wizard{
		detector: detector,
		reader:   bufio.NewReader(os.Stdin),
	}
}

// Run runs the wizard flow interactively.
func (w *Wizard) Run() (ProjectInfo, error) {
	var err error

	w.info, err = w.detector.Detect(context.Background())
	if err != nil {
		return w.info, fmt.Errorf("falha ao detectar projeto: %w", err)
	}

	// Step 1: Project name
	w.info.ProjectName, err = w.askString("Nome do projeto:", w.info.ProjectName, nil)
	if err != nil {
		return w.info, err
	}

	// Step 2: Description
	w.info.Description, err = w.askString("Descrição em uma linha:", "", nil)
	if err != nil {
		return w.info, err
	}

	// Step 3: Test command
	w.info.TestCommand, err = w.askString("Como rodar os testes?", w.info.TestCommand, nil)
	if err != nil {
		return w.info, err
	}

	// Step 4: Build command
	w.info.BuildCommand, err = w.askString("Como compilar?", w.info.BuildCommand, nil)
	if err != nil {
		return w.info, err
	}

	// Step 5: Conventions
	conv, err := w.askString("Convenções importantes (opcional):", "", nil)
	if err != nil {
		return w.info, err
	}
	if conv != "" {
		w.info.Conventions = append(w.info.Conventions, conv)
	}

	return w.info, nil
}

// askString prompts the user for a string value.
func (w *Wizard) askString(question string, defaultValue string, validator func(string) error) (string, error) {
	fmt.Printf("\n%s", question)
	if defaultValue != "" {
		fmt.Printf(" [%s]", defaultValue)
	}
	fmt.Print("\n> ")

	input, err := w.reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("falha ao ler input: %w", err)
	}

	value := strings.TrimSpace(input)
	if value == "" {
		value = defaultValue
	}

	if validator != nil {
		if err := validator(value); err != nil {
			return "", err
		}
	}

	return value, nil
}

// RunNonInteractive runs the wizard in non-interactive mode using detected values.
func (w *Wizard) RunNonInteractive(ctx context.Context) (ProjectInfo, error) {
	return w.detector.Detect(ctx)
}
