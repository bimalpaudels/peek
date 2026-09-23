package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func runBunTest(t *testing.T, source string, targetLine *int) *ExecutionResult {
	t.Helper()
	r, err := NewBunRunner()
	if err != nil {
		t.Skipf("skipping bun runner test: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := r.Execute(ctx, "test.ts", source, targetLine, 30)
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") || strings.Contains(err.Error(), "permission denied") {
			t.Skipf("skipping test due to environment execution restriction: %v", err)
		}
		t.Fatalf("unexpected runner error: %v", err)
	}
	return res
}

func TestBunRunner_BasicExecution(t *testing.T) {
	source := "const x: number = 10;\nconst y: number = 20;\nx + y;\n"
	res := runBunTest(t, source, nil)
	if len(res.Blocks) != 1 || !strings.Contains(strings.Join(res.Blocks[0].Outputs, "\n"), "➜ 30") {
		t.Errorf("expected '➜ 30', got %v", res.Blocks)
	}
}

func TestBunRunner_DependencySlicing(t *testing.T) {
	source := `throw new Error("unrelated heavy error");

function add(a: number, b: number): number {
    return a + b;
}

add(20, 22);
`
	targetLine := 7
	res := runBunTest(t, source, &targetLine)
	if len(res.Blocks) != 1 || !strings.Contains(strings.Join(res.Blocks[0].Outputs, "\n"), "➜ 42") {
		t.Errorf("expected '➜ 42' via dependency slicing, got %v", res.Blocks)
	}
}

func TestBunRunner_AsyncExecution(t *testing.T) {
	source := `async function fetchData(n: number) {
    return { value: n * 2 };
}

const data = await fetchData(21);
data.value;
`
	targetLine := 6
	res := runBunTest(t, source, &targetLine)
	if len(res.Blocks) != 1 || !strings.Contains(strings.Join(res.Blocks[0].Outputs, "\n"), "➜ 42") {
		t.Errorf("expected '➜ 42' via top-level await, got %v", res.Blocks)
	}
}

func TestBunRunner_StdoutCapture(t *testing.T) {
	source := `console.log("fetching records...");
console.log("done");
`
	targetLine := 2
	res := runBunTest(t, source, &targetLine)
	if len(res.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(res.Blocks))
	}
	out := strings.Join(res.Blocks[0].Outputs, "\n")
	if !strings.Contains(out, "❯ done") {
		t.Errorf("expected '❯ done', got: %s", out)
	}
}

func TestBunRunner_ObjectFormatting(t *testing.T) {
	source := `const user = {
    id: 101,
    name: "Alex",
    roles: ["admin", "dev"],
};
user;
`
	targetLine := 6
	res := runBunTest(t, source, &targetLine)
	if len(res.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(res.Blocks))
	}
	out := strings.Join(res.Blocks[0].Outputs, "\n")
	if !strings.Contains(out, "id: 101") || !strings.Contains(out, "name: \"Alex\"") {
		t.Errorf("expected formatted object, got: %s", out)
	}
}

func TestBunRunner_Mutations(t *testing.T) {
	source := `const items = [1, 2];
items.push(3);
items;
`
	targetLine := 3
	res := runBunTest(t, source, &targetLine)
	if len(res.Blocks) != 1 || !strings.Contains(strings.Join(res.Blocks[0].Outputs, "\n"), "[ 1, 2, 3 ]") {
		t.Errorf("expected mutated array to be included, got %v", res.Blocks)
	}
}

func TestBunRunner_SyntaxError(t *testing.T) {
	source := "const broken = (\n"
	res := runBunTest(t, source, nil)
	if res.SyntaxError == nil {
		t.Fatalf("expected syntax error, got nil")
	}
}

func TestBunRunner_WholeFileExecution(t *testing.T) {
	source := `const a = 5;
a * 2;

console.log("hello");
`
	res := runBunTest(t, source, nil)
	if len(res.Blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(res.Blocks))
	}

	b0 := strings.Join(res.Blocks[0].Outputs, "\n")
	if !strings.Contains(b0, "➜ 10") {
		t.Errorf("expected ➜ 10 in block 0, got: %s", b0)
	}

	b1 := strings.Join(res.Blocks[1].Outputs, "\n")
	if !strings.Contains(b1, "❯ hello") {
		t.Errorf("expected ❯ hello in block 1, got: %s", b1)
	}
}

