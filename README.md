# peek

> A fast, minimal in-file scratchpad for Python.

`peek` runs Python code directly inside your `.py` files and writes the output right next to your code as comments (`x  # ➜ 10`).

No notebooks. No terminal REPLs or executions. Just write standard Python, evaluate a line, see what happened right there, and wipe the comments whenever you're done.

---

## Prerequisites

`peek` uses [`uv`](https://github.com/astral-sh/uv) to execute Python slices blazingly fast with zero environment headaches:

```bash
# macOS / Linux
curl -LsSf https://astral.sh/uv/install.sh | sh

# Or via Homebrew
brew install uv
```

## Installation

Install the pre-compiled binary for macOS or Linux directly to `~/.local/bin/peek`:

```bash
curl -fsSL https://raw.githubusercontent.com/bimalpaudels/peep/main/install.sh | sh
```

*(To install to a system directory like `/usr/local/bin`, run `curl -fsSL https://raw.githubusercontent.com/bimalpaudels/peep/main/install.sh | INSTALL_DIR=/usr/local/bin sh`)*

> Building from source? See the [Development](#development) section below.

## Editor Integrations

`peek` works directly from your terminal, but I guess that defeats the whole purpose of it. The intended experience is binding it to a single shortcut in your editor (e.g., `Shift + Enter` to run the current line, and `Cmd + Shift + C` to clean the file).

### VS Code

Add to `.vscode/tasks.json` (or your User Tasks):
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
  },
  {
    "key": "cmd+shift+c",
    "command": "workbench.action.tasks.runTask",
    "args": "Peek: Clean File",
    "when": "editorTextFocus && editorLangId == python"
  }
]
```

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

### Other Editors

- **Cursor, Windsurf, Antigravity, etc.**: Since these are based on VS Code, the exact same `.vscode/tasks.json` and `keybindings.json` setup above should work with minimal changes. Although that hasn't been tested.
- **Neovim, Emacs, etc.**: Because `peek` is just a standard CLI taking `file:line` arguments, it can be wired into any editor or terminal environment with custom keymaps or task runners. Configurations for these haven't been officially tested yet—if you have a setup you love, PRs and configs are welcome!

## Usage

1. **Evaluate at cursor**: Place your cursor on any line in your `.py` file and press `Shift + Enter`. `peek` slices the code, runs required upstream dependencies, and writes the output right into your file.
2. **Clean up**: When you're done tinkering or ready to commit, press `Cmd + Shift + C` to wipe all generated comments clean.

## Examples

### Inline vs. Block Output

Single-line values fit inline, while printed stdout and exceptions drop directly underneath:

```python
# Inline expression
x = 10 * 42
x  # ➜ 420

# Captured stdout
print("Fetching records...")
# ❯ Fetching records...

# Errors & exceptions
1 / 0
# ✕ ZeroDivisionError: division by zero
```

### Formatted Dictionaries & Collections

Larger dictionaries and collections automatically format 1 line per key/item beneath the statement:

```python
user = {
    "id": 101,
    "name": "Alex",
    "roles": ["admin", "developer"],
    "active": True,
}
user
# ➜ {
# ➜   'id': 101,
# ➜   'name': 'Alex',
# ➜   'roles': ['admin', 'developer'],
# ➜   'active': True,
# ➜ }
```

### Async & Top-Level Await

No wrapper functions needed. Top-level `await` works directly, and coroutine expressions or `asyncio.gather(...)` auto-resolve:

```python
import asyncio

async def fetch_val(n):
    return n * 10

await fetch_val(5)  # ➜ 50
asyncio.gather(fetch_val(1), fetch_val(2))  # ➜ [10, 20]
```

### Dependency Slicing

Targeting a line runs only what that line needs, skipping unrelated or heavy setup:

```python
# Expensive setup (automatically skipped!)
data = load_large_dataset()

