package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sc/internal/runner"
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
		strings.HasPrefix(rest, "…") ||
		strings.HasPrefix(rest, "Out:") ||
		strings.HasPrefix(rest, "Error:")
}

// formatOutputComment ensures the output line is prefixed with '# '.
func formatOutputComment(out string) string {
	cleanOut := strings.TrimRight(out, "\r\n")
	if strings.HasPrefix(cleanOut, "#") {
		return cleanOut
	}
	trimmed := strings.TrimSpace(cleanOut)
	if strings.HasPrefix(trimmed, "➜") ||
		strings.HasPrefix(trimmed, "❯") ||
		strings.HasPrefix(trimmed, "✕") ||
		strings.HasPrefix(trimmed, "…") ||
		strings.HasPrefix(trimmed, "=>") {
		return fmt.Sprintf("# %s", cleanOut)
	}
	return fmt.Sprintf("# => %s", cleanOut)
}

// splitCodeAndOutput separates code lines from any trailing output comments.
func splitCodeAndOutput(lines []string) ([]string, []string) {
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}

	start := end
	for start > 0 && isOutputComment(lines[start-1]) {
		start--
	}

	var outLines []string
	for _, l := range lines[start:end] {
		outLines = append(outLines, strings.TrimRight(l, "\r\n"))
	}
	return lines[:start], outLines
}

// findCommentPos finds the unquoted '#' comment start index on a single line, or -1 if none.
func findCommentPos(line string) int {
	i := 0
	n := len(line)
	for i < n {
		ch := line[i]
		if ch == '#' {
			return i
		}
		if i+3 <= n && (line[i:i+3] == `"""` || line[i:i+3] == `'''`) {
			q := line[i : i+3]
			i += 3
			for i < n {
				if strings.HasPrefix(line[i:], q) {
					i += 3
					break
				}
				i++
			}
			continue
		}
		if ch == '"' || ch == '\'' {
			q := ch
			i++
			for i < n {
				if line[i] == q {
					backslashes := 0
					for k := i - 1; k >= 0 && line[k] == '\\'; k-- {
						backslashes++
					}
					if backslashes%2 == 0 {
						i++
						break
					}
				}
				i++
			}
			continue
		}
		i++
	}
	return -1
}

// stripInlineOutputComment removes any trailing scratchpad output comment from a single code line.
func stripInlineOutputComment(line string) (string, bool) {
	pos := findCommentPos(line)
	if pos == -1 {
		return line, false
	}
	comment := line[pos:]
	if isOutputComment(comment) {
		cleanCode := strings.TrimRight(line[:pos], " \t")
		return cleanCode, true
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

		// Clean any previous inline output comment on start line
		cleanStartLine, _ := stripInlineOutputComment(lines[startIdx])
		lines[startIdx] = cleanStartLine

		blockLines := lines[startIdx:endIdx]
		codeLines, _ := splitCodeAndOutput(blockLines)

		// Check if output can be placed inline:
		// 1. Exactly 1 output line
		// 2. Output is an evaluated expression (starts with ➜), not a print log (❯) or error (✕)
		// 3. Statement is single-line (len(codeLines) == 1)
		// 4. Combined length fits within 100 characters
		isInline := false
		if len(b.Outputs) == 1 && len(codeLines) == 1 {
			outStr := b.Outputs[0]
			trimmedOut := strings.TrimSpace(outStr)
			if strings.HasPrefix(trimmedOut, "➜") {
				formatted := formatOutputComment(outStr)
				if len(lines[startIdx])+2+len(formatted) <= 100 {
					isInline = true
				}
			}
		}

		if isInline {
			formatted := formatOutputComment(b.Outputs[0])
			lines[startIdx] = fmt.Sprintf("%s  %s", lines[startIdx], formatted)

			var newLines []string
			newLines = append(newLines, lines[:startIdx+1]...)
			newLines = append(newLines, lines[endIdx:]...)
			lines = newLines
		} else {
			var formattedOutputs []string
			for _, out := range b.Outputs {
				formattedOutputs = append(formattedOutputs, formatOutputComment(out))
			}

			var newBlock []string
			newBlock = append(newBlock, codeLines...)
			newBlock = append(newBlock, formattedOutputs...)

			var newLines []string
			newLines = append(newLines, lines[:startIdx]...)
			newLines = append(newLines, newBlock...)
			newLines = append(newLines, lines[endIdx:]...)

			lines = newLines
		}
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
	tmpFile, err := os.CreateTemp(dir, ".sc_tmp_*")
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

	// Flush to disk before renaming
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return err
	}

	if err := tmpFile.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, targetPath)
}
