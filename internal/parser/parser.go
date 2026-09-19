package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"peek/internal/runner"
)

// isOutputComment reports whether the trimmed line is a scratchpad output comment.
func isOutputComment(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "#") {
		return false
	}
	rest := strings.TrimSpace(trimmed[1:])
	return strings.HasPrefix(rest, "=>") ||
		strings.HasPrefix(rest, "➜") ||
		strings.HasPrefix(rest, "❯") ||
		strings.HasPrefix(rest, "✕") ||
		strings.HasPrefix(rest, "…")
}

// formatOutputComment ensures the output line is prefixed with '# '.
func formatOutputComment(out string) string {
	cleanOut := strings.TrimRight(out, "\r\n")
	if strings.HasPrefix(cleanOut, "#") {
		return cleanOut
	}
	return fmt.Sprintf("# %s", cleanOut)
}

// splitCodeAndOutput strips trailing output comments from block lines and returns the remaining code lines.
func splitCodeAndOutput(lines []string) []string {
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	for end > 0 && isOutputComment(lines[end-1]) {
		end--
	}
	return lines[:end]
}

// stripInlineOutputComment removes any trailing scratchpad output comment from a single code line.
func stripInlineOutputComment(line string) (string, bool) {
	inQuote := byte(0)
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if inQuote != 0 {
			if ch == inQuote {
				backslashes := 0
				for k := i - 1; k >= 0 && line[k] == '\\'; k-- {
					backslashes++
				}
				if backslashes%2 == 0 {
					inQuote = 0
				}
			}
			continue
		}
		if ch == '"' || ch == '\'' {
			if i+2 < len(line) && line[i+1] == ch && line[i+2] == ch {
				q := line[i : i+3]
				i += 3
				for i < len(line) {
					if strings.HasPrefix(line[i:], q) {
						i += 2
						break
					}
					i++
				}
				continue
			}
			inQuote = ch
			continue
		}
		if ch == '#' && isOutputComment(line[i:]) {
			return strings.TrimRight(line[:i], " \t"), true
		}
	}
	return line, false
}

// ApplyBlockOutputs splices the formatted outputs of the given blocks into content.
// Short single-line expression outputs are attached inline (e.g. `x  # ➜ 10`),
// while multi-line, long, or printed (stdout) outputs are placed below.
func ApplyBlockOutputs(content string, blockResults []runner.BlockResult) (string, error) {
	if len(blockResults) == 0 {
		return content, nil
	}

	eol := "\n"
	if strings.Contains(content, "\r\n") {
		eol = "\r\n"
	}

	lines := strings.Split(content, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], "\r")
	}

	// Sort blocks descending by StartLine so line numbers of earlier blocks remain stable
	sorted := make([]runner.BlockResult, len(blockResults))
	copy(sorted, blockResults)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].StartLine > sorted[j].StartLine
	})

	for _, b := range sorted {
		if b.StartLine < 1 || b.EndLine > len(lines) || b.StartLine > b.EndLine {
			continue
		}

		startIdx := b.StartLine - 1
		endIdx := b.EndLine

		// Clean any previous inline output comments across statement lines
		for i := startIdx; i < endIdx; i++ {
			lines[i], _ = stripInlineOutputComment(lines[i])
		}

		codeLines := splitCodeAndOutput(lines[startIdx:endIdx])

		// Check if output can be placed inline:
		// 1. Exactly 1 output line
		// 2. Output is an evaluated expression (starts with ➜), not a print log (❯) or error (✕)
		// 3. Statement is single-line (len(codeLines) == 1)
		// 4. Combined length fits within 100 characters
		isInline := len(b.Outputs) == 1 && len(codeLines) == 1 &&
			strings.HasPrefix(strings.TrimSpace(b.Outputs[0]), "➜") &&
			len(lines[startIdx])+2+len(formatOutputComment(b.Outputs[0])) <= 100

		var newBlock []string
		if isInline {
			newBlock = []string{fmt.Sprintf("%s  %s", lines[startIdx], formatOutputComment(b.Outputs[0]))}
		} else {
			newBlock = append(newBlock, codeLines...)
			for _, out := range b.Outputs {
				newBlock = append(newBlock, formatOutputComment(out))
			}
		}

		lines = append(lines[:startIdx], append(newBlock, lines[endIdx:]...)...)
	}

	return strings.Join(lines, eol), nil
}

// CleanOutputs removes all managed output comment lines (both inline and standalone).
func CleanOutputs(content string) string {
	eol := "\n"
	if strings.Contains(content, "\r\n") {
		eol = "\r\n"
	}
	lines := strings.Split(content, "\n")
	var kept []string
	for _, l := range lines {
		trimmed := strings.TrimRight(l, "\r")
		if isOutputComment(trimmed) {
			// Entire line is an output comment -> drop
			continue
		}
		// Strip trailing inline output comment if present
		cleaned, _ := stripInlineOutputComment(trimmed)
		kept = append(kept, cleaned)
	}
	return strings.Join(kept, eol)
}

// AtomicWrite writes content safely using a temporary file in the same directory,
// preserving original file permissions and resolving symlinks.
func AtomicWrite(filePath string, content string) error {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return err
	}

	// Resolve symlink so we write to the target file instead of overwriting the symlink itself
	targetPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		targetPath = absPath
	}

	// Capture existing permissions, default to 0644 if file doesn't exist
	var mode os.FileMode = 0644
	if fi, err := os.Stat(targetPath); err == nil {
		mode = fi.Mode().Perm()
	}

	dir := filepath.Dir(targetPath)
	tmpFile, err := os.CreateTemp(dir, ".peek_tmp_*")
	if err != nil {
		return err
	}
	tmpName := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()

	// Apply original permissions to temp file
	if err := tmpFile.Chmod(mode); err != nil {
		_ = tmpFile.Close()
		return err
	}

	if _, err := tmpFile.WriteString(content); err != nil {
		_ = tmpFile.Close()
		return err
	}

	if err := tmpFile.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, targetPath)
}
