package runner

import (
	"context"
	"strings"
	"testing"
	"time"
)

func runTest(t *testing.T, source string, targetLine *int) *ExecutionResult {
	t.Helper()
	r, err := NewPythonRunner()
	if err != nil {
		t.Skipf("skipping python runner test: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := r.Execute(ctx, "test.py", source, targetLine, 30)
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") || strings.Contains(err.Error(), "permission denied") {
			t.Skipf("skipping test due to environment execution restriction: %v", err)
		}
		t.Fatalf("unexpected runner error: %v", err)
	}
	return res
}

func TestPythonRunner_BasicExecution(t *testing.T) {
	source := "x = 10\ny = 20\nx + y\n"
	res := runTest(t, source, nil)
	if len(res.Blocks) != 1 || !strings.Contains(strings.Join(res.Blocks[0].Outputs, "\n"), "➜ 30") {
		t.Errorf("expected '➜ 30', got %v", res.Blocks)
	}
}

func TestPythonRunner_MultiBlockUpstream(t *testing.T) {
	source := "x = 42\n\ny = x * 2\ny\n"
	targetLine := 4
	res := runTest(t, source, &targetLine)
	if len(res.Blocks) != 1 || !strings.Contains(strings.Join(res.Blocks[0].Outputs, "\n"), "➜ 84") {
		t.Errorf("expected '➜ 84', got %v", res.Blocks)
	}
}

func TestPythonRunner_FunctionWithInternalBlankLines(t *testing.T) {
	source := `def calculate(items):
    total = 0

    for x in items:
        total += x

    return total

calculate([1, 2, 3, 4])
`
	targetLine := 9
	res := runTest(t, source, &targetLine)
	if len(res.Blocks) != 1 || !strings.Contains(strings.Join(res.Blocks[0].Outputs, "\n"), "➜ 10") {
		t.Errorf("expected '➜ 10', got %v", res.Blocks)
	}
}

func TestPythonRunner_WholeFileExecution(t *testing.T) {
	source := `x = [1, 2, 3]
[i * 10 for i in x]

total = sum(x)
total
`
	res := runTest(t, source, nil)
	if len(res.Blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(res.Blocks))
	}

	b0Out := strings.Join(res.Blocks[0].Outputs, "\n")
	if !strings.Contains(b0Out, "[10, 20, 30]") {
		t.Errorf("block 0 missing expected output, got: %s", b0Out)
	}

	b1Out := strings.Join(res.Blocks[1].Outputs, "\n")
	if !strings.Contains(b1Out, "➜ 6") {
		t.Errorf("block 1 missing expected output, got: %s", b1Out)
	}
}

func TestPythonRunner_SyntaxError(t *testing.T) {
	source := "def broken(\n    pass\n"
	res := runTest(t, source, nil)
	if res.SyntaxError == nil {
		t.Fatalf("expected syntax error, got nil")
	}
}

func TestPythonRunner_TracebackLineAccuracy(t *testing.T) {
	source := "x = 1\n\ny = 2\n\n1 / 0\n"
	targetLine := 5
	res := runTest(t, source, &targetLine)
	if len(res.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(res.Blocks))
	}

	combined := strings.Join(res.Blocks[0].Outputs, "\n")
	if !strings.Contains(combined, "ZeroDivisionError") || !strings.Contains(combined, "line 5") {
		t.Errorf("expected ZeroDivisionError referencing line 5, got: %s", combined)
	}
}

func TestPythonRunner_FormatterProof(t *testing.T) {
	sourceA := "x = 10\ny = 20\nx + y"
	sourceB := "\n\nx = 10\n\n\ny = 20\n\nx + y\n\n"

	resA := runTest(t, sourceA, nil)
	resB := runTest(t, sourceB, nil)

	if len(resA.Blocks) != 1 || len(resB.Blocks) != 1 {
		t.Fatalf("expected 1 output statement each, got A=%d, B=%d", len(resA.Blocks), len(resB.Blocks))
	}

	outA := strings.Join(resA.Blocks[0].Outputs, "\n")
	outB := strings.Join(resB.Blocks[0].Outputs, "\n")
	if outA != outB || !strings.Contains(outA, "➜ 30") {
		t.Errorf("expected matching outputs:\nA: %s\nB: %s", outA, outB)
	}
}

