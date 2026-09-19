import ast
import contextlib
import io
import json
import sys
import traceback


def _is_output_line(line):
    s = line.strip()
    if not s.startswith("#"):
        return False
    rest = s[1:].strip()
    return any(rest.startswith(p) for p in ("=>", "➜", "❯", "✕", "…"))


def _format_outputs(std_lines, result_repr, error_lines, max_lines):
    raw = []
    for l in std_lines:
        raw.append(f"❯ {l}")
    if result_repr is not None:
        lines = result_repr.splitlines()
        if lines:
            raw.append(f"➜ {lines[0]}")
            for extra in lines[1:]:
                raw.append(f"  {extra}")
    for el in error_lines:
        raw.append(f"✕ {el}")
    if len(raw) > max_lines:
        raw = raw[:max_lines] + [f"… [truncated: {len(raw)-max_lines} lines hidden]"]
    return raw


def _extract_symbols(node):
    """Extract (defines: set, reads: set, is_side_effect: bool) for an AST statement node."""
    defines = set()
    reads = set()
    is_side_effect = False

    # 1. Imports
    if isinstance(node, ast.Import):
        for alias in node.names:
            name = alias.asname or alias.name.split(".")[0]
            defines.add(name)
        return defines, reads, is_side_effect

    if isinstance(node, ast.ImportFrom):
        for alias in node.names:
            if alias.name == "*":
                is_side_effect = True
            else:
                defines.add(alias.asname or alias.name)
        return defines, reads, is_side_effect

    # 2. Function definitions
    if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
        defines.add(node.name)
        for deco in node.decorator_list:
            for sub in ast.walk(deco):
                if isinstance(sub, ast.Name) and isinstance(sub.ctx, ast.Load):
                    reads.add(sub.id)
        local_names = set()
        args = node.args
        for arg in args.posonlyargs + args.args + args.kwonlyargs:
            local_names.add(arg.arg)
        if args.vararg:
            local_names.add(args.vararg.arg)
        if args.kwarg:
            local_names.add(args.kwarg.arg)

        explicit_globals = set()
        for sub in ast.walk(node):
            if isinstance(sub, ast.Global):
                explicit_globals.update(sub.names)
            elif isinstance(sub, ast.Name) and isinstance(sub.ctx, ast.Store):
                if sub.id not in explicit_globals:
                    local_names.add(sub.id)

        for sub in ast.walk(node):
            if isinstance(sub, ast.Name) and isinstance(sub.ctx, ast.Load):
                if sub.id not in local_names:
                    reads.add(sub.id)
        return defines, reads, is_side_effect

    # 3. Class definitions
    if isinstance(node, ast.ClassDef):
        defines.add(node.name)
        for base in node.bases:
            for sub in ast.walk(base):
                if isinstance(sub, ast.Name) and isinstance(sub.ctx, ast.Load):
                    reads.add(sub.id)
        for deco in node.decorator_list:
            for sub in ast.walk(deco):
                if isinstance(sub, ast.Name) and isinstance(sub.ctx, ast.Load):
                    reads.add(sub.id)
        for sub in ast.walk(node):
            if isinstance(sub, ast.Name) and isinstance(sub.ctx, ast.Load):
                reads.add(sub.id)
        return defines, reads, is_side_effect

    # 4. Assignments
    if isinstance(node, ast.Assign):
        for target in node.targets:
            for sub in ast.walk(target):
                if isinstance(sub, ast.Name):
                    defines.add(sub.id)
                    if isinstance(sub.ctx, ast.Load):
                        reads.add(sub.id)
        for sub in ast.walk(node.value):
            if isinstance(sub, ast.Name) and isinstance(sub.ctx, ast.Load):
                reads.add(sub.id)
        return defines, reads, is_side_effect

    if isinstance(node, ast.AnnAssign):
        for sub in ast.walk(node.target):
            if isinstance(sub, ast.Name):
                defines.add(sub.id)
        if node.value:
            for sub in ast.walk(node.value):
                if isinstance(sub, ast.Name) and isinstance(sub.ctx, ast.Load):
                    reads.add(sub.id)
        return defines, reads, is_side_effect

    if isinstance(node, ast.AugAssign):
        for sub in ast.walk(node.target):
            if isinstance(sub, ast.Name):
                defines.add(sub.id)
                reads.add(sub.id)
        for sub in ast.walk(node.value):
            if isinstance(sub, ast.Name) and isinstance(sub.ctx, ast.Load):
                reads.add(sub.id)
        return defines, reads, is_side_effect

    # 5. Method calls on objects (e.g. items.append(x) mutates items)
    if isinstance(node, ast.Expr) and isinstance(node.value, ast.Call):
        call = node.value
        if isinstance(call.func, ast.Attribute):
            curr = call.func.value
            while isinstance(curr, ast.Attribute):
                curr = curr.value
            if isinstance(curr, ast.Name):
                defines.add(curr.id)
                reads.add(curr.id)

    # 6. General fallback for expressions and other statements
    for sub in ast.walk(node):
        if isinstance(sub, ast.Name):
            if isinstance(sub.ctx, ast.Store):
                defines.add(sub.id)
            elif isinstance(sub.ctx, ast.Load):
                reads.add(sub.id)
        elif isinstance(sub, ast.Attribute) and sub.attr in ("path", "environ"):
            is_side_effect = True

    return defines, reads, is_side_effect


