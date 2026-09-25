package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"peek/internal/config"
)

func setupTestSession(t *testing.T) (*Server, *mcp.ClientSession, func()) {
	t.Helper()
	ctx := context.Background()

	cfg := config.DefaultConfig()
	srv := NewServer(cfg, "0.3.0")

	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	serverSession, err := srv.MCPServer().Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect failed: %v", err)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect failed: %v", err)
	}

	cleanup := func() {
		clientSession.Close()
		serverSession.Wait()
	}

	return srv, clientSession, cleanup
}

func TestMCPServer_ListTools(t *testing.T) {
	ctx := context.Background()
	_, clientSession, cleanup := setupTestSession(t)
	defer cleanup()

	toolsResult, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	if len(toolsResult.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(toolsResult.Tools))
	}

	tool := toolsResult.Tools[0]
	if tool.Name != "peek_eval" {
		t.Errorf("expected tool name 'peek_eval', got %q", tool.Name)
	}
}

func TestMCPServer_ListAndReadResources(t *testing.T) {
	ctx := context.Background()
	_, clientSession, cleanup := setupTestSession(t)
	defer cleanup()

	resourcesResult, err := clientSession.ListResources(ctx, nil)
	if err != nil {
		t.Fatalf("ListResources failed: %v", err)
	}

	if len(resourcesResult.Resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resourcesResult.Resources))
	}

	// Read peek://config
	cfgRes, err := clientSession.ReadResource(ctx, &mcp.ReadResourceParams{URI: "peek://config"})
	if err != nil {
		t.Fatalf("ReadResource(peek://config) failed: %v", err)
	}
	if len(cfgRes.Contents) == 0 || cfgRes.Contents[0].Text == "" {
		t.Errorf("expected non-empty config resource contents")
	}

	// Read peek://environment
	envRes, err := clientSession.ReadResource(ctx, &mcp.ReadResourceParams{URI: "peek://environment"})
	if err != nil {
		t.Fatalf("ReadResource(peek://environment) failed: %v", err)
	}
	if len(envRes.Contents) == 0 || envRes.Contents[0].Text == "" {
		t.Errorf("expected non-empty environment resource contents")
	}
}

func TestMCPServer_CallToolErrors(t *testing.T) {
	ctx := context.Background()
	_, clientSession, cleanup := setupTestSession(t)
	defer cleanup()

	// Missing file argument
	res, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "peek_eval",
		Arguments: map[string]any{"file": ""},
	})
	if err != nil {
		t.Fatalf("unexpected call error: %v", err)
	}
	if !res.IsError {
		t.Errorf("expected IsError=true for empty file")
	}

	// Non-existent file
	res, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "peek_eval",
		Arguments: map[string]any{"file": "non_existent_file_xyz.py"},
	})
	if err != nil {
		t.Fatalf("unexpected call error: %v", err)
	}
	if !res.IsError {
		t.Errorf("expected IsError=true for non-existent file")
	}
}

func TestMCPServer_CallToolExecution(t *testing.T) {
	ctx := context.Background()
	_, clientSession, cleanup := setupTestSession(t)
	defer cleanup()

	// Create temporary Python test file
	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "test_eval.py")
	content := "a = 10\nb = 20\nc = a + b\nc\n"
	if err := os.WriteFile(pyFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Evaluate targeted line (line 4: `c`)
	res, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "peek_eval",
		Arguments: map[string]any{
			"file": pyFile,
			"line": 4,
		},
	})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}
	if res.IsError {
		errMsg := res.Content[0].(*mcp.TextContent).Text
		if strings.Contains(errMsg, "operation not permitted") || strings.Contains(errMsg, "permission denied") {
			t.Skipf("skipping execution test due to environment execution restriction: %v", errMsg)
		}
		t.Fatalf("expected successful evaluation, got error: %v", errMsg)
	}

	textContent, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}
	if textContent.Text != "30" {
		t.Errorf("expected output '30', got %q", textContent.Text)
	}

	// Evaluate full file (line omitted)
	resFull, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "peek_eval",
		Arguments: map[string]any{
			"file": pyFile,
		},
	})
	if err != nil {
		t.Fatalf("CallTool for full file failed: %v", err)
	}
	if resFull.IsError {
		t.Fatalf("expected successful full file evaluation, got error: %v", resFull.Content[0].(*mcp.TextContent).Text)
	}
}