func TestPythonRunner_DependencySlicing(t *testing.T) {
	source := `unrelated_error = 1 / 0

def calculate(a, b):
    return a * b

calculate(6, 7)
`
	targetLine := 6
	res := runTest(t, source, &targetLine)
	if len(res.Blocks) != 1 || !strings.Contains(strings.Join(res.Blocks[0].Outputs, "\n"), "➜ 42") {
		t.Errorf("expected '➜ 42' via dependency slicing, got %v", res.Blocks)
	}
}

func TestPythonRunner_BlankLineNoOp(t *testing.T) {
	source := "x = 10\n\n# A comment followed by blank lines\n\ny = 20\n"
	targetLine := 4
	res := runTest(t, source, &targetLine)
	if len(res.Blocks) != 0 {
		t.Fatalf("expected 0 blocks for blank line, got %d", len(res.Blocks))
	}
}

func TestPythonRunner_FunctionDefNoOp(t *testing.T) {
	source := `def calculate(a, b):
    diff = a - b
    return diff

calculate(10, 5)
`
	targetLine := 2
	res := runTest(t, source, &targetLine)
	if len(res.Blocks) != 0 {
		t.Fatalf("expected 0 blocks for function definition line, got %d", len(res.Blocks))
	}
}

func TestPythonRunner_SubscriptMutationSlicing(t *testing.T) {
	source := `arr = [1, 2, 3]
arr[0] = 999
arr
`
	targetLine := 3
	res := runTest(t, source, &targetLine)
	if len(res.Blocks) != 1 || !strings.Contains(strings.Join(res.Blocks[0].Outputs, "\n"), "999") {
		t.Errorf("expected subscript mutation to be included, got %v", res.Blocks)
	}
}

func TestPythonRunner_AttributeMutationSlicing(t *testing.T) {
	source := `class Box:
    pass

b = Box()
b.val = 42
b.val
`
	targetLine := 6
	res := runTest(t, source, &targetLine)
	if len(res.Blocks) != 1 || !strings.Contains(strings.Join(res.Blocks[0].Outputs, "\n"), "42") {
		t.Errorf("expected attribute mutation to be included, got %v", res.Blocks)
	}
}

func TestPythonRunner_CleanStaleInlineComment(t *testing.T) {
	source := "x = 10  # ➜ 999\n"
	res := runTest(t, source, nil)
	if len(res.Blocks) != 1 {
		t.Fatalf("expected 1 block reporting statement with stale comment, got %d", len(res.Blocks))
	}
	if len(res.Blocks[0].Outputs) != 0 {
		t.Errorf("expected 0 outputs for assignment, got %v", res.Blocks[0].Outputs)
	}
}

func TestPythonRunner_PrettyPrint(t *testing.T) {
	source := `from dataclasses import dataclass

@dataclass
class User:
    id: int
    name: str

class FakeModel:
    def model_dump(self):
        return {"id": 1, "roles": ["admin", "editor"], "meta": {"active": True, "score": 99.5}}

u = User(1, 'Alice')
u

m = FakeModel()
m

small = {"a": 1}
small
`
	res := runTest(t, source, nil)
	if len(res.Blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(res.Blocks))
	}

	// 1. Dataclass: formatted as dict
	uOut := strings.Join(res.Blocks[0].Outputs, "\n")
	if !strings.Contains(uOut, "{'id': 1, 'name': 'Alice'}") {
		t.Errorf("expected dataclass to format as dict, got: %s", uOut)
	}

	// 2. FakeModel (Pydantic v2 duck-typed): multi-line pretty-printed
	mOut := res.Blocks[1].Outputs
	if len(mOut) <= 1 {
		t.Errorf("expected multi-line pretty-printed output for complex model, got %d lines: %v", len(mOut), mOut)
	}

	// 3. Small dict: single-line compact
	sOut := res.Blocks[2].Outputs
	if len(sOut) != 1 || !strings.Contains(sOut[0], "{'a': 1}") {
		t.Errorf("expected small dict to remain compact single-line, got: %v", sOut)
	}
}

