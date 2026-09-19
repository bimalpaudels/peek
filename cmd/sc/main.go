package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"sc/internal/parser"
	"sc/internal/runner"
)

func printUsage() {
	fmt.Fprintf(os.Stderr, `sc: universal fast in-file scratchpad

Usage:
  sc <file:line>             Evaluate block at line number (e.g. sc main.py:15)
  sc <file> <line>           Evaluate block at line number (e.g. sc main.py 15)
  sc <file>                  Evaluate all blocks top-to-bottom
  sc <file> --clean          Strip all comment outputs from file

Options:
  --clean                    Remove all '# =>' output comments
  --max-lines int            Max output lines per block (default 30)
  --timeout int              Execution timeout in seconds (default 10)
`)
}

func parseArgs(args []string) (filePath string, lineNo *int, clean bool, maxLines int, timeout int, err error) {
	maxLines = 30
	timeout = 10
	var positional []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--clean":
			clean = true
		case "--max-lines":
			if i+1 < len(args) {
				i++
				maxLines, _ = strconv.Atoi(args[i])
			}
		case "--timeout":
			if i+1 < len(args) {
				i++
				timeout, _ = strconv.Atoi(args[i])
			}
		default:
			if !strings.HasPrefix(args[i], "--") {
				positional = append(positional, args[i])
			}
		}
	}

	if len(positional) == 0 {
		return "", nil, false, 0, 0, fmt.Errorf("no target file specified")
	}

	filePath = positional[0]
	if parts := strings.Split(filePath, ":"); len(parts) == 2 {
		if num, err := strconv.Atoi(parts[1]); err == nil {
			filePath = parts[0]
			lineNo = &num
		}
	} else if len(positional) > 1 {
		if num, err := strconv.Atoi(positional[1]); err == nil {
			lineNo = &num
		}
	}

	return filePath, lineNo, clean, maxLines, timeout, nil
}

func main() {
	filePath, lineNo, clean, maxLines, timeout, err := parseArgs(os.Args[1:])
	if err != nil {
		printUsage()
		os.Exit(1)
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sc error: invalid path: %v\n", err)
		os.Exit(1)
	}

	contentBytes, err := os.ReadFile(absPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sc error: file not found: %s\n", filePath)
		os.Exit(1)
	}
	content := string(contentBytes)

	if clean {
		cleaned := parser.CleanOutputs(content)
		if err := parser.AtomicWrite(absPath, cleaned); err != nil {
			fmt.Fprintf(os.Stderr, "sc error writing file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Cleaned output comments from %s\n", filePath)
		return
	}

	blocks := parser.ParseBlocks(content)
	if len(blocks) == 0 {
		fmt.Fprintf(os.Stderr, "sc: no code blocks found in %s\n", filePath)
		return
	}

	pyRunner, err := runner.NewPythonRunner()
	if err != nil {
		fmt.Fprintf(os.Stderr, "sc error: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	var targetBlocks []parser.Block
	if lineNo != nil {
		target := parser.FindBlockByLine(blocks, *lineNo)
		if target == nil {
			fmt.Fprintf(os.Stderr, "sc error: could not locate block for line %d\n", *lineNo)
			os.Exit(1)
		}
		targetBlocks = []parser.Block{*target}
	} else {
		targetBlocks = blocks
	}

	currentContent := content
	for _, b := range targetBlocks {
		var upstream []string
		for i := 0; i < b.Index; i++ {
			upstream = append(upstream, blocks[i].Code)
		}

		outputs, err := pyRunner.ExecuteBlocks(ctx, absPath, upstream, b.Code, maxLines)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sc runner error in block %d: %v\n", b.Index, err)
			os.Exit(1)
		}

		currentContent, err = parser.UpdateBlockOutput(currentContent, b.Index, outputs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sc error updating output: %v\n", err)
			os.Exit(1)
		}
	}

	if err := parser.AtomicWrite(absPath, currentContent); err != nil {
		fmt.Fprintf(os.Stderr, "sc error writing file: %v\n", err)
		os.Exit(1)
	}
}
