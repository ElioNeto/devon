package init

import (
	"strings"
	"testing"
)

func TestValidateTemplate(t *testing.T) {
	err := ValidateTemplate()
	if err != nil {
		t.Fatalf("ValidateTemplate() failed: %v", err)
	}
}

func TestGenerateDEVONmd(t *testing.T) {
	tests := []struct {
		name    string
		info    ProjectInfo
		contains []string
	}{
		{
			name: "go project",
			info: ProjectInfo{
				ProjectName:   "test-go",
				Description:  "Go project",
				Language:      Language{Name: "Go", Version: "1.21"},
				BuildCommand:  "go build ./...",
				TestCommand:   "go test ./...",
				Conventions:  []string{"Use gofmt"},
			},
			contains: []string{"# test-go", "Go 1.21", "go build ./...", "go test ./...", "Use gofmt"},
		},
		{
			name: "node project",
			info: ProjectInfo{
				ProjectName:   "test-node",
				Description:  "Node project",
				Language:      Language{Name: "JavaScript", Version: ""},
				BuildCommand:  "npm run build",
				TestCommand:   "npm test",
			},
			contains: []string{"# test-node", "JavaScript", "npm run build", "npm test"},
		},
		{
			name: "no build command",
			info: ProjectInfo{
				ProjectName:   "test-no-build",
				Description:  "No build",
				Language:      Language{Name: "Python", Version: ""},
			},
			contains: []string{"# test-no-build", "Python"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := tt.info.GenerateDEVONmd()
			for _, s := range tt.contains {
				if !strings.Contains(content, s) {
					t.Errorf("GenerateDEVONmd() does not contain %q, content:\n%s", s, content)
				}
			}
		})
	}
}

func TestGenerateDEVONmd_Fallback(t *testing.T) {
	// Test that generateManual is called if template is invalid
	// To test this, we can temporarily break the template, but since template is const, maybe not.
	// Instead, test that generateManual produces correct output.
	info := ProjectInfo{
		ProjectName:   "fallback-test",
		Description:  "Fallback",
		Language:      Language{Name: "Go", Version: "1.21"},
		BuildCommand:  "go build",
	}
	content := info.generateManual()
	if !strings.Contains(content, "# fallback-test") {
		t.Errorf("generateManual() missing project name, content: %s", content)
	}
}
