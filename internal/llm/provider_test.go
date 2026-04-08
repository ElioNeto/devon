package llm

import (
	"errors"
	"testing"
)

func TestText(t *testing.T) {
	d := Text("hello")
	if d.Type != "text" {
		t.Errorf("Type = %q", d.Type)
	}
	if d.Text() != "hello" {
		t.Errorf("Text() = %q", d.Text())
	}
}

func TestToolCallDelta(t *testing.T) {
	tc := ToolCall{Type: "function", Function: ToolCallFunction{Name: "bash"}}
	d := ToolCallDelta(tc)
	if d.Type != "tool_call" {
		t.Errorf("Type = %q", d.Type)
	}
	if d.Tool == nil {
		t.Fatal("Tool should not be nil")
	}
	if d.Tool.Function.Name != "bash" {
		t.Errorf("Tool.Function.Name = %q", d.Tool.Function.Name)
	}
}

func TestDoneDelta_WithoutUsage(t *testing.T) {
	d := DoneDelta()
	if d.Type != "done" {
		t.Errorf("Type = %q", d.Type)
	}
	if d.Usage != nil {
		t.Errorf("Usage should be nil, got %+v", d.Usage)
	}
}

func TestDoneDelta_WithUsage(t *testing.T) {
	u := &Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30}
	d := DoneDelta(u)
	if d.Type != "done" {
		t.Errorf("Type = %q", d.Type)
	}
	if d.Usage == nil {
		t.Fatal("Usage should not be nil")
	}
	if d.Usage.TotalTokens != 30 {
		t.Errorf("Usage.TotalTokens = %d", d.Usage.TotalTokens)
	}
}

func TestDoneDelta_NilUsage(t *testing.T) {
	d := DoneDelta(nil)
	if d.Type != "done" {
		t.Errorf("Type = %q", d.Type)
	}
	if d.Usage != nil {
		t.Error("Usage should be nil for nil input")
	}
}

func TestErrorDelta(t *testing.T) {
	err := errors.New("test error")
	d := ErrorDelta(err)
	if d.Type != "error" {
		t.Errorf("Type = %q", d.Type)
	}
	if d.Err == nil {
		t.Fatal("Err should not be nil")
	}
	if d.Err.Error() != "test error" {
		t.Errorf("Err = %q", d.Err.Error())
	}
}

func TestDelta_TextMethod(t *testing.T) {
	// Text() on non-text delta should return empty
	d := DoneDelta()
	if d.Text() != "" {
		t.Errorf("Text() on done delta should be empty, got %q", d.Text())
	}
}
