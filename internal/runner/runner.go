package runner

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"peek/internal/config"
)

// BlockResult represents the execution output and location of a single block.
type BlockResult struct {
	StartLine int      `json:"start_line"` // 1-indexed, inclusive
	EndLine   int      `json:"end_line"`   // 1-indexed, inclusive
	Outputs   []string `json:"outputs"`
}

// SyntaxErrorDetails captures file-level syntax errors.
type SyntaxErrorDetails struct {
	Line int    `json:"line"`
	Col  int    `json:"col"`
	Msg  string `json:"msg"`
}

// ExecutionResult is returned by the runner for a file execution run.
type ExecutionResult struct {
	Blocks      []BlockResult       `json:"blocks"`
	SyntaxError *SyntaxErrorDetails `json:"syntax_error,omitempty"`
	Error       string              `json:"error,omitempty"`
}

// Runner represents a language-specific execution engine.
type Runner interface {
	Execute(ctx context.Context, filePath string, source string, targetLine *int, maxLines int) (*ExecutionResult, error)
	CommentPrefix() string
}

// ForFile returns the appropriate language Runner for the target file path.
func ForFile(filePath string, cfg *config.Config) (Runner, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".py":
		uvPath := ""
		if cfg != nil {
			uvPath = cfg.UVPath
		}
		pyRunner, err := NewPythonRunner(uvPath)
		if err != nil {
			return nil, err
		}
		if cfg != nil {
			pyRunner.MaxStrLen = cfg.MaxStrLen
			pyRunner.PythonVersion = cfg.PythonVersion
		}
		return pyRunner, nil
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		bunPath := ""
		if cfg != nil {
			bunPath = cfg.BunPath
		}
		bunRunner, err := NewBunRunner(bunPath)
		if err != nil {
			return nil, err
		}
		if cfg != nil {
			bunRunner.MaxStrLen = cfg.MaxStrLen
		}
		return bunRunner, nil
	default:
		return nil, fmt.Errorf("unsupported file type: %q", ext)
	}
}

