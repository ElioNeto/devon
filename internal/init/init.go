package init

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ProjectInfo holds detected project information.
type ProjectInfo struct {
	ProjectName    string
	Description    string
	TestCommand    string
	BuildCommand   string
	Language       Language
	Framework      string
	PackageManager string
	Commands       map[string]string
	 conventions   []string
}

// Language represents a detected programming language.
type Language struct {
	Name    string
	Version string
}

// Detector handles project detection and initialization.
type Detector struct {
	workDir string
	info    ProjectInfo
}

// NewDetector creates a new project detector for the given work directory.
func NewDetector(workDir string) *Detector {
	return &Detector{
		workDir: workDir,
		info: ProjectInfo{
			Commands: make(map[string]string),
		},
	}
}

// Detect analyzes the project and returns detected information.
func (d *Detector) Detect() (ProjectInfo, error) {
	// Get project name from workdir
	d.info.ProjectName = filepath.Base(d.workDir)
	if d.info.ProjectName == "" {
		return d.info, errors.New("não foi possível obter nome do diretório")
	}

	// Detect language and framework
	if err := d.detectLanguage(); err != nil {
		return d.info, fmt.Errorf("falha ao detectar linguagem: %w", err)
	}

	// Detect framework
	d.detectFramework()

	// Detect package manager
	d.detectPackageManager()

	// Detect commands
	d.detectCommands()

	return d.info, nil
}

// detectLanguage identifies the programming language.
func (d *Detector) detectLanguage() error {
	goMod := filepath.Join(d.workDir, "go.mod")
	packageJSON := filepath.Join(d.workDir, "package.json")
	cargoToml := filepath.Join(d.workDir, "Cargo.toml")
	pyprojectToml := filepath.Join(d.workDir, "pyproject.toml")
	requirementsTxt := filepath.Join(d.workDir, "requirements.txt")

	switch {
	case fileExists(goMod):
		d.info.Language.Name = "Go"
		if version, err := d.detectGoVersion(); err == nil {
			d.info.Language.Version = version
		}
		return nil

	case fileExists(packageJSON):
		d.info.Language.Name = "JavaScript"
		if version, err := d.detectNodeFramework(); err == nil && version != "" {
			d.info.Framework = version
		}
		return nil

	case fileExists(cargoToml):
		d.info.Language.Name = "Rust"
		return nil

	case fileExists(pyprojectToml) || fileExists(requirementsTxt):
		d.info.Language.Name = "Python"
		return nil

	default:
		d.info.Language.Name = "Unknown"
		return nil
	}
}

// detectGoVersion extracts Go version from go.mod.
func (d *Detector) detectGoVersion() (string, error) {
	content, err := os.ReadFile(filepath.Join(d.workDir, "go.mod"))
	if err != nil {
		return "", err
	}

	pattern := `^go\s+([0-9]+\.[0-9]+)`
	re := regexp.MustCompile(pattern)
	match := re.FindStringSubmatch(string(content))

	if len(match) > 1 {
		return "1." + match[1], nil
	}

	return "1.x", nil
}

// detectNodeFramework identifies Node.js frameworks from package.json.
func (d *Detector) detectNodeFramework() (string, error) {
	content, err := os.ReadFile(filepath.Join(d.workDir, "package.json"))
	if err != nil {
		return "", err
	}

	var pkg struct {
		Name         string
		Dependencies map[string]string `json:"dependencies"`
	}

	if err := json.Unmarshal(content, &pkg); err != nil {
		return "", err
	}

	frameworks := []struct {
		packageName string
		framework   string
	}{
		{"next", "Next.js"},
		{"react", "React"},
		{"express", "Express"},
		{"vue", "Vue"},
		{"angular", "Angular"},
		{"svelte", "Svelte"},
	}

	for _, fw := range frameworks {
		if _, exists := pkg.Dependencies[fw.packageName]; exists {
			return fw.framework, nil
		}
	}

	return "", nil
}