func TestPythonRunner_AsyncExecution(t *testing.T) {
	source := `import asyncio

async def fetch_data(val):
    await asyncio.sleep(0.001)
    return {"val": val * 2}

# 1. Top-level await expression
await fetch_data(10)

# 2. Top-level await assignment
res = await fetch_data(20)
res

# 3. Unawaited call to async function (auto-await)
fetch_data(30)
`
	res := runTest(t, source, nil)
	if len(res.Blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(res.Blocks))
	}

	// 1. await fetch_data(10) -> val: 20
	b0 := strings.Join(res.Blocks[0].Outputs, "\n")
	if !strings.Contains(b0, "{'val': 20}") {
		t.Errorf("expected top-level await expression result, got: %s", b0)
	}

	// 2. res -> val: 40
	b1 := strings.Join(res.Blocks[1].Outputs, "\n")
	if !strings.Contains(b1, "{'val': 40}") {
		t.Errorf("expected top-level await assignment result, got: %s", b1)
	}

	// 3. fetch_data(30) -> auto-await -> val: 60
	b2 := strings.Join(res.Blocks[2].Outputs, "\n")
	if !strings.Contains(b2, "{'val': 60}") {
		t.Errorf("expected auto-awaited coroutine result, got: %s", b2)
	}
}

func TestPythonRunner_AsyncStateAcrossStatements(t *testing.T) {
	source := `import asyncio

queue = asyncio.Queue()
await queue.put("shared_item")
item = await queue.get()
item
`
	res := runTest(t, source, nil)
	if len(res.Blocks) != 1 {
		t.Fatalf("expected 1 output block, got %d", len(res.Blocks))
	}
	out := strings.Join(res.Blocks[0].Outputs, "\n")
	if !strings.Contains(out, "'shared_item'") {
		t.Errorf("expected shared queue item, got: %s", out)
	}
}

func TestPythonRunner_AsyncScopeHygiene(t *testing.T) {
	source := `import asyncio

async def fail():
    raise ValueError("intentional error")

try:
    await fail()
except Exception:
    pass

"__peek_result__" in globals()
`
	res := runTest(t, source, nil)
	if len(res.Blocks) != 1 {
		t.Fatalf("expected 1 output block, got %d", len(res.Blocks))
	}
	out := strings.Join(res.Blocks[0].Outputs, "\n")
	if !strings.Contains(out, "False") {
		t.Errorf("expected __peek_result__ to not leak into globals, got: %s", out)
	}
}

func TestPythonRunner_AsyncNestedInSync(t *testing.T) {
	source := `def make_handler():
    async def inner():
        return 99
    return inner

h = make_handler()
h()
`
	res := runTest(t, source, nil)
	if len(res.Blocks) != 1 {
		t.Fatalf("expected 1 output block, got %d", len(res.Blocks))
	}
	out := strings.Join(res.Blocks[0].Outputs, "\n")
	if !strings.Contains(out, "99") {
		t.Errorf("expected auto-awaited result from inner coroutine, got: %s", out)
	}
}

func TestPythonRunner_AsyncComprehension(t *testing.T) {
	source := `import asyncio

async def agen():
    for i in range(3):
        await asyncio.sleep(0.001)
        yield i * 5

[x async for x in agen()]
`
	res := runTest(t, source, nil)
	if len(res.Blocks) != 1 {
		t.Fatalf("expected 1 output block, got %d", len(res.Blocks))
	}
	out := strings.Join(res.Blocks[0].Outputs, "\n")
	if !strings.Contains(out, "[0, 5, 10]") {
		t.Errorf("expected async comprehension output, got: %s", out)
	}
}

func TestPythonRunner_AsyncGatherAutoAwait(t *testing.T) {
	source := `import asyncio

async def fetch(x):
    await asyncio.sleep(0.001)
    return x * 10

asyncio.gather(fetch(1), fetch(2))
`
	res := runTest(t, source, nil)
	if len(res.Blocks) != 1 {
		t.Fatalf("expected 1 output block, got %d", len(res.Blocks))
	}
	out := strings.Join(res.Blocks[0].Outputs, "\n")
	if !strings.Contains(out, "[10, 20]") {
		t.Errorf("expected auto-awaited asyncio.gather output, got: %s", out)
	}
}

