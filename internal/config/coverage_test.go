package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigErrorsAndDefaults(t *testing.T) {
	t.Setenv("TUPINAMBA_CONFIG_DIR", "")
	if !strings.HasSuffix(Dir(), ".tupinamba") {
		t.Fatalf("default dir=%q", Dir())
	}
	oldHome := userHomeDir
	t.Cleanup(func() { userHomeDir = oldHome })
	userHomeDir = func() (string, error) { return "", errors.New("home failed") }
	if Dir() != ".tupinamba" {
		t.Fatal("expected fallback dir")
	}
	if err := Load("missing.json", &struct{}{}); err == nil {
		t.Fatal("expected missing file error")
	}
	configDir := t.TempDir()
	t.Setenv("TUPINAMBA_CONFIG_DIR", configDir)
	if err := os.WriteFile(filepath.Join(configDir, "bad.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Load("bad.json", &struct{}{}); err == nil {
		t.Fatal("expected JSON error")
	}
	fileDir := t.TempDir() + "/file"
	if err := os.WriteFile(fileDir, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TUPINAMBA_CONFIG_DIR", fileDir)
	if err := Save("x.json", struct{}{}); err == nil {
		t.Fatal("expected mkdir error")
	}
	t.Setenv("TUPINAMBA_CONFIG_DIR", t.TempDir())
	if err := Save("x.json", make(chan int)); err == nil {
		t.Fatal("expected marshal error")
	}
}

type tempFileStub struct {
	name     string
	writeErr error
	closeErr error
}

func (f tempFileStub) Name() string              { return f.name }
func (f tempFileStub) Write([]byte) (int, error) { return 0, f.writeErr }
func (f tempFileStub) Close() error              { return f.closeErr }

func TestSaveTemporaryFileErrors(t *testing.T) {
	t.Setenv("TUPINAMBA_CONFIG_DIR", t.TempDir())
	oldCreate := createTempFile
	t.Cleanup(func() { createTempFile = oldCreate })
	createTempFile = func(string, string) (tempFile, error) { return nil, errors.New("create failed") }
	if err := Save("x.json", struct{}{}); err == nil {
		t.Fatal("expected create error")
	}
	createTempFile = func(string, string) (tempFile, error) {
		return tempFileStub{name: "/tmp/config", writeErr: errors.New("write failed")}, nil
	}
	if err := Save("x.json", struct{}{}); err == nil {
		t.Fatal("expected write error")
	}
	createTempFile = func(string, string) (tempFile, error) {
		return tempFileStub{name: "/tmp/config", closeErr: errors.New("close failed")}, nil
	}
	if err := Save("x.json", struct{}{}); err == nil {
		t.Fatal("expected close error")
	}
}