// detectFramework identifies the web/mobile framework.
func (d *Detector) detectFramework() {
	switch d.info.Language.Name {
	case "Go":
		if fileExists(filepath.Join(d.workDir, "gin.go")) ||
			filepathExistsGlob(d.workDir, "**/*gin*") {
			d.info.Framework = "Gin"
		} else if filepathExistsGlob(d.workDir, "**/echo/**") {
			d.info.Framework = "Echo"
		}
	case "JavaScript":
		// Already handled in detectNodeFramework
	}
}

// detectPackageManager identifies the package manager.
func (d *Detector) detectPackageManager() {
	switch d.info.Language.Name {
	case "Go":
		d.info.PackageManager = "go modules"
	case "JavaScript":
		if fileExists(filepath.Join(d.workDir, "package-lock.json")) {
			d.info.PackageManager = "npm"
		} else if fileExists(filepath.Join(d.workDir, "yarn.lock")) {
			d.info.PackageManager = "yarn"
		} else if fileExists(filepath.Join(d.workDir, "pnpm-lock.yaml")) {
			d.info.PackageManager = "pnpm"
		} else {
			d.info.PackageManager = "npm (package.json)"
		}
	case "Rust":
		d.info.PackageManager = "cargo"
	case "Python":
		d.info.PackageManager = "pip"
		if fileExists(filepath.Join(d.workDir, "poetry.lock")) {
			d.info.PackageManager = "poetry"
		}
	}
}

// detectCommands looks for build and test commands.
func (d *Detector) detectCommands() {
	// Check Makefile
	if content, err := os.ReadFile(filepath.Join(d.workDir, "Makefile")); err == nil {
		d.parseMakefile(string(content))
	}

	// Check package.json scripts
	if content, err := os.ReadFile(filepath.Join(d.workDir, "package.json")); err == nil {
		d.parsePackageScripts(string(content))
	}

	// Check pyproject.toml
	if content, err := os.ReadFile(filepath.Join(d.workDir, "pyproject.toml")); err == nil {
		d.parsePyproject(string(content))
	}

	// Set defaults if not found
	if d.info.TestCommand == "" {
		d.info.TestCommand = getDefaultTestCommand(d.info.Language.Name)
	}
	if d.info.BuildCommand == "" {
		d.info.BuildCommand = getDefaultBuildCommand(d.info.Language.Name)
	}
}

// parseMakefile extracts build/test commands from Makefile.
func (d *Detector) parseMakefile(content string) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "test:") || strings.HasPrefix(line, "test ") {
			d.info.TestCommand = strings.TrimPrefix(line, "test:")
			d.info.TestCommand = strings.TrimSpace(d.info.TestCommand)
		}
		if strings.HasPrefix(line, "build:") || strings.HasPrefix(line, "build ") {
			d.info.BuildCommand = strings.TrimPrefix(line, "build:")
			d.info.BuildCommand = strings.TrimSpace(d.info.BuildCommand)
		}
	}

	// Also check for common patterns
	if d.info.TestCommand == "" && fileExists(filepath.Join(d.workDir, "Makefile")) {
		d.info.Commands["make test"] = "make test"
	}
	if d.info.BuildCommand == "" && fileExists(filepath.Join(d.workDir, "Makefile")) {
		d.info.Commands["make build"] = "make build"
	}
}

// parsePackageScripts extracts scripts from package.json.
func (d *Detector) parsePackageScripts(content string) {
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}

	if err := json.Unmarshal([]byte(content), &pkg); err != nil {
		return
	}

	if testCmd, ok := pkg.Scripts["test"]; ok {
		d.info.TestCommand = testCmd
	}
	if buildCmd, ok := pkg.Scripts["build"]; ok {
		d.info.BuildCommand = buildCmd
	}
}

