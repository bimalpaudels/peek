package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"peek/internal/config"
	"peek/internal/mcp"
	"peek/internal/parser"
	"peek/internal/runner"
)

var Version = "dev"

func getFormattedVersion() string {
	if Version == "dev" {
		return "dev"
	}
	if strings.HasPrefix(Version, "v") {
		return Version
	}
	return "v" + Version
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `peek: universal fast in-file scratchpad (%s)

Usage:
  peek <file:line>             Evaluate statement/block at line number (e.g. peek main.py:15)
  peek <file> <line>           Evaluate statement/block at line number (e.g. peek main.py 15)
  peek <file>                  Evaluate all statements top-to-bottom
  peek <file> --clean          Strip all comment outputs from file
  peek mcp [--debug]           Start stdio MCP server for AI agents

Options:
  --clean                      Remove all scratchpad output comments
  --max-lines int              Max output lines per statement (default 30)
  --timeout int                Execution timeout in seconds (default 10)
  --config                     Print config file path (auto-creates if missing)
  -v, --version                Show version information
  -h, --help                   Show this help message
`, getFormattedVersion())
}

func parseArgs(args []string, userCfg ...*config.Config) (filePath string, lineNo *int, clean bool, maxLines int, timeout int, err error) {
	cfg := config.DefaultConfig()
	if len(userCfg) > 0 && userCfg[0] != nil {
		cfg = userCfg[0]
	}

	maxLines = cfg.MaxLines
	timeout = cfg.Timeout
	var positional []string

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "-h" || args[i] == "--help":
			printUsage()
			os.Exit(0)
		case args[i] == "-v" || args[i] == "--version":
			fmt.Printf("peek %s\n", getFormattedVersion())
			os.Exit(0)
		case args[i] == "--config":
			path, _ := config.EnsureConfigFile()
			fmt.Println(path)
			os.Exit(0)
		case args[i] == "--clean":
			clean = true
		case args[i] == "--max-lines":
			if i+1 < len(args) {
				i++
				if val, convErr := strconv.Atoi(args[i]); convErr == nil && val > 0 {
					maxLines = val
				}
			}
		case strings.HasPrefix(args[i], "--max-lines="):
			if val, convErr := strconv.Atoi(strings.TrimPrefix(args[i], "--max-lines=")); convErr == nil && val > 0 {
				maxLines = val
			}
		case args[i] == "--timeout":
			if i+1 < len(args) {
				i++
				if val, convErr := strconv.Atoi(args[i]); convErr == nil && val > 0 {
					timeout = val
				}
			}
		case strings.HasPrefix(args[i], "--timeout="):
			if val, convErr := strconv.Atoi(strings.TrimPrefix(args[i], "--timeout=")); convErr == nil && val > 0 {
				timeout = val
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
	cfg := config.Load()

	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		debug := false
		for _, arg := range os.Args[2:] {
			if arg == "--debug" {
				debug = true
			}
		}
		srv := mcp.NewServer(cfg, getFormattedVersion())
		if err := srv.Serve(context.Background(), debug); err != nil {
			fmt.Fprintf(os.Stderr, "peek mcp error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	filePath, lineNo, clean, maxLines, timeout, err := parseArgs(os.Args[1:], cfg)
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

	r, err := runner.ForFile(absPath, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "peek error: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	result, err := r.Execute(ctx, absPath, content, lineNo, maxLines)
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

	updated, err := parser.ApplyBlockOutputs(content, result.Blocks, r.CommentPrefix(), cfg.LineWidth)
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
