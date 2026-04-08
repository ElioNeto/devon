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

	if strings.Contains(ctx, "Branch do Git") {
		t.Error("should not contain 'Branch do Git' in non-git directory")
	}
	if !strings.Contains(ctx, "Diretório de trabalho") {
		t.Error("should contain 'Diretório de trabalho'")
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

func findRepoRoot(t *testing.T, start string) string {
	t.Helper()
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root")
		}
		dir = parent
	}
}

func TestBuildProjectContext_GitBranch(t *testing.T) {
	// Use the actual repo directory
	cwd, _ := os.Getwd()
	repoRoot := findRepoRoot(t, cwd)

	ctx := BuildProjectContext(repoRoot)

	// Should contain "Branch do Git" since this is a git repo
	if !strings.Contains(ctx, "Branch do Git") {
		t.Errorf("expected 'Branch do Git' in repo context, got:\n%s", ctx)
	}
}
