package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/seifreed/tupinambacli/internal/client"
)

func TestFileStateStoreRoundTrip(t *testing.T) {
	t.Setenv("TUPINAMBA_CONFIG_DIR", t.TempDir())
	store := fileStateStore{}
	wantSession := client.Session{Cookie: "secret-cookie"}
	if err := store.SaveSession(wantSession); err != nil {
		t.Fatal(err)
	}
	gotSession, err := store.LoadSession()
	if err != nil || gotSession != wantSession {
		t.Fatalf("session=%#v err=%v", gotSession, err)
	}
	wantCache := client.CatalogCache{SavedAt: time.Unix(100, 0).UTC(), Products: []client.Product{{ID: 2, Handle: "coffee"}}}
	if err := store.SaveCatalogCache(wantCache); err != nil {
		t.Fatal(err)
	}
	gotCache, err := store.LoadCatalogCache()
	if err != nil || !gotCache.SavedAt.Equal(wantCache.SavedAt) || len(gotCache.Products) != 1 || gotCache.Products[0].Handle != "coffee" {
		t.Fatalf("cache=%#v err=%v", gotCache, err)
	}
}

func TestRunHandlesExitAndCancellation(t *testing.T) {
	oldServer, oldLogger := runServer, slog.Default()
	t.Cleanup(func() {
		runServer = oldServer
		slog.SetDefault(oldLogger)
	})
	cases := []struct {
		name string
		err  error
		want int
	}{
		{name: "normal exit", want: 0},
		{name: "canceled", err: context.Canceled, want: 0},
		{name: "failure", err: errors.New("transport failure"), want: 1},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("TUPINAMBA_LOG_FORMAT", "text")
			runServer = func(ctx context.Context, shop *client.Client, transport mcp.Transport) error {
				if shop == nil {
					t.Fatal("server received a nil shop client")
				}
				if _, ok := transport.(*mcp.StdioTransport); !ok {
					t.Fatalf("transport=%T", transport)
				}
				return test.err
			}
			if got := run(); got != test.want {
				t.Fatalf("run()=%d, want %d", got, test.want)
			}
		})
	}
}

func TestMainUsesExitFunction(t *testing.T) {
	oldServer, oldExit, oldLogger := runServer, exit, slog.Default()
	t.Cleanup(func() {
		runServer, exit = oldServer, oldExit
		slog.SetDefault(oldLogger)
	})
	t.Setenv("TUPINAMBA_LOG_LEVEL", "error")
	runServer = func(context.Context, *client.Client, mcp.Transport) error { return nil }
	called := false
	exit = func(code int) {
		called = true
		if code != 0 {
			t.Fatalf("exit code=%d", code)
		}
	}
	main()
	if !called {
		t.Fatal("main did not call the exit function")
	}
}

func TestParseLogLevel(t *testing.T) {
	for _, test := range []struct {
		value string
		want  slog.Level
		valid bool
	}{
		{"", slog.LevelInfo, true},
		{"trace", slog.Level(-8), true},
		{"DEBUG", slog.LevelDebug, true},
		{"warn", slog.LevelWarn, true},
		{"warning", slog.LevelWarn, true},
		{"error", slog.LevelError, true},
		{"nope", slog.LevelInfo, false},
	} {
		got, valid := parseLogLevel(test.value)
		if got != test.want || valid != test.valid {
			t.Errorf("parseLogLevel(%q)=(%v,%v), want (%v,%v)", test.value, got, valid, test.want, test.valid)
		}
	}
}

func TestInitLoggerDefaultsAndWarnsOnInvalidEnvironment(t *testing.T) {
	oldStderr, oldLogger := os.Stderr, slog.Default()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = writer
	t.Setenv("TUPINAMBA_LOG_LEVEL", "invalid")
	t.Setenv("TUPINAMBA_LOG_FORMAT", "yaml")
	initLogger()
	slog.Info("test event", "event_name", "test.event")
	_ = writer.Close()
	os.Stderr = oldStderr
	slog.SetDefault(oldLogger)
	data, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"logging.level.invalid", "logging.format.invalid", `"event_name":"test.event"`} {
		if !strings.Contains(text, want) {
			t.Errorf("logger output missing %q: %s", want, text)
		}
	}
}

func TestStateStoreUsesLogicalFileNames(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TUPINAMBA_CONFIG_DIR", dir)
	if err := (fileStateStore{}).SaveSession(client.Session{Cookie: "hidden"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, sessionFileName)); err != nil {
		t.Fatal(err)
	}
}

func TestRunServerUsesMCPTransport(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	done := make(chan error, 1)
	go func() { done <- runServer(ctx, client.New(fileStateStore{}), serverTransport) }()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "integration-test", Version: "1"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
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

func TestClaudeDesktopStdioProcessListsTools(t *testing.T) {
	t.Setenv("TUPINAMBA_CONFIG_DIR", t.TempDir())
	t.Setenv("TUPINAMBA_LOG_LEVEL", "error")
	transport := &mcp.CommandTransport{Command: exec.Command("go", "run", ".")}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "desktop-smoke", Version: "1"}, nil).Connect(context.Background(), transport, nil)
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
	for _, name := range []string{"search_products", "calculate_total", "cart_status", "plan_cart_change", "apply_cart_change"} {
		if !found[name] {
			t.Errorf("stdio server did not expose %s", name)
		}
	}
}
