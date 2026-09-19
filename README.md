# sc: Universal Fast In-File Scratchpad (Go Binary)

`sc` is an ultra-fast, stateless in-file scratchpad engine written in **Go**. It lets you write algorithms, functions, and logic in standard `.py` files, evaluate blocks directly on demand, and inject outputs as managed `# =>` comments with zero friction.

Under the hood, Python execution is dispatched strictly through **`uv`**.

---

## Why this architecture?

1. **Standalone Compiled Binary**: Written in Go with zero runtime dependencies. Instant startup and minimal memory footprint.
2. **Stateless & Deterministic**: When evaluating block $K$, blocks $0 \dots K-1$ run silently in memory to populate the scope, and block $K$ executes with expression and stdout capture. No zombie daemons, no stale mutated lists, no socket files in `/tmp`.
3. **Natural Python**: No `# %%` markers or special syntax required. Blocks are separated by standard blank lines.
4. **Clean Codebase**: All outputs are managed `# =>` comments. A single `--clean` command strips them all, leaving pristine production code.
5. **Decoupled from IDEs**: Works directly in any terminal. Ready to be wrapped by a tiny VS Code or Neovim extension whenever you want.

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
go build -ldflags="-s -w" -o sc ./cmd/sc
```

---

## Usage

```bash
# 1. Evaluate block at cursor line 21
sc solution.py:21

# Or with line number as separate argument
sc solution.py 21

# 2. Evaluate all blocks in file sequentially
sc solution.py

# 3. Strip all '# =>' output comments
sc solution.py --clean

# 4. Custom output line limit (default 30)
sc solution.py:21 --max-lines 50
```

---

## Example

Write standard Python code in `solution.py`:

```python
# Block 1: Setup
x = [1, 2, 3, 4]
y = [i * 10 for i in x]
y

# Block 2: Algorithm
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
sc solution.py:15
```

Your file updates automatically:

```python
# Block 1: Setup
x = [1, 2, 3, 4]
y = [i * 10 for i in x]
y

# Block 2: Algorithm
def two_sum(nums, target):
    seen = {}
    for i, n in enumerate(nums):
        diff = target - n
        if diff in seen:
            return [seen[diff], i]
        seen[n] = i

two_sum([2, 7, 11, 15], 9)
# => Out: [0, 1]
```

When you're finished and want to paste into LeetCode:

```bash
sc solution.py --clean
```
All `# =>` comments vanish instantly.
