package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMode(t *testing.T) {
	tests := []struct {
		input string
		want  Mode
	}{
		{"safe", ModeSafe},
		{"SAFE", ModeSafe},
		{"yolo", ModeYolo},
		{"YOLO", ModeYolo},
		{"auto", ModeAuto},
		{"", ModeAuto},
		{"unknown", ModeAuto},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			if got := ParseMode(tc.input); got != tc.want {
				t.Errorf("ParseMode(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestMode_String(t *testing.T) {
	tests := []struct {
		mode Mode
		want string
	}{
		{ModeAuto, "auto"},
		{ModeSafe, "safe"},
		{ModeYolo, "yolo"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.mode.String(); got != tc.want {
				t.Errorf("Mode(%v).String() = %q, want %q", tc.mode, got, tc.want)
			}
		})
	}
}

func TestIsLocalURL(t *testing.T) {
	for _, tc := range []struct {
		url  string
		want bool
	}{
		{"http://localhost:11434/v1", true},
		{"http://127.0.0.1:8080/v1", true},
		{"http://10.0.0.5:8080/v1", true},
		{"https://api.openai.com/v1", false},
		{"https://openrouter.ai/api/v1", false},
	} {
		t.Run(tc.url, func(t *testing.T) {
			if got := isLocalURL(tc.url); got != tc.want {
				t.Errorf("isLocalURL(%q) = %v, want %v", tc.url, got, tc.want)
			}
		})
	}
}

func TestLoad_Success(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	content := []byte(
		"DEVON_API_KEY=test-key\n" +
			"DEVON_MODEL=gpt-4\n" +
			"DEVON_BASE_URL=https://api.openai.com/v1\n" +
			"DEVON_MODE=auto\n" +
			"DEVON_TIMEOUT=60\n",
	)
	if err := os.WriteFile(envFile, content, 0o644); err != nil {
		t.Fatal(err)
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()
	// Clear env vars that could interfere
	os.Unsetenv("DEVON_API_KEY")
	os.Unsetenv("DEVON_MODEL")
	os.Unsetenv("DEVON_BASE_URL")
	os.Unsetenv("DEVON_MODE")
	os.Unsetenv("DEVON_TIMEOUT")

	cfg, err := Load(envFile)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.APIKey != "test-key" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "test-key")
	}
	if cfg.Model != "gpt-4" {
		t.Errorf("Model = %q, want %q", cfg.Model, "gpt-4")
	}
	if cfg.Mode != ModeAuto {
		t.Errorf("Mode = %v, want %v", cfg.Mode, ModeAuto)
	}
	if cfg.MaxAgentLoops != 10 {
		t.Errorf("MaxAgentLoops = %d, want 10", cfg.MaxAgentLoops)
	}
	if cfg.Timeout.Seconds() != 60 {
		t.Errorf("Timeout = %v, want 60s", cfg.Timeout)
	}
}

func TestLoad_EnvFileNotFound_Succeeds(t *testing.T) {
	for _, k := range []string{"DEVON_API_KEY", "DEVON_MODEL", "DEVON_BASE_URL", "DEVON_MODE", "DEVON_TIMEOUT"} {
		os.Unsetenv(k)
	}
	os.Setenv("DEVON_MODEL", "gpt-4")
	os.Setenv("DEVON_BASE_URL", "http://localhost:11434/v1")
	defer func() {
		os.Unsetenv("DEVON_MODEL")
		os.Unsetenv("DEVON_BASE_URL")
	}()

	_, err := Load(".env.nonexistent")
	if err != nil {
		t.Fatalf("Load() with nonexistent env file should succeed: %v", err)
	}
}

func TestLoad_EmptyDir_Succeeds(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)
	oldWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()
	for _, k := range []string{"DEVON_API_KEY", "DEVON_MODEL", "DEVON_BASE_URL", "DEVON_MODE", "DEVON_TIMEOUT"} {
		os.Unsetenv(k)
	}
	os.Setenv("DEVON_API_KEY", "k")
	os.Setenv("DEVON_MODEL", "m")
	defer func() {
		os.Unsetenv("DEVON_API_KEY")
		os.Unsetenv("DEVON_MODEL")
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.WorkDir != dir {
		t.Errorf("WorkDir = %q, want %q", cfg.WorkDir, dir)
	}
	if cfg.APIKey != "k" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "k")
	}
	if cfg.Model != "m" {
		t.Errorf("Model = %q, want %q", cfg.Model, "m")
	}
	if cfg.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("BaseURL = %q, want default", cfg.BaseURL)
	}
	if cfg.Mode != ModeAuto {
		t.Errorf("Mode = %v, want %v", cfg.Mode, ModeAuto)
	}
	if cfg.MaxAgentLoops != 10 {
		t.Errorf("MaxAgentLoops = %d, want 10", cfg.MaxAgentLoops)
	}
}

