package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
				commit(blockStartIdx, i, currentChunk)
				currentChunk = nil
				inBlock = false
			}
		} else {
			if !inBlock {
				inBlock = true
				blockStartIdx = i
			}
			currentChunk = append(currentChunk, l)
		}
	}

	if inBlock && len(currentChunk) > 0 {
		commit(blockStartIdx, len(lines), currentChunk)
	}

	return blocks
}

func splitCodeAndOutput(lines []string) ([]string, []string) {
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}

	start := end
	for start > 0 && strings.HasPrefix(strings.TrimSpace(lines[start-1]), outputMarkerPrefix) {
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

// UpdateBlockOutput replaces the output comments of the given block.
func UpdateBlockOutput(content string, blockIndex int, newOutputs []string) (string, error) {
	blocks := ParseBlocks(content)
	if blockIndex < 0 || blockIndex >= len(blocks) {
		return "", fmt.Errorf("block index %d out of range (total %d blocks)", blockIndex, len(blocks))
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
		if !strings.HasPrefix(out, outputMarkerPrefix) {
			formattedOutputs = append(formattedOutputs, fmt.Sprintf("%s %s", outputMarkerPrefix, out))
		} else {
			formattedOutputs = append(formattedOutputs, out)
		}
	}

	var allLines []string
	allLines = append(allLines, lines[:target.StartLine-1]...)
	allLines = append(allLines, blockLines[:bodyOffset]...)
	allLines = append(allLines, codeLines...)
	allLines = append(allLines, formattedOutputs...)
	allLines = append(allLines, lines[target.EndLine:]...)

	return strings.Join(allLines, "\n"), nil
}

// CleanOutputs removes all lines starting with '# =>'.
func CleanOutputs(content string) string {
	lines := strings.Split(content, "\n")
	var kept []string
	for _, l := range lines {
		if !strings.HasPrefix(strings.TrimSpace(l), outputMarkerPrefix) {
			kept = append(kept, l)
		}
	}
	return strings.Join(kept, "\n")
}

// AtomicWrite writes content safely using a temporary file in the same directory.
func AtomicWrite(filePath string, content string) error {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return err
	}
	dir := filepath.Dir(absPath)

	tmpFile, err := os.CreateTemp(dir, ".sc_tmp_*")
	if err != nil {
		return err
	}
	tmpName := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()

	if _, err := tmpFile.WriteString(content); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, absPath)
}
