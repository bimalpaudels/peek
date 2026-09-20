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
	BunPath       string `json:"bun_path"`
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
		BunPath:       "",
	}
}

// GetConfigPath returns the standard location of the configuration file (~/.config/peek/config.toml).
func GetConfigPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); strings.TrimSpace(xdg) != "" {
		return filepath.Join(xdg, "peek", "config.toml")
	}

	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		return filepath.Join(home, ".config", "peek", "config.toml")
	}

	configDir, _ := os.UserConfigDir()
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
		case "bun_path":
			cfg.BunPath = val
		}
	}
	return cfg, nil
}

const DefaultConfigTemplate = `# peek configuration file

# Maximum lines of output per statement before truncation
max_lines = 30

# Execution timeout in seconds
timeout = 10

# Maximum line width for inline comment formatting
line_width = 100

# Maximum character length for strings in dictionaries before truncation
max_str_len = 140

# Automatically search for and load .env files
load_env = true

# Optional custom path to uv binary (defaults to auto-detection)
# uv_path = "/usr/local/bin/uv"

# Optional Python version for uv execution (defaults to system uv default)
# python_version = "3.12"

# Optional custom path to bun binary (defaults to auto-detection)
# bun_path = "/usr/local/bin/bun"
`

// EnsureConfigFile creates the default configuration file if it does not already exist.
func EnsureConfigFile() (string, error) {
	path := GetConfigPath()
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return path, err
	}

	if err := os.WriteFile(path, []byte(DefaultConfigTemplate), 0644); err != nil {
		return path, err
	}

	return path, nil
}

// Load reads the global config file if present, auto-creates it if missing,
// or returns the defaults on any error.
func Load() *Config {
	path := GetConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		// Attempt to auto-create the starter config template
		_, _ = EnsureConfigFile()
		return DefaultConfig()
	}
	cfg, err := Parse(string(data))
	if err != nil {
		return DefaultConfig()
	}
	return cfg
}
