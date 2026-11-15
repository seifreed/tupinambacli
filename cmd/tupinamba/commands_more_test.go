package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/seifreed/tupinambacli/internal/client"
)

func TestSaveSessionLogsSourceWithoutCookie(t *testing.T) {
	t.Setenv("TUPINAMBA_CONFIG_DIR", t.TempDir())
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	defer slog.SetDefault(previous)
	if err := saveSession(client.Session{Cookie: "session=secret"}, "stdin"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs.String(), `"event_name":"auth.session.saved"`) || !strings.Contains(logs.String(), `"source":"stdin"`) {
		t.Fatalf("missing safe session event: %s", logs.String())
	}
	if strings.Contains(logs.String(), "session=secret") {
		t.Fatalf("session value leaked: %s", logs.String())
	}
}

func replaceStdin(t *testing.T, content string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString(content); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = old; _ = r.Close() })
}

func TestAuthCommands(t *testing.T) {
	t.Setenv("TUPINAMBA_CONFIG_DIR", t.TempDir())
	oldBrowser := cookiesFromBrowser
	cookiesFromBrowser = func(string) (string, error) { return "cart=browser", nil }
	t.Cleanup(func() { cookiesFromBrowser = oldBrowser })
	if err := cmdLogin([]string{"--from-browser", "chrome"}); err != nil {
		t.Fatal(err)
	}
	if err := cmdSetCookie([]string{"cart=secret"}); err == nil || strings.Contains(err.Error(), "cart=secret") {
		t.Fatalf("set-cookie accepted or exposed a command-line secret: %v", err)
	}
	replaceStdin(t, " \n")
	if err := cmdSetCookie([]string{"--stdin"}); err == nil {
		t.Fatal("expected empty cookie error")
	}
	replaceStdin(t, "cart=stdin\n")
	if err := cmdSetCookie([]string{"--stdin"}); err != nil {
		t.Fatal(err)
	}
	closed, _, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_ = closed.Close()
	oldStdin := os.Stdin
	os.Stdin = closed
	if err := cmdSetCookie([]string{"--stdin"}); err == nil {
		t.Fatal("expected stdin read error")
	}
	os.Stdin = oldStdin
	file := t.TempDir() + "/capture.har"
	har := `{"log":{"entries":[{"request":{"url":"https://capsulastupinamba.com/cart.js","headers":[{"name":"Cookie","value":"cart=har"}]}}]}}`
	if err := os.WriteFile(file, []byte(har), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cmdImportHAR([]string{"--file", file}); err != nil {
		t.Fatal(err)
	}
	badHAR := t.TempDir() + "/bad.har"
	if err := os.WriteFile(badHAR, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cmdImportHAR([]string{"--file", badHAR}); err == nil {
		t.Fatal("expected HAR cookie error")
	}
	if err := cmdImportHAR([]string{"--file", t.TempDir() + "/missing.har"}); err == nil {
		t.Fatal("expected HAR file error")
	}
	if err := saveSession(client.Session{}, "test"); err != nil {
		t.Fatal(err)
	}
	badDir := t.TempDir() + "/not-a-dir"
	if err := os.WriteFile(badDir, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TUPINAMBA_CONFIG_DIR", badDir)
	if err := saveSession(client.Session{}, "test"); err == nil {
		t.Fatal("expected save error")
	}
	if err := cmdWhoami([]string{"extra"}); err == nil {
		t.Fatal("expected whoami usage error")
	}
}

func TestHARAndHostValidation(t *testing.T) {
	for _, raw := range []string{`{`, `{"log":{"entries":[{"request":{"url":"://bad"}}]}}`, `{"log":{"entries":[]}}`} {
		if _, err := cookieFromHAR([]byte(raw)); err == nil {
			t.Fatalf("cookieFromHAR(%q) succeeded", raw)
		}
	}
	if !isTupinambaHost("capsulastupinamba.com") || !isTupinambaHost("shop.capsulastupinamba.com.") {
		t.Fatal("expected Tupinamba hosts to pass")
	}
	if isTupinambaHost("capsulastupinamba.com.example.com") {
		t.Fatal("expected unrelated host to fail")
	}
}

func TestCartCommands(t *testing.T) {
	useTestClient(t, http.HandlerFunc(fullHandler))
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"add", "alpha", "2", "--json"}, "item_count"},
		{[]string{"set", "alpha", "0", "--variant", "SKU-1", "--toon"}, "item_count"},
		{[]string{"clear"}, "item_count"},
		{[]string{"get"}, "item_count"},
	} {
		out, err := captureStdout(t, func() error { return cmdCart(tc.args) })
		if err != nil || !strings.Contains(out, tc.want) {
			t.Fatalf("cart %v output=%q err=%v", tc.args, out, err)
		}
	}
	if err := cmdCart([]string{"add", "alpha", "1", "--max", "0.01"}); err == nil {
		t.Fatal("expected line max error")
	}
	if err := cmdCart([]string{"add", "alpha", "1", "--max", "0"}); err == nil {
		t.Fatal("expected zero line max error")
	}
	if err := cmdCart([]string{"add", "alpha", "nope"}); err == nil {
		t.Fatal("expected quantity error")
	}
	if err := cmdCart([]string{"add", "missing", "1", "--max", "10"}); err == nil {
		t.Fatal("expected variant resolution error")
	}
	if err := cmdCart([]string{"add", "alpha", "--max", "10"}); err == nil {
		t.Fatal("expected cart usage error")
	}
	if err := cmdCart([]string{"add", "alpha", strconv.Itoa(int(^uint(0) >> 1)), "--max", "999999999"}); err == nil {
		t.Fatal("expected line arithmetic error")
	}
	if _, err := finishCart(nil, errors.New("failed"), outputFlags{}); err == nil {
		t.Fatal("expected cart error")
	}
	if checkoutStatus(true) != "ready" || checkoutStatus(false) != "empty_cart" {
		t.Fatal("unexpected checkout status")
	}
}

