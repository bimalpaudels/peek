package runner

import (
	"context"
)

// BlockResult represents the execution output and location of a single block.
type BlockResult struct {
	Index     int      `json:"index"`
	StartLine int      `json:"start_line"` // 1-indexed, inclusive
	EndLine   int      `json:"end_line"`   // 1-indexed, inclusive
	Outputs   []string `json:"outputs"`
	Error     string   `json:"error,omitempty"`
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
}

