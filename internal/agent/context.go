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

	return b.String()
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
