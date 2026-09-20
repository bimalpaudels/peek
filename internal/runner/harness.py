import ast
import asyncio
import contextlib
import dataclasses
import inspect
import io
import json
import os
import pprint
import sys
import traceback
import types

OUTPUT_PREFIXES = ("=>", "➜", "❯", "✕", "…")


def _is_output_line(line):
    s = line.strip()
    if not s.startswith("#"):
        return False
    rest = s[1:].strip()
    return any(rest.startswith(p) for p in OUTPUT_PREFIXES)


def _has_output_comment(line):
    s = line.strip()
    if not s:
        return False
    if s.startswith("#") and any(s[1:].strip().startswith(p) for p in OUTPUT_PREFIXES):
        return True
    if "#" in line:
        idx = line.rfind("#")
        comment = line[idx + 1 :].strip()
        if any(comment.startswith(p) for p in OUTPUT_PREFIXES):
            return True
    return False


def _get_root_name(node):
    curr = node
    while isinstance(curr, (ast.Attribute, ast.Subscript)):
        curr = curr.value
    if isinstance(curr, ast.Name):
        return curr.id
    return None


def _normalize_obj(obj, depth=0, max_depth=5):
    if depth > max_depth:
        return "…"

    # Pydantic v2
    if hasattr(obj, "model_dump") and callable(obj.model_dump):
        try:
            return _normalize_obj(obj.model_dump(), depth + 1, max_depth)
        except Exception:
            pass

    # Dataclasses
    if dataclasses.is_dataclass(obj) and not isinstance(obj, type):
        try:
            return _normalize_obj(dataclasses.asdict(obj), depth + 1, max_depth)
        except Exception:
            pass

    if isinstance(obj, dict):
        return {k: _normalize_obj(v, depth + 1, max_depth) for k, v in obj.items()}
    if isinstance(obj, (list, tuple)):
        converted = [_normalize_obj(item, depth + 1, max_depth) for item in obj]
        return tuple(converted) if isinstance(obj, tuple) else converted
    if isinstance(obj, set):
        return {_normalize_obj(item, depth + 1, max_depth) for item in obj}
    return obj


def _compact_val(v, max_str=140):
    if v is None or isinstance(v, (int, float, bool)):
        return repr(v)
    if isinstance(v, str):
        if len(v) > max_str:
            return repr(v[:max_str] + "…")
        return repr(v)
    if isinstance(v, dict):
        if not v:
            return "{}"
        if len(v) <= 4:
            natural = "{" + ", ".join(f"{repr(k)}: {repr(val)}" for k, val in v.items()) + "}"
            if len(natural) <= 100:
                return natural
        if len(v) <= 3:
            items = [f"{repr(k)}: {_compact_val(val, max_str=40)}" for k, val in v.items()]
            s = "{" + ", ".join(items) + "}"
            if len(s) <= 100:
                return s
        items = [f"{repr(k)}: {_compact_val(val, max_str=30)}" for k, val in list(v.items())[:2]]
        items.append(f"… ({len(v)} keys)")
        return "{" + ", ".join(items) + "}"
    if isinstance(v, (list, tuple, set)):
        if not v:
            return "[]" if isinstance(v, list) else ("()" if isinstance(v, tuple) else "set()")
        open_b, close_b = ("[", "]") if isinstance(v, list) else (("(", ")") if isinstance(v, tuple) else ("{", "}"))
        if len(v) <= 5:
            natural = open_b + ", ".join(repr(x) for x in v) + close_b
            if len(natural) <= 100:
                return natural
        if len(v) <= 2:
            s = open_b + ", ".join(_compact_val(x, max_str=50) for x in v) + close_b
            if len(s) <= 100:
                return s
        items = [_compact_val(x, max_str=35) for x in list(v)[:2]]
        items.append(f"… ({len(v)} items)")
        return open_b + ", ".join(items) + close_b
    return repr(v)


def _format_value(val, max_str=140):
    if val is None:
        return None

    normalized = _normalize_obj(val)
    rep = repr(normalized)

    # 1. If it fits on a single line (<= 80 chars), keep compact representation inline
    if len(rep) <= 80 and "\n" not in rep:
        return rep

    # 2. Top-level Dict: strictly 1 line per key
    if isinstance(normalized, dict):
        lines = ["{"]
        for k, v in normalized.items():
            lines.append(f"  {repr(k)}: {_compact_val(v, max_str=max_str)},")
        lines.append("}")
        return "\n".join(lines)

    # 3. Top-level List / Tuple / Set: 1 line per item
    if isinstance(normalized, (list, tuple, set)):
        open_b, close_b = ("[", "]") if isinstance(normalized, list) else (("(", ")") if isinstance(normalized, tuple) else ("{", "}"))
        lines = [open_b]
        for item in normalized:
            lines.append(f"  {_compact_val(item, max_str=max_str)},")
        lines.append(close_b)
        return "\n".join(lines)

    return rep


