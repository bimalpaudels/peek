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
	UVPath        string
	PythonVersion string
	MaxStrLen     int
}

func NewPythonRunner(customUVPath ...string) (*PythonRunner, error) {
	var uvPath string
	if len(customUVPath) > 0 && strings.TrimSpace(customUVPath[0]) != "" {
		uvPath = strings.TrimSpace(customUVPath[0])
	} else {
		// Look for uv in PATH or standard user directories
		var err error
		uvPath, err = exec.LookPath("uv")
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
	}

	if uvPath == "" {
		return nil, fmt.Errorf("peek requires 'uv' to run Python, but 'uv' was not found on PATH.\nPlease install uv: curl -LsSf https://astral.sh/uv/install.sh | sh")
	}

	return &PythonRunner{
		UVPath:    uvPath,
		MaxStrLen: 140,
	}, nil
}

//go:embed harness.py
var pythonHarness string

type pythonPayload struct {
	FilePath   string `json:"file_path"`
	Source     string `json:"source"`
	TargetLine *int   `json:"target_line,omitempty"`
	MaxLines   int    `json:"max_lines"`
	MaxStrLen  int    `json:"max_str_len,omitempty"`
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

	maxStr := p.MaxStrLen
	if maxStr <= 0 {
		maxStr = 140
	}

	payloadBytes, err := json.Marshal(pythonPayload{
		FilePath:   absPath,
		Source:     source,
		TargetLine: targetLine,
		MaxLines:   maxLines,
		MaxStrLen:  maxStr,
	})
	if err != nil {
		return nil, err
	}

	harnessPath := getHarnessPath()
	var cmd *exec.Cmd
	args := []string{"run"}
	if p.PythonVersion != "" {
		args = append(args, "--python", p.PythonVersion)
	}
	args = append(args, "python")
	if harnessPath != "" {
		args = append(args, harnessPath)
	} else {
		args = append(args, "-c", pythonHarness)
	}
	cmd = exec.CommandContext(ctx, p.UVPath, args...)
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(payloadBytes)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("execution timed out")
		}
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

func getHarnessPath() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	peekDir := filepath.Join(cacheDir, "peek")
	if err := os.MkdirAll(peekDir, 0755); err != nil {
		return ""
	}
	harnessFile := filepath.Join(peekDir, "harness.py")

	// If file exists and matches embedded version, reuse it
	if data, err := os.ReadFile(harnessFile); err == nil && string(data) == pythonHarness {
		return harnessFile
	}

	if err := os.WriteFile(harnessFile, []byte(pythonHarness), 0644); err == nil {
		return harnessFile
	}
	return ""
}
