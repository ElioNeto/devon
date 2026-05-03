package init

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDetector_Detect(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(t *testing.T, dir string)
		wantLang     string
		wantBuildCmd string
		wantTestCmd  string
		wantHasCI    bool
		wantHasDocker bool
		wantErr      bool
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
			wantLang:     "Go",
			wantBuildCmd: "go build ./...",
			wantTestCmd:  "go test ./...",
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
			wantLang:     "JavaScript",
			wantBuildCmd: "next build",
			wantTestCmd:  "jest",
		},
		{
			name: "python project with requirements.txt",
			setup: func(t *testing.T, dir string) {
				err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("flask==2.0\n"), 0644)
				if err != nil {
					t.Fatalf("failed to create requirements.txt: %v", err)
				}
			},
			wantLang:     "Python",
			wantBuildCmd: "",
			wantTestCmd:  "pytest",
		},
		{
			name: "unknown project",
			setup: func(t *testing.T, dir string) {
				// No project files
			},
			wantLang:     "Unknown",
			wantBuildCmd: "",
			wantTestCmd:  "",
		},
		// TODO 9: Rust (Cargo.toml) detection
		{
			name: "rust project with Cargo.toml",
			setup: func(t *testing.T, dir string) {
				err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname = \"test-rust\"\n"), 0644)
				if err != nil {
					t.Fatalf("failed to create Cargo.toml: %v", err)
				}
			},
			wantLang:     "Rust",
			wantBuildCmd: "cargo build",
			wantTestCmd:  "cargo test",
		},
		// TODO 11: CI detection
		{
			name: "with CI detection (.github/workflows)",
			setup: func(t *testing.T, dir string) {
				// Create go.mod so language is detected
				err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0644)
				if err != nil {
					t.Fatalf("failed to create go.mod: %v", err)
				}
				// Create .github/workflows directory
				err = os.MkdirAll(filepath.Join(dir, ".github", "workflows"), 0755)
				if err != nil {
					t.Fatalf("failed to create .github/workflows: %v", err)
				}
			},
			wantLang:     "Go",
			wantBuildCmd: "go build ./...",
			wantTestCmd:  "go test ./...",
			wantHasCI:    true,
		},
		// TODO 11: docker-compose detection
		{
			name: "with docker-compose detection",
			setup: func(t *testing.T, dir string) {
				// Create go.mod so language is detected
				err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0644)
				if err != nil {
					t.Fatalf("failed to create go.mod: %v", err)
				}
				// Create docker-compose.yml
				err = os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("version: '3'\n"), 0644)
				if err != nil {
					t.Fatalf("failed to create docker-compose.yml: %v", err)
				}
			},
			wantLang:      "Go",
			wantBuildCmd:  "go build ./...",
			wantTestCmd:   "go test ./...",
			wantHasDocker: true,
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
			if info.TestCommand != tt.wantTestCmd {
				t.Errorf("TestCommand = %v, want %v", info.TestCommand, tt.wantTestCmd)
			}
			if info.HasCI != tt.wantHasCI {
				t.Errorf("HasCI = %v, want %v", info.HasCI, tt.wantHasCI)
			}
			if info.HasDocker != tt.wantHasDocker {
				t.Errorf("HasDocker = %v, want %v", info.HasDocker, tt.wantHasDocker)
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

// TODO 10: Makefile parsing unit test
func TestParseMakefile(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantBuild   string
		wantTest    string
	}{
		{
			name: "build and test targets",
			content: "build: go build ./...\ntest: go test ./...\n",
			wantBuild: "go build ./...",
			wantTest:  "go test ./...",
		},
		{
			name: "build target only",
			content: "build: npm run build\n",
			wantBuild: "npm run build",
			wantTest:  "",
		},
		{
			name: "test target only",
			content: "test: pytest\n",
			wantBuild: "",
			wantTest:  "pytest",
		},
		{
			name: "no build or test targets",
			content: "clean: rm -rf dist\nlint: run golangci-lint\n",
			wantBuild: "",
			wantTest:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			makefilePath := filepath.Join(tmpDir, "Makefile")
			err := os.WriteFile(makefilePath, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("failed to create Makefile: %v", err)
			}

			detector := NewDetector(tmpDir)
			detector.parseMakefile(tt.content)

			if detector.info.BuildCommand != tt.wantBuild {
				t.Errorf("BuildCommand = %q, want %q", detector.info.BuildCommand, tt.wantBuild)
			}
			if detector.info.TestCommand != tt.wantTest {
				t.Errorf("TestCommand = %q, want %q", detector.info.TestCommand, tt.wantTest)
			}
		})
	}
}
