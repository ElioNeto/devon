package init

import (
	"context"
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
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
	Conventions   []string // Exported to allow template access
	HasCI         bool     // Whether CI configuration (.github/workflows) was detected
	HasDocker     bool     // Whether Docker configuration (docker-compose.yml) was detected
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
// ctx is used for cancellation and deadlines.
func (d *Detector) Detect(ctx context.Context) (ProjectInfo, error) {
	if ctx.Err() != nil {
		return d.info, ctx.Err()
	}

	// Get project name from workdir
	d.info.ProjectName = filepath.Base(d.workDir)
	if d.info.ProjectName == "" {
		return d.info, errors.New("não foi possível obter nome do diretório")
	}

	// Detect language and framework
	if err := d.detectLanguage(ctx); err != nil {
		return d.info, fmt.Errorf("falha ao detectar linguagem: %w", err)
	}

	// For Go projects, use module name from go.mod if available
	if d.info.Language.Name == "Go" {
		goModPath := filepath.Join(d.workDir, "go.mod")
		if content, err := os.ReadFile(goModPath); err == nil {
			// Parse module line: "module <name>"
			re := regexp.MustCompile(`^module\s+(\S+)`)
			if match := re.FindStringSubmatch(string(content)); len(match) > 1 {
				d.info.ProjectName = match[1]
			}
		}
	}

	// Detect framework
	d.detectFramework()

	// Detect package manager
	d.detectPackageManager()

	// Detect commands
	d.detectCommands()

	// Detect CI configuration
	d.detectCI()

	// Detect Docker configuration
	d.detectDocker()

	// Detect dev dependencies and add to conventions
	deps := d.detectDevDependencies()
	for _, dep := range deps {
		d.info.Conventions = append(d.info.Conventions, "Ferramenta detectada: "+dep)
	}

	return d.info, nil
}

// detectLanguage identifies the programming language.
func (d *Detector) detectLanguage(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	goMod := filepath.Join(d.workDir, "go.mod")
	packageJSON := filepath.Join(d.workDir, "package.json")
	cargoToml := filepath.Join(d.workDir, "Cargo.toml")
	pyprojectToml := filepath.Join(d.workDir, "pyproject.toml")
	requirementsTxt := filepath.Join(d.workDir, "requirements.txt")

	switch {
	case fileExists(goMod):
		d.info.Language.Name = "Go"
		if version, err := d.detectGoVersion(ctx); err == nil {
			d.info.Language.Version = version
		}
		return nil

	case fileExists(packageJSON):
		d.info.Language.Name = "JavaScript"
		if version, err := d.detectNodeFramework(ctx); err == nil && version != "" {
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
func (d *Detector) detectGoVersion(ctx context.Context) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	content, err := os.ReadFile(filepath.Join(d.workDir, "go.mod"))
	if err != nil {
		return "", err
	}

	pattern := `^go\s+([0-9]+\.[0-9]+)`
	re := regexp.MustCompile(pattern)
	match := re.FindStringSubmatch(string(content))

	if len(match) > 1 {
		return match[1], nil
	}

	return "1.x", nil
}

// detectNodeFramework identifies Node.js frameworks from package.json.
func (d *Detector) detectNodeFramework(ctx context.Context) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

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
	matches, _ := doublestar.Glob(os.DirFS(root), pattern)
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

// detectCI checks for CI configuration in .github/workflows.
func (d *Detector) detectCI() {
	ciDir := filepath.Join(d.workDir, ".github", "workflows")
	info, err := os.Stat(ciDir)
	d.info.HasCI = err == nil && info.IsDir()
}

// detectDocker checks for Docker configuration.
func (d *Detector) detectDocker() {
	dockerFile := filepath.Join(d.workDir, "docker-compose.yml")
	d.info.HasDocker = fileExists(dockerFile)
}
