package browserauth

import (
	"bytes"
	"crypto/pbkdf2"
	"crypto/sha1"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBrowserCookieStoreAndImport(t *testing.T) {
	home := t.TempDir()
	store := filepath.Join(home, "Library/Application Support/Google/Chrome/Default/Network/Cookies")
	if err := os.MkdirAll(filepath.Dir(store), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store, []byte("sqlite"), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "sqlite3")
	key, err := pbkdf2.Key(sha1.New, "secret", []byte("saltysalt"), 1003, 16)
	if err != nil {
		t.Fatal(err)
	}
	encoded := encryptedCookie(t, key, []byte("value"), true)
	script := "#!/bin/sh\nprintf 'capsulastupinamba.com\\tsession\\tvalue\\t00\\ncapsulastupinamba.com\\tencrypted\\t\\t" + encoded + "\\n\\ninvalid\\n'\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	security := filepath.Join(filepath.Dir(bin), "security")
	if err := os.WriteFile(security, []byte("#!/bin/sh\nprintf secret\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("PATH", filepath.Dir(bin)+string(os.PathListSeparator)+os.Getenv("PATH"))
	oldDarwin := isDarwin
	isDarwin = true
	t.Cleanup(func() { isDarwin = oldDarwin })
	got, err := chromeCookiesMac()
	if err != nil || got["session"] != "value" {
		t.Fatalf("cookies=%v err=%v", got, err)
	}
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	session, err := CookiesFromBrowser("")
	if err != nil || !strings.Contains(session, "session=value") {
		t.Fatalf("session=%+v err=%v", session, err)
	}
	if !strings.Contains(logs.String(), `"event_name":"auth.browser_cookie_import.completed"`) || strings.Contains(logs.String(), "session=value") {
		t.Fatalf("missing safe browser import event: %s", logs.String())
	}
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nprintf invalid\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := CookiesFromBrowser("chrome"); err == nil {
		t.Fatal("expected empty cookie error")
	}
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nprintf 'capsulastupinamba.com\\tencrypted\\t\\t00\\n'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(security, []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := CookiesFromBrowser("chrome"); err == nil || !strings.Contains(logs.String(), `"event_name":"auth.browser_cookie_key.unavailable"`) {
		t.Fatalf("expected unavailable key warning, err=%v logs=%s", err, logs.String())
	}
	if err := os.WriteFile(security, []byte("#!/bin/sh\nprintf secret\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := CookiesFromBrowser("chrome"); err == nil || !strings.Contains(logs.String(), `"event_name":"auth.browser_cookie_decryption.failed"`) {
		t.Fatalf("expected decrypt warning, err=%v logs=%s", err, logs.String())
	}
	root, err := os.OpenRoot(home)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	}()
	if got, err := firstExisting(root, "missing", "Library/Application Support/Google/Chrome/Default/Network/Cookies"); err != nil || got == "" {
		t.Fatalf("first existing=%q err=%v", got, err)
	}
	if _, err := firstExisting(root, "missing"); err == nil {
		t.Fatal("expected missing cookie store")
	}
}
