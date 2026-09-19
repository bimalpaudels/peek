# peek: Universal Fast In-File Scratchpad (Go Binary)

`peek` is an ultra-fast, stateless in-file scratchpad engine written in **Go**. It lets you write algorithms, functions, and logic in standard `.py` files, evaluate statements directly on demand, and inject outputs as clean, managed comments (such as inline `x  # ➜ 10` or stdout blocks) with zero friction.

Under the hood, Python statement parsing, dependency slicing, and execution are dispatched strictly through **`uv`** via an embedded native AST runner.

---

## Why this architecture?

1. **Standalone Compiled Binary**: Written in Go with zero external runtime dependencies. Instant startup and minimal memory footprint.
2. **Native AST Statement Tracking & Program Slicing**: Parses code using Python's native `ast` module. Targeting a statement automatically computes its dependency graph, running only required upstream code and bypassing unrelated lines. Immune to arbitrary blank lines or code formatters (`black`, `ruff`).
3. **Stateless & Deterministic**: Preceding required dependencies run silently in memory to populate scope, while target statements execute with expression and stdout capture. No zombie daemons, no stale mutated lists, no socket files in `/tmp`.
4. **Smart Inline & Block Output Comments**: Single-line expression results are cleanly placed inline (e.g. `y  # ➜ [10, 20]`), while printed stdout and errors are placed beneath the statement (`# ❯ stdout`, `# ✕ error`).
5. **Clean Codebase**: A single `peek file.py --clean` command strips all output comments (both inline and multi-line), leaving pristine production code ready for git or LeetCode.
6. **Decoupled from IDEs**: Works directly in any terminal and easily integrates with VS Code, Neovim, or tmux keybindings.

---

## Installation & Build

Build and install using `make`:

```bash
# 1. Build minimal binary (with stripped debug symbols)
make build

# 2. Install to ~/.local/bin (or specify INSTALL_DIR=/usr/local/bin)
make install

# 3. Update (rebuild + reinstall)
make update

# 4. Uninstall from system
make uninstall
```

Or build manually:
```bash
go build -ldflags="-s -w" -o peek ./cmd/peek
```

---

## Usage

```bash
# 1. Evaluate statement/block at cursor line 21
peek solution.py:21

# Or with line number as separate argument
peek solution.py 21

# 2. Evaluate all statements in file sequentially
peek solution.py

# 3. Strip all scratchpad output comments
peek solution.py --clean

# 4. Custom output line limit (default 30) or timeout (default 10s)
peek solution.py:21 --max-lines 50 --timeout 5
```

---

## Example

Write standard Python code in `solution.py`:

```python
# Setup
x = [1, 2, 3, 4]
y = [i * 10 for i in x]
y

# Algorithm
def two_sum(nums, target):
    seen = {}
    for i, n in enumerate(nums):
        diff = target - n
        if diff in seen:
            return [seen[diff], i]
        seen[n] = i

two_sum([2, 7, 11, 15], 9)
```

Run:
```bash
peek solution.py
```

Your file updates automatically:

```python
# Setup
x = [1, 2, 3, 4]
y = [i * 10 for i in x]
y  # ➜ [10, 20, 30, 40]

# Algorithm
def two_sum(nums, target):
    seen = {}
    for i, n in enumerate(nums):
        diff = target - n
        if diff in seen:
            return [seen[diff], i]
        seen[n] = i

two_sum([2, 7, 11, 15], 9)  # ➜ [0, 1]
```

If a statement prints to stdout or raises an exception:

```python
print(f"Total: {sum(y)}")
# ❯ Total: 100

1 / 0
# ✕ ZeroDivisionError: division by zero
```

When you're finished and want to commit or paste into LeetCode:

```bash
peek solution.py --clean
```
All scratchpad comments vanish instantly.

---

## Project Structure

```text
├── cmd/peek/           # CLI entry point, flag parsing, command routing
├── internal/
│   ├── parser/         # Output comment detection, formatting, and atomic file I/O
│   └── runner/         # Execution engine (uv dispatch, embedded harness.py)
└── Makefile            # Build, test, and install targets
```
