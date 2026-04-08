package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootCommand_Help(t *testing.T) {
	root := buildRootCommand()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--help"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(out.String(), "devon") {
		t.Error("help output should contain 'devon'")
	}
}

func TestRootCommand_Version(t *testing.T) {
	root := buildRootCommand()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--version"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	output := out.String()
	if output == "" {
		t.Error("version output is empty")
	}
}

func TestDoctorCommand_NoModel(t *testing.T) {
	t.Setenv("DEVON_MODEL", "")
	t.Setenv("DEVON_API_KEY", "")

	root := buildRootCommand()
	root.SetArgs([]string{"doctor"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when DEVON_MODEL is not set")
	}
}

func TestDoctorCommand_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping short mode")
	}
	t.Setenv("DEVON_MODEL", "test")
	t.Setenv("DEVON_BASE_URL", "http://localhost:11434/v1")
	t.Setenv("DEVON_API_KEY", "")

	root := buildRootCommand()
	root.SetArgs([]string{"doctor", "--env", ""})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error because doctor is not mocked")
	}
	// Error is expected since we can't connect to a real provider
	// but the doctor function should be called
	t.Logf("doctor error (expected): %v", err)
}

func TestRunAgent_MissingModel(t *testing.T) {
	t.Setenv("DEVON_MODEL", "")
	t.Setenv("DEVON_API_KEY", "")

	root := buildRootCommand()
	root.SetArgs([]string{})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when DEVON_MODEL is not set")
	}
}

func TestRunAgent_WithModel(t *testing.T) {
	dir := t.TempDir()
	// Create a minimal .env
	t.Setenv("DEVON_MODEL", "test")
	t.Setenv("DEVON_BASE_URL", "http://localhost:11434/v1")

	// Save and restore cwd
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	root := buildRootCommand()
	root.SetArgs([]string{"--env", ".nonexistent"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	_ = root.Execute()
	t.Log("runAgent executed without panic")
}

func TestRootCommand_UnknownSubcommand(t *testing.T) {
	root := buildRootCommand()
	root.SetArgs([]string{"nonexistent"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
}

func buildRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:     "devon",
		Short:   "Agente de cdigo com TUI",
		Version: "test",
		RunE:    runAgent,
	}

	root.PersistentFlags().String("mode", "auto", "Modo")
	root.PersistentFlags().String("model", "", "Modelo")
	root.PersistentFlags().String("env", ".env", "Env file")

	doctor := &cobra.Command{
		Use:   "doctor",
		Short: "Valida configuracao",
		RunE:  runDoctor,
	}
	root.AddCommand(doctor)

	runCmd := &cobra.Command{
		Use:   "run <tarefa>",
		Short: "Executa uma tarefa",
		Args:  cobra.MinimumNArgs(1),
		RunE:  runTask,
	}
	runCmd.Flags().String("mode", "auto", "Modo de permissao: auto | safe | yolo")
	root.AddCommand(runCmd)

	return root
}

func TestMain_Function(t *testing.T) {
	// main() calls root.Execute() which calls runAgent
	// We can't easily intercept this without os.Exit(1)
	// so we test the functions it calls instead
	t.Run("runAgent_NoEnvFile", func(t *testing.T) {
		root := buildRootCommand()
		root.SetArgs([]string{"--env", ""})
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		t.Setenv("DEVON_MODEL", "test")
		t.Setenv("DEVON_BASE_URL", "http://localhost:11434/v1")
		
		_ = root.Execute()
	})
	
	t.Run("runAgent_EmptyEnvFile", func(t *testing.T) {
		root := buildRootCommand()
		root.SetArgs([]string{"--env", ".", "--mode", "safe", "--model", "mymodel"})
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		
		_ = root.Execute()
	})
	
	t.Run("runAgent_YoloMode", func(t *testing.T) {
		root := buildRootCommand()
		root.SetArgs([]string{"--mode", "yolo"})
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		t.Setenv("DEVON_MODEL", "test")
		t.Setenv("DEVON_BASE_URL", "http://localhost:11434/v1")
		
		_ = root.Execute()
	})

	t.Run("runDoctor_NoModel", func(t *testing.T) {
		root := buildRootCommand()
		root.SetArgs([]string{"doctor"})
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		
		_ = root.Execute()
	})
}

func TestNewRootCommand(t *testing.T) {
	cmd := newRootCommand()
	if cmd == nil {
		t.Fatal("newRootCommand() returned nil")
	}
	if cmd.Use != "devon" {
		t.Errorf("Use = %q, want devon", cmd.Use)
	}
	if cmd.Version != "test" && cmd.Version != "dev" {
		t.Errorf("Version = %q, expected test or dev", cmd.Version)
	}
	if cmd.HasSubCommands() {
		t.Log("has doctor subcommand (expected)")
	}
}

func TestNewRootCommand_DoctorSubCommand(t *testing.T) {
	cmd := newRootCommand()
	doctor, _, _ := cmd.Find([]string{"doctor"})
	if doctor == nil {
		t.Fatal("doctor subcommand not found")
	}
	if doctor.Use != "doctor" {
		t.Errorf("doctor Use = %q, want doctor", doctor.Use)
	}
}

// ------------------------------------------------------------------
//  run subcommand tests
// ------------------------------------------------------------------

func TestNewRootCommand_RunSubCommand(t *testing.T) {
	cmd := newRootCommand()
	run, _, _ := cmd.Find([]string{"run"})
	if run == nil {
		t.Fatal("run subcommand not found")
	}
	if run.Use != "run <tarefa>" {
		t.Errorf("run Use = %q, want 'run <tarefa>'", run.Use)
	}
}

func TestRunCommand_NoArgs(t *testing.T) {
	root := buildRootCommand()
	root.SetArgs([]string{"run"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when run has no arguments")
	}
}

func TestRunCommand_WithArgs(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	t.Setenv("DEVON_MODEL", "test")
	t.Setenv("DEVON_BASE_URL", "http://localhost:11434/v1")

	root := buildRootCommand()
	root.SetArgs([]string{"run", "do something"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	_ = root.Execute()
	// Expectation is graceful failure due to no real LLM, but no panic
	t.Log("run command executed without panic")
}

func TestExitCoder(t *testing.T) {
	t.Run("configError returns code 2", func(t *testing.T) {
		err := configError(fmt.Errorf("config broken"))
		if err.ExitCode() != 2 {
			t.Errorf("ExitCode() = %d, want 2", err.ExitCode())
		}
		if err.Error() != "config broken" {
			t.Errorf("Error() = %q, want 'config broken'", err.Error())
		}
	})

	t.Run("exitError returns code 1", func(t *testing.T) {
		err := exitError(fmt.Errorf("execution failed"))
		if err.ExitCode() != 1 {
			t.Errorf("ExitCode() = %d, want 1", err.ExitCode())
		}
		if err.Error() != "execution failed" {
			t.Errorf("Error() = %q, want 'execution failed'", err.Error())
		}
	})
}

func TestHasStdinPipe_NoPipe(t *testing.T) {
	// In test context, stdin is not a pipe
	if hasStdinPipe() {
		t.Error("hasStdinPipe() should be false in test context")
	}
}
