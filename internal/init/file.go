package init

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Exists checks if a file or directory exists at path.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// WriteFile writes the DEVON.md content to disk atomically, respecting the force flag.
// If path exists and force is false, prompts the user to open in editor or overwrite.
// If force is true, overwrites the existing file.
func WriteFile(path, content string, force bool) error {
	// Check if file exists
	if Exists(path) {
		if !force {
			if err := promptExistingFile(path, &force); err != nil {
				return err
			}
			// If user chose to open the editor, don't write
			if !force {
				return nil
			}
		}
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

// promptExistingFile handles user interaction when DEVON.md already exists and force is false.
// Updates the force flag if user chooses to overwrite.
func promptExistingFile(path string, force *bool) error {
	fmt.Fprintf(os.Stderr, "\n%s já existe. Abrir no editor ou sobrescrever?\n", path)
	fmt.Fprintf(os.Stderr, "  [1] Abrir no $EDITOR (padrão)\n")
	fmt.Fprintf(os.Stderr, "  [2] Sobrescrever\n")
	fmt.Print("  Escolha: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input != "1" && input != "" {
		*force = true
		return nil
	}

	// Open in editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nano"
	}
	fmt.Fprintf(os.Stderr, "Abrindo %s no editor...\n", path)
	editorCmd := exec.Command(editor, path)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr
	return editorCmd.Run()
}
