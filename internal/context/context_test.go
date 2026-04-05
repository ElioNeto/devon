package context

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDetect_WorkDir(t *testing.T) {
	dir := t.TempDir()
	pc := Detect(dir)
	if pc.WorkDir != dir {
		t.Errorf("expected WorkDir %q, got %q", dir, pc.WorkDir)
	}
}

func TestDetect_GitBranch_NoRepo(t *testing.T) {
	dir := t.TempDir()
	pc := Detect(dir)
	if pc.GitBranch != "unknown" {
		t.Errorf("expected 'unknown' branch, got %q", pc.GitBranch)
	}
}

func TestDetect_GitBranch_InRepo(t *testing.T) {
	dir := t.TempDir()
	// Create a fake .git to see if Detect picks it up
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	pc := Detect(dir)
	// git rev-parse will fail since .git is not a real repo
	if pc.GitBranch != "unknown" {
		t.Errorf("expected 'unknown' branch, got %q", pc.GitBranch)
	}
}

func TestDetect_DevonMD(t *testing.T) {
	dir := t.TempDir()
	content := "# My Project\nRoot context."
	if err := os.WriteFile(filepath.Join(dir, "DEVON.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	pc := Detect(dir)
	if pc.DevonMD != content {
		t.Errorf("expected DevonMD %q, got %q", content, pc.DevonMD)
	}
	if pc.Summary == "" {
		t.Error("Summary should not be empty when DEVON.md exists")
	}
	if !strings.Contains(pc.Summary, "DEVON.md loaded") {
		t.Error("Summary should mention DEVON.md")
	}
}

func TestDetect_DevonMD_Missing(t *testing.T) {
	dir := t.TempDir()
	pc := Detect(dir)
	if pc.DevonMD != "" {
		t.Errorf("expected empty DevonMD, got %q", pc.DevonMD)
	}
	if strings.Contains(pc.Summary, "DEVON.md loaded") {
		t.Error("Summary should not mention DEVON.md when not found")
	}
}

func TestDetect_Languages(t *testing.T) {
	dir := t.TempDir()
	files := []string{"main.go", "utils.go", "index.js", "app.ts", "index.css", "README.md"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	pc := Detect(dir)
	if pc.Languages[".go"] != 2 {
		t.Errorf("expected 2 .go files, got %d", pc.Languages[".go"])
	}
	if pc.Languages[".js"] != 1 {
		t.Errorf("expected 1 .js file, got %d", pc.Languages[".js"])
	}
	if pc.Languages[".ts"] != 1 {
		t.Errorf("expected 1 .ts file, got %d", pc.Languages[".ts"])
	}
}

func TestDetect_Languages_IgnoresDotDirs(t *testing.T) {
	dir := t.TempDir()
	nm := filepath.Join(dir, "node_modules", "pkg", "index.js")
	if err := os.MkdirAll(filepath.Dir(nm), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nm, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	pc := Detect(dir)
	if pc.Languages[".js"] != 0 {
		t.Errorf("expected 0 .js files (node_modules skipped), got %d", pc.Languages[".js"])
	}
}

func TestDetect_Languages_IgnoresGitDir(t *testing.T) {
	dir := t.TempDir()
	gitObj := filepath.Join(dir, ".git", "objects", "pack", "file.o")
	if err := os.MkdirAll(filepath.Dir(gitObj), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gitObj, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	pc := Detect(dir)
	if pc.Languages[".o"] != 0 {
		t.Errorf("expected 0 .o files (.git skipped), got %d", pc.Languages[".o"])
	}
}

func TestDetect_Languages_IgnoresVendorDir(t *testing.T) {
	dir := t.TempDir()
	vendor := filepath.Join(dir, "vendor", "pkg", "util.go")
	if err := os.MkdirAll(filepath.Dir(vendor), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(vendor, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	pc := Detect(dir)
	if pc.Languages[".go"] != 0 {
		t.Errorf("expected 0 .go files (vendor skipped), got %d", pc.Languages[".go"])
	}
}

func TestDetect_SummaryContainsWorkDir(t *testing.T) {
	dir := t.TempDir()
	pc := Detect(dir)
	if !strings.Contains(pc.Summary, "Working directory") {
		t.Error("Summary should mention working directory")
	}
}

func TestDetect_LanguagesInSummary(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.py"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	pc := Detect(dir)
	if !strings.Contains(pc.Summary, "Detected languages") {
		t.Errorf("Summary should list languages, got: %q", pc.Summary)
	}
	if !strings.Contains(pc.Summary, ".py") {
		t.Errorf("Summary should mention .py, got: %q", pc.Summary)
	}
}

func TestDetect_FileLimit(t *testing.T) {
	dir := t.TempDir()
	// Create 600 files to test the 500 file limit
	for i := 0; i < 600; i++ {
		f := filepath.Join(dir, "file"+strconv.Itoa(i)+".txt")
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	pc := Detect(dir)
	// Should have stopped at 500 files
	if pc.Languages[".txt"] > 500 {
		t.Errorf("expected <= 500 .txt files (limit), got %d", pc.Languages[".txt"])
	}
}

func TestDetect_NestedDir(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "src", "pkg")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "util.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	pc := Detect(dir)
	if pc.Languages[".go"] != 2 {
		t.Errorf("expected 2 .go files in nested dirs, got %d", pc.Languages[".go"])
	}
}

func TestDetect_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	pc := Detect(dir)
	if len(pc.Languages) != 0 {
		t.Errorf("expected 0 languages in empty dir, got %d", len(pc.Languages))
	}
}

func TestDetect_DevonMD_inSummary(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "DEVON.md"), []byte("project: test"), 0o644); err != nil {
		t.Fatal(err)
	}

	pc := Detect(dir)
	if !strings.Contains(pc.DevonMD, "project: test") {
		t.Errorf("expected DevonMD content, got %q", pc.DevonMD)
	}
}

func TestDetect_MultipleExtensions(t *testing.T) {
	dir := t.TempDir()
	extensions := []string{".go", ".py", ".js", ".ts", ".rs", ".rb"}
	for i, ext := range extensions {
		if err := os.WriteFile(filepath.Join(dir, "f"+strconv.Itoa(i)+ext), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	pc := Detect(dir)
	if len(pc.Languages) != len(extensions) {
		t.Errorf("expected %d languages, got %d", len(extensions), len(pc.Languages))
	}
	for _, ext := range extensions {
		if pc.Languages[ext] != 1 {
			t.Errorf("expected 1 %s file, got %d", ext, pc.Languages[ext])
		}
	}
}

func TestProjectContext_IsNotEmpty(t *testing.T) {
	pc := &ProjectContext{}
	if pc.Languages != nil {
		t.Error("nil map should not cause panic on access if not initialized")
	}
}

func TestDetect_PanicFree_NilDir(t *testing.T) {
	// Ensure Detect doesn't panic on empty dir
	pc := Detect("")
	if pc == nil {
		t.Error("Detect should return non-nil for empty dir")
	}
}
