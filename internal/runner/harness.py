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
            defines.add(alias.asname or alias.name.split(".")[0])
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
        local_names = {
            arg.arg
            for arg in node.args.posonlyargs + node.args.args + node.args.kwonlyargs
        }
        if node.args.vararg:
            local_names.add(node.args.vararg.arg)
        if node.args.kwarg:
            local_names.add(node.args.kwarg.arg)

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
        for sub in ast.walk(node):
            if isinstance(sub, ast.Name) and isinstance(sub.ctx, ast.Load):
                reads.add(sub.id)
        return defines, reads, is_side_effect

    # 4. Method calls on objects (e.g. items.append(x) mutates items)
    if isinstance(node, ast.Expr) and isinstance(node.value, ast.Call):
        call = node.value
        if isinstance(call.func, ast.Attribute):
            curr = call.func.value
            while isinstance(curr, ast.Attribute):
                curr = curr.value
            if isinstance(curr, ast.Name):
                defines.add(curr.id)
                reads.add(curr.id)

    # 5. Augmented assignments (e.g. x += 1 loads target)
    if isinstance(node, ast.AugAssign) and isinstance(node.target, ast.Name):
        reads.add(node.target.id)

    # 6. General fallback for all assignments, expressions, and statements
    for sub in ast.walk(node):
        if isinstance(sub, ast.Name):
            if isinstance(sub.ctx, ast.Store):
                defines.add(sub.id)
            elif isinstance(sub.ctx, ast.Load):
                reads.add(sub.id)

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

    def execute_node(node):
        is_expr = isinstance(node, ast.Expr)
        code = compile(
            ast.Expression(body=node.value) if is_expr else ast.Module(body=[node], type_ignores=[]),
            filename=file_path,
            mode="eval" if is_expr else "exec",
        )
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf), contextlib.redirect_stderr(buf):
            try:
                val = eval(code, scope) if is_expr else exec(code, scope)
                return buf.getvalue(), val, None
            except Exception:
                return buf.getvalue(), None, sys.exc_info()

    target_idx = None
    if target_line is not None:
        for idx, stmt in enumerate(statements):
            if stmt["start_line"] <= target_line <= stmt["end_line"]:
                target_idx = idx
                break

        # If target_line lands on a blank line or comment: strict no-op
        if target_idx is None:
            print(json.dumps({"blocks": []}))
            return

        # If target statement is an inert declaration (FunctionDef, AsyncFunctionDef, ClassDef): strict no-op
        target_node = statements[target_idx]["node"]
        if isinstance(target_node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)):
            print(json.dumps({"blocks": []}))
            return

    # If target_line is specified, apply program slicing
    needed_indices = set(range(len(statements)))
    if target_idx is not None:
        needed_indices = set(_compute_dependencies(statements, target_idx))

    results = []

    for stmt in statements:
        if stmt["index"] not in needed_indices:
            continue

        captured_io, val, exc_info = execute_node(stmt["node"])

        # Silent upstream execution
        if target_idx is not None and stmt["index"] < target_idx:
            if exc_info:
                err_line = traceback.format_exception_only(exc_info[0], exc_info[1])[-1].strip()
                print(
                    json.dumps({
                        "error": f"Upstream statement at line {stmt['start_line']} error: {err_line}",
                        "blocks": [],
                    })
                )
                return
            continue

        result_repr = repr(val) if val is not None else None
        error_lines = []
        if exc_info:
            tb = traceback.format_exception(*exc_info)
            filtered = [
                l for l in tb if not ("harness.py" in l or ("<string>" in l and "exec(" in l))
            ]
            error_lines = "".join(filtered).strip().splitlines()

        all_std = captured_io.splitlines() if captured_io else []
        outputs = _format_outputs(all_std, result_repr, error_lines, max_lines)

        has_existing_outputs = stmt["end_line"] > stmt["node"].end_lineno
        if target_idx is not None or outputs or has_existing_outputs:
            results.append({
                "start_line": stmt["start_line"],
                "end_line": stmt["end_line"],
                "outputs": outputs,
            })
            if target_idx is not None:
                break

        if error_lines:
            break

    print(json.dumps({"blocks": results}))


if __name__ == "__main__":
    run()
