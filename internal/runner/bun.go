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

// BunRunner executes TypeScript and JavaScript code strictly using `bun run`.
type BunRunner struct {
	BunPath   string
	MaxStrLen int
}

func NewBunRunner(customBunPath ...string) (*BunRunner, error) {
	var bunPath string
	if len(customBunPath) > 0 && strings.TrimSpace(customBunPath[0]) != "" {
		bunPath = strings.TrimSpace(customBunPath[0])
	} else {
		// Look for bun in PATH or standard user directories
		var err error
		bunPath, err = exec.LookPath("bun")
		if err != nil {
			home, _ := os.UserHomeDir()
			candidates := []string{
				filepath.Join(home, ".bun", "bin", "bun"),
				filepath.Join(home, ".local", "bin", "bun"),
				"/opt/homebrew/bin/bun",
				"/usr/local/bin/bun",
			}
			for _, c := range candidates {
				if fi, err := os.Stat(c); err == nil && fi.Mode()&0111 != 0 {
					bunPath = c
					break
				}
			}
		}
	}

	if bunPath == "" {
		return nil, fmt.Errorf("peek requires 'bun' to run TypeScript/JavaScript, but 'bun' was not found on PATH.\nPlease install bun: curl -fsSL https://bun.sh/install | bash")
	}

	return &BunRunner{
		BunPath:   bunPath,
		MaxStrLen: 140,
	}, nil
}

func (b *BunRunner) CommentPrefix() string {
	return "//"
}

//go:embed harness.js
var bunHarness string

type bunPayload struct {
	FilePath   string `json:"file_path"`
	Source     string `json:"source"`
	TargetLine *int   `json:"target_line,omitempty"`
	MaxLines   int    `json:"max_lines"`
	MaxStrLen  int    `json:"max_str_len,omitempty"`
}

func (b *BunRunner) Execute(
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

	maxStr := b.MaxStrLen
	if maxStr <= 0 {
		maxStr = 140
	}

	payloadBytes, err := json.Marshal(bunPayload{
		FilePath:   absPath,
		Source:     source,
		TargetLine: targetLine,
		MaxLines:   maxLines,
		MaxStrLen:  maxStr,
	})
	if err != nil {
		return nil, err
	}

	harnessPath := getBunHarnessPath()
	var cmd *exec.Cmd
	if harnessPath != "" {
		cmd = exec.CommandContext(ctx, b.BunPath, "run", harnessPath)
	} else {
		cmd = exec.CommandContext(ctx, b.BunPath, "run", "-e", bunHarness)
	}
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
		return nil, fmt.Errorf("bun execution failed: %v\n%s", err, errOutput)
	}

	var resp ExecutionResult
	if err := json.Unmarshal(stdoutBuf.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse runner output: %v (raw: %s)", err, stdoutBuf.String())
	}

	return &resp, nil
}

func getBunHarnessPath() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	peekDir := filepath.Join(cacheDir, "peek")
	if err := os.MkdirAll(peekDir, 0755); err != nil {
		return ""
	}
	harnessFile := filepath.Join(peekDir, "harness.js")

	// If file exists and matches embedded version, reuse it
	if data, err := os.ReadFile(harnessFile); err == nil && string(data) == bunHarness {
		return harnessFile
	}

	if err := os.WriteFile(harnessFile, []byte(bunHarness), 0644); err == nil {
		return harnessFile
	}
	return ""
}
