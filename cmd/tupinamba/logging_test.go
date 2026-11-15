package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestNewLoggerUsesJSONAndConfiguredLevel(t *testing.T) {
	t.Setenv("TUPINAMBA_ENV", "production")
	var out bytes.Buffer
	logger := newLogger(&out, "debug")
	logger.Debug("diagnostic", "event_name", "test.debug")
	var event map[string]any
	if err := json.Unmarshal(out.Bytes(), &event); err != nil {
		t.Fatalf("invalid JSON log: %v", err)
	}
	if event["level"] != "DEBUG" || event["event_name"] != "test.debug" {
		t.Fatalf("unexpected event: %#v", event)
	}
	if event["service"] != "tupinamba" || event["environment"] != "production" {
		t.Fatalf("missing service field: %#v", event)
	}
	if event["time"].(string)[len(event["time"].(string))-1:] != "Z" {
		t.Fatalf("timestamp is not UTC: %v", event["time"])
	}
	out.Reset()
	newLogger(&out, "invalid").Debug("hidden")
	if out.Len() != 0 {
		t.Fatalf("default level should suppress debug: %s", out.String())
	}
	out.Reset()
	newLoggerWithFormat(&out, "trace", "json").Log(context.Background(), traceLevel, "trace event")
	if !strings.Contains(out.String(), `"level":"TRACE"`) {
		t.Fatalf("TRACE level missing: %s", out.String())
	}
	out.Reset()
	newLoggerWithFormat(&out, "debug", "json").Log(context.Background(), traceLevel, "hidden trace")
	if out.Len() != 0 {
		t.Fatalf("DEBUG configuration should suppress TRACE: %s", out.String())
	}
}

func TestLoggerHonorsEveryConfiguredLevel(t *testing.T) {
	levels := []struct {
		name  string
		value slog.Level
	}{
		{name: "trace", value: traceLevel},
		{name: "debug", value: slog.LevelDebug},
		{name: "info", value: slog.LevelInfo},
		{name: "warn", value: slog.LevelWarn},
		{name: "error", value: slog.LevelError},
	}
	for _, configured := range levels {
		t.Run(configured.name, func(t *testing.T) {
			for _, event := range levels {
				var out bytes.Buffer
				newLogger(&out, configured.name).Log(context.Background(), event.value, "level check")
				if got := out.Len() > 0; got != (event.value >= configured.value) {
					t.Errorf("configured %s with event %s emitted=%v", configured.name, event.name, got)
				}
			}
		})
	}
}

func TestNewCorrelationIDIsUniqueAndDoesNotClaimTraceSemantics(t *testing.T) {
	first, err := newCorrelationID()
	if err != nil {
		t.Fatal(err)
	}
	second, err := newCorrelationID()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 32 || first == second {
		t.Fatalf("invalid or repeated correlation IDs: %q, %q", first, second)
	}
}

func TestLoggerFormatAndFileDestination(t *testing.T) {
	var out bytes.Buffer
	newLoggerWithFormat(&out, "info", "text").Info("human readable")
	if strings.Contains(out.String(), `"msg"`) || !strings.Contains(out.String(), "msg=\"human readable\"") {
		t.Fatalf("unexpected text format: %s", out.String())
	}
	path := t.TempDir() + "/tupinamba.log"
	t.Setenv("TUPINAMBA_LOG_FILE", path)
	logger, closeLogs, err := configuredLogger(&out)
	if err != nil {
		t.Fatal(err)
	}
	logger.Info("file event")
	closeLogs()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("log file permissions = %o", info.Mode().Perm())
	}
}

func TestLoggerFallsBackWhenFileCannotOpen(t *testing.T) {
	path := t.TempDir() + "/missing-parent/app.log"
	t.Setenv("TUPINAMBA_LOG_FILE", path)
	var stderr bytes.Buffer
	logger, closeLogs, err := configuredLogger(&stderr)
	if err == nil {
		t.Fatal("expected log destination error")
	}
	defer closeLogs()
	logger.Warn("log destination unavailable; using stderr", "event_name", "logging.destination.fallback", "error_type", "open")
	if !strings.Contains(stderr.String(), `"event_name":"logging.destination.fallback"`) {
		t.Fatalf("fallback event missing: %s", stderr.String())
	}
}

func TestRotateLogFileCapsRetentionAndKeepsBackupPrivate(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := os.WriteFile(filepath.Join(dir, "app.log"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.log.1"), []byte("stale backup"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := rotateLogFile(root, "app.log"); err != nil {
		t.Fatal(err)
	}
	if backup, err := os.ReadFile(filepath.Join(dir, "app.log.1")); err != nil || string(backup) != "stale backup" {
		t.Fatalf("small file changed backup: %q err=%v", backup, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.log"), bytes.Repeat([]byte{'x'}, maxLogFileSize), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.log.1"), []byte("stale backup"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := rotateLogFile(root, "app.log"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "app.log.1"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != maxLogFileSize || info.Mode().Perm() != 0o600 {
		t.Fatalf("backup size=%d mode=%o", info.Size(), info.Mode().Perm())
	}
	if _, err := os.Stat(filepath.Join(dir, "app.log")); !os.IsNotExist(err) {
		t.Fatalf("current file was not rotated, err=%v", err)
	}
}

func TestLoggerEscapesControlCharactersAndWritesConcurrently(t *testing.T) {
	var out bytes.Buffer
	logger := newLogger(&out, "debug")
	logger.Info("untrusted\nforged", "value", "one\r\ntwo")
	const workers = 32
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			logger.Info("concurrent event", "index", i)
		}(i)
	}
	wg.Wait()
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != workers+1 {
		t.Fatalf("got %d log lines, want %d: %s", len(lines), workers+1, out.String())
	}
	for i, line := range lines {
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("line %d is not valid JSON: %v", i, err)
		}
	}
	if strings.Contains(lines[0], "forged\n") || !strings.Contains(lines[0], `\n`) {
		t.Fatalf("control character was not escaped: %s", lines[0])
	}
}
