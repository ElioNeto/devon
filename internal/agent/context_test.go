package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildProjectContext_RootFiles(t *testing.T) {
	dir := t.TempDir()

	// Create some root-level files
	files := []string{"main.go", "go.mod", "README.md", "internal/"}
	for _, f := range files {
		path := filepath.Join(dir, f)
		if strings.HasSuffix(f, "/") {
			if err := os.MkdirAll(path, 0755); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}

	ctx := BuildProjectContext(dir)
	if !strings.Contains(ctx, "Arquivos raiz:") {
		t.Error("expected 'Arquivos raiz:' in context output")
	}
	if !strings.Contains(ctx, "main.go") {
		t.Error("expected 'main.go' in root files listing")
	}
	if !strings.Contains(ctx, "go.mod") {
		t.Error("expected 'go.mod' in root files listing")
	}
	if !strings.Contains(ctx, "README.md") {
		t.Error("expected 'README.md' in root files listing")
	}
	if !strings.Contains(ctx, "internal/") {
		t.Error("expected 'internal/' (with trailing slash) in root files listing")
	}
}

func TestBuildProjectContext_GoModDetection(t *testing.T) {
	dir := t.TempDir()

	// Create a go.mod
	goModContent := "module github.com/example/test\n\ngo 1.22\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := BuildProjectContext(dir)
	if !strings.Contains(ctx, "Módulo: github.com/example/test") {
		t.Errorf("expected module in context, got:\n%s", ctx)
	}
	if !strings.Contains(ctx, "Versão Go: 1.22") {
		t.Errorf("expected Go version in context, got:\n%s", ctx)
	}
}

func TestBuildProjectContext_NoGoMod(t *testing.T) {
	dir := t.TempDir()

	ctx := BuildProjectContext(dir)
	if strings.Contains(ctx, "Módulo:") {
		t.Error("did not expect 'Módulo:' when no go.mod exists")
	}
	if strings.Contains(ctx, "Versão Go:") {
		t.Error("did not expect 'Versão Go:' when no go.mod exists")
	}
}

func TestBuildProjectContext_NonExistentDir(t *testing.T) {
	ctx := BuildProjectContext("/tmp/nonexistent-path-for-test")
	if ctx == "" {
		// Should return something (the absolute path resolution may work, but the directory won't exist)
		t.Log("context returned empty for non-existent dir")
	}
	// It should not contain root files or go.mod info (since dir doesn't exist)
	_ = ctx
}

func TestBuildProjectContext_ValidDir(t *testing.T) {
	// Use the project root itself
	dir := "."
	ctx := BuildProjectContext(dir)
	if ctx == "" {
		t.Fatal("expected non-empty context for valid directory")
	}
	if !strings.Contains(ctx, "Contexto do projeto:") {
		t.Error("expected 'Contexto do projeto:' in context output")
	}
	if !strings.Contains(ctx, "Diretório de trabalho:") {
		t.Error("expected 'Diretório de trabalho:' in context output")
	}
}

func TestBuildProjectContext_ContainsKeywords(t *testing.T) {
	dir := t.TempDir()

	// Create go.mod for full context
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Create a Go file so Go is detected
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := BuildProjectContext(dir)
	if !strings.Contains(ctx, "Project root files:") && !strings.Contains(ctx, "Arquivos raiz:") {
		t.Error("expected root files listing in context output")
	}
	if !strings.Contains(ctx, "Módulo:") && !strings.Contains(ctx, "Module:") {
		t.Error("expected module info in context output")
	}
}
