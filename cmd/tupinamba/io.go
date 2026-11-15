package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type readRoot interface {
	ReadFile(string) ([]byte, error)
	Close() error
}

var openReadRoot = func(path string) (readRoot, error) { return os.OpenRoot(path) }

func readLines(path string) (out []string, result error) {
	started := time.Now()
	var b []byte
	source := "file"
	if path == "-" {
		source = "stdin"
		b, result = io.ReadAll(os.Stdin)
	} else {
		b, result = readPathFile(path)
	}
	if result != nil {
		logInputRead("input.lines.read", source, len(b), time.Since(started), result)
		return nil, result
	}
	defer func() { logInputRead("input.lines.read", source, len(b), time.Since(started), result) }()
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(strings.Split(line, "#")[0])
		if line != "" {
			out = append(out, line)
		}
	}
	return out, nil
}

func readPathFile(path string) ([]byte, error) {
	root, err := openReadRoot(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	data, err := root.ReadFile(filepath.Base(path))
	if closeErr := root.Close(); err == nil {
		err = closeErr
	}
	return data, err
}

func logInputRead(event, source string, size int, duration time.Duration, err error) {
	level, outcome := slog.LevelDebug, "success"
	attrs := []any{"component", "cli", "operation", "input_read", "event_name", event, "source", source, "bytes_read", size, "duration_ms", duration.Milliseconds()}
	if err != nil {
		level, outcome = slog.LevelWarn, "error"
		attrs = append(attrs, "error_type", fmt.Sprintf("%T", err))
	}
	attrs = append(attrs, "outcome", outcome)
	slog.Default().Log(context.Background(), level, "input read completed", attrs...)
}
