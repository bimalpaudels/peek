# peek

> A fast, minimal in-file scratchpad for Python.

`peek` runs Python code directly inside your `.py` files and writes the output right next to your code as comments (`x  # ➜ 10`).

No notebooks. No copy-pasting into terminal REPLs. Just write standard Python, evaluate a line or the whole file, see what happened right there, and wipe the comments clean whenever you're done.

Under the hood, it's a single Go binary that uses Python's native AST and `uv` to figure out dependencies, run only what's needed, and get out of your way.

---

## Why was this made?

I was practicing some simple algorithms with IPython, but I kept making stupid mistakes in a nested loop. Correcting them became a chore—re-running snippets, copying lines back and forth, or dealing with stale state in the terminal.

That's when I went looking for an "in-file" REPL. But I definitely wasn't looking for Jupyter notebooks or anything that heavy. That's where the idea came from.

The plan is to sit right between quick terminal REPLs and full-blown notebook engines. Fast feedback right in your editor, zero extra ceremony.

---

## Who is this for?

- **Algorithm practice & LeetCode**: Prototype your logic line-by-line, inspect intermediate lists or dicts inline, catch off-by-one errors immediately, and strip all outputs with one keybind before submitting.
- **Backend & Data scripts**: Quickly iterate on data transformations, regex, SQL/ORM queries, Pydantic models, or async pipelines without booting a server or creating throwaway test files.
- **Scratching an itch**: When you just want to know *"what does this snippet actually evaluate to right now?"* without leaving your editor.

---

## Who is this NOT for?

- If you work on large datasets and are looking for a Jupyter alternative with graphs and tables, this is not for you.
- This is not a REPL replacement either. IPython or standard terminal shells remain better for quick interactive command-line experimentation.
- `peek` is an interactive scratchpad for developer flow, not an automated test runner to replace `pytest` in CI pipelines.

---

## Why is it built like this?

- **Single compiled binary (Go)**: Instant startup, zero dependencies to install or maintain `peek` itself. Plus, building it in Go leaves plenty of room down the road to orchestrate runners for other languages besides Python.
- **Smart dependency slicing**: If you run line 40, `peek` uses Python's native `ast` to figure out and run only what line 40 actually depends on. Formatter changes (`ruff`, `black`) or empty lines won't throw it off.
- **Stateless (no zombie kernels)**: Runs in-memory and exits cleanly. No lingering background daemons, no stale variables mutating between runs, and no mysterious socket files left in `/tmp`.
- **Inline & clean output**: Expression values sit right at the end of the line (`# ➜ 10`). Printed stdout (`# ❯`) and errors (`# ✕`) sit neatly underneath.
- **One-command cleanup**: Run `peek file.py --clean` and every generated comment disappears. Your code stays clean and git-ready.
- **Editor agnostic**: It's just a CLI tool. You don't need a dedicated plugin ecosystem—just map a keybinding in Zed, Neovim, or VS Code to run the command.

---

## Prerequisites

- **uv** (Required runtime for fast, stateless Python execution):
  ```bash
  # Install uv (macOS / Linux)
  curl -LsSf https://astral.sh/uv/install.sh | sh
  # Or via Homebrew
  brew install uv
  ```
- **Go 1.22+** (Only needed if building from source)

---

## Installation

### Option 1: Quick Install (`curl | sh`)

Install the latest pre-compiled binary for macOS or Linux directly to `~/.local/bin/peek`:

```bash
curl -fsSL https://raw.githubusercontent.com/bimalpaudels/peep/main/install.sh | sh
```

*(To install to a system directory like `/usr/local/bin`: `curl -fsSL https://raw.githubusercontent.com/bimalpaudels/peep/main/install.sh | INSTALL_DIR=/usr/local/bin sh`)*

---

### Option 2: Build from Source (requires Go 1.22+)

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

Say you're working in `solution.py`:

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

Run `peek solution.py`:

```bash
peek solution.py
```

Your file updates right in place:

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

If a statement prints or raises an exception, `peek` drops it right underneath:

```python
print(f"Total: {sum(y)}")
# ❯ Total: 100

1 / 0
# ✕ ZeroDivisionError: division by zero
```

