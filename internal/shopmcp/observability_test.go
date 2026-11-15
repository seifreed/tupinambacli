package shopmcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestToolErrorsProduceStructuredSafeLogs(t *testing.T) {
	previous := slog.Default()
	var buffer bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buffer, nil)).With("service", "tupinamba"))
	t.Cleanup(func() { slog.SetDefault(previous) })
	t.Setenv("TUPINAMBA_ENV", "test")
	New(nil).logTool(context.Background(), "search_products", time.Now(), errors.New("secret-token=do-not-log"))
	var event map[string]any
	if err := json.Unmarshal(buffer.Bytes(), &event); err != nil {
		t.Fatal(err)
	}
	if event["level"] != "WARN" || event["outcome"] != "error" || event["event_name"] != "mcp.tool.completed" || event["environment"] != "test" || event["service"] != "tupinamba" {
		t.Fatalf("unexpected structured event: %#v", event)
	}
	if _, ok := event["duration_ms"]; !ok {
		t.Fatalf("duration missing: %#v", event)
	}
	if strings.Contains(buffer.String(), "do-not-log") || strings.Contains(buffer.String(), "secret-token") {
		t.Fatal("error message leaked to logs")
	}
}
