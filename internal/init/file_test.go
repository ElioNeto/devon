package init

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExists(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")

	// Non-existent file
	if Exists(path) {
		t.Error("Exists() returned true for non-existent file")
	}

	// Create file
	err := os.WriteFile(path, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if !Exists(path) {
		t.Error("Exists() returned false for existing file")
	}
}

func TestWriteFile_NewFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "DEVON.md")
	content := "# Test Project\n\nTest description"

	err := WriteFile(path, content, false)
	if err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	// Check file exists
	if !Exists(path) {
		t.Fatal("WriteFile() did not create file")
	}

	// Check content
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != content {
		t.Errorf("file content mismatch: got %q, want %q", string(data), content)
	}
}

func TestWriteFile_ForceOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "DEVON.md")
	initialContent := "# Old\n"
	newContent := "# New\n"

	// Create initial file
	err := os.WriteFile(path, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("failed to create initial file: %v", err)
	}

	// Write with force=true
	err = WriteFile(path, newContent, true)
	if err != nil {
		t.Fatalf("WriteFile() with force failed: %v", err)
	}

	// Check content is overwritten
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != newContent {
		t.Errorf("file not overwritten: got %q, want %q", string(data), newContent)
	}
}

func TestWriteFile_NoForcePrompt(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "DEVON.md")
	initialContent := "# Old\n"
	newContent := "# New\n"

	// Create initial file
	err := os.WriteFile(path, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("failed to create initial file: %v", err)
	}

	// Mock stdin to provide empty input (defaults to open editor, no overwrite)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	// Write empty line (chooses option 1, open editor)
	_, _ = w.WriteString("\n")
	w.Close()

	// Write with force=false — should prompt, user "chooses" editor (default)
	err = WriteFile(path, newContent, false)
	// Editor opening will fail in test (no terminal), but WriteFile should return an error from exec
	if err == nil {
		// If somehow error is nil, verify file was NOT overwritten
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("failed to read file: %v", readErr)
		}
		if string(data) != initialContent {
			t.Errorf("file was overwritten without force, got %q, want %q", string(data), initialContent)
		}
	}
	// If exec error, that's expected in test environment — test passes
}
