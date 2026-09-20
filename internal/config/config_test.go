package config

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxLines != 30 {
		t.Errorf("expected MaxLines=30, got %d", cfg.MaxLines)
	}
	if cfg.Timeout != 10 {
		t.Errorf("expected Timeout=10, got %d", cfg.Timeout)
	}
	if cfg.LineWidth != 100 {
		t.Errorf("expected LineWidth=100, got %d", cfg.LineWidth)
	}
	if cfg.MaxStrLen != 140 {
		t.Errorf("expected MaxStrLen=140, got %d", cfg.MaxStrLen)
	}
	if !cfg.LoadEnv {
		t.Errorf("expected LoadEnv=true, got %v", cfg.LoadEnv)
	}
}

func TestParseConfig(t *testing.T) {
	toml := `
# Global peek settings
max_lines = 50
timeout = 25 # custom timeout
line_width = 120
max_str_len = 200
load_env = false
uv_path = "/custom/bin/uv"
python_version = "3.12"
`
	cfg, err := Parse(toml)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if cfg.MaxLines != 50 {
		t.Errorf("expected MaxLines=50, got %d", cfg.MaxLines)
	}
	if cfg.Timeout != 25 {
		t.Errorf("expected Timeout=25, got %d", cfg.Timeout)
	}
	if cfg.LineWidth != 120 {
		t.Errorf("expected LineWidth=120, got %d", cfg.LineWidth)
	}
	if cfg.MaxStrLen != 200 {
		t.Errorf("expected MaxStrLen=200, got %d", cfg.MaxStrLen)
	}
	if cfg.LoadEnv {
		t.Errorf("expected LoadEnv=false, got %v", cfg.LoadEnv)
	}
	if cfg.UVPath != "/custom/bin/uv" {
		t.Errorf("expected UVPath='/custom/bin/uv', got %q", cfg.UVPath)
	}
	if cfg.PythonVersion != "3.12" {
		t.Errorf("expected PythonVersion='3.12', got %q", cfg.PythonVersion)
	}
}

func TestParseEmptyOrCommentsOnly(t *testing.T) {
	toml := `
# Only comments
# timeout = 99
`
	cfg, err := Parse(toml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Timeout != 10 {
		t.Errorf("expected default timeout 10, got %d", cfg.Timeout)
	}
}
