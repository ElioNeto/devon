package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildProjectContext_NoGit(t *testing.T) {
	// Create a temp directory with no git
	tmpDir := t.TempDir()

	ctx := BuildProjectContext(tmpDir)

	if strings.Contains(ctx, "Git branch") {
		t.Error("should not contain 'Git branch' in non-git directory")
	}
	if !strings.Contains(ctx, "Working directory") {
		t.Error("should contain 'Working directory'")
	}
}

func TestBuildProjectContext_DetectsLanguages(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .go and .py files
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "app.py"), []byte("print('hello')"), 0o644)

	ctx := BuildProjectContext(tmpDir)

	if !strings.Contains(ctx, "Go") || !strings.Contains(ctx, "Python") {
		t.Errorf("expected 'Go, Python' in context, got:\n%s", ctx)
	}
}

func TestBuildProjectContext_GitBranch(t *testing.T) {
	// Use the actual repo directory
	cwd, _ := os.Getwd()
	// Go up to repo root
	repoRoot := filepath.Join(filepath.Dir(cwd), filepath.Dir(filepath.Dir(cwd)))

	ctx := BuildProjectContext(repoRoot)

	// Should contain "Git branch" since this is a git repo
	if !strings.Contains(ctx, "Git branch") {
		t.Errorf("expected 'Git branch' in repo context, got:\n%s", ctx)
	}
}
