package runner

import (
	"context"
	"strings"
	"testing"
	"time"
)

func checkRunnerErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") || strings.Contains(err.Error(), "permission denied") {
			t.Skipf("skipping test due to environment execution restriction: %v", err)
		}
		t.Fatalf("unexpected runner error: %v", err)
	}
}

func TestPythonRunner_BasicExecution(t *testing.T) {
	r, err := NewPythonRunner()
	if err != nil {
		t.Skipf("skipping python runner test: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	source := `x = 10
y = 20
x + y
`
	res, err := r.Execute(ctx, "test.py", source, nil, 30)
	checkRunnerErr(t, err)

	if len(res.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(res.Blocks))
	}

	found := false
	for _, out := range res.Blocks[0].Outputs {
		if strings.Contains(out, "➜ 30") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected '➜ 30', got %v", res.Blocks[0].Outputs)
	}
}

func TestPythonRunner_MultiBlockUpstream(t *testing.T) {
	r, err := NewPythonRunner()
	if err != nil {
		t.Skipf("skipping python runner test: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	source := `x = 42

y = x * 2
y
`
	targetLine := 4
	res, err := r.Execute(ctx, "test.py", source, &targetLine, 30)
	checkRunnerErr(t, err)

	if len(res.Blocks) != 1 {
		t.Fatalf("expected 1 target block output, got %d", len(res.Blocks))
	}

	found := false
	for _, out := range res.Blocks[0].Outputs {
		if strings.Contains(out, "➜ 84") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected '➜ 84', got %v", res.Blocks[0].Outputs)
	}
}

func TestPythonRunner_FunctionWithInternalBlankLines(t *testing.T) {
	r, err := NewPythonRunner()
	if err != nil {
		t.Skipf("skipping python runner test: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	source := `def calculate(items):
    total = 0

    for x in items:
        total += x

    return total

calculate([1, 2, 3, 4])
`
	targetLine := 9
	res, err := r.Execute(ctx, "test.py", source, &targetLine, 30)
	checkRunnerErr(t, err)

	if len(res.Blocks) != 1 {
		t.Fatalf("expected 1 target block output, got %d", len(res.Blocks))
	}

	found := false
	for _, out := range res.Blocks[0].Outputs {
		if strings.Contains(out, "➜ 10") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected '➜ 10', got %v", res.Blocks[0].Outputs)
	}
}

func TestPythonRunner_WholeFileExecution(t *testing.T) {
	r, err := NewPythonRunner()
	if err != nil {
		t.Skipf("skipping python runner test: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	source := `x = [1, 2, 3]
[i * 10 for i in x]

total = sum(x)
total
`
	res, err := r.Execute(ctx, "test.py", source, nil, 30)
	checkRunnerErr(t, err)

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
	r, err := NewPythonRunner()
	if err != nil {
		t.Skipf("skipping python runner test: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	source := `def broken(
    pass
`
	res, err := r.Execute(ctx, "test.py", source, nil, 30)
	checkRunnerErr(t, err)

	if res.SyntaxError == nil {
		t.Fatalf("expected syntax error, got nil")
	}
}

func TestPythonRunner_TracebackLineAccuracy(t *testing.T) {
	r, err := NewPythonRunner()
	if err != nil {
		t.Skipf("skipping python runner test: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	source := `x = 1

y = 2

1 / 0
`
	targetLine := 5
	res, err := r.Execute(ctx, "test.py", source, &targetLine, 30)
	checkRunnerErr(t, err)

	if len(res.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(res.Blocks))
	}

	combined := strings.Join(res.Blocks[0].Outputs, "\n")
	if !strings.Contains(combined, "ZeroDivisionError") {
		t.Errorf("expected ZeroDivisionError, got: %s", combined)
	}
	if !strings.Contains(combined, "line 5") {
		t.Errorf("expected traceback to reference line 5, got: %s", combined)
	}
}

func TestPythonRunner_FormatterProof(t *testing.T) {
	r, err := NewPythonRunner()
	if err != nil {
		t.Skipf("skipping python runner test: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sourceA := `x = 10
y = 20
x + y`

	sourceB := `

x = 10


y = 20

x + y

`
	resA, err := r.Execute(ctx, "test.py", sourceA, nil, 30)
	checkRunnerErr(t, err)
	resB, err := r.Execute(ctx, "test.py", sourceB, nil, 30)
	checkRunnerErr(t, err)

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
	r, err := NewPythonRunner()
	if err != nil {
		t.Skipf("skipping python runner test: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Upstream statement has a deliberate runtime crash (1 / 0).
	// Target statement calculate(6, 7) only depends on def calculate.
	// With AST dependency slicing, the crash must be completely bypassed.
	source := `unrelated_error = 1 / 0

def calculate(a, b):
    return a * b

calculate(6, 7)
`
	targetLine := 6
	res, err := r.Execute(ctx, "test.py", source, &targetLine, 30)
	checkRunnerErr(t, err)

	if len(res.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(res.Blocks))
	}

	found := false
	for _, out := range res.Blocks[0].Outputs {
		if strings.Contains(out, "➜ 42") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected '➜ 42' via dependency slicing, got %v", res.Blocks[0].Outputs)
	}
}

func TestPythonRunner_BlankLineNoOp(t *testing.T) {
	r, err := NewPythonRunner()
	if err != nil {
		t.Skipf("skipping python runner test: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	source := `x = 10

# A comment followed by blank lines

y = 20
`
	// Line 3 is a comment, Line 4 is blank
	targetLine := 4
	res, err := r.Execute(ctx, "test.py", source, &targetLine, 30)
	checkRunnerErr(t, err)

	if len(res.Blocks) != 0 {
		t.Fatalf("expected 0 blocks for blank line, got %d", len(res.Blocks))
	}
}

func TestPythonRunner_FunctionDefNoOp(t *testing.T) {
	r, err := NewPythonRunner()
	if err != nil {
		t.Skipf("skipping python runner test: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	source := `def calculate(a, b):
    diff = a - b
    return diff

calculate(10, 5)
`
	// Line 2 is inside the function body
	targetLine := 2
	res, err := r.Execute(ctx, "test.py", source, &targetLine, 30)
	checkRunnerErr(t, err)

	if len(res.Blocks) != 0 {
		t.Fatalf("expected 0 blocks for function definition line, got %d", len(res.Blocks))
	}
}


