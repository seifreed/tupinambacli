package main

import (
	"log/slog"
	"os"
	"strings"
)

func initLogger() {
	level, validLevel := parseLogLevel(os.Getenv("TUPINAMBA_LOG_LEVEL"))
	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	switch strings.ToLower(strings.TrimSpace(os.Getenv("TUPINAMBA_LOG_FORMAT"))) {
	case "", "json":
		handler = slog.NewJSONHandler(os.Stderr, opts)
	case "text":
		handler = slog.NewTextHandler(os.Stderr, opts)
	default:
		handler = slog.NewJSONHandler(os.Stderr, opts)
	}
	logger := slog.New(handler).With("service", "tupinamba")
	slog.SetDefault(logger)
	if !validLevel {
		logger.Warn("unknown log level; using info", "component", "desktop_mcp", "operation", "logger_init", "event_name", "logging.level.invalid")
	}
	format := strings.ToLower(strings.TrimSpace(os.Getenv("TUPINAMBA_LOG_FORMAT")))
	if format != "" && format != "json" && format != "text" {
		logger.Warn("unknown log format; using JSON", "component", "desktop_mcp", "operation", "logger_init", "event_name", "logging.format.invalid")
	}
}

func parseLogLevel(value string) (slog.Level, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "info":
		return slog.LevelInfo, true
	case "trace":
		return slog.Level(-8), true
	case "debug":
		return slog.LevelDebug, true
	case "warn", "warning":
		return slog.LevelWarn, true
	case "error":
		return slog.LevelError, true
	default:
		return slog.LevelInfo, false
	}
}