func TestPythonRunner_TargetAssignment(t *testing.T) {
	source := "x = 10 * 42\n"
	targetLine := 1
	res := runTest(t, source, &targetLine)
	if len(res.Blocks) != 1 || !strings.Contains(strings.Join(res.Blocks[0].Outputs, "\n"), "➜ 420") {
		t.Errorf("expected '➜ 420', got %v", res.Blocks)
	}
}

func TestPythonRunner_AnnotatedAndAugmentedAssign(t *testing.T) {
	source := "x: int = 100\nx += 50\n"
	targetLine := 2
	res := runTest(t, source, &targetLine)
	if len(res.Blocks) != 1 || !strings.Contains(strings.Join(res.Blocks[0].Outputs, "\n"), "➜ 150") {
		t.Errorf("expected '➜ 150', got %v", res.Blocks)
	}
}

func TestPythonRunner_MultiAssignment(t *testing.T) {
	source := "a, b = 1, 2\n"
	targetLine := 1
	res := runTest(t, source, &targetLine)
	if len(res.Blocks) != 1 || !strings.Contains(strings.Join(res.Blocks[0].Outputs, "\n"), "➜ a=1, b=2") {
		t.Errorf("expected '➜ a=1, b=2', got %v", res.Blocks)
	}
}

func TestPythonRunner_ListAndDictMutation(t *testing.T) {
	// List mutation via .append()
	sourceList := "nums = [1, 2]\nnums.append(3)\n"
	targetLine := 2
	resList := runTest(t, sourceList, &targetLine)
	if len(resList.Blocks) != 1 || !strings.Contains(strings.Join(resList.Blocks[0].Outputs, "\n"), "➜ [1, 2, 3]") {
		t.Errorf("expected '➜ [1, 2, 3]', got %v", resList.Blocks)
	}

	// Dict subscript mutation
	sourceDict := "d = {}\nd[\"key\"] = \"val\"\n"
	resDict := runTest(t, sourceDict, &targetLine)
	if len(resDict.Blocks) != 1 || !strings.Contains(strings.Join(resDict.Blocks[0].Outputs, "\n"), "'key': 'val'") {
		t.Errorf("expected dict mutation output, got %v", resDict.Blocks)
	}
}

func TestPythonRunner_IgnoredVariable(t *testing.T) {
	source := "_ = 123\n"
	targetLine := 1
	res := runTest(t, source, &targetLine)
	if len(res.Blocks) != 1 || len(res.Blocks[0].Outputs) != 0 {
		t.Errorf("expected empty outputs for '_ = 123', got %v", res.Blocks)
	}
}

func TestForFile(t *testing.T) {
	// 1. Python file returns PythonRunner with prefix #
	r, err := ForFile("test.py", nil)
	if err != nil {
		// In environments without uv installed, ForFile returns uv not found error, which is expected
		if !strings.Contains(err.Error(), "requires 'uv'") {
			t.Fatalf("unexpected error for test.py: %v", err)
		}
	} else {
		if r.CommentPrefix() != "#" {
			t.Errorf("expected comment prefix # for python, got %q", r.CommentPrefix())
		}
	}

	// 2. TypeScript / JavaScript file returns BunRunner with prefix //
	for _, ext := range []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"} {
		r, err := ForFile("app"+ext, nil)
		if err != nil {
			if !strings.Contains(err.Error(), "requires 'bun'") {
				t.Fatalf("unexpected error for %s: %v", ext, err)
			}
		} else {
			if r.CommentPrefix() != "//" {
				t.Errorf("expected comment prefix // for %s, got %q", ext, r.CommentPrefix())
			}
		}
	}

	// 3. Unsupported file type returns error
	_, err = ForFile("notes.txt", nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported file type") {
		t.Errorf("expected unsupported file type error, got %v", err)
	}
}




