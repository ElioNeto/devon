package tui

import (
	"bytes"
	"os"
	"testing"

	"github.com/ElioNeto/devon/internal/config"
)

func TestRun_NoError(t *testing.T) {
	cfg := &config.Config{
		Model:   "test-model",
		BaseURL: "http://localhost:11434/v1",
		WorkDir: "/tmp/test",
		Mode:    config.ModeAuto,
	}
	err := Run(cfg)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
}

func TestRun_OutputContainsInfo(t *testing.T) {
	cfg := &config.Config{
		Model:   "gpt-4",
		BaseURL: "http://localhost:11434/v1",
		WorkDir: "/tmp/mydir",
		Mode:    config.ModeSafe,
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	_ = Run(cfg)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !bytes.Contains([]byte(output), []byte("gpt-4")) {
		t.Error("output should contain model name")
	}
	if !bytes.Contains([]byte(output), []byte("safe")) {
		t.Error("output should contain mode")
	}
}

func TestRun_WithDoc(t *testing.T) {
	cfg := &config.Config{
		Model:     "test",
		BaseURL:   "http://localhost:11434/v1",
		WorkDir:   "/tmp",
		Mode:      config.ModeYolo,
		ContextDoc: "project docs",
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	_ = Run(cfg)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !bytes.Contains([]byte(output), []byte("project docs")) {
		t.Error("output should contain context doc")
	}
	if !bytes.Contains([]byte(output), []byte("yolo")) {
		t.Error("output should contain yolo mode")
	}
}
