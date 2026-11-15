package shopmcp

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"
)

func (s *Server) logTool(ctx context.Context, tool string, started time.Time, err error) {
	level, outcome := slog.LevelDebug, "success"
	if tool == "plan_cart_change" || tool == "apply_cart_change" {
		level = slog.LevelInfo
	}
	if err != nil {
		level, outcome = slog.LevelWarn, "error"
	}
	attrs := []any{"component", "shop_mcp", "operation", "tool." + tool, "event_name", "mcp.tool.completed", "tool", tool, "duration_ms", time.Since(started).Milliseconds(), "outcome", outcome}
	if environment := os.Getenv("TUPINAMBA_ENV"); environment != "" {
		attrs = append(attrs, "environment", environment)
	}
	if err != nil {
		attrs = append(attrs, "error_type", fmt.Sprintf("%T", err))
	}
	slog.Default().Log(ctx, level, "MCP tool completed", attrs...)
}
