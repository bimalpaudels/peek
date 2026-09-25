package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"peek/internal/config"
	"peek/internal/runner"
)

// SliceArgs defines the input schema for the peek_slice tool.
type SliceArgs struct {
	File     string `json:"file" jsonschema:"Relative or absolute path to the target file (.py, .ts, .js)"`
	Line     *int   `json:"line,omitempty" jsonschema:"Optional 1-indexed target line number. If omitted or null, evaluates all statements top-to-bottom."`
	Timeout  *int   `json:"timeout,omitempty" jsonschema:"Optional execution timeout in seconds (overrides default config)."`
	MaxLines *int   `json:"max_lines,omitempty" jsonschema:"Optional maximum output lines per block (overrides default config)."`
}

// Server wraps the MCP server and peek configuration.
type Server struct {
	cfg     *config.Config
	version string
	mcpSrv  *mcp.Server
}

// NewServer creates a new MCP server configured with tools and resources.
func NewServer(cfg *config.Config, version string) *Server {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	if version == "" {
		version = "dev"
	}

	opts := &mcp.ServerOptions{
		Instructions: "peek evaluates target lines or full files in Python and TypeScript using AST dependency slicing. " +
			"Targeting a line executes only its upstream dependencies, skipping unrelated expensive operations or side effects. " +
			"Files are never modified on disk during evaluation.",
	}

	mcpSrv := mcp.NewServer(&mcp.Implementation{
		Name:    "peek",
		Version: version,
	}, opts)

	s := &Server{
		cfg:     cfg,
		version: version,
		mcpSrv:  mcpSrv,
	}

	s.registerTools()
	s.registerResources()

	return s
}

// MCPServer returns the underlying *mcp.Server instance.
func (s *Server) MCPServer() *mcp.Server {
	return s.mcpSrv
}

// Serve starts the server over stdio with optional debug logging to stderr.
func (s *Server) Serve(ctx context.Context, debug bool) error {
	var t mcp.Transport = &mcp.StdioTransport{}
	if debug {
		t = &mcp.LoggingTransport{Transport: t, Writer: os.Stderr}
	}
	return s.mcpSrv.Run(ctx, t)
}

// RunTransport runs the server on a custom MCP transport (useful for tests and custom integrations).
func (s *Server) RunTransport(ctx context.Context, t mcp.Transport) error {
	return s.mcpSrv.Run(ctx, t)
}

func (s *Server) registerTools() {
	mcp.AddTool(s.mcpSrv, &mcp.Tool{
		Name: "peek_slice",
		Description: "Evaluates a target line or an entire file in Python or TypeScript using AST dependency slicing. " +
			"Runs required upstream dependencies and returns evaluated values and stdout without modifying the file on disk.",
	}, s.handleSlice)
}

func (s *Server) registerResources() {
	// peek://config - exposes active peek settings
	s.mcpSrv.AddResource(&mcp.Resource{
		Name:        "Peek Config",
		Description: "Active configuration settings for peek (timeout, max lines, runner paths).",
		URI:         "peek://config",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		data, err := json.MarshalIndent(s.cfg, "", "  ")
		if err != nil {
			return nil, err
		}
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      req.Params.URI,
					MIMEType: "application/json",
					Text:     string(data),
				},
			},
		}, nil
	})

	// peek://environment - runtime presence and environment information
	s.mcpSrv.AddResource(&mcp.Resource{
		Name:        "Peek Environment",
		Description: "Environment status and detected interpreter runners for Python and TypeScript.",
		URI:         "peek://environment",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		uvPath, _ := exec.LookPath("uv")
		bunPath, _ := exec.LookPath("bun")

		envInfo := map[string]any{
			"version": s.version,
			"python": map[string]any{
				"uv_available": uvPath != "",
				"uv_path":      uvPath,
			},
			"typescript": map[string]any{
				"bun_available": bunPath != "",
				"bun_path":       bunPath,
			},
		}

		data, err := json.MarshalIndent(envInfo, "", "  ")
		if err != nil {
			return nil, err
		}
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      req.Params.URI,
					MIMEType: "application/json",
					Text:     string(data),
				},
			},
		}, nil
	})
}

func (s *Server) handleSlice(ctx context.Context, req *mcp.CallToolRequest, args SliceArgs) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(args.File) == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Missing or invalid 'file' argument"},
			},
			IsError: true,
		}, nil, nil
	}

	absPath, err := filepath.Abs(args.File)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("peek error: invalid path: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	contentBytes, err := os.ReadFile(absPath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("peek error: file not found: %s", args.File)},
			},
			IsError: true,
		}, nil, nil
	}
	content := string(contentBytes)

	r, err := runner.ForFile(absPath, s.cfg)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("peek error: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	timeoutSec := s.cfg.Timeout
	if args.Timeout != nil && *args.Timeout > 0 {
		timeoutSec = *args.Timeout
	} else if timeoutSec <= 0 {
		timeoutSec = 10
	}

	evalCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	maxLines := s.cfg.MaxLines
	if args.MaxLines != nil && *args.MaxLines > 0 {
		maxLines = *args.MaxLines
	}

	var targetLine *int
	if args.Line != nil && *args.Line > 0 {
		lineVal := *args.Line
		targetLine = &lineVal
	}

	result, err := r.Execute(evalCtx, absPath, content, targetLine, maxLines)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("peek runner error: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	if result.SyntaxError != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("peek syntax error: %s", result.SyntaxError.Msg)},
			},
			IsError: true,
		}, nil, nil
	}

	if result.Error != "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("peek error: %s", result.Error)},
			},
			IsError: true,
		}, nil, nil
	}

	if len(result.Blocks) == 0 {
		msg := "(no output: target line is inert or blank)"
		if targetLine == nil {
			msg = "(no output)"
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: msg},
			},
		}, nil, nil
	}

	var allOutputs []string
	for _, b := range result.Blocks {
		for _, out := range b.Outputs {
			allOutputs = append(allOutputs, out)
		}
	}

	if len(allOutputs) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "(no output)"},
			},
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: strings.Join(allOutputs, "\n")},
		},
	}, nil, nil
}