func TestCartWriteRejectsInvalidMaximumAndPrice(t *testing.T) {
	if _, err := cartWrite(nil, "add", []string{"alpha", "1", "--max", "invalid"}); err == nil {
		t.Fatal("expected invalid maximum")
	}
	useTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/products/alpha.js" {
			jsonResponse(w, client.Product{Title: "Alpha", Variants: []client.Variant{{ID: 1, Available: true, Price: json.RawMessage(`"bad"`)}}})
			return
		}
		t.Fatalf("unexpected request %s", r.URL.Path)
	}))
	if _, err := cartWrite(newClient(), "add", []string{"alpha", "1", "--max", "10"}); err == nil {
		t.Fatal("expected invalid price")
	}
}

func TestOutputAndInputHelpers(t *testing.T) {
	if got := truncate("short", 10); got != "short" || truncate("long", 2) != "lo..." {
		t.Fatal("unexpected truncation")
	}
	if got, want := truncate(strings.Repeat("a", 499)+"éclair", 500), strings.Repeat("a", 499)+"é..."; got != want {
		t.Fatalf("unicode truncation = %q, want %q", got, want)
	}
	if got := truncate("text", -1); got != "..." {
		t.Fatalf("negative truncation = %q", got)
	}
	replaceStdin(t, "alpha # comment\n\n beta\n")
	lines, err := readLines("-")
	if err != nil || len(lines) != 2 || lines[1] != "beta" {
		t.Fatalf("lines=%v err=%v", lines, err)
	}
	if err := emitTOON(nil); err != nil {
		t.Fatal(err)
	}
	if err := emitTOON(make(chan int)); err == nil {
		t.Fatal("expected JSON marshal error")
	}
	if err := emitTOON(json.Number("1e1000")); err == nil {
		t.Fatal("expected JSON conversion error")
	}
	oldMarshal := marshalTOON
	marshalTOON = func(any) (string, error) { return "", errors.New("toon error") }
	if err := emitTOON(nil); err == nil {
		t.Fatal("expected TOON marshal error")
	}
	marshalTOON = oldMarshal
}
