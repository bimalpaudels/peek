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

// Inline Python harness that parses files via Python's native AST
// and executes at the statement level without any blank-line heuristics.
const pythonHarness = `
import ast, contextlib, io, json, sys, traceback

def _format_outputs(std_lines, result_repr, error_lines, max_lines):
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
    return raw

def run():
    try:
        payload = json.loads(sys.stdin.read())
    except Exception as e:
        print(json.dumps({"error": f"Failed to parse payload: {e}"}))
        return

    file_path = payload.get("file_path") or "<scratchpad>"
    source = payload.get("source") or ""
    target_line = payload.get("target_line")
    max_lines = payload.get("max_lines") or 30

    if not source.strip():
        print(json.dumps({"blocks": []}))
        return

    try:
        tree = ast.parse(source, filename=file_path)
    except SyntaxError as e:
        err_msg = e.msg or "syntax error"
        line = e.lineno or 1
        col = e.offset or 1
        print(json.dumps({
            "syntax_error": {"line": line, "col": col, "msg": f"SyntaxError at line {line}, col {col}: {err_msg}"},
            "blocks": []
        }))
        return

    if not tree.body:
        print(json.dumps({"blocks": []}))
        return

    source_lines = source.splitlines()

    # Extract all top-level statements from AST
    statements = []
    for idx, node in enumerate(tree.body):
        start_ln = node.lineno
        end_ln = node.end_lineno
        # Include any existing '# =>' comments directly following this statement
        while end_ln < len(source_lines) and source_lines[end_ln].strip().startswith("# =>"):
            end_ln += 1

        statements.append({
            "index": idx,
            "start_line": start_ln,
            "end_line": end_ln,
            "node": node,
        })

    scope = {
        "__name__": "__main__",
        "__file__": file_path,
        "__doc__": None,
        "__builtins__": __builtins__,
    }

    def compile_stmt(node):
        if isinstance(node, ast.Expr):
            eval_mod = ast.Expression(body=node.value)
            return None, compile(eval_mod, filename=file_path, mode="eval")
        else:
            exec_mod = ast.Module(body=[node], type_ignores=[])
            return compile(exec_mod, filename=file_path, mode="exec"), None

    target_idx = None
    if target_line is not None:
        target_idx = len(statements) - 1
        for idx, stmt in enumerate(statements):
            if stmt["start_line"] <= target_line <= stmt["end_line"]:
                target_idx = idx
                break
            if stmt["start_line"] > target_line:
                target_idx = max(0, idx - 1)
                break

    results = []

    for stmt in statements:
        exec_code, eval_code = compile_stmt(stmt["node"])

        # Silent upstream execution
        if target_idx is not None and stmt["index"] < target_idx:
            dummy_io = io.StringIO()
            with contextlib.redirect_stdout(dummy_io), contextlib.redirect_stderr(dummy_io):
                try:
                    if exec_code:
                        exec(exec_code, scope)
                    if eval_code:
                        eval(eval_code, scope)
                except Exception:
                    exc_type, exc_val, _ = sys.exc_info()
                    err_line = traceback.format_exception_only(exc_type, exc_val)[-1].strip()
                    print(json.dumps({
                        "error": f"Upstream statement at line {stmt['start_line']} error: {err_line}",
                        "blocks": []
                    }))
                    return
            continue

        stdout_buf = io.StringIO()
        stderr_buf = io.StringIO()
        result_repr = None
        error_lines = []

        with contextlib.redirect_stdout(stdout_buf), contextlib.redirect_stderr(stderr_buf):
            try:
                if exec_code:
                    exec(exec_code, scope)
                if eval_code:
                    val = eval(eval_code, scope)
                    if val is not None:
                        result_repr = repr(val)
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

        outputs = _format_outputs(all_std, result_repr, error_lines, max_lines)

        has_existing_outputs = False
        if stmt["end_line"] > stmt["node"].end_lineno:
            has_existing_outputs = True

        if target_idx is not None:
            results.append({
                "index": stmt["index"],
                "start_line": stmt["start_line"],
                "end_line": stmt["end_line"],
                "outputs": outputs,
            })
            break
        else:
            if outputs or has_existing_outputs:
                results.append({
                    "index": stmt["index"],
                    "start_line": stmt["start_line"],
                    "end_line": stmt["end_line"],
                    "outputs": outputs,
                })

        if error_lines:
            break

    print(json.dumps({"blocks": results}))

if __name__ == "__main__":
    run()
`

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