// parsePyproject extracts scripts from pyproject.toml.
func (d *Detector) parsePyproject(content string) {
	// Simple regex-based parsing for common patterns
	testRe := regexp.MustCompile(`test\s*=\s*"([^"]+)"`)
	buildRe := regexp.MustCompile(`build\s*=\s*"([^"]+)"`)

	if matches := testRe.FindStringSubmatch(content); len(matches) > 1 {
		d.info.TestCommand = matches[1]
	}
	if matches := buildRe.FindStringSubmatch(content); len(matches) > 1 {
		d.info.BuildCommand = matches[1]
	}
}

// getDefaultTestCommand returns a default test command for the language.
func getDefaultTestCommand(lang string) string {
	switch lang {
	case "Go":
		return "go test ./..."
	case "JavaScript", "TypeScript":
		return "npm test"
	case "Rust":
		return "cargo test"
	case "Python":
		return "pytest"
	default:
		return ""
	}
}

// getDefaultBuildCommand returns a default build command for the language.
func getDefaultBuildCommand(lang string) string {
	switch lang {
	case "Go":
		return "go build ./..."
	case "JavaScript", "TypeScript":
		return "npm run build"
	case "Rust":
		return "cargo build"
	case "Python":
		return ""
	default:
		return ""
	}
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// filepathExistsGlob checks if any file matches the glob pattern.
func filepathExistsGlob(root, pattern string) bool {
	matches, _ := filepath.Glob(filepath.Join(root, "**", pattern))
	return len(matches) > 0
}

// detectDevDependencies checks for common development dependencies.
func (d *Detector) detectDevDependencies() []string {
	var deps []string

	if fileExists(filepath.Join(d.workDir, "go.mod")) {
		if _, err := exec.LookPath("golangci-lint"); err == nil {
			deps = append(deps, "golangci-lint")
		}
	}

	return deps
}

// InteractiveQuestion represents a question to ask the user.
type InteractiveQuestion struct {
	Question      string
	Default       string
	ValidatorFunc func(string) error
}

// Wizard handles interactive user input for initialization.
type Wizard struct {
	detector    *Detector
	reader      *bufio.Reader
	info        ProjectInfo
	questions   []InteractiveQuestion
	currentStep int
}

// NewWizard creates a new interactive wizard.
func NewWizard(detector *Detector) *Wizard {
	return &Wizard{
		detector: detector,
		reader:   bufio.NewReader(os.Stdin),
	}
}

// Run runs the wizard flow interactively.
func (w *Wizard) Run() (ProjectInfo, error) {
	var err error

	w.info, err = w.detector.Detect()
	if err != nil {
		return w.info, fmt.Errorf("falha ao detectar projeto: %w", err)
	}

	// Step 1: Project name
	w.info.ProjectName, err = w.askString("Nome do projeto:", w.info.ProjectName, nil)
	if err != nil {
		return w.info, err
	}

	// Step 2: Description
	w.info.Description, err = w.askString("Descrição em uma linha:", "", nil)
	if err != nil {
		return w.info, err
	}

	// Step 3: Test command
	w.info.TestCommand, err = w.askString("Como rodar os testes?", w.info.TestCommand, nil)
	if err != nil {
		return w.info, err
	}

	// Step 4: Build command
	w.info.BuildCommand, err = w.askString("Como compilar?", w.info.BuildCommand, nil)
	if err != nil {
		return w.info, err
	}

	// Step 5: Conventions
	conv, err := w.askString("Convenções importantes (opcional):", "", nil)
	if err != nil {
		return w.info, err
	}
	if conv != "" {
		w.info.conventions = append(w.info.conventions, conv)
	}

	return w.info, nil
}

// askString prompts the user for a string value.
func (w *Wizard) askString(question string, defaultValue string, validator func(string) error) (string, error) {
	fmt.Printf("\n%s", question)
	if defaultValue != "" {
		fmt.Printf(" [%s]", defaultValue)
	}
	fmt.Print("\n> ")

	input, err := w.reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("falha ao ler input: %w", err)
	}

	value := strings.TrimSpace(input)
	if value == "" {
		value = defaultValue
	}

	if validator != nil {
		if err := validator(value); err != nil {
			return "", err
		}
	}

	return value, nil
}

