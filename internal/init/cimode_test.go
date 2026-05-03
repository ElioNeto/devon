package init

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRunCI_WriteFile(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "DEVON.md")

	// Create go.mod to detect Go project
	err := writeGoMod(t, tmpDir, "ci-test-project")
	if err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	// Run CI mode with yes=true
	err = RunCI(context.Background(), tmpDir, outputPath, true)
	if err != nil {
		t.Fatalf("RunCI() failed: %v", err)
	}

	// Check file exists
	if !Exists(outputPath) {
		t.Fatal("RunCI() did not create DEVON.md")
	}

	// Check content
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read DEVON.md: %v", err)
	}
	if !contains(string(content), "ci-test-project") {
		t.Errorf("DEVON.md missing project name, content: %s", content)
	}
}

func TestRunCI_OverwriteWithYes(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "DEVON.md")

	// Create existing DEVON.md
	initialContent := "# Old Project\n"
	err := os.WriteFile(outputPath, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("failed to create initial DEVON.md: %v", err)
	}

	// Create go.mod
	err = writeGoMod(t, tmpDir, "new-project")
	if err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	// Run CI with yes=true (should overwrite)
	err = RunCI(context.Background(), tmpDir, outputPath, true)
	if err != nil {
		t.Fatalf("RunCI() failed: %v", err)
	}

	// Check content is overwritten
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read DEVON.md: %v", err)
	}
	if contains(string(content), "Old Project") {
		t.Error("DEVON.md was not overwritten with yes=true")
	}
}

func TestRunCI_NoYesExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "DEVON.md")

	// Create existing DEVON.md
	initialContent := "# Existing\n"
	err := os.WriteFile(outputPath, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("failed to create initial DEVON.md: %v", err)
	}

	// Create go.mod
	err = writeGoMod(t, tmpDir, "new-project")
	if err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	// Run CI with yes=false (should prompt, but in test we skip interactive)
	// Since we can't interact, this will call promptExistingFile which returns nil but doesn't write
	// So DEVON.md should remain unchanged
	err = RunCI(context.Background(), tmpDir, outputPath, false)
	// In test, promptExistingFile will not overwrite, so file remains initial
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read DEVON.md: %v", err)
	}
	if string(content) != initialContent {
		t.Errorf("DEVON.md changed without yes flag, got %q, want %q", string(content), initialContent)
	}
}

// helper to check string contains
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// writeGoMod is a helper to create go.mod (already defined in wizard_test.go, but repeat here for clarity)
func writeGoMod(t *testing.T, dir, moduleName string) error {
	t.Helper()
	content := "module " + moduleName + "\n\ngo 1.21\n"
	return os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0644)
}