def _format_outputs(std_lines, result_repr, error_lines, max_lines):
    raw = []
    for l in std_lines:
        raw.append(f"❯ {l}")
    if result_repr is not None:
        for l in result_repr.splitlines():
            raw.append(f"➜ {l}")
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
            elif (
                isinstance(sub, ast.Name)
                and isinstance(sub.ctx, ast.Store)
                and sub.id not in explicit_globals
            ):
                local_names.add(sub.id)

        for sub in ast.walk(node):
            if (
                isinstance(sub, ast.Name)
                and isinstance(sub.ctx, ast.Load)
                and sub.id not in local_names
            ):
                reads.add(sub.id)
        return defines, reads, is_side_effect

    # 3. Class definitions
    if isinstance(node, ast.ClassDef):
        defines.add(node.name)
        for sub in ast.walk(node):
            if isinstance(sub, ast.Name) and isinstance(sub.ctx, ast.Load):
                reads.add(sub.id)
        return defines, reads, is_side_effect

    # 4. Detect mutations on objects and collections (e.g. arr[0] = 1, obj.attr = 2, matrix[i].append(x))
    for sub in ast.walk(node):
        if isinstance(sub, ast.Assign):
            for t in sub.targets:
                root = _get_root_name(t)
                if root:
                    defines.add(root)
                    reads.add(root)
        elif isinstance(sub, (ast.AugAssign, ast.AnnAssign)):
            root = _get_root_name(sub.target)
            if root:
                defines.add(root)
                reads.add(root)
        elif isinstance(sub, ast.Delete):
            for t in sub.targets:
                root = _get_root_name(t)
                if root:
                    defines.add(root)
                    reads.add(root)
        elif isinstance(sub, ast.Call) and isinstance(sub.func, ast.Attribute):
            root = _get_root_name(sub.func.value)
            if root:
                defines.add(root)
                reads.add(root)

    # 5. General fallback for all assignments, expressions, and statements
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
    except (json.JSONDecodeError, OSError, ValueError) as e:
        print(json.dumps({"error": f"Failed to parse payload: {e}"}))
        return

    file_path = payload.get("file_path") or "<scratchpad>"
    source = payload.get("source") or ""
    target_line = payload.get("target_line")
    max_lines = payload.get("max_lines") or 30
    max_str_len = payload.get("max_str_len") or 140

    if file_path and file_path != "<scratchpad>":
        file_dir = os.path.dirname(os.path.abspath(file_path))
        if file_dir and file_dir not in sys.path:
            sys.path.insert(0, file_dir)

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
        start_ln = getattr(node, "lineno", 1)
        end_ln = getattr(node, "end_lineno", None) or start_ln
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
        has_async = any(
            isinstance(sub, (ast.Await, ast.AsyncFor, ast.AsyncWith))
            for sub in ast.walk(node)
        )
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf), contextlib.redirect_stderr(buf):
            try:
                if has_async:
                    if is_expr:
                        assign_node = ast.Assign(
                            targets=[ast.Name(id="__peek_result__", ctx=ast.Store())],
                            value=node.value,
                        )
                        ast.fix_missing_locations(assign_node)
                        mod = ast.Module(body=[assign_node], type_ignores=[])
                    else:
                        mod = ast.Module(body=[node], type_ignores=[])

                    code = compile(
                        mod,
                        filename=file_path,
                        mode="exec",
                        flags=ast.PyCF_ALLOW_TOP_LEVEL_AWAIT,
                    )
                    if code.co_flags & inspect.CO_COROUTINE:
                        func = types.FunctionType(code, scope)
                        asyncio.run(func())
                    else:
                        exec(code, scope)  # noqa: S102
                    val = scope.pop("__peek_result__", None) if is_expr else None
                    return buf.getvalue(), val, None
                else:
                    code = compile(
                        ast.Expression(body=node.value) if is_expr else ast.Module(body=[node], type_ignores=[]),
                        filename=file_path,
                        mode="eval" if is_expr else "exec",
                    )
                    val = eval(code, scope) if is_expr else exec(code, scope)  # noqa: S102
                    if is_expr and inspect.iscoroutine(val):
                        val = asyncio.run(val)
                    return buf.getvalue(), val, None
            except (Exception, SystemExit):  # noqa: BLE001
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

        result_repr = _format_value(val, max_str=max_str_len)
        error_lines = []
        if exc_info:
            tb = traceback.format_exception(*exc_info)
            filtered = []
            skip = False
            for l in tb:
                if any(x in l for x in ("harness.py", "asyncio/runners.py", "asyncio/base_events.py")) or (
                    "<string>" in l and "exec(" in l
                ):
                    skip = True
                    continue
                if skip and l.startswith("    "):
                    continue
                skip = False
                filtered.append(l)
            error_lines = "".join(filtered).strip().splitlines()

        all_std = captured_io.splitlines() if captured_io else []
        outputs = _format_outputs(all_std, result_repr, error_lines, max_lines)

        orig_end = getattr(stmt["node"], "end_lineno", None) or stmt["start_line"]
        has_existing_outputs = stmt["end_line"] > orig_end or any(
            _has_output_comment(source_lines[ln - 1])
            for ln in range(stmt["start_line"], stmt["end_line"] + 1)
            if ln - 1 < len(source_lines)
        )
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
