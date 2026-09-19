package runner

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// PythonRunner executes Python code strictly using `uv run python`.
type PythonRunner struct {
	UVPath string
}

func NewPythonRunner() (*PythonRunner, error) {
	// Look for uv in PATH or standard user directories
	uvPath, err := exec.LookPath("uv")
	if err != nil {
		home, _ := os.UserHomeDir()
		candidates := []string{
			filepath.Join(home, ".local", "bin", "uv"),
			filepath.Join(home, ".cargo", "bin", "uv"),
			"/opt/homebrew/bin/uv",
			"/usr/local/bin/uv",
		}
		for _, c := range candidates {
			if fi, err := os.Stat(c); err == nil && fi.Mode()&0111 != 0 {
				uvPath = c
				break
			}
		}
	}

	if uvPath == "" {
		return nil, fmt.Errorf("sc requires 'uv' to run Python, but 'uv' was not found on PATH.\nPlease install uv: curl -LsSf https://astral.sh/uv/install.sh | sh")
	}

	return &PythonRunner{UVPath: uvPath}, nil
}

//go:embed harness.py
var pythonHarness string

type pythonPayload struct {
	FilePath   string `json:"file_path"`
	Source     string `json:"source"`
	TargetLine *int   `json:"target_line,omitempty"`
	MaxLines   int    `json:"max_lines"`
}

func (p *PythonRunner) Execute(
	ctx context.Context,
	filePath string,
	source string,
	targetLine *int,
	maxLines int,
) (*ExecutionResult, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(absPath)

	payloadBytes, err := json.Marshal(pythonPayload{
		FilePath:   absPath,
		Source:     source,
		TargetLine: targetLine,
		MaxLines:   maxLines,
	})
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, p.UVPath, "run", "python", "-c", pythonHarness)
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(payloadBytes)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		errOutput := stderrBuf.String()
		if strings.TrimSpace(errOutput) == "" {
			errOutput = stdoutBuf.String()
		}
		return nil, fmt.Errorf("uv execution failed: %v\n%s", err, errOutput)
	}

	var resp ExecutionResult
	if err := json.Unmarshal(stdoutBuf.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse runner output: %v (raw: %s)", err, stdoutBuf.String())
	}

	return &resp, nil
}
