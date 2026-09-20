package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds the user preferences for peek execution and formatting.
type Config struct {
	MaxLines      int    `json:"max_lines"`
	Timeout       int    `json:"timeout"`
	LineWidth     int    `json:"line_width"`
	MaxStrLen     int    `json:"max_str_len"`
	LoadEnv       bool   `json:"load_env"`
	UVPath        string `json:"uv_path"`
	PythonVersion string `json:"python_version"`
}

// DefaultConfig returns the standard built-in configuration defaults.
func DefaultConfig() *Config {
	return &Config{
		MaxLines:      30,
		Timeout:       10,
		LineWidth:     100,
		MaxStrLen:     140,
		LoadEnv:       true,
		UVPath:        "",
		PythonVersion: "",
	}
}

// GetConfigPath returns the standard location of the configuration file (~/.config/peek/config.toml).
func GetConfigPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "peek", "config.toml")
}

// Parse parses a TOML / key-value configuration string into a Config struct.
func Parse(content string) (*Config, error) {
	cfg := DefaultConfig()
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])

		// Strip inline comment: val could have `# comment`
		inQuote := false
		cleanIdx := -1
		for i, ch := range val {
			if ch == '"' || ch == '\'' {
				inQuote = !inQuote
			} else if ch == '#' && !inQuote {
				cleanIdx = i
				break
			}
		}
		if cleanIdx != -1 {
			val = strings.TrimSpace(val[:cleanIdx])
		}

		// Unquote string if quoted
		if (strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) ||
			(strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) {
			if len(val) >= 2 {
				val = val[1 : len(val)-1]
			}
		}

		switch key {
		case "max_lines":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.MaxLines = n
			}
		case "timeout":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.Timeout = n
			}
		case "line_width":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.LineWidth = n
			}
		case "max_str_len":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.MaxStrLen = n
			}
		case "load_env":
			if b, err := strconv.ParseBool(val); err == nil {
				cfg.LoadEnv = b
			}
		case "uv_path":
			cfg.UVPath = val
		case "python_version":
			cfg.PythonVersion = val
		}
	}
	return cfg, nil
}

// Load reads the global config file if present, or returns the defaults if missing.
func Load() *Config {
	path := GetConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return DefaultConfig()
	}
	cfg, err := Parse(string(data))
	if err != nil {
		return DefaultConfig()
	}
	return cfg
}
