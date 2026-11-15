package main

import (
	"bytes"
	"flag"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/seifreed/tupinambacli/internal/client"
)

func TestInputReadLogsMetadataWithoutPathOrContent(t *testing.T) {
	path := t.TempDir() + "/private-name.txt"
	content := "private-business-reference"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(previous)
	if _, err := readLines(path); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(logs.String(), content) || strings.Contains(logs.String(), path) || !strings.Contains(logs.String(), `"event_name":"input.lines.read"`) {
		t.Fatalf("unexpected input read log: %s", logs.String())
	}
}

func TestRunLogsPanicsWithoutExposingPanicValue(t *testing.T) {
	path := t.TempDir() + "/app.log"
	t.Setenv("TUPINAMBA_LOG_FILE", path)
	oldClient, oldLogger := newClient, slog.Default()
	newClient = func() *client.Client { panic("secret-panic-value") }
	defer func() {
		newClient = oldClient
		slog.SetDefault(oldLogger)
	}()
	if code := run([]string{"whoami"}); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	logs, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logs), `"event_name":"command.panic"`) || !strings.Contains(string(logs), `"outcome":"fatal"`) || !strings.Contains(string(logs), `"stack_trace"`) || !strings.Contains(string(logs), "main.go") {
		t.Fatalf("panic event missing: %s", logs)
	}
	if strings.Contains(string(logs), "secret-panic-value") {
		t.Fatalf("panic value leaked: %s", logs)
	}
}

func TestRunLogsMissingAndUnknownCommandsWithoutEchoingInput(t *testing.T) {
	for _, args := range [][]string{nil, {"secret\nforged"}} {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		previousStderr, previousLogger := os.Stderr, slog.Default()
		os.Stderr = w
		code := run(args)
		os.Stderr = previousStderr
		slog.SetDefault(previousLogger)
		_ = w.Close()
		output, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			t.Fatal(err)
		}
		if code != 2 || strings.Contains(string(output), "secret") || strings.Contains(string(output), "forged") {
			t.Fatalf("code=%d stderr=%q", code, output)
		}
		eventName := `"event_name":"command.missing"`
		if len(args) != 0 {
			eventName = `"event_name":"command.unknown"`
		}
		if !strings.Contains(string(output), eventName) {
			t.Fatalf("missing structured event: %q", output)
		}
	}
}

func TestParseFlagsAfterPositional(t *testing.T) {
	fs := flag.NewFlagSet("x", flag.ContinueOnError)
	limit := fs.Int("limit", 10, "")
	if err := parseFlags(fs, []string{"cafe", "--limit", "3"}); err != nil {
		t.Fatal(err)
	}
	if *limit != 3 {
		t.Fatalf("limit = %d", *limit)
	}
	if fs.Arg(0) != "cafe" {
		t.Fatalf("arg = %q", fs.Arg(0))
	}
}

func TestRunReturnsFlagErrors(t *testing.T) {
	if code := run([]string{"search", "--unknown"}); code != 1 {
		t.Fatalf("exit code = %d", code)
	}
}

func TestMaxFlagUsesEnvironment(t *testing.T) {
	t.Setenv("TUPINAMBA_MAX_EUR", "12.50")
	fs := flag.NewFlagSet("x", flag.ContinueOnError)
	max := addMaxFlag(fs, "")
	if *max != "12.50" {
		t.Fatalf("max = %q", *max)
	}
}

func TestCookieFromHAR(t *testing.T) {
	got, err := cookieFromHAR([]byte(`{
		"log": {"entries": [{
			"request": {"url": "https://capsulastupinamba.com/cart.js", "headers": [
				{"name": "User-Agent", "value": "x"},
				{"name": "Cookie", "value": "cart=abc; localization=ES"}
			]}
		}]}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if got != "cart=abc; localization=ES" {
		t.Fatalf("cookie = %q", got)
	}
}

func TestCookieFromHARRejectsOtherHosts(t *testing.T) {
	_, err := cookieFromHAR([]byte(`{
		"log": {"entries": [{
			"request": {"url": "https://example.com/", "headers": [
				{"name": "Cookie", "value": "session=untrusted"}
			]}
		}]}
	}`))
	if err == nil {
		t.Fatal("expected unrelated HAR cookie to be rejected")
	}
}

func TestCookieFromHARRejectsInsecureRequestCookies(t *testing.T) {
	got, err := cookieFromHAR([]byte(`{
		"log": {"entries": [
			{"request": {"url": "http://capsulastupinamba.com/cart.js", "headers": [
				{"name": "Cookie", "value": "cart=insecure"}
			]}},
			{"request": {"url": "https://capsulastupinamba.com/cart.js", "headers": [
				{"name": "Cookie", "value": "cart=secure"}
			]}}
		]}
	}`))
	if err != nil || got != "cart=secure" {
		t.Fatalf("cookieFromHAR() = %q, %v", got, err)
	}
	if _, err := cookieFromHAR([]byte(`{
		"log": {"entries": [{"request": {"url": "http://capsulastupinamba.com/cart.js", "headers": [
			{"name": "Cookie", "value": "cart=insecure"}
		]}}]}
	}`)); err == nil {
		t.Fatal("accepted cookie from insecure HAR request")
	}
}