def _compute_dependencies(statements, target_idx):
    """Compute the transitive closure of statement indices needed to evaluate target_idx."""
    needed = {target_idx}
    unresolved_reads = set(statements[target_idx]["reads"])

    changed = True
    while changed:
        changed = False
        for idx in range(target_idx - 1, -1, -1):
            stmt = statements[idx]
            if stmt["is_side_effect"] and idx not in needed:
                needed.add(idx)
                unresolved_reads.update(stmt["reads"])
                changed = True
                continue

            if (stmt["defines"] & unresolved_reads) and idx not in needed:
                needed.add(idx)
                unresolved_reads.update(stmt["reads"])
                changed = True

    return sorted(needed)


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
        print(
            json.dumps({
                "syntax_error": {
                    "line": line,
                    "col": col,
                    "msg": f"SyntaxError at line {line}, col {col}: {err_msg}",
                },
                "blocks": [],
            })
        )
        return

    if not tree.body:
        print(json.dumps({"blocks": []}))
        return

    source_lines = source.splitlines()

    # Extract all top-level statements from AST with their symbols
    statements = []
    for idx, node in enumerate(tree.body):
        start_ln = node.lineno
        end_ln = node.end_lineno
        while end_ln < len(source_lines) and _is_output_line(source_lines[end_ln]):
            end_ln += 1

        defines, reads, is_side_fx = _extract_symbols(node)
        statements.append({
            "index": idx,
            "start_line": start_ln,
            "end_line": end_ln,
            "node": node,
            "defines": defines,
            "reads": reads,
            "is_side_effect": is_side_fx,
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

    # If target_line is specified, apply program slicing
    needed_indices = set(range(len(statements)))
    if target_idx is not None:
        needed_indices = set(_compute_dependencies(statements, target_idx))

    results = []

    for stmt in statements:
        if stmt["index"] not in needed_indices:
            continue

        exec_code, eval_code = compile_stmt(stmt["node"])

        # Silent upstream execution
        if target_idx is not None and stmt["index"] < target_idx:
            dummy_io = io.StringIO()
            with (
                contextlib.redirect_stdout(dummy_io),
                contextlib.redirect_stderr(dummy_io),
            ):
                try:
                    if exec_code:
                        exec(exec_code, scope)
                    if eval_code:
                        eval(eval_code, scope)
                except Exception:
                    exc_type, exc_val, _ = sys.exc_info()
                    err_line = traceback.format_exception_only(
                        exc_type, exc_val
                    )[-1].strip()
                    print(
                        json.dumps({
                            "error": (
                                f"Upstream statement at line {stmt['start_line']}"
                                f" error: {err_line}"
                            ),
                            "blocks": [],
                        })
                    )
                    return
            continue

        # Target statement (or sequential execution when target_idx is None)
        stdout_buf = io.StringIO()
        stderr_buf = io.StringIO()
        result_repr = None
        error_lines = []

        with (
            contextlib.redirect_stdout(stdout_buf),
            contextlib.redirect_stderr(stderr_buf),
        ):
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
                filtered = [
                    l for l in tb if not ("<string>" in l and "exec(" in l)
                ]
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
