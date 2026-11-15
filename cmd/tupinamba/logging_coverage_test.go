package main

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func restoreLogHooks(t *testing.T) {
	t.Helper()
	chmod, stat, open := chmodLogFile, statLogFile, openRotatedLogFile
	close, remove, rename, random := closeRotatedLogFile, removeRotatedLogFile, renameRotatedLogFile, readCorrelationRandom
	t.Cleanup(func() {
		chmodLogFile, statLogFile, openRotatedLogFile = chmod, stat, open
		closeRotatedLogFile, removeRotatedLogFile, renameRotatedLogFile, readCorrelationRandom = close, remove, rename, random
	})
}

func largeLogRoot(t *testing.T) (*os.Root, string) {
	t.Helper()
	dir := t.TempDir()
	name := "app.log"
	if err := os.WriteFile(filepath.Join(dir, name), make([]byte, maxLogFileSize), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	return root, name
}

func TestRotateLogFileReportsOperationErrors(t *testing.T) {
	for _, operation := range []string{"stat", "open", "chmod", "close", "remove", "rename"} {
		t.Run(operation, func(t *testing.T) {
			restoreLogHooks(t)
			root, name := largeLogRoot(t)
			failure := errors.New(operation + " failed")
			switch operation {
			case "stat":
				statLogFile = func(*os.Root, string) (os.FileInfo, error) { return nil, failure }
			case "open":
				openRotatedLogFile = func(*os.Root, string) (*os.File, error) { return nil, failure }
			case "chmod":
				chmodLogFile = func(*os.File) error { return failure }
			case "close":
				closeRotatedLogFile = func(file *os.File) error {
					_ = file.Close()
					return failure
				}
			case "remove":
				removeRotatedLogFile = func(*os.Root, string) error { return failure }
			case "rename":
				renameRotatedLogFile = func(*os.Root, string, string) error { return failure }
			}
			if err := rotateLogFile(root, name); !errors.Is(err, failure) {
				t.Fatalf("rotateLogFile() error = %v", err)
			}
		})
	}
}

func TestRotateLogFileWithoutExistingBackup(t *testing.T) {
	root, name := largeLogRoot(t)
	if err := rotateLogFile(root, name); err != nil {
		t.Fatal(err)
	}
	if _, err := root.Stat(name + ".1"); err != nil {
		t.Fatalf("rotated backup missing: %v", err)
	}
}

func TestConfiguredLoggerReturnsPermissionSetupFailure(t *testing.T) {
	restoreLogHooks(t)
	t.Setenv("TUPINAMBA_LOG_FILE", filepath.Join(t.TempDir(), "app.log"))
	failure := errors.New("chmod failed")
	chmodLogFile = func(*os.File) error { return failure }
	if _, closeLogs, err := configuredLogger(os.Stderr); !errors.Is(err, failure) {
		closeLogs()
		t.Fatalf("configuredLogger() error = %v", err)
	}
}

func TestNewCorrelationIDReturnsRandomSourceError(t *testing.T) {
	restoreLogHooks(t)
	failure := errors.New("random source failed")
	readCorrelationRandom = func([]byte) (int, error) { return 0, failure }
	if _, err := newCorrelationID(); !errors.Is(err, failure) {
		t.Fatalf("newCorrelationID() error = %v", err)
	}
}

func TestRunLogsLoggerInitializationFallback(t *testing.T) {
	t.Setenv("TUPINAMBA_LOG_FILE", filepath.Join(t.TempDir(), "missing", "app.log"))
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	_, runErr := captureStdout(t, func() error {
		if code := run([]string{"version"}); code != 0 {
			return errors.New("version command failed")
		}
		return nil
	})
	_ = w.Close()
	os.Stderr = oldStderr
	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	_ = r.Close()
	if runErr != nil || !strings.Contains(string(output), `"event_name":"logging.destination.fallback"`) {
		t.Fatalf("run error=%v logs=%s", runErr, output)
	}
}