// RunNonInteractive runs the wizard in non-interactive mode using detected values.
func (w *Wizard) RunNonInteractive() (ProjectInfo, error) {
	return w.detector.Detect()
}

const devonMarkdownTemplate = `# {{.ProjectName}}

{{.Description}}

## Stack

{{if .Framework}}- {{.Framework}}{{end}}
- {{.Language.Name}} {{.Language.Version}}
{{if .PackageManager}}- {{.PackageManager}}{{end}}

## Comandos

{{if .BuildCommand}}- Compilar: ` + "`" + `{{.BuildCommand}}` + "`" + `
{{end}}{{if .TestCommand}}- Testes: ` + "`" + `{{.TestCommand}}` + "`" + `
{{end}}{{if .conventions}}{{if .conventions}}- Verificar lint: ` + "`" + `golangci-lint run ./...` + "`" + `
{{end}}{{end}}
{{if .conventions}}## Convenções

{{range .conventions}}- {{.}}
{{end}}{{end}}
`

// GenerateDEVONmd generates the DEVON.md content.
func (p *ProjectInfo) GenerateDEVONmd() string {
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
	if len(p.conventions) > 0 {
		content.WriteString("## Convenções\n\n")
		for _, conv := range p.conventions {
			content.WriteString(fmt.Sprintf("- %s\n", conv))
		}
		content.WriteString("\n")
	}

	return strings.TrimSpace(content.String())
}

// WriteFile writes the DEVON.md to disk atomically.
func WriteFile(path, content string, force bool) error {
	// Check if file exists
	if _, err := os.Stat(path); err == nil {
		if !force {
			// File exists, offer to open or overwrite
			fmt.Fprintf(os.Stderr, "\n%s já existe. Abrir no editor ou sobrescrever?\n", path)
			fmt.Fprintf(os.Stderr, "  [1] Abrir no $EDITOR (padrão)\n")
			fmt.Fprintf(os.Stderr, "  [2] Sobrescrever\n")
			fmt.Print("  Escolha: ")

			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)

			if input != "1" && input != "" {
				force = true
			}
		}
	}

	if !force {
		// Ask to open in editor
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "nano"
		}
		fmt.Fprintf(os.Stderr, "Abrindo %s no editor...\n", path)
		return nil
	}

	// Atomic write: temp file + rename
	tmpFile, err := os.CreateTemp(filepath.Dir(path), ".tmp-devon-")
	if err != nil {
		return fmt.Errorf("criar arquivo temporário: %w", err)
	}
	tmpPath := tmpFile.Name()

	_, err = tmpFile.WriteString(content)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("escrever arquivo temporário: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("fechar arquivo temporário: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renomear para destino: %w", err)
	}

	return nil
}

// PrintSummary prints a summary of the detected project.
func PrintSummary(info ProjectInfo) {
	fmt.Printf("\nProjeto detectado em: %s\n", info.ProjectName)
	fmt.Printf("Linguagem: %s %s\n", info.Language.Name, info.Language.Version)
	if info.Framework != "" {
		fmt.Printf("Framework: %s\n", info.Framework)
	}
	fmt.Printf("Gerenciador de pacotes: %s\n", info.PackageManager)

	fmt.Printf("\nComandos detectados:\n")
	if info.BuildCommand != "" {
		fmt.Printf("  - Build:  %s\n", info.BuildCommand)
	}
	if info.TestCommand != "" {
		fmt.Printf("  - Testes: %s\n", info.TestCommand)
	}
}

// DetectFromGitRemotedetects project info from git remote.
func DetectFromGitRemote(workDir string) (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	url := strings.TrimSpace(string(output))

	// Extract project name from URL
	// GitHub: git@github.com:user/repo.git
	// HTTPS: https://github.com/user/repo.git
	re := regexp.MustCompile(`/([^/]+?)(?:\.git)?$`)
	matches := re.FindStringSubmatch(url)
	if len(matches) > 1 {
		return matches[1], nil
	}

	return "", nil
}
