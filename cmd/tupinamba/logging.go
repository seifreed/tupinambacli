package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gofrs/flock"
)

const maxLogFileSize = 10 << 20
const traceLevel slog.Level = -8

var chmodLogFile = func(file *os.File) error { return file.Chmod(0o600) }
var statLogFile = func(root *os.Root, name string) (os.FileInfo, error) { return root.Stat(name) }
var openRotatedLogFile = func(root *os.Root, name string) (*os.File, error) { return root.Open(name) }
var closeRotatedLogFile = func(file *os.File) error { return file.Close() }
var removeRotatedLogFile = func(root *os.Root, name string) error { return root.Remove(name) }
var renameRotatedLogFile = func(root *os.Root, oldName, newName string) error {
	return root.Rename(oldName, newName)
}
var readCorrelationRandom = rand.Read

type fallbackWriter struct {
	primary  io.Writer
	fallback io.Writer
	mu       sync.Mutex
}

func (w *fallbackWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if n, err := w.primary.Write(p); n == len(p) && err == nil {
		return n, nil
	}
	return w.fallback.Write(p)
}

func newLogger(w io.Writer, configuredLevel string) *slog.Logger {
	return newLoggerWithFormat(w, configuredLevel, "json")
}

func newLoggerWithFormat(w io.Writer, configuredLevel, format string) *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(configuredLevel) {
	case "trace":
		level = traceLevel
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	opts := &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.TimeKey {
				attr.Value = slog.TimeValue(attr.Value.Time().UTC())
			}
			if attr.Key == slog.LevelKey && attr.Value.Any() == traceLevel {
				attr.Value = slog.StringValue("TRACE")
			}
			return attr
		},
	}
	var handler slog.Handler = slog.NewJSONHandler(w, opts)
	if strings.EqualFold(format, "text") {
		handler = slog.NewTextHandler(w, opts)
	}
	logger := slog.New(handler).With("service", "tupinamba")
	if environment := os.Getenv("TUPINAMBA_ENV"); environment != "" {
		logger = logger.With("environment", environment)
	}
	return logger
}

func configuredLogger(stderr io.Writer) (*slog.Logger, func(), error) {
	w := stderr
	fallback := func(err error) (*slog.Logger, func(), error) {
		return newLoggerWithFormat(w, os.Getenv("TUPINAMBA_LOG_LEVEL"), os.Getenv("TUPINAMBA_LOG_FORMAT")), func() {}, err
	}
	var file *os.File
	var root *os.Root
	var fileLock *flock.Flock
	if path := os.Getenv("TUPINAMBA_LOG_FILE"); path != "" {
		var err error
		root, err = os.OpenRoot(filepath.Dir(path))
		if err != nil {
			return fallback(err)
		}
		name := filepath.Base(path)
		fileLock = flock.New(path+".lock", flock.SetPermissions(0o600))
		locked, err := fileLock.TryLock()
		if err != nil || !locked {
			_ = fileLock.Close()
			_ = root.Close()
			if err == nil {
				err = errors.New("log destination is already in use")
			}
			return fallback(err)
		}
		if err := rotateLogFile(root, name); err != nil {
			_ = fileLock.Close()
			_ = root.Close()
			return fallback(err)
		}
		opened, err := root.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			_ = fileLock.Close()
			_ = root.Close()
			return fallback(err)
		}
		if err := chmodLogFile(opened); err != nil {
			_ = opened.Close()
			_ = fileLock.Close()
			_ = root.Close()
			return fallback(err)
		}
		file, w = opened, opened
	}
	if file != nil {
		w = &fallbackWriter{primary: file, fallback: stderr}
	}
	logger := newLoggerWithFormat(w, os.Getenv("TUPINAMBA_LOG_LEVEL"), os.Getenv("TUPINAMBA_LOG_FORMAT"))
	if file == nil {
		return logger, func() {}, nil
	}
	return logger, func() { _ = file.Close(); _ = root.Close(); _ = fileLock.Close() }, nil
}

func rotateLogFile(root *os.Root, name string) error {
	info, err := statLogFile(root, name)
	if errors.Is(err, os.ErrNotExist) || err == nil && info.Size() < maxLogFileSize {
		return nil
	}
	if err != nil {
		return err
	}
	current, err := openRotatedLogFile(root, name)
	if err != nil {
		return err
	}
	chmodErr := chmodLogFile(current)
	closeErr := closeRotatedLogFile(current)
	if chmodErr != nil {
		return chmodErr
	}
	if closeErr != nil {
		return closeErr
	}
	backup := name + ".1"
	if err := removeRotatedLogFile(root, backup); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := renameRotatedLogFile(root, name, backup); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func newCorrelationID() (string, error) {
	var id [16]byte
	if _, err := readCorrelationRandom(id[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(id[:]), nil
}
