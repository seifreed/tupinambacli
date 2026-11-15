package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

func TestLoggerRejectsConcurrentFileOwnerAndReleasesLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	t.Setenv("TUPINAMBA_LOG_FILE", path)
	var firstStderr bytes.Buffer
	first, closeFirst, err := configuredLogger(&firstStderr)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFirst()
	first.Info("log destination acquired")
	lockInfo, err := os.Stat(path + ".lock")
	if err != nil || lockInfo.Mode().Perm() != 0o600 {
		t.Fatalf("lock file mode=%v err=%v", lockInfo, err)
	}

	var secondStderr bytes.Buffer
	second, closeSecond, err := configuredLogger(&secondStderr)
	if err == nil {
		closeSecond()
		t.Fatal("concurrent logger unexpectedly acquired the file")
	}
	second.Warn("log destination unavailable; using stderr", "event_name", "logging.destination.fallback")
	closeSecond()
	if !strings.Contains(secondStderr.String(), `"event_name":"logging.destination.fallback"`) {
		t.Fatalf("fallback event missing: %s", secondStderr.String())
	}

	closeFirst()
	third, closeThird, err := configuredLogger(&firstStderr)
	if err != nil {
		t.Fatalf("logger did not reacquire released file: %v", err)
	}
	third.Info("file lock reacquired")
	closeThird()
}

func TestLoggerFallsBackWhenRotationFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	if err := os.WriteFile(path, bytes.Repeat([]byte{'x'}, maxLogFileSize), 0o600); err != nil {
		t.Fatal(err)
	}
	backup := path + ".1"
	if err := os.Mkdir(backup, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backup, "keep"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TUPINAMBA_LOG_FILE", path)
	var stderr bytes.Buffer
	logger, closeLogs, err := configuredLogger(&stderr)
	if err == nil {
		t.Fatal("expected rotation error")
	}
	defer closeLogs()
	logger.Warn("log destination unavailable; using stderr", "event_name", "logging.destination.fallback")
	if !strings.Contains(stderr.String(), `"event_name":"logging.destination.fallback"`) {
		t.Fatalf("fallback event missing: %s", stderr.String())
	}
}

func TestLoggerFallsBackWhenDestinationIsDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TUPINAMBA_LOG_FILE", path)
	var stderr bytes.Buffer
	logger, closeLogs, err := configuredLogger(&stderr)
	if err == nil {
		t.Fatal("expected file destination error")
	}
	defer closeLogs()
	logger.Warn("log destination unavailable; using stderr", "event_name", "logging.destination.fallback")
	if !strings.Contains(stderr.String(), `"event_name":"logging.destination.fallback"`) {
		t.Fatalf("fallback event missing: %s", stderr.String())
	}
}

func TestLoggerFallsBackWhenFileWriteFails(t *testing.T) {
	for name, primary := range map[string]io.Writer{"failed": failedWriter{}, "short": shortWriter{}} {
		t.Run(name, func(t *testing.T) {
			var stderr bytes.Buffer
			logger := newLogger(&fallbackWriter{primary: primary, fallback: &stderr}, "info")
			logger.Warn("disk write failed", "event_name", "logging.write.fallback")
			if !strings.Contains(stderr.String(), `"event_name":"logging.write.fallback"`) {
				t.Fatalf("write fallback missing: %s", stderr.String())
			}
		})
	}
}
