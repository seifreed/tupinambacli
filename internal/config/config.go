package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

var userHomeDir = os.UserHomeDir

type tempFile interface {
	Name() string
	Write([]byte) (int, error)
	Close() error
}

var createTempFile = func(dir, pattern string) (tempFile, error) {
	return os.CreateTemp(dir, pattern)
}

func Dir() string {
	if d := os.Getenv("TUPINAMBA_CONFIG_DIR"); d != "" {
		return d
	}
	home, err := userHomeDir()
	if err != nil || home == "" {
		return ".tupinamba"
	}
	return filepath.Join(home, ".tupinamba")
}

func Load(name string, v any) (result error) {
	started := time.Now()
	defer func() { logConfigOperation("load", name, started, result) }()
	path, err := configPath(name)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := root.Close(); result == nil {
			result = closeErr
		}
	}()
	b, err := root.ReadFile(filepath.Base(path))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func Save(name string, v any) (result error) {
	started := time.Now()
	defer func() { logConfigOperation("save", name, started, result) }()
	dir := Dir()
	path, err := configPath(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := createTempFile(dir, name+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func logConfigOperation(operation, name string, started time.Time, err error) {
	outcome := "success"
	if err != nil {
		outcome = "error"
	}
	attrs := []any{"component", "config", "operation", "config." + operation, "event_name", "config." + operation, "config_name", name, "duration_ms", time.Since(started).Milliseconds(), "outcome", outcome}
	if err != nil {
		attrs = append(attrs, "error_type", fmt.Sprintf("%T", err))
	}
	slog.Debug("configuration file operation completed", attrs...)
}

func configPath(name string) (string, error) {
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name {
		return "", fmt.Errorf("invalid config file name %q", name)
	}
	return filepath.Join(Dir(), name), nil
}
