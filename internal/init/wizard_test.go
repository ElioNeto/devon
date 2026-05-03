package init

import (
	"bufio"
	"context"
	"strings"
	"testing"
)

func TestWizard_RunNonInteractive(t *testing.T) {
	// Create temp dir with go.mod
	tmpDir := t.TempDir()
	err := writeGoMod(t, tmpDir, "test-project")
	if err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	detector := NewDetector(tmpDir)
	wizard := NewWizard(detector)

	info, err := wizard.RunNonInteractive(context.Background())
	if err != nil {
		t.Fatalf("RunNonInteractive() failed: %v", err)
	}

	if info.ProjectName != "test-project" {
		t.Errorf("ProjectName = %q, want %q", info.ProjectName, "test-project")
	}
	if info.Language.Name != "Go" {
		t.Errorf("Language.Name = %q, want %q", info.Language.Name, "Go")
	}
}

func TestWizard_askString(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		defaultVal  string
		want        string
		wantErr     bool
	}{
		{
			name:       "with input",
			input:      "test-input\n",
			defaultVal: "",
			want:       "test-input",
		},
		{
			name:       "empty input uses default",
			input:      "\n",
			defaultVal: "default-val",
			want:       "default-val",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a wizard with mocked reader
			w := &Wizard{
				reader: bufio.NewReader(strings.NewReader(tt.input)),
			}

			got, err := w.askString("Question:", tt.defaultVal, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("askString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("askString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWizard_Run(t *testing.T) {
	// Mock the reader to provide inputs for all steps
	inputs := strings.Join([]string{
		"",               // Project name: use default
		"Test description", // Description
		"go test ./...",   // Test command
		"go build ./...",  // Build command
		"Convention 1",    // Convention
	}, "\n") + "\n"

	tmpDir := t.TempDir()
	err := writeGoMod(t, tmpDir, "test-project")
	if err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	detector := NewDetector(tmpDir)
	wizard := NewWizard(detector)
	// Override reader with mocked input
	wizard.reader = bufio.NewReader(strings.NewReader(inputs))

	info, err := wizard.Run()
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	if info.Description != "Test description" {
		t.Errorf("Description = %q, want %q", info.Description, "Test description")
	}
	if info.TestCommand != "go test ./..." {
		t.Errorf("TestCommand = %q, want %q", info.TestCommand, "go test ./...")
	}
	if len(info.Conventions) < 1 {
		t.Errorf("conventions = %v, want at least [Convention 1]", info.Conventions)
	} else if info.Conventions[len(info.Conventions)-1] != "Convention 1" {
		t.Errorf("conventions = %v, want last element 'Convention 1'", info.Conventions)
	}
}

// writeGoMod is defined in cimode_test.go
