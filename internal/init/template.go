package init

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
)

// devonMarkdownTemplate is the Go template for DEVON.md with dynamic placeholders.
const devonMarkdownTemplate = `# {{.ProjectName}}

{{.Description}}

## Stack

{{if .Framework}}- {{.Framework}}
{{end}}- {{.Language.Name}} {{.Language.Version}}
{{if .PackageManager}}- {{.PackageManager}}
{{end}}
## Comandos

{{if .BuildCommand}}- Compilar: ` + "`" + `{{.BuildCommand}}` + "`" + `
{{end}}{{if .TestCommand}}- Testes: ` + "`" + `{{.TestCommand}}` + "`" + `
{{end}}
{{if .Conventions}}## Convenções

{{range .Conventions}}- {{.}}
{{end}}{{end}}`

// ValidateTemplate checks that the DEVON.md template is valid and renders correctly.
func ValidateTemplate() error {
	tmpl, err := template.New("devon-md").Parse(devonMarkdownTemplate)
	if err != nil {
		return fmt.Errorf("invalid template syntax: %w", err)
	}

	// Test render with sample data to ensure all placeholders are valid
	sample := ProjectInfo{
		ProjectName:    "test-project",
		Description:    "A sample project for testing",
		Language:       Language{Name: "Go", Version: "1.21"},
		Framework:      "Gin",
		PackageManager:  "go modules",
		BuildCommand:   "go build ./...",
		TestCommand:    "go test ./...",
		Conventions:   []string{"Use gofmt", "Write table-driven tests"},
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, sample); err != nil {
		return fmt.Errorf("template render failed: %w", err)
	}

	// Check that output is non-empty
	if buf.Len() == 0 {
		return fmt.Errorf("template rendered empty output")
	}

	return nil
}

// GenerateDEVONmd generates the DEVON.md content using the dynamic template.
func (p *ProjectInfo) GenerateDEVONmd() string {
	tmpl, err := template.New("devon-md").Parse(devonMarkdownTemplate)
	if err != nil {
		// Fallback to manual generation if template is invalid (should not happen if validated)
		return p.generateManual()
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, p); err != nil {
		return p.generateManual()
	}

	return strings.TrimSpace(buf.String())
}

// generateManual provides a fallback manual generation if template parsing fails.
func (p *ProjectInfo) generateManual() string {
	// Build stack section
	var stack []string
	if p.Framework != "" {
		stack = append(stack, p.Framework)
	}
	stack = append(stack, p.Language.Name+" "+p.Language.Version)
	if p.PackageManager != "" {
		stack = append(stack, p.PackageManager)
	}

	// Build commands section
	var commands []string
	if p.BuildCommand != "" {
		commands = append(commands, fmt.Sprintf("- Compilar: `%s`", p.BuildCommand))
	}
	if p.TestCommand != "" {
		commands = append(commands, fmt.Sprintf("- Testes: `%s`", p.TestCommand))
	}

	// Add common commands based on language
	pkgName := filepath.Base(p.ProjectName)
	switch p.Language.Name {
	case "Go":
		if len(commands) == 0 && p.BuildCommand == "" {
			commands = append(commands, fmt.Sprintf("- Compilar: `go build -o %s ./cmd/%s`", pkgName, pkgName))
		}
		if len(commands) <= 1 && p.TestCommand == "" {
			commands = append(commands, "- Verificar lint: `golangci-lint run ./...`")
		}
	case "JavaScript", "TypeScript":
		if len(commands) == 0 {
			commands = append(commands, "- Compilar: `npm run build`")
			commands = append(commands, "- Testes: `npm test`")
		}
	}

	// Sort commands for consistent output
	sort.Strings(commands)

	// Generate content
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# %s\n\n", p.ProjectName))
	content.WriteString(p.Description)
	content.WriteString("\n\n")

	// Stack section
	content.WriteString("## Stack\n\n")
	for _, item := range stack {
		if item != "" {
			content.WriteString(fmt.Sprintf("- %s\n", item))
		}
	}
	content.WriteString("\n")

	// Commands section
	content.WriteString("## Comandos\n\n")
	for _, cmd := range commands {
		content.WriteString(cmd)
		content.WriteString("\n")
	}
	content.WriteString("\n")

	// Conventions section (if any)
	if len(p.Conventions) > 0 {
		content.WriteString("## Convenções\n\n")
		for _, conv := range p.Conventions {
			content.WriteString(fmt.Sprintf("- %s\n", conv))
		}
		content.WriteString("\n")
	}

	return strings.TrimSpace(content.String())
}
