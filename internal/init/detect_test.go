package init

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDetector_Detect(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T, dir string)
		wantLang    string
		wantBuildCmd string
		wantErr     bool
	}{
		{
			name: "go project",
			setup: func(t *testing.T, dir string) {
				// Create go.mod
				err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0644)
				if err != nil {
					t.Fatalf("failed to create go.mod: %v", err)
				}
			},
			wantLang:    "Go",
			wantBuildCmd: "go build ./...",
		},
		{
			name: "node project with package.json",
			setup: func(t *testing.T, dir string) {
				pkgJSON := `{
					"name": "test-node",
					"scripts": {
						"build": "next build",
						"test": "jest"
					}
				}`
				err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0644)
				if err != nil {
					t.Fatalf("failed to create package.json: %v", err)
				}
			},
			wantLang:    "JavaScript",
			wantBuildCmd: "next build",
		},
		{
			name: "python project with requirements.txt",
			setup: func(t *testing.T, dir string) {
				err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("flask==2.0\n"), 0644)
				if err != nil {
					t.Fatalf("failed to create requirements.txt: %v", err)
				}
			},
			wantLang:    "Python",
			wantBuildCmd: "",
		},
		{
			name: "unknown project",
			setup: func(t *testing.T, dir string) {
				// No project files
			},
			wantLang:    "Unknown",
			wantBuildCmd: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp dir
			tmpDir := t.TempDir()
			tt.setup(t, tmpDir)

			detector := NewDetector(tmpDir)
			info, err := detector.Detect(context.Background())

			if (err != nil) != tt.wantErr {
				t.Fatalf("Detect() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if info.Language.Name != tt.wantLang {
				t.Errorf("Language.Name = %v, want %v", info.Language.Name, tt.wantLang)
			}
			if info.BuildCommand != tt.wantBuildCmd {
				t.Errorf("BuildCommand = %v, want %v", info.BuildCommand, tt.wantBuildCmd)
			}
		})
	}
}

func TestGetDefaultBuildCommand(t *testing.T) {
	tests := []struct {
		lang string
		want string
	}{
		{"Go", "go build ./..."},
		{"JavaScript", "npm run build"},
		{"TypeScript", "npm run build"},
		{"Rust", "cargo build"},
		{"Python", ""},
		{"Unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			got := getDefaultBuildCommand(tt.lang)
			if got != tt.want {
				t.Errorf("getDefaultBuildCommand(%q) = %q, want %q", tt.lang, got, tt.want)
			}
		})
	}
}

func TestGetDefaultTestCommand(t *testing.T) {
	tests := []struct {
		lang string
		want string
	}{
		{"Go", "go test ./..."},
		{"JavaScript", "npm test"},
		{"TypeScript", "npm test"},
		{"Rust", "cargo test"},
		{"Python", "pytest"},
		{"Unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			got := getDefaultTestCommand(tt.lang)
			if got != tt.want {
				t.Errorf("getDefaultTestCommand(%q) = %q, want %q", tt.lang, got, tt.want)
			}
		})
	}
}
