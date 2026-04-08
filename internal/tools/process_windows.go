//go:build windows

package tools

import (
	"os/exec"
)

func setProcessGroup(cmd *exec.Cmd) {
	// Not supported on Windows in this way
}

func killProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process != nil {
		return cmd.Process.Kill()
	}
	return nil
}