func TestLoad_DEVONMD(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "DEVON.md"), []byte("project context"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()
	for _, k := range []string{"DEVON_API_KEY", "DEVON_MODEL"} {
		os.Setenv(k, "k")
	}
	defer func() {
		os.Unsetenv("DEVON_API_KEY")
		os.Unsetenv("DEVON_MODEL")
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.ContextDoc != "project context" {
		t.Errorf("ContextDoc = %q, want %q", cfg.ContextDoc, "project context")
	}
}

func TestLoad_Validation_MissingModel(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()
	for _, k := range []string{"DEVON_MODEL", "DEVON_API_KEY"} {
		os.Unsetenv(k)
	}
	_, err := Load("")
	if err == nil {
		t.Fatal("expected error for missing model")
	}
}

func TestLoad_Validation_MissingKey_NonLocal(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()
	os.Setenv("DEVON_MODEL", "gpt-4")
	os.Setenv("DEVON_BASE_URL", "https://api.openai.com/v1")
	defer func() {
		os.Unsetenv("DEVON_MODEL")
		os.Unsetenv("DEVON_BASE_URL")
	}()
	_, err := Load("")
	if err == nil {
		t.Fatal("expected error for missing key with non-local URL")
	}
}

func TestLoad_DEVON_MAX_LOOPS(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want int
	}{
		{"default when unset", "", 10},
		{"explicit value", "5", 5},
		{"invalid falls back to default", "abc", 10},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			os.Unsetenv("DEVON_MAX_LOOPS")
			if tc.val != "" {
				os.Setenv("DEVON_MAX_LOOPS", tc.val)
				defer os.Unsetenv("DEVON_MAX_LOOPS")
			}
			// Need a temp dir to avoid side effects
			dir := t.TempDir()
			oldWd, _ := os.Getwd()
			if err := os.Chdir(dir); err != nil {
				t.Fatal(err)
			}
			os.Setenv("DEVON_MODEL", "m")
			os.Setenv("DEVON_BASE_URL", "http://localhost:11434/v1")
			defer func() {
				os.Unsetenv("DEVON_MODEL")
				os.Unsetenv("DEVON_BASE_URL")
				os.Chdir(oldWd)
			}()
			cfg, err := Load("")
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}
			if cfg.MaxAgentLoops != tc.want {
				t.Errorf("MaxAgentLoops = %d, want %d", cfg.MaxAgentLoops, tc.want)
			}
		})
	}
}

func TestLoad_LocalURL_NoKeyRequired(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()
	os.Setenv("DEVON_MODEL", "m")
	os.Setenv("DEVON_BASE_URL", "http://localhost:11434/v1")
	defer func() {
		os.Unsetenv("DEVON_MODEL")
		os.Unsetenv("DEVON_BASE_URL")
	}()
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() with local URL should not require key: %v", err)
	}
	if cfg.Model != "m" {
		t.Errorf("Model = %q, want %q", cfg.Model, "m")
	}
}

func TestLoad_DEVON_MAX_HISTORY_TURNS(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want int
	}{
		{"default when unset", "", 20},
		{"explicit value", "10", 10},
		{"invalid falls back to default", "abc", 20},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			os.Unsetenv("DEVON_MAX_HISTORY_TURNS")
			if tc.val != "" {
				os.Setenv("DEVON_MAX_HISTORY_TURNS", tc.val)
				defer os.Unsetenv("DEVON_MAX_HISTORY_TURNS")
			}
			dir := t.TempDir()
			oldWd, _ := os.Getwd()
			if err := os.Chdir(dir); err != nil {
				t.Fatal(err)
			}
			os.Setenv("DEVON_MODEL", "m")
			os.Setenv("DEVON_BASE_URL", "http://localhost:11434/v1")
			defer func() {
				os.Unsetenv("DEVON_MODEL")
				os.Unsetenv("DEVON_BASE_URL")
				os.Chdir(oldWd)
			}()
			cfg, err := Load("")
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}
			if cfg.MaxHistoryTurns != tc.want {
				t.Errorf("MaxHistoryTurns = %d, want %d", cfg.MaxHistoryTurns, tc.want)
			}
		})
	}
}

func TestLoad_DEVON_MAX_TOOL_RESULT_CHARS(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want int
	}{
		{"default when unset", "", 4000},
		{"explicit value", "2000", 2000},
		{"invalid falls back to default", "abc", 4000},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			os.Unsetenv("DEVON_MAX_TOOL_RESULT_CHARS")
			if tc.val != "" {
				os.Setenv("DEVON_MAX_TOOL_RESULT_CHARS", tc.val)
				defer os.Unsetenv("DEVON_MAX_TOOL_RESULT_CHARS")
			}
			dir := t.TempDir()
			oldWd, _ := os.Getwd()
			if err := os.Chdir(dir); err != nil {
				t.Fatal(err)
			}
			os.Setenv("DEVON_MODEL", "m")
			os.Setenv("DEVON_BASE_URL", "http://localhost:11434/v1")
			defer func() {
				os.Unsetenv("DEVON_MODEL")
				os.Unsetenv("DEVON_BASE_URL")
				os.Chdir(oldWd)
			}()
			cfg, err := Load("")
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}
			if cfg.MaxToolResultChars != tc.want {
				t.Errorf("MaxToolResultChars = %d, want %d", cfg.MaxToolResultChars, tc.want)
			}
		})
	}
}

func TestGetEnvInt(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  string
		val  string
		def  int
		want int
	}{
		{"empty", "TEST_INT_EMPTY", "", 42, 42},
		{"valid", "TEST_INT_VALID", "100", 0, 100},
		{"bad", "TEST_INT_BAD", "abc", 99, 99},
	} {
		t.Run(tc.name, func(t *testing.T) {
			os.Setenv(tc.key, tc.val)
			defer os.Unsetenv(tc.key)
			got := getEnvInt(tc.key, tc.def)
			if got != tc.want {
				t.Errorf("getEnvInt(%q, %d) = %d, want %d", tc.key, tc.def, got, tc.want)
			}
		})
	}
}
