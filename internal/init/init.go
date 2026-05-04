package init

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

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

// DetectFromGitRemote detects project info from git remote.
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
