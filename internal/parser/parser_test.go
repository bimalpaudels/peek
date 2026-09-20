package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"peek/internal/runner"
)

func TestApplyBlockOutputs(t *testing.T) {
	code := `x = 10
x + 5

y = 20
print("hello")
y * 2`

	results := []runner.BlockResult{
		{
			StartLine: 2,
			EndLine:   2,
			Outputs:   []string{"➜ 15"},
		},
		{
			StartLine: 4,
			EndLine:   5,
			Outputs:   []string{"❯ hello", "❯ world"},
		},
		{
			StartLine: 6,
			EndLine:   6,
			Outputs:   []string{"➜ 40"},
		},
	}

	updated, err := ApplyBlockOutputs(code, results)
	if err != nil {
		t.Fatalf("ApplyBlockOutputs failed: %v", err)
	}

	// Block 0 and 2 are short single-line expressions -> placed inline
	// Block 1 has multiple output lines -> placed below
	expected := `x = 10
x + 5  # ➜ 15

y = 20
print("hello")
# ❯ hello
# ❯ world
y * 2  # ➜ 40`

	if updated != expected {
		t.Errorf("updated content mismatch:\nGOT:\n%s\nWANT:\n%s", updated, expected)
	}

	// Now test re-running block 2 with an updated output
	results2 := []runner.BlockResult{
		{
			StartLine: 8,
			EndLine:   8,
			Outputs:   []string{"➜ 999"},
		},
	}
	updated2, err := ApplyBlockOutputs(updated, results2)
	if err != nil {
		t.Fatalf("ApplyBlockOutputs failed on re-run: %v", err)
	}

	expected2 := `x = 10
x + 5  # ➜ 15

y = 20
print("hello")
# ❯ hello
# ❯ world
y * 2  # ➜ 999`

	if updated2 != expected2 {
		t.Errorf("re-run content mismatch:\nGOT:\n%s\nWANT:\n%s", updated2, expected2)
	}
}

func TestCleanOutputs(t *testing.T) {
	code := `x = 10  # ➜ 10
print("run")
# ❯ run
# => 10
# ✕ division by zero
y = 20`

	cleaned := CleanOutputs(code)

	for _, bad := range []string{"# ➜ 10", "# ❯ run", "# => 10", "# ✕ division by zero"} {
		if strings.Contains(cleaned, bad) {
			t.Errorf("CleanOutputs failed to strip %q; got:\n%s", bad, cleaned)
		}
	}

	expected := `x = 10
print("run")
y = 20`

	if strings.TrimSpace(cleaned) != expected {
		t.Errorf("CleanOutputs mismatch:\nGOT:\n%s\nWANT:\n%s", cleaned, expected)
	}
}

func TestCRLFHandling(t *testing.T) {
	crlfCode := "x = 1\r\n# => 1\r\ny = 2\r\n"
	cleaned := CleanOutputs(crlfCode)
	if !strings.Contains(cleaned, "\r\n") {
		t.Errorf("expected CRLF line endings preserved in CleanOutputs")
	}
	if strings.Contains(cleaned, "# =>") {
		t.Errorf("expected output comments removed")
	}

	results := []runner.BlockResult{
		{
			StartLine: 1,
			EndLine:   1,
			Outputs:   []string{"➜ 42"},
		},
	}
	applied, err := ApplyBlockOutputs(cleaned, results)
	if err != nil {
		t.Fatalf("ApplyBlockOutputs failed on CRLF: %v", err)
	}
	if !strings.Contains(applied, "\r\n") {
		t.Errorf("expected CRLF line endings preserved in ApplyBlockOutputs")
	}
	if strings.Contains(applied, "\r\r") {
		t.Errorf("unexpected carriage return duplication in ApplyBlockOutputs")
	}
}

func TestAtomicWritePermissionsAndSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "script.py")

	// Create file with 0755 permissions
	initialContent := "print('hello')\n"
	if err := os.WriteFile(targetFile, []byte(initialContent), 0755); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}
	if err := os.Chmod(targetFile, 0755); err != nil {
		t.Fatalf("failed to chmod target file: %v", err)
	}

	// Create symlink pointing to targetFile
	symlinkPath := filepath.Join(tmpDir, "link_to_script.py")
	if err := os.Symlink(targetFile, symlinkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// Perform atomic write via the symlink
	newContent := "print('world')\n"
	if err := AtomicWrite(symlinkPath, newContent); err != nil {
		t.Fatalf("AtomicWrite failed: %v", err)
	}

	// Check that symlink is still a symlink
	linkFi, err := os.Lstat(symlinkPath)
	if err != nil {
		t.Fatalf("failed to lstat symlink: %v", err)
	}
	if linkFi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected %s to remain a symlink, but mode is %v", symlinkPath, linkFi.Mode())
	}

	// Check that target file was updated
	data, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("failed to read target file: %v", err)
	}
	if string(data) != newContent {
		t.Errorf("target file content = %q, want %q", string(data), newContent)
	}

	// Check that 0755 permissions were preserved
	targetFi, err := os.Stat(targetFile)
	if err != nil {
		t.Fatalf("failed to stat target file: %v", err)
	}
	if targetFi.Mode().Perm() != 0755 {
		t.Errorf("target file perm = %v, want %v", targetFi.Mode().Perm(), 0755)
	}
}

func TestApplyBlockOutputs_EmptyOutputsCleansStaleComments(t *testing.T) {
	code := `x = 10  # ➜ 999
y = 20
# ❯ old output
z = 30`

	results := []runner.BlockResult{
		{
			StartLine: 1,
			EndLine:   1,
			Outputs:   nil, // statement now produces no output
		},
		{
			StartLine: 2,
			EndLine:   3,
			Outputs:   nil, // multiline statement now produces no output
		},
	}

	updated, err := ApplyBlockOutputs(code, results)
	if err != nil {
		t.Fatalf("ApplyBlockOutputs failed: %v", err)
	}

	expected := `x = 10
y = 20
z = 30`

	if updated != expected {
		t.Errorf("expected stale comments wiped:\nGOT:\n%s\nWANT:\n%s", updated, expected)
	}
}

func TestApplyBlockOutputs_PrettyPrintedMultiline(t *testing.T) {
	code := `user = get_user()
user`

	results := []runner.BlockResult{
		{
			StartLine: 2,
			EndLine:   2,
			Outputs: []string{
				"➜ {'id': 1,",
				"➜  'name': 'Alice',",
				"➜  'roles': ['admin']}",
			},
		},
	}

	updated, err := ApplyBlockOutputs(code, results)
	if err != nil {
		t.Fatalf("ApplyBlockOutputs failed: %v", err)
	}

	expected := `user = get_user()
user
# ➜ {'id': 1,
# ➜  'name': 'Alice',
# ➜  'roles': ['admin']}`

	if updated != expected {
		t.Errorf("expected multi-line pretty print comment block:\nGOT:\n%s\nWANT:\n%s", updated, expected)
	}
}

