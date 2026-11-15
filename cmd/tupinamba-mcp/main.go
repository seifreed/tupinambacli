package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/seifreed/tupinambacli/internal/client"
	"github.com/seifreed/tupinambacli/internal/config"
	"github.com/seifreed/tupinambacli/internal/desktopmcp"
)

const sessionFileName = "session.json"
const catalogCacheFileName = "catalog.json"

var exit = os.Exit
var runServer = func(ctx context.Context, shop *client.Client, transport mcp.Transport) error {
	return desktopmcp.New(shop).MCP().Run(ctx, transport)
}

type fileStateStore struct{}

func (fileStateStore) LoadSession() (client.Session, error) {
	var session client.Session
	err := config.Load(sessionFileName, &session)
	return session, err
}

func (fileStateStore) SaveSession(session client.Session) error {
	return config.Save(sessionFileName, session)
}

func (fileStateStore) LoadCatalogCache() (client.CatalogCache, error) {
	var cache client.CatalogCache
	err := config.Load(catalogCacheFileName, &cache)
	return cache, err
}

func (fileStateStore) SaveCatalogCache(cache client.CatalogCache) error {
	return config.Save(catalogCacheFileName, cache)
}

func main() { exit(run()) }

func run() int {
	initLogger()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	slog.Info("Claude Desktop MCP server started", "component", "desktop_mcp", "operation", "server_start", "event_name", "mcp.server.started")
	if err := runServer(ctx, client.New(fileStateStore{}), &mcp.StdioTransport{}); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("Claude Desktop MCP server stopped", "component", "desktop_mcp", "operation", "serve", "event_name", "mcp.server.failed", "error_type", fmt.Sprintf("%T", err))
		return 1
	}
	slog.Info("Claude Desktop MCP server stopped", "component", "desktop_mcp", "operation", "server_stop", "event_name", "mcp.server.stopped")
	return 0
}
