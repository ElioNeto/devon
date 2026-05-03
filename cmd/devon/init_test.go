package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestInitCommand_YesFlag(t *testing.T) {
	// Create temp dir with go.mod
	tmpDir := t.TempDir()
	err := writeGoModForTest(t, tmpDir, "integration-test")
	if err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	// Save current dir, change to tmpDir
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	// Create root command and add init subcommand
	root := newRootCommand()
	root.SetArgs([]string{"init", "--yes"})

	// Capture output
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)

	// Execute command
	err = root.ExecuteContext(context.Background())
	if err != nil {
		t.Fatalf("init command failed: %v, output: %s", err, out.String())
	}

	// Check DEVON.md exists
	devonPath := filepath.Join(tmpDir, "DEVON.md")
	if _, err := os.Stat(devonPath); err != nil {
		t.Fatalf("DEVON.md not created: %v", err)
	}

	// Check content
	content, err := os.ReadFile(devonPath)
	if err != nil {
		t.Fatalf("failed to read DEVON.md: %v", err)
	}
	if !bytes.Contains(content, []byte("integration-test")) {
		t.Errorf("DEVON.md missing project name, content: %s", content)
	}
}

func TestInitCommand_ForceFlag(t *testing.T) {
	tmpDir := t.TempDir()
	err := writeGoModForTest(t, tmpDir, "force-test")
	if err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	// Create existing DEVON.md
	existingContent := "# Old\n"
	devonPath := filepath.Join(tmpDir, "DEVON.md")
	err = os.WriteFile(devonPath, []byte(existingContent), 0644)
	if err != nil {
		t.Fatalf("failed to create existing DEVON.md: %v", err)
	}

	// Change to tmpDir
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	// Run init with --yes and --force
	root := newRootCommand()
	root.SetArgs([]string{"init", "--yes", "--force"})

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)

	err = root.ExecuteContext(context.Background())
	if err != nil {
		t.Fatalf("init command failed: %v, output: %s", err, out.String())
	}

	// Check content is overwritten
	content, err := os.ReadFile(devonPath)
	if err != nil {
		t.Fatalf("failed to read DEVON.md: %v", err)
	}
	if bytes.Contains(content, []byte("Old")) {
		t.Error("DEVON.md was not overwritten with --force")
	}
}

// helper to write go.mod for tests
func writeGoModForTest(t *testing.T, dir, moduleName string) error {
	t.Helper()
	content := "module " + moduleName + "\n\ngo 1.21\n"
	return os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0644)
}

// TestRunInit is a unit test for the runInit function
func TestRunInit(t *testing.T) {
	// Create temp dir
	tmpDir := t.TempDir()
	err := writeGoModForTest(t, tmpDir, "run-init-test")
	if err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	// Create a cobra command with init subcommand
	root := newRootCommand()
	initCmd, _, err := root.Find([]string{"init"})
	if err != nil {
		t.Fatalf("failed to find init command: %v", err)
	}

	// Set flags
	initCmd.Flags().Set("yes", "true")
	initCmd.Flags().Set("force", "false")

	// Change to tmpDir
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	// Run the command
	err = runInit(initCmd, []string{})
	if err != nil {
		t.Fatalf("runInit failed: %v", err)
	}

	// Check DEVON.md exists
	devonPath := filepath.Join(tmpDir, "DEVON.md")
	if _, err := os.Stat(devonPath); err != nil {
		t.Fatalf("DEVON.md not created: %v", err)
	}
}
