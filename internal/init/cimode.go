package init

import (
	"context"
	"fmt"
)

// RunCI runs the init process in non-interactive CI mode with --yes flag support.
// When yes is true, all prompts are skipped and existing DEVON.md is overwritten.
// In CI environments (non-TTY), this mode is automatically selected if --yes is passed.
func RunCI(ctx context.Context, workDir, outputPath string, yes bool) error {
	detector := NewDetector(workDir)
	wizard := NewWizard(detector)

	// Get project info non-interactively (no user input)
	info, err := wizard.RunNonInteractive(ctx)
	if err != nil {
		return fmt.Errorf("falha ao detectar projeto: %w", err)
	}

	// Generate DEVON.md content from detected info
	content := info.GenerateDEVONmd()

	// Write file: force overwrite if --yes is set
	force := yes
	if err := WriteFile(outputPath, content, force); err != nil {
		return fmt.Errorf("falha ao escrever DEVON.md: %w", err)
	}

	return nil
}