# Target line 5 with your cursor
y = 10 * 42
y  # ➜ 420 (runs instantly; line 2 is never executed)
```

> 💡 **Looking for more?** See the [`examples/`](./examples) directory for complete scripts:
> - [`01_basics.py`](./examples/01_basics.py): Expressions, stdout, errors, dict & list formatting
> - [`02_slicing.py`](./examples/02_slicing.py): Dependency tree-shaking & object mutations
> - [`03_async_showcase.py`](./examples/03_async_showcase.py): Top-level await, auto-await, shared event loops
> - [`04_errors_and_tracebacks.py`](./examples/04_errors_and_tracebacks.py): Traceback snippets & exception handling
> - [`05_pipeline.py`](./examples/05_pipeline.py): Multi-stage data pipelines
> - [`06_algorithms_and_tricks.py`](./examples/06_algorithms_and_tricks.py): Dataclasses, algorithms & comprehensions

## CLI Usage

You can also run `peek` directly from your terminal:

```bash
# Evaluate statement/block at cursor line 21
peek solution.py:21

# Evaluate all statements in the file sequentially
peek solution.py

# Strip all scratchpad output comments
peek solution.py --clean

# Custom output line limit (default 30) or timeout (default 10s)
peek solution.py:21 --max-lines 50 --timeout 5
```

## Development

### Project Structure

```text
├── cmd/peek/           # CLI entry point, flag parsing, command routing
├── internal/
│   ├── config/         # CLI flag and execution configurations
│   ├── parser/         # Comment detection, formatting, atomic file I/O
│   └── runner/         # Execution engine (uv dispatch, embedded harness.py)
├── examples/           # Ready-to-run showcase scripts
└── Makefile            # Build, test, and install targets
```

### Building from Source (Go 1.22+)

```bash
# Build binary
make build

# Install to ~/.local/bin (or specify INSTALL_DIR=/usr/local/bin)
make install

# Run tests
go test ./...
```

## Features

### What's there
- **Inline values & clean blocks**: Expressions sit inline (`# ➜ 10`), while stdout (`# ❯`) and errors (`# ✕`) sit neatly below.
- **Smart dependency slicing**: Uses Python's native `ast` to run only what your target line actually needs. Formatter tweaks (`ruff`, `black`) or empty lines won't break it.
- **Stateless execution (no zombie kernels)**: Runs in-memory and exits cleanly. No background daemons, no stale variables mutating between runs, no `/tmp` sockets.
- **First-class async support**:
  - Top-level `await` without wrapper functions (`res = await client.get(...)`).
  - Auto-awaiting for coroutines, tasks, and `asyncio.gather(...)`.
  - Shared event loop across the file execution for queues, locks, and persistent sessions.
- **Instant cleanup**: Run `peek file.py --clean` and every comment disappears.
- **Editor agnostic**: Works anywhere you can map a keybinding to a shell command.

### What's coming
- **Multi-language runners**: Built in Go so the core orchestrator can support runners for TypeScript and Go
- **As IDE-Extensions**: For easier integration to the workflow.
- **Config file**: While there exists a config file now, it isn't strongly integrated yet.

## Why was this made?

I was practicing some simple algorithms with IPython, but I kept making stupid mistakes in a nested loop. Correcting them became a chore—re-running snippets, copying lines back and forth, or dealing with stale state in the terminal.

That's when I went looking for an "in-file" REPL. But I definitely wasn't looking for Jupyter notebooks or anything that heavy. That's where the idea came from.

The plan is to sit right between quick terminal REPLs and full-blown notebook engines: fast feedback right inside your editor with zero extra ceremony.

## Who is this for?

- **Algorithm practice & LeetCode**: Prototype your logic line-by-line, inspect intermediate lists or dicts inline, catch off-by-one errors immediately, and strip all outputs with one keybind before submitting.
- **Backend & Data scripts**: Quickly iterate on data transformations, regex, SQL/ORM queries, Pydantic models, or async pipelines without booting a server or creating throwaway test files.
- **Scratching an itch**: When you just want to know *"what does this snippet actually evaluate to right now?"* without leaving your editor.

## Limitations

- **Not a Jupyter alternative**: If you work on massive datasets and need interactive charts, scatter plots, or 100-column dataframes, stick with Jupyter. `peek` is for writing and sanity-checking code, not authoring data science reports.
- **Not a REPL replacement**: IPython or standard terminal shells remain better for quick, throwaway 1-liners where you don't even want to touch a file.
- **Not a test runner**: `peek` is a scratchpad to keep you in the flow while coding. It's not an assertion framework to replace `pytest` in your CI pipeline.
- **Unassigned side effects**: Dependency slicing follows AST data flow. Standalone setup calls with no inputs or outputs (like a void `await init_db()`) won't be picked up when targeting a specific downstream line unless you assign it (`_ = await init_db()`) or run the whole file (`peek file.py`).