When you're done tinkering and ready to commit or paste into LeetCode:

```bash
peek solution.py --clean
```

All scratchpad comments vanish. Clean code, no residue.

---

## Async Support (Top-Level Await)

You don't need `asyncio.run(...)` or wrapper boilerplate. Async works right out of the box:

- **Top-level `await`**: Await coroutines directly:
  ```python
  res = await client.get("/items")
  res.json()  # ➜ {"items": [1, 2, 3]}
  ```
- **Auto-awaiting**: If an expression returns a coroutine, task, or `asyncio.gather(...)`, `peek` awaits it for you automatically:
  ```python
  asyncio.gather(fetch(1), fetch(2))  # ➜ [10, 20]
  ```
- **Shared event loop**: A single event loop stays alive across the run, so things like `asyncio.Queue`, locks, or persistent client sessions won't break across lines.
- **A quick note on side effects**: Dependency slicing tracks variables. If you have a standalone setup call with no inputs/outputs (like `await init_db()`), assign it to a throwaway variable (`_ = await init_db()`) or evaluate the entire file (`peek file.py`).

---

## Editor Integration

`peek` works directly from your terminal, but the intended experience is binding it to a single shortcut in your editor (e.g. `Shift + Enter` to run the current line, and `Cmd + Shift + C` to clean the file).

### Zed

Add to `~/.config/zed/tasks.json`:
```json
[
  {
    "label": "Peek: Run Current Line",
    "command": "peek",
    "args": ["$ZED_FILE:$ZED_ROW"],
    "save": "current",
    "shell": { "program": "sh" },
    "reveal": "never",
    "hide": "always"
  },
  {
    "label": "Peek: Clean File",
    "command": "peek",
    "args": ["$ZED_FILE", "--clean"],
    "save": "current",
    "shell": { "program": "sh" },
    "reveal": "never",
    "hide": "always"
  }
]
```

Add to `~/.config/zed/keymap.json`:
```json
[
  {
    "context": "Editor && (language == Python || extension == py)",
    "bindings": {
      "shift-enter": ["task::Spawn", { "task_name": "Peek: Run Current Line" }],
      "cmd-shift-c": ["task::Spawn", { "task_name": "Peek: Clean File" }]
    }
  }
]
```

---

### Neovim

Add to your `init.lua`:
```lua
-- Evaluate line at cursor (saves file, runs peek, reloads buffer)
vim.keymap.set('n', '<leader>x', function()
  local file = vim.fn.expand('%:p')
  local line = vim.fn.line('.')
  vim.cmd('write')
  vim.fn.system(string.format('peek "%s:%d"', file, line))
  vim.cmd('edit!')
end, { desc = "Peek: Evaluate line at cursor" })

-- Clean scratchpad output comments
vim.keymap.set('n', '<leader>xc', function()
  local file = vim.fn.expand('%:p')
  vim.cmd('write')
  vim.fn.system(string.format('peek "%s" --clean', file))
  vim.cmd('edit!')
end, { desc = "Peek: Clean comments" })
```

---

### VS Code

Add to `.vscode/tasks.json` (or User Tasks):
```json
{
  "version": "2.0.0",
  "tasks": [
    {
      "label": "Peek: Run Current Line",
      "type": "shell",
      "command": "peek \"${file}:${lineNumber}\"",
      "presentation": { "reveal": "never", "close": true }
    },
    {
      "label": "Peek: Clean File",
      "type": "shell",
      "command": "peek \"${file}\" --clean",
      "presentation": { "reveal": "never", "close": true }
    }
  ]
}
```

Add to `keybindings.json`:
```json
[
  {
    "key": "shift+enter",
    "command": "workbench.action.tasks.runTask",
    "args": "Peek: Run Current Line",
    "when": "editorTextFocus && editorLangId == python"
  }
]
```

---

## Project Structure

```text
├── cmd/peek/           # CLI entry point, flag parsing, command routing
├── internal/
│   ├── parser/         # Output comment detection, formatting, and atomic file I/O
│   └── runner/         # Execution engine (uv dispatch, embedded harness.py)
└── Makefile            # Build, test, and install targets
```
