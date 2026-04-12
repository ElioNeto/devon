package tools

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ElioNeto/devon/internal/config"
)

func TestRegistry_RegisterAndDef(t *testing.T) {
	r := NewRegistry()
	r.Register(&BashTool{})

	if r.Defs() == nil {
		t.Error("Defs() returned nil")
	}

	tool, ok := r.Get("bash")
	if !ok {
		t.Fatal("Get('bash') returned false")
	}
	if tool.Name() != "bash" {
		t.Errorf("expected name 'bash', got %q", tool.Name())
	}

	b, err := json.MarshalIndent(r.Defs(), "", "  ")
	if err != nil {
		t.Fatalf("Defs() JSON marshal failed: %v", err)
	}
	if len(b) == 0 {
		t.Error("Defs() produced empty output")
	}
}

func TestRegistry_GetNotFound(t *testing.T) {
	r := NewRegistry()
	if _, ok := r.Get("nonexistent"); ok {
		t.Error("unexpected tool found")
	}
}

func TestRegisterBuiltin_Count(t *testing.T) {
	r := NewRegistry()
	RegisterBuiltin(r, ".", 0, config.SandboxConfig{})

	defs := r.Defs()
	if len(defs) < 7 {
		t.Errorf("expected at least 7 builtin tools, got %d", len(defs))
	}

	names := make(map[string]bool)
	for _, d := range defs {
		names[d.Function.Name] = true
	}
	for _, want := range []string{"bash", "read", "write", "edit", "glob", "grep", "list_dir"} {
		if !names[want] {
			t.Errorf("builtin tool %q not registered", want)
		}
	}
}

func TestRegisterBuiltinWithConfig(t *testing.T) {
	r := NewRegistry()
	RegisterBuiltinWithConfig(r, ".", 5*time.Second, config.SandboxConfig{}, 100, 20, 16*1024)

	defs := r.Defs()
	if len(defs) < 5 {
		t.Errorf("expected at least 5 builtin tools, got %d", len(defs))
	}

	grepTool, ok := r.Get("grep")
	if !ok {
		t.Fatal("RegisterBuiltinWithConfig did not register grep")
	}
	gt := grepTool.(*GrepTool)
	if gt.MaxLines != 100 {
		t.Errorf("GrepTool MaxLines = %d, want 100", gt.MaxLines)
	}
	if gt.MaxFiles != 20 {
		t.Errorf("GrepTool MaxFiles = %d, want 20", gt.MaxFiles)
	}
	if gt.MaxMatchSize != 16*1024 {
		t.Errorf("GrepTool MaxMatchSize = %d, want %d", gt.MaxMatchSize, 16*1024)
	}
}

func TestRegistry_RegisterPanicOnDuplicate(t *testing.T) {
	r := NewRegistry()
	r.Register(&BashTool{})
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate Register")
		}
	}()
	r.Register(&BashTool{})
}
