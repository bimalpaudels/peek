package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"sc/internal/runner"
)

var (
	explicitMarkerRegex = regexp.MustCompile(`^\s*#\s*%%`)
	outputMarkerPrefix  = "# =>"
)

// Block represents a single logical cell/block of code in a file.
type Block struct {
	Index       int
	StartLine   int // 1-indexed, inclusive
	EndLine     int // 1-indexed, inclusive
	Code        string
	OutputLines []string
}

// ParseBlocks parses file content into blocks.
// If explicit '# %%' markers exist, splits by markers.
// Otherwise, splits by one or more blank lines.
func ParseBlocks(content string) []Block {
	rawLines := strings.Split(content, "\n")
	if len(rawLines) == 0 {
		return nil
	}

	hasExplicit := false
	for _, l := range rawLines {
		if explicitMarkerRegex.MatchString(l) {
			hasExplicit = true
			break
		}
	}

	if hasExplicit {
		return parseExplicit(rawLines)
	}
	return parseBlankLines(rawLines)
}

func parseExplicit(lines []string) []Block {
	var markerIndices []int
	for idx, l := range lines {
		if explicitMarkerRegex.MatchString(l) {
			markerIndices = append(markerIndices, idx)
		}
	}
	if len(markerIndices) == 0 || markerIndices[0] != 0 {
		markerIndices = append([]int{0}, markerIndices...)
	}

	var blocks []Block
	num := len(markerIndices)
	for i := 0; i < num; i++ {
		startIdx := markerIndices[i]
		endIdx := len(lines)
		if i+1 < num {
			endIdx = markerIndices[i+1]
		}

		chunk := lines[startIdx:endIdx]
		body := chunk
		if len(chunk) > 0 && explicitMarkerRegex.MatchString(chunk[0]) {
			body = chunk[1:]
		}

		codeLines, outLines := splitCodeAndOutput(body)
		blocks = append(blocks, Block{
			Index:       i,
			StartLine:   startIdx + 1,
			EndLine:     endIdx,
			Code:        strings.Join(codeLines, "\n"),
			OutputLines: outLines,
		})
	}
	return blocks
}

func parseBlankLines(lines []string) []Block {
	var blocks []Block
	inBlock := false
	blockStartIdx := 0
	var currentChunk []string

	inQuote := ""
	bracketDepth := 0

	commit := func(start, end int, chunk []string) {
		if len(chunk) == 0 {
			return
		}
		codeLines, outLines := splitCodeAndOutput(chunk)
		hasCode := false
		for _, l := range codeLines {
			if strings.TrimSpace(l) != "" {
				hasCode = true
				break
			}
		}
		if hasCode {
			blocks = append(blocks, Block{
				Index:       len(blocks),
				StartLine:   start + 1,
				EndLine:     end,
				Code:        strings.Join(codeLines, "\n"),
				OutputLines: outLines,
			})
		}
	}

	for i, l := range lines {
		isBlank := strings.TrimSpace(l) == ""

		if isBlank {
			if inBlock {
				isSplit := false
				if inQuote == "" && bracketDepth == 0 {
					next, found := nextNonBlankLine(lines, i+1)
					if !found || (!isIndented(next) && !isCompoundContinuation(next)) {
						isSplit = true
					}
				}

				if isSplit {
					commit(blockStartIdx, i, currentChunk)
					currentChunk = nil
					inBlock = false
					inQuote = ""
					bracketDepth = 0
				} else {
					// Blank line within an indented body, bracket, or multi-line string
					currentChunk = append(currentChunk, l)
				}
			}
		} else {
			if !inBlock {
				inBlock = true
				blockStartIdx = i
				inQuote = ""
				bracketDepth = 0
			}
			currentChunk = append(currentChunk, l)
			inQuote, bracketDepth = scanPythonLine(l, inQuote, bracketDepth)
		}
	}

	if inBlock && len(currentChunk) > 0 {
		commit(blockStartIdx, len(lines), currentChunk)
	}

	return blocks
}

func isIndented(line string) bool {
	trimmed := strings.TrimRight(line, "\r\n")
	if strings.TrimSpace(trimmed) == "" {
		return false
	}
	return trimmed[0] == ' ' || trimmed[0] == '\t'
}

func isCompoundContinuation(line string) bool {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "except") ||
		strings.HasPrefix(trimmed, "finally") ||
		strings.HasPrefix(trimmed, "else:") ||
		strings.HasPrefix(trimmed, "elif ") ||
		strings.HasPrefix(trimmed, "elif(") {
		return true
	}
	return false
}

