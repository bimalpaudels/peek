package main

import (
	"testing"

	"peek/internal/config"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantFile     string
		wantLine     *int
		wantClean    bool
		wantMaxLines int
		wantTimeout  int
		wantErr      bool
	}{
		{
			name:         "file:line syntax",
			args:         []string{"solution.py:21"},
			wantFile:     "solution.py",
			wantLine:     intPtr(21),
			wantMaxLines: 30,
			wantTimeout:  10,
		},
		{
			name:         "file and line positional syntax",
			args:         []string{"solution.py", "21"},
			wantFile:     "solution.py",
			wantLine:     intPtr(21),
			wantMaxLines: 30,
			wantTimeout:  10,
		},
		{
			name:         "file only",
			args:         []string{"main.py"},
			wantFile:     "main.py",
			wantLine:     nil,
			wantMaxLines: 30,
			wantTimeout:  10,
		},
		{
			name:         "clean flag",
			args:         []string{"main.py", "--clean"},
			wantFile:     "main.py",
			wantClean:    true,
			wantMaxLines: 30,
			wantTimeout:  10,
		},
		{
			name:         "custom options",
			args:         []string{"foo.py:10", "--max-lines", "50", "--timeout", "5"},
			wantFile:     "foo.py",
			wantLine:     intPtr(10),
			wantMaxLines: 50,
			wantTimeout:  5,
		},
		{
			name:         "custom options with equals syntax",
			args:         []string{"foo.py:10", "--max-lines=50", "--timeout=5"},
			wantFile:     "foo.py",
			wantLine:     intPtr(10),
			wantMaxLines: 50,
			wantTimeout:  5,
		},
		{
			name:         "path with colon or drive letter",
			args:         []string{`C:\projects\app.py:42`},
			wantFile:     `C:\projects\app.py`,
			wantLine:     intPtr(42),
			wantMaxLines: 30,
			wantTimeout:  10,
		},
		{
			name:         "typescript file with line",
			args:         []string{"index.ts:15"},
			wantFile:     "index.ts",
			wantLine:     intPtr(15),
			wantMaxLines: 30,
			wantTimeout:  10,
		},
		{
			name:         "tsx file with clean flag",
			args:         []string{"component.tsx", "--clean"},
			wantFile:     "component.tsx",
			wantClean:    true,
			wantMaxLines: 30,
			wantTimeout:  10,
		},
		{
			name:    "empty args error",
			args:    []string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, line, clean, maxLines, timeout, err := parseArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if file != tt.wantFile {
				t.Errorf("file = %q, want %q", file, tt.wantFile)
			}
			if (line == nil && tt.wantLine != nil) || (line != nil && tt.wantLine == nil) {
				t.Errorf("line = %v, want %v", line, tt.wantLine)
			} else if line != nil && tt.wantLine != nil && *line != *tt.wantLine {
				t.Errorf("line = %v, want %v", *line, *tt.wantLine)
			}
			if clean != tt.wantClean {
				t.Errorf("clean = %v, want %v", clean, tt.wantClean)
			}
			if maxLines != tt.wantMaxLines {
				t.Errorf("maxLines = %d, want %d", maxLines, tt.wantMaxLines)
			}
			if timeout != tt.wantTimeout {
				t.Errorf("timeout = %d, want %d", timeout, tt.wantTimeout)
			}
		})
	}
}

func intPtr(i int) *int {
	return &i
}

func TestParseArgs_WithCustomConfig(t *testing.T) {
	customCfg, err := parseCustomConfig(`
max_lines = 45
timeout = 20
`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1. Should use custom config defaults when no CLI flags given
	file, _, _, maxLines, timeout, err := parseArgs([]string{"demo.py"}, customCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if file != "demo.py" {
		t.Errorf("expected demo.py, got %s", file)
	}
	if maxLines != 45 {
		t.Errorf("expected maxLines=45 from config, got %d", maxLines)
	}
	if timeout != 20 {
		t.Errorf("expected timeout=20 from config, got %d", timeout)
	}

	// 2. CLI flags should override config
	_, _, _, maxLines2, timeout2, err := parseArgs([]string{"demo.py", "--max-lines", "99", "--timeout", "3"}, customCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if maxLines2 != 99 {
		t.Errorf("expected CLI override maxLines=99, got %d", maxLines2)
	}
	if timeout2 != 3 {
		t.Errorf("expected CLI override timeout=3, got %d", timeout2)
	}
}

func parseCustomConfig(s string) (*config.Config, error) {
	return config.Parse(s)
}

