package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleCode = `x = 10
y = 20
x + y

def add(a, b):
    return a + b
add(x, y)
# => Out: 30

z = add(x, y) * 2
print(f"Total: {z}")`

func TestParseBlocks(t *testing.T) {
	blocks := ParseBlocks(sampleCode)
	if len(blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(blocks))
	}

	if blocks[0].Index != 0 || !strings.Contains(blocks[0].Code, "x = 10") {
		t.Errorf("unexpected block 0: %+v", blocks[0])
	}
	if len(blocks[0].OutputLines) != 0 {
		t.Errorf("expected empty outputs for block 0, got %v", blocks[0].OutputLines)
	}

	if blocks[1].Index != 1 || !strings.Contains(blocks[1].Code, "def add") {
		t.Errorf("unexpected block 1: %+v", blocks[1])
	}
	if len(blocks[1].OutputLines) != 1 || blocks[1].OutputLines[0] != "# => Out: 30" {
		t.Errorf("unexpected outputs for block 1: %v", blocks[1].OutputLines)
	}

	if blocks[2].Index != 2 || !strings.Contains(blocks[2].Code, "z = add") {
		t.Errorf("unexpected block 2: %+v", blocks[2])
	}
}

func TestFindBlockByLine(t *testing.T) {
	blocks := ParseBlocks(sampleCode)

	b0 := FindBlockByLine(blocks, 2)
	if b0 == nil || b0.Index != 0 {
		t.Errorf("expected block 0 for line 2, got %v", b0)
	}

	b1 := FindBlockByLine(blocks, 6)
	if b1 == nil || b1.Index != 1 {
		t.Errorf("expected block 1 for line 6, got %v", b1)
	}

	// Blank line 4 should map to preceding block 0
	bBlank := FindBlockByLine(blocks, 4)
	if bBlank == nil || bBlank.Index != 0 {
		t.Errorf("expected block 0 for line 4, got %v", bBlank)
	}
}

func TestUpdateBlockOutput(t *testing.T) {
	updated, err := UpdateBlockOutput(sampleCode, 0, []string{"Out: 30"})
	if err != nil {
		t.Fatalf("UpdateBlockOutput failed: %v", err)
	}

	blocks := ParseBlocks(updated)
	if len(blocks[0].OutputLines) != 1 || blocks[0].OutputLines[0] != "# => Out: 30" {
		t.Errorf("expected updated output on block 0, got %v", blocks[0].OutputLines)
	}
	// Block 1 output must remain intact
	if len(blocks[1].OutputLines) != 1 || blocks[1].OutputLines[0] != "# => Out: 30" {
		t.Errorf("block 1 output corrupted: %v", blocks[1].OutputLines)
	}
}

func TestCleanOutputs(t *testing.T) {
	cleaned := CleanOutputs(sampleCode)
	if strings.Contains(cleaned, "# => Out: 30") {
		t.Errorf("clean failed to remove comment: %s", cleaned)
	}
	blocks := ParseBlocks(cleaned)
	for _, b := range blocks {
		if len(b.OutputLines) > 0 {
			t.Errorf("expected 0 outputs in cleaned code, block %d has %v", b.Index, b.OutputLines)
		}
	}
}

func TestCRLFHandling(t *testing.T) {
	crlfCode := "x = 1\r\n\r\ny = 2\r\n# => Out: 2\r\n"
	cleaned := CleanOutputs(crlfCode)
	if !strings.Contains(cleaned, "\r\n") {
		t.Errorf("expected CRLF line endings preserved in CleanOutputs")
	}
	if strings.Contains(cleaned, "# =>") {
		t.Errorf("expected output comments removed")
	}

	updated, err := UpdateBlockOutput(crlfCode, 0, []string{"Out: 1"})
	if err != nil {
		t.Fatalf("UpdateBlockOutput failed: %v", err)
	}
	if !strings.Contains(updated, "\r\n") {
		t.Errorf("expected CRLF line endings preserved in UpdateBlockOutput")
	}
	if strings.Contains(updated, "\r\r") {
		t.Errorf("unexpected carriage return duplication in UpdateBlockOutput")
	}
}

func TestAtomicWritePermissionsAndSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "script.py")

	// Create a file with 0755 executable permissions
	initialContent := "print('hello')\n"
	if err := os.WriteFile(targetFile, []byte(initialContent), 0755); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}
	// Explicitly chmod to ensure umask didn't mask permissions
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
