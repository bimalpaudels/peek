package runner

import (
	"bytes"
	"context"
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

// Inline Python harness that compiles code with AST splitting:
// exec() upstream blocks and body, eval() trailing expression, capture stdout/stderr.
const pythonHarness = `
import ast, contextlib, io, json, sys, traceback

def run():
    payload = json.loads(sys.stdin.read())
    file_path = payload.get("file_path") or "<scratchpad>"
    upstream = payload.get("upstream") or []
    target = payload.get("target") or ""
    max_lines = payload.get("max_lines") or 30

    scope = {
        "__name__": "__main__",
        "__file__": file_path,
        "__doc__": None,
        "__builtins__": __builtins__,
    }

    # 1. Execute upstream blocks silently
    dummy_io = io.StringIO()
    with contextlib.redirect_stdout(dummy_io), contextlib.redirect_stderr(dummy_io):
        for i, code in enumerate(upstream):
            if not code.strip():
                continue
            try:
                exec(compile(code, filename=file_path, mode="exec"), scope)
            except Exception:
                exc_type, exc_val, _ = sys.exc_info()
                err_line = traceback.format_exception_only(exc_type, exc_val)[-1].strip()
                sys.stdout.write(json.dumps({"error": f"Upstream block {i} error: {err_line}", "outputs": []}) + "\n")
                return

    # 2. Execute target block with capture
    if not target.strip():
        print(json.dumps({"outputs": []}))
        return

    stdout_buf = io.StringIO()
    stderr_buf = io.StringIO()
    result_repr = None
    error_lines = []

    try:
        tree = ast.parse(target, filename=file_path)
    except SyntaxError:
        err = traceback.format_exc().strip().splitlines()
        error_lines = [f"SyntaxError: {err[-1]}"] if err else ["SyntaxError"]
        _respond([], None, error_lines, max_lines)
        return

    if not tree.body:
        print(json.dumps({"outputs": []}))
        return

    last_expr = None
    if isinstance(tree.body[-1], ast.Expr):
        last_expr = tree.body.pop()

    exec_code = None
    eval_code = None

    try:
        if tree.body:
            exec_code = compile(tree, filename=file_path, mode="exec")
        if last_expr is not None:
            eval_expr = ast.Expression(body=last_expr.value)
            eval_code = compile(eval_expr, filename=file_path, mode="eval")
    except Exception:
        err = traceback.format_exc().strip().splitlines()
        error_lines = [f"CompileError: {err[-1]}"]
        _respond([], None, error_lines, max_lines)
        return

    with contextlib.redirect_stdout(stdout_buf), contextlib.redirect_stderr(stderr_buf):
        try:
            if exec_code:
                exec(exec_code, scope)
            if eval_code:
                res = eval(eval_code, scope)
                if res is not None:
                    result_repr = repr(res)
        except Exception:
            exc_type, exc_val, exc_tb = sys.exc_info()
            tb = traceback.format_exception(exc_type, exc_val, exc_tb)
            filtered = [l for l in tb if not ("<string>" in l and "exec(" in l)]
            error_lines = "".join(filtered).strip().splitlines()

    captured_out = stdout_buf.getvalue()
    captured_err = stderr_buf.getvalue()

    all_std = []
    if captured_out:
        all_std.extend(captured_out.splitlines())
    if captured_err:
        all_std.extend(captured_err.splitlines())

    _respond(all_std, result_repr, error_lines, max_lines)

def _respond(std_lines, result_repr, error_lines, max_lines):
    raw = list(std_lines)
    if result_repr is not None:
        lines = result_repr.splitlines()
        if lines:
            raw.append(f"Out: {lines[0]}")
            for extra in lines[1:]:
                raw.append(f"     {extra}")
    for el in error_lines:
        raw.append(f"Error: {el}")

    if len(raw) > max_lines:
        raw = raw[:max_lines] + [f"... [truncated: {len(raw)-max_lines} lines hidden]"]

    print(json.dumps({"outputs": raw}))

if __name__ == "__main__":
    run()
`

type harnessPayload struct {
	FilePath string   `json:"file_path"`
	Upstream []string `json:"upstream"`
	Target   string   `json:"target"`
	MaxLines int      `json:"max_lines"`
}

type harnessResponse struct {
	Outputs []string `json:"outputs"`
	Error   string   `json:"error,omitempty"`
}

func (p *PythonRunner) ExecuteBlocks(
	ctx context.Context,
	filePath string,
	upstreamBlocks []string,
	targetBlock string,
	maxLines int,
) ([]string, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(absPath)

	if upstreamBlocks == nil {
		upstreamBlocks = []string{}
	}

	payloadBytes, err := json.Marshal(harnessPayload{
		FilePath: absPath,
		Upstream: upstreamBlocks,
		Target:   targetBlock,
		MaxLines: maxLines,
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

	var resp harnessResponse
	if err := json.Unmarshal(stdoutBuf.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse runner output: %v (raw: %s)", err, stdoutBuf.String())
	}

	if resp.Error != "" {
		return []string{resp.Error}, nil
	}

	return resp.Outputs, nil
}
