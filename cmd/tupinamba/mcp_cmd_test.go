package main

import (
	"context"
	"errors"
	"os/exec"
	"time"

	"github.com/seifreed/tupinambacli/internal/client"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestCmdMCPValidatesArgsAndHandlesServerResults(t *testing.T) {
	previous := runMCPServer
	t.Cleanup(func() { runMCPServer = previous })
	calls := 0
	runMCPServer = func(ctx context.Context, transport mcp.Transport) error {
		calls++
		if _, ok := transport.(*mcp.StdioTransport); !ok {
			t.Fatalf("transport=%T", transport)
		}
		return nil
	}
	if err := cmdMCP([]string{"unexpected"}); err == nil {
		t.Fatal("accepted extra arguments")
	}
	if calls != 0 {
		t.Fatal("started server with invalid arguments")
	}
	if err := cmdMCP(nil); err != nil {
		t.Fatalf("normal stop: %v", err)
	}
	if code := run([]string{"mcp"}); code != 0 {
		t.Fatalf("mcp command exit code=%d", code)
	}
	runMCPServer = func(context.Context, mcp.Transport) error { return context.Canceled }
	if err := cmdMCP(nil); err != nil {
		t.Fatalf("cancellation: %v", err)
	}
	want := errors.New("server failure")
	runMCPServer = func(context.Context, mcp.Transport) error { return want }
	if err := cmdMCP(nil); !errors.Is(err, want) {
		t.Fatalf("error=%v, want %v", err, want)
	}
}

func TestTupinambaBinaryExposesMCPToolsOverStdio(t *testing.T) {
	t.Setenv("TUPINAMBA_CONFIG_DIR", t.TempDir())
	t.Setenv("TUPINAMBA_LOG_LEVEL", "error")
	transport := &mcp.CommandTransport{Command: exec.Command("go", "run", ".", "mcp")}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "codex-smoke", Version: "1"}, nil).Connect(context.Background(), transport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close MCP session: %v", err)
		}
	})
	found := map[string]bool{}
	for tool, err := range session.Tools(context.Background(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		found[tool.Name] = true
	}
	for _, name := range []string{"search_products", "get_product", "cart_status", "calculate_total", "plan_cart_change", "apply_cart_change"} {
		if !found[name] {
			t.Errorf("tupinamba mcp did not expose %s", name)
		}
	}
}

func TestRunMCPServerUsesSDKTransport(t *testing.T) {
	previousClient := newClient
	newClient = func() *client.Client { return &client.Client{} }
	t.Cleanup(func() { newClient = previousClient })
	ctx, cancel := context.WithCancel(context.Background())
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	done := make(chan error, 1)
	go func() { done <- runMCPServer(ctx, serverTransport) }()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "mcp-server-test", Version: "1"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, err := range session.Tools(ctx, nil) {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("server returned %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("MCP server did not stop")
	}
}

func TestRunVersionCommand(t *testing.T) {
	if code := run([]string{"version"}); code != 0 {
		t.Fatalf("version exit code=%d", code)
	}
}
