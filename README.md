# peek

> A fast, minimal in-file scratchpad for Python & TypeScript.

`peek` runs Python and TypeScript/JavaScript code directly inside your source files and writes the output right next to your code as comments (`x  # ➜ 10` or `x  // ➜ 10`). It also includes an MCP server so that an AI agent can easily evaluate and debug with minimal tokens and without polluting the context.

No notebooks. No terminal REPLs or executions.

---

## Prerequisites

* **Python**: [`uv`](https://github.com/astral-sh/uv) executes Python slices blazingly fast with zero environment headaches:
  ```bash
  # macOS / Linux
  curl -LsSf https://astral.sh/uv/install.sh | sh

  # Or via Homebrew
  brew install uv
  ```

* **TypeScript / JavaScript**: [`bun`](https://bun.sh) powers TypeScript, JavaScript, JSX, and TSX execution natively:
  ```bash
  # macOS / Linux
  curl -fsSL https://bun.sh/install | bash

  # Or via Homebrew
  brew install oven-sh/bun/bun
  ```

## Installation

Install the pre-compiled binary for macOS or Linux directly to `~/.local/bin/peek`:

```bash
curl -fsSL https://raw.githubusercontent.com/bimalpaudels/peep/main/install.sh | sh
```

*(To install to a system directory like `/usr/local/bin`, run `curl -fsSL https://raw.githubusercontent.com/bimalpaudels/peep/main/install.sh | INSTALL_DIR=/usr/local/bin sh`)*

> Building from source? See the [Development](#development) section below.

## Editor Integrations

`peek` works directly from your terminal, but the intended experience is binding it to a single shortcut in your editor (e.g., `Shift + Enter` to run the current line, and `Cmd + Shift + C` to clean the file).

### VS Code / Cursor / Windsurf / Antigravity

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
    "when": "editorTextFocus && (editorLangId == python || editorLangId == typescript || editorLangId == javascript || editorLangId == typescriptreact || editorLangId == javascriptreact)"
  },
  {
    "key": "cmd+shift+c",
    "command": "workbench.action.tasks.runTask",
    "args": "Peek: Clean File",
    "when": "editorTextFocus && (editorLangId == python || editorLangId == typescript || editorLangId == javascript || editorLangId == typescriptreact || editorLangId == javascriptreact)"
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
    "context": "Editor && (language == Python || language == TypeScript || language == JavaScript || extension == py || extension == ts || extension == tsx || extension == js || extension == jsx)",
    "bindings": {
      "shift-enter": ["task::Spawn", { "task_name": "Peek: Run Current Line" }],
      "cmd-shift-c": ["task::Spawn", { "task_name": "Peek: Clean File" }]
    }
  }
]
```

### Other Editors (Neovim, Emacs, etc.)

Because `peek` is a standard CLI taking `file:line` arguments, it can be wired into any editor or terminal environment with custom keymaps or task runners.

---

## Model Context Protocol (MCP) Server

`peek` includes a built-in [Model Context Protocol](https://modelcontextprotocol.io) server over standard I/O for AI agents.

> [!WARNING]
> **Active Code Execution**: `peek` is **not a read-only tool**. It executes code slices on your host system with your user permissions and environment. Be mindful when granting MCP tool access to autonomous agents.

```bash
peek mcp
# Or with debug logging to stderr:
peek mcp --debug
```

### Adding to AI Clients

Add Peek to your agent's MCP configuration (e.g. `claude_desktop_config.json` or `.cursor/mcp.json`):

```json
{
  "mcpServers": {
    "peek": {
      "command": "peek",
      "args": ["mcp"]
    }
  }
}
```

### Tools & Resources

* **`peek_slice`**: Evaluates a statement or expression at a target line in any Python or TypeScript file or project using AST dependency slicing without modifying files on disk.
  * `file` (string, required): Relative or absolute path to the target file.
  * `line` (int, required): 1-indexed target line number to evaluate.
  * `timeout` (int, optional): Execution timeout in seconds.
  * `max_lines` (int, optional): Maximum output lines per block.
* **`peek://environment`**: Reports runtime interpreter status (`uv` and `bun` presence and paths).
* **`peek://config`**: Exposes active execution settings.

---

## Usage

1. **Evaluate at cursor**: Place your cursor on any line in your `.py`, `.ts`, `.tsx`, `.js`, or `.jsx` file and press `Shift + Enter`. `peek` slices the code, runs required upstream dependencies, and writes the output right into your file.
2. **Clean up**: When you're done tinkering or ready to commit, press `Cmd + Shift + C` to wipe all generated comments clean.

---

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

In TypeScript / JavaScript files, comments format with double slashes:

```typescript
const count = 42;
count // ➜ 42

console.log("Processing batch...");
// ❯ Processing batch...
```

### Formatted Dictionaries & Collections

Larger dictionaries, objects, and collections automatically format 1 line per key/item beneath the statement:

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
> - **Python**:
>   - [`01_basics.py`](./examples/01_basics.py): Expressions, stdout, errors, dict & list formatting
>   - [`02_slicing.py`](./examples/02_slicing.py): Dependency tree-shaking & object mutations
>   - [`03_async_showcase.py`](./examples/03_async_showcase.py): Top-level await, auto-await, shared event loops
>   - [`04_errors_and_tracebacks.py`](./examples/04_errors_and_tracebacks.py): Traceback snippets & exception handling
>   - [`05_pipeline.py`](./examples/05_pipeline.py): Multi-stage data pipelines
>   - [`06_algorithms_and_tricks.py`](./examples/06_algorithms_and_tricks.py): Dataclasses, algorithms & comprehensions
> - **TypeScript & React**:
>   - [`01_ts_basics.ts`](./examples/01_ts_basics.ts): TypeScript expressions, async, objects, and console output
>   - [`02_ts_slicing_and_mutations.ts`](./examples/02_ts_slicing_and_mutations.ts): AST tree-shaking & object mutations in TS
>   - [`03_ts_async_and_streams.ts`](./examples/03_ts_async_and_streams.ts): Top-level await and async streams
>   - [`04_ts_errors_and_tracebacks.ts`](./examples/04_ts_errors_and_tracebacks.ts): Stack traces and error reporting
>   - [`07_react_tsx.tsx`](./examples/07_react_tsx.tsx): Direct React component & JSX inspection
>   - [`08_tsx_data_table.tsx`](./examples/08_tsx_data_table.tsx): Data tables and complex TSX structures
> - **Benchmarks**:
>   - [`09_agent_benchmark.py`](./examples/09_agent_benchmark.py): AI agent benchmark suite

---

## CLI Usage

You can run `peek` directly from your terminal:

```bash
# Evaluate statement/block at cursor line 21 (both formats supported)
peek solution.py:21
peek solution.ts 21

# Evaluate all statements in the file sequentially
peek solution.py

# Strip all scratchpad output comments
peek solution.py --clean

# Custom output line limit (default 30) or timeout (default 10s)
peek solution.py:21 --max-lines 50 --timeout 5

# Start the MCP server for AI assistants
peek mcp
```

---

## Development

### Project Structure

```text
├── cmd/peek/           # CLI entry point, flag parsing, command routing
├── internal/
│   ├── config/         # Execution configurations
│   ├── mcp/            # Stdio Model Context Protocol (MCP) server for AI agents
│   ├── parser/         # Comment detection, formatting, atomic file I/O
│   └── runner/         # Execution engines (uv/Python, bun/TypeScript)
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

---

## Features

### What's there
* **Inline values & clean blocks**: Expressions sit inline (`# ➜ 10` or `// ➜ 10`), while stdout (`# ❯`) and errors (`# ✕`) sit neatly below.
* **Smart dependency slicing**: Uses AST data-flow analysis to run only what your target line actually needs. Formatter tweaks (`ruff`, `prettier`) or empty lines won't break it.
* **Multi-language support**: First-class runners for Python (via `uv`) and TypeScript/JavaScript/TSX/JSX (via `bun`).
* **AI agent ready (MCP)**: Built-in Model Context Protocol server exposing `peek_slice` for agentic workflows without file pollution.
* **Stateless execution (no zombie kernels)**: Runs in-memory and exits cleanly. No background daemons, no stale variables mutating between runs, no `/tmp` sockets.
* **First-class async support**:
  * Top-level `await` without wrapper functions (`res = await client.get(...)`).
  * Auto-awaiting for coroutines, tasks, and `asyncio.gather(...)`.
  * Shared event loop across file execution.
* **Instant cleanup**: Run `peek file.py --clean` and every comment disappears.
* **Editor agnostic**: Works anywhere you can map a keybinding to a shell command.

### What's coming
* **More features**: Features targetted for AI agents that can potentially improve dev workflow.
* **Native IDE Extensions**: For even tighter editor integration.
* **Config file**: While there exists a config file now, it isn't strongly integrated yet.

---

## Why was this made?

I was practicing some simple algorithms with IPython, but I kept making stupid mistakes in a nested loop. Correcting them became a chore—re-running snippets, copying lines back and forth, or dealing with stale state in the terminal.

That's when I went looking for an "in-file" REPL. But I definitely wasn't looking for Jupyter notebooks or anything that heavy. That's where the idea came from.

The plan is to sit right between quick terminal REPLs and full-blown notebook engines: fast feedback right inside your editor with zero extra ceremony.

## Who is this for?

* **Algorithm practice & LeetCode**: Prototype your logic line-by-line, inspect intermediate lists or dicts inline, catch off-by-one errors immediately, and strip all outputs with one keybind before submitting.
* **Backend & Data scripts**: Quickly iterate on data transformations, regex, SQL/ORM queries, Pydantic models, or async pipelines without booting a server or creating throwaway test files.
* **Scratching an itch**: When you just want to know *"what does this snippet actually evaluate to right now?"* without leaving your editor.

## Limitations

* **Not a Jupyter alternative**: If you work on massive datasets and need interactive charts, scatter plots, or 100-column dataframes, stick with Jupyter. `peek` is for writing and sanity-checking code, not authoring data science reports.
* **Not a REPL replacement**: IPython or standard terminal shells remain better for quick, throwaway 1-liners where you don't even want to touch a file.
* **Not a test runner**: `peek` is a scratchpad to keep you in the flow while coding. It's not an assertion framework to replace `pytest` in your CI pipeline.
* **Unassigned side effects**: Dependency slicing follows AST data flow. Standalone setup calls with no inputs or outputs (like a void `await init_db()`) won't be picked up when targeting a specific downstream line unless you assign it (`_ = await init_db()`) or run the whole file (`peek file.py`).
