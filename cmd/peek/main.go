package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"peek/internal/parser"
	"peek/internal/runner"
)

func printUsage() {
	fmt.Fprintf(os.Stderr, `peek: universal fast in-file scratchpad

Usage:
  peek <file:line>             Evaluate statement/block at line number (e.g. peek main.py:15)
  peek <file> <line>           Evaluate statement/block at line number (e.g. peek main.py 15)
  peek <file>                  Evaluate all statements top-to-bottom
  peek <file> --clean          Strip all comment outputs from file

Options:
  --clean                      Remove all scratchpad output comments
  --max-lines int              Max output lines per statement (default 30)
  --timeout int                Execution timeout in seconds (default 10)
  -h, --help                   Show this help message
`)
}

func parseArgs(args []string) (filePath string, lineNo *int, clean bool, maxLines int, timeout int, err error) {
	maxLines = 30
	timeout = 10
	var positional []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help":
			printUsage()
			os.Exit(0)
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
	if lastColon := strings.LastIndex(filePath, ":"); lastColon != -1 {
		if num, err := strconv.Atoi(filePath[lastColon+1:]); err == nil {
			filePath = filePath[:lastColon]
			lineNo = &num
		}
	}
	if lineNo == nil && len(positional) > 1 {
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
		fmt.Fprintf(os.Stderr, "peek error: invalid path: %v\n", err)
		os.Exit(1)
	}

	contentBytes, err := os.ReadFile(absPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "peek error: file not found: %s\n", filePath)
		os.Exit(1)
	}
	content := string(contentBytes)

	if clean {
		cleaned := parser.CleanOutputs(content)
		if cleaned == content {
			return
		}
		if err := parser.AtomicWrite(absPath, cleaned); err != nil {
			fmt.Fprintf(os.Stderr, "peek error writing file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Cleaned output comments from %s\n", filePath)
		return
	}

	pyRunner, err := runner.NewPythonRunner()
	if err != nil {
		fmt.Fprintf(os.Stderr, "peek error: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	result, err := pyRunner.Execute(ctx, absPath, content, lineNo, maxLines)
	if err != nil {
		fmt.Fprintf(os.Stderr, "peek runner error: %v\n", err)
		os.Exit(1)
	}

	if result.SyntaxError != nil {
		fmt.Fprintf(os.Stderr, "peek: %s\n", result.SyntaxError.Msg)
		os.Exit(1)
	}

	if result.Error != "" {
		fmt.Fprintf(os.Stderr, "peek error: %s\n", result.Error)
		os.Exit(1)
	}

	if len(result.Blocks) == 0 {
		return
	}

	updated, err := parser.ApplyBlockOutputs(content, result.Blocks)
	if err != nil {
		fmt.Fprintf(os.Stderr, "peek error updating output: %v\n", err)
		os.Exit(1)
	}

	if updated == content {
		return
	}

	if err := parser.AtomicWrite(absPath, updated); err != nil {
		fmt.Fprintf(os.Stderr, "peek error writing file: %v\n", err)
		os.Exit(1)
	}
}
