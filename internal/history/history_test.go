package history

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSessionDir_CreatesDirectory(t *testing.T) {
	workDir := "/some/project/path"

	dir, err := SessionDir(workDir)
	if err != nil {
		t.Fatalf("SessionDir() error: %v", err)
	}

	if !filepath.IsAbs(dir) {
		t.Errorf("SessionDir() returned relative path: %q", dir)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("session directory does not exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("session path is not a directory")
	}
}

func TestSessionDir_SameInputSameOutput(t *testing.T) {
	dir1, _ := SessionDir("/same/input")
	dir2, _ := SessionDir("/same/input")
	if dir1 != dir2 {
		t.Errorf("SessionDir not deterministic: %q vs %q", dir1, dir2)
	}
}

func TestSessionDir_DifferentInputDifferentOutput(t *testing.T) {
	dir1, _ := SessionDir("/project/a")
	dir2, _ := SessionDir("/project/b")
	if dir1 == dir2 {
		t.Error("different inputs produced same session dir")
	}
}
