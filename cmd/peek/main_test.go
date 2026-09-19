package main

import (
	"testing"
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
			name:         "path with colon or drive letter",
			args:         []string{`C:\projects\app.py:42`},
			wantFile:     `C:\projects\app.py`,
			wantLine:     intPtr(42),
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
