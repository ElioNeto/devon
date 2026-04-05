// Package context provides project context detection for the system prompt.
package context

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// ProjectContext contains detected information about the project.
type ProjectContext struct {
	WorkDir   string         `json:"work_dir"`
	GitBranch string         `json:"git_branch"`
	Languages map[string]int `json:"languages"` // extension -> count
	DevonMD   string         `json:"devon_md"`  // contents of DEVON.md
	Summary   string         `json:"summary"`
}

// Detect gathers project context for the given working directory.
func Detect(workDir string) *ProjectContext {
	pc := &ProjectContext{
		WorkDir:   workDir,
		Languages: make(map[string]int),
	}

	pc.detectGitBranch()
	pc.detectLanguages(workDir)
	pc.readDevonMD(workDir)
	pc.buildSummary()

	return pc
}

func (pc *ProjectContext) detectGitBranch() {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = pc.WorkDir
	out, err := cmd.Output()
	if err != nil {
		pc.GitBranch = "unknown"
		return
	}
	pc.GitBranch = strings.TrimSpace(string(out))
}

func (pc *ProjectContext) detectLanguages(dir string) {
	limit := 500

	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || name == ".devon" {
				return filepath.SkipDir
			}
			return nil
		}
		if limit <= 0 {
			return filepath.SkipAll
		}
		limit--

		ext := filepath.Ext(d.Name())
		if ext == "" {
			return nil
		}
		pc.Languages[ext]++
		return nil
	})
}

func (pc *ProjectContext) readDevonMD(dir string) {
	data, err := os.ReadFile(filepath.Join(dir, "DEVON.md"))
	if err == nil {
		pc.DevonMD = string(data)
	}
}

func (pc *ProjectContext) buildSummary() {
	var sb strings.Builder
	sb.WriteString("Working directory: " + pc.WorkDir + "\n")

	if pc.GitBranch != "" && pc.GitBranch != "unknown" {
		sb.WriteString("Git branch: " + pc.GitBranch + "\n")
	}

	if len(pc.Languages) > 0 {
		sb.WriteString("Detected languages:\n")
		exts := make([]string, 0, len(pc.Languages))
		for ext := range pc.Languages {
			exts = append(exts, ext)
		}
		sort.Strings(exts)
		for _, ext := range exts {
			sb.WriteString("  " + ext + ": " + itoaF(pc.Languages[ext]) + " files\n")
		}
	}

	if pc.DevonMD != "" {
		sb.WriteString("\nDEVON.md loaded:\n" + pc.DevonMD + "\n")
	}

	pc.Summary = sb.String()
}

func itoaF(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
