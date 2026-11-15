package config

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("TUPINAMBA_CONFIG_DIR", t.TempDir())
	in := struct {
		Cookie string `json:"cookie"`
	}{Cookie: "cart=abc"}
	if err := Save("session.json", in); err != nil {
		t.Fatal(err)
	}
	var out struct {
		Cookie string `json:"cookie"`
	}
	if err := Load("session.json", &out); err != nil {
		t.Fatal(err)
	}
	if out.Cookie != in.Cookie {
		t.Fatalf("cookie = %q", out.Cookie)
	}
	info, err := os.Stat(filepath.Join(Dir(), "session.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v", info.Mode().Perm())
	}
}

func TestConfigOperationsLogMetadataWithoutContents(t *testing.T) {
	t.Setenv("TUPINAMBA_CONFIG_DIR", t.TempDir())
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(previous)
	secret := struct {
		Token string `json:"token"`
	}{Token: "private-token-value"}
	if err := Save("credentials.json", secret); err != nil {
		t.Fatal(err)
	}
	var loaded struct {
		Token string `json:"token"`
	}
	if err := Load("credentials.json", &loaded); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs.String(), `"event_name":"config.save"`) || !strings.Contains(logs.String(), `"event_name":"config.load"`) {
		t.Fatalf("missing config operation events: %s", logs.String())
	}
	if strings.Contains(logs.String(), secret.Token) {
		t.Fatalf("configuration content leaked into logs: %s", logs.String())
	}
}

func TestConfigFailureLogPreservesFileNameAndErrorOutcome(t *testing.T) {
	t.Setenv("TUPINAMBA_CONFIG_DIR", t.TempDir())
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	if err := Load("missing.json", &struct{}{}); err == nil {
		t.Fatal("expected missing config error")
	}
	var event map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &event); err != nil {
		t.Fatalf("decode config log: %v", err)
	}
	if event["config_name"] != "missing.json" || event["outcome"] != "error" || event["error_type"] == nil {
		t.Fatalf("unexpected config failure event: %v", event)
	}
}

func TestRejectsConfigPathTraversal(t *testing.T) {
	t.Setenv("TUPINAMBA_CONFIG_DIR", t.TempDir())
	var out struct{}
	if err := Load("../session.json", &out); err == nil {
		t.Fatal("expected traversal path to be rejected")
	}
	if err := Save("../session.json", out); err == nil {
		t.Fatal("expected traversal path to be rejected")
	}
}
