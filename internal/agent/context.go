// Package agent — project context detection for system prompt.
package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var extLang = map[string]string{
	".go":   "Go",
	".py":   "Python",
	".ts":   "TypeScript",
	".js":   "JavaScript",
	".rs":   "Rust",
	".java": "Java",
	".rb":   "Ruby",
	".php":  "PHP",
	".cs":   "C#",
	".cpp":  "C++",
}

// BuildProjectContext returns a string with environment information
// to be appended to the system prompt.
func BuildProjectContext(workDir string) string {
	abs, err := filepath.Abs(workDir)
	if err != nil {
		abs = workDir
	}

	var b strings.Builder
	b.WriteString("Contexto do projeto:\n")
	b.WriteString("- Diretório de trabalho: ")
	b.WriteString(abs)
	b.WriteString("\n")

	// Git branch (2s timeout)
	if branch := gitBranch(workDir); branch != "" {
		b.WriteString("- Branch do Git: ")
		b.WriteString(branch)
		b.WriteString("\n")
	}

	// Detected languages
	if langs := detectLanguages(workDir); len(langs) > 0 {
		b.WriteString("- Linguagens detectadas: ")
		b.WriteString(strings.Join(langs, ", "))
		b.WriteString("\n")
	}

	// Root-level files (up to 30 entries)
	if files := listRootFiles(workDir); files != "" {
		b.WriteString("- Arquivos raiz: ")
		b.WriteString(files)
		b.WriteString("\n")
	}

	// go.mod detection
	if module, goVer := readGoMod(workDir); module != "" {
		b.WriteString("- Módulo: ")
		b.WriteString(module)
		b.WriteString("\n")
		if goVer != "" {
			b.WriteString("- Versão Go: ")
			b.WriteString(goVer)
			b.WriteString("\n")
		}
	}

	return b.String()
}

// listRootFiles returns a comma-separated string of up to 30 root-level entries.
func listRootFiles(workDir string) string {
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return ""
	}

	var names []string
	for i, e := range entries {
		if i >= 30 {
			names = append(names, "...")
			break
		}
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		names = append(names, name)
	}

	return strings.Join(names, ", ")
}

// readGoMod extracts the module name and Go version from go.mod.
// Returns empty strings if go.mod cannot be read or parsed.
func readGoMod(workDir string) (module, goVersion string) {
	data, err := os.ReadFile(filepath.Join(workDir, "go.mod"))
	if err != nil {
		return "", ""
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			module = strings.TrimPrefix(line, "module ")
		}
		if strings.HasPrefix(line, "go ") {
			goVersion = strings.TrimPrefix(line, "go ")
		}
	}
	return module, goVersion
}

func gitBranch(workDir string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", workDir, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func detectLanguages(workDir string) []string {
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return nil
	}

	seen := make(map[string]bool)
	var langs []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if lang, ok := extLang[ext]; ok && !seen[lang] {
			seen[lang] = true
			langs = append(langs, lang)
		}
	}
	sort.Strings(langs)
	return langs
}