func nextNonBlankLine(lines []string, start int) (string, bool) {
	for i := start; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "" && !isOutputComment(trimmed) {
			return lines[i], true
		}
	}
	return "", false
}

func scanPythonLine(line string, inQuote string, bracketDepth int) (string, int) {
	i := 0
	n := len(line)

	if inQuote != "" {
		for i < n {
			if strings.HasPrefix(line[i:], inQuote) {
				backslashes := 0
				for k := i - 1; k >= 0 && line[k] == '\\'; k-- {
					backslashes++
				}
				if backslashes%2 == 0 {
					qLen := len(inQuote)
					inQuote = ""
					i += qLen
					break
				}
			}
			i++
		}
	}

	for i < n {
		ch := line[i]

		if ch == '#' {
			break
		}

		if i+3 <= n && (line[i:i+3] == `"""` || line[i:i+3] == `'''`) {
			q := line[i : i+3]
			i += 3
			closed := false
			for i < n {
				if strings.HasPrefix(line[i:], q) {
					backslashes := 0
					for k := i - 1; k >= 0 && line[k] == '\\'; k-- {
						backslashes++
					}
					if backslashes%2 == 0 {
						i += 3
						closed = true
						break
					}
				}
				i++
			}
			if !closed {
				inQuote = q
				break
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

		switch ch {
		case '(', '[', '{':
			bracketDepth++
		case ')', ']', '}':
			if bracketDepth > 0 {
				bracketDepth--
			}
		}
		i++
	}

	return inQuote, bracketDepth
}

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

func formatOutputComment(out string) string {
	cleanOut := strings.TrimRight(out, "\r\n")
	if strings.HasPrefix(cleanOut, "#") {
		return cleanOut
	}
	trimmed := strings.TrimSpace(cleanOut)
	if strings.HasPrefix(trimmed, "➜") ||
		strings.HasPrefix(trimmed, "❯") ||
		strings.HasPrefix(trimmed, "✕") ||
		strings.HasPrefix(trimmed, "…") {
		return fmt.Sprintf("# %s", cleanOut)
	}
	if strings.HasPrefix(trimmed, "=>") {
		return fmt.Sprintf("# %s", cleanOut)
	}
	return fmt.Sprintf("# => %s", cleanOut)
}

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

// FindBlockByLine finds the block containing lineNo (1-indexed).
// If lineNo lands on a blank line, it selects the preceding block.
func FindBlockByLine(blocks []Block, lineNo int) *Block {
	if len(blocks) == 0 {
		return nil
	}
	for i := range blocks {
		if blocks[i].StartLine <= lineNo && lineNo <= blocks[i].EndLine {
			return &blocks[i]
		}
	}

	var prev *Block = &blocks[0]
	for i := range blocks {
		if blocks[i].StartLine > lineNo {
			return prev
		}
		prev = &blocks[i]
	}
	return &blocks[len(blocks)-1]
}

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

// UpdateBlockOutput replaces the output comments of the given block.
func UpdateBlockOutput(content string, blockIndex int, newOutputs []string) (string, error) {
	blocks := ParseBlocks(content)
	if blockIndex < 0 || blockIndex >= len(blocks) {
		return "", fmt.Errorf("block index %d out of range (total %d blocks)", blockIndex, len(blocks))
	}

	eol := "\n"
	if strings.Contains(content, "\r\n") {
		eol = "\r\n"
	}

	lines := strings.Split(content, "\n")
	target := blocks[blockIndex]

	bodyOffset := 0
	blockLines := lines[target.StartLine-1 : target.EndLine]
	if len(blockLines) > 0 && explicitMarkerRegex.MatchString(blockLines[0]) {
		bodyOffset = 1
	}

	codeLines, _ := splitCodeAndOutput(blockLines[bodyOffset:])

	var formattedOutputs []string
	for _, out := range newOutputs {
		formattedOutputs = append(formattedOutputs, formatOutputComment(out))
	}

	var allLines []string
	for _, l := range lines[:target.StartLine-1] {
		allLines = append(allLines, strings.TrimRight(l, "\r"))
	}
	for _, l := range blockLines[:bodyOffset] {
		allLines = append(allLines, strings.TrimRight(l, "\r"))
	}
	for _, l := range codeLines {
		allLines = append(allLines, strings.TrimRight(l, "\r"))
	}
	allLines = append(allLines, formattedOutputs...)
	for _, l := range lines[target.EndLine:] {
		allLines = append(allLines, strings.TrimRight(l, "\r"))
	}

	return strings.Join(allLines, eol), nil
}

// CleanOutputs removes all managed output comment lines (both inline and new-line).
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