func TestBunRunner_TypeScriptInterfacesAndTypes(t *testing.T) {
	source := `interface Product {
    id: number;
    title: string;
}

type Price = number;

const p: Product = { id: 1, title: "Book" };
const cost: Price = 19.99;
p.title;
`
	targetLine := 10
	res := runBunTest(t, source, &targetLine)
	if len(res.Blocks) != 1 || !strings.Contains(strings.Join(res.Blocks[0].Outputs, "\n"), "➜ \"Book\"") {
		t.Errorf("expected '➜ \"Book\"', got %v", res.Blocks)
	}
}

func TestBunRunner_ZeroDiskFootprint(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "scratchpad.ts")
	source := `const a = 100;
const b = 200;
a + b;
`
	if err := os.WriteFile(filePath, []byte(source), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	r, err := NewBunRunner()
	if err != nil {
		t.Skipf("skipping bun runner test: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	targetLine := 3
	res, err := r.Execute(ctx, filePath, source, &targetLine, 30)
	if err != nil {
		t.Fatalf("unexpected runner error: %v", err)
	}

	if len(res.Blocks) != 1 || !strings.Contains(strings.Join(res.Blocks[0].Outputs, "\n"), "➜ 300") {
		t.Errorf("expected '➜ 300', got %v", res.Blocks)
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read temp dir: %v", err)
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".peek_tmp_") {
			t.Errorf("found unwanted temporary file on disk: %s", entry.Name())
		}
	}
}

func TestBunRunner_RelativeImportsInMemory(t *testing.T) {
	tmpDir := t.TempDir()
	helperPath := filepath.Join(tmpDir, "helper.ts")
	helperCode := `export function multiply(a: number, b: number): number {
    return a * b;
}
`
	if err := os.WriteFile(helperPath, []byte(helperCode), 0644); err != nil {
		t.Fatalf("failed to write helper file: %v", err)
	}

	mainPath := filepath.Join(tmpDir, "main.ts")
	mainSource := `import { multiply } from "./helper";

const ans = multiply(6, 7);
ans;
`
	r, err := NewBunRunner()
	if err != nil {
		t.Skipf("skipping bun runner test: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	targetLine := 4
	res, err := r.Execute(ctx, mainPath, mainSource, &targetLine, 30)
	if err != nil {
		t.Fatalf("unexpected runner error: %v", err)
	}

	if len(res.Blocks) != 1 || !strings.Contains(strings.Join(res.Blocks[0].Outputs, "\n"), "➜ 42") {
		t.Errorf("expected '➜ 42' via in-memory relative import resolution, got %v", res.Blocks)
	}
}

func TestBunRunner_NonMutatingCallsNotSliced(t *testing.T) {
	source := `const items = [10, 20, 30];
const explosive = () => { throw new Error("explosive called"); };
// Non-mutating methods must not trigger dependency slice
items.map(() => explosive());
items.filter(() => explosive());
items.slice(0, 1);

items[0];
`
	targetLine := 8
	res := runBunTest(t, source, &targetLine)
	if len(res.Blocks) != 1 || !strings.Contains(strings.Join(res.Blocks[0].Outputs, "\n"), "➜ 10") {
		t.Errorf("expected non-mutating calls to be omitted during slicing, got %v", res.Blocks)
	}
}

func TestBunRunner_MapSetMutations(t *testing.T) {
	source := `const m = new Map<string, number>();
m.set("answer", 42);
m.get("answer");
`
	targetLine := 3
	res := runBunTest(t, source, &targetLine)
	if len(res.Blocks) != 1 || !strings.Contains(strings.Join(res.Blocks[0].Outputs, "\n"), "➜ 42") {
		t.Errorf("expected map mutation to be sliced in, got %v", res.Blocks)
	}
}

func TestBunRunner_BareExportEmptyModuleMarker(t *testing.T) {
	source := `export {};

async function compute() {
    return 42;
}

await compute();
`
	// 1. Target line 1 (export {}) - strict no-op
	targetLine1 := 1
	res1 := runBunTest(t, source, &targetLine1)
	if len(res1.Blocks) != 0 {
		t.Errorf("expected 0 blocks for export {} line, got %v", res1.Blocks)
	}

	// 2. Target line 7 (await compute()) - evaluates cleanly with export {} present
	targetLine7 := 7
	res7 := runBunTest(t, source, &targetLine7)
	if len(res7.Blocks) != 1 || !strings.Contains(strings.Join(res7.Blocks[0].Outputs, "\n"), "➜ 42") {
		t.Errorf("expected '➜ 42', got %v", res7.Blocks)
	}
}
