package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/seifreed/tupinambacli/internal/desktopmcp"
)

var runMCPServer = func(ctx context.Context, transport mcp.Transport) error {
	return desktopmcp.New(newClient()).MCP().Run(ctx, transport)
}

func cmdMCP(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("mcp does not accept arguments")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := runMCPServer(ctx, &mcp.StdioTransport{}); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
