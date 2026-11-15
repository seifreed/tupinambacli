package browserauth

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func encryptedCookie(t *testing.T, key, plain []byte, prefix bool) string {
	t.Helper()
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	plain = append(plain, bytes.Repeat([]byte{byte(block.BlockSize() - len(plain)%block.BlockSize())}, block.BlockSize()-len(plain)%block.BlockSize())...)
	out := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, bytes.Repeat([]byte(" "), block.BlockSize())).CryptBlocks(out, plain)
	if prefix {
		out = append([]byte("v10"), out...)
	}
	return hex.EncodeToString(out)
}
func TestBrowserCookieHelpers(t *testing.T) {
	oldDarwin := isDarwin
	isDarwin = false
	if _, err := CookiesFromBrowser("chrome"); err == nil {
		t.Fatal("expected non-macOS error")
	}
	isDarwin = oldDarwin
	if _, err := CookiesFromBrowser("firefox"); err == nil {
		t.Fatal("expected unsupported browser error")
	}
	if _, err := chromeSafeStorageKey(); err == nil && runtime.GOOS != "darwin" {
		t.Fatal("expected security command error")
	}
	key := bytes.Repeat([]byte{1}, 16)
	host := "capsulastupinamba.com"
	hashed := sha256.Sum256([]byte(host))
	badPaddingBlock, _ := aes.NewCipher(key)
	badPadding := bytes.Repeat([]byte{'x'}, badPaddingBlock.BlockSize())
	badPadding[len(badPadding)-1] = 0
	badPaddingRaw := make([]byte, len(badPadding))
	cipher.NewCBCEncrypter(badPaddingBlock, bytes.Repeat([]byte(" "), badPaddingBlock.BlockSize())).CryptBlocks(badPaddingRaw, badPadding)
	for _, tc := range []struct {
		name string
		raw  string
	}{
		{"hex", "bad"},
		{"key", encryptedCookie(t, bytes.Repeat([]byte{2}, 16), []byte("x"), false)},
		{"length", hex.EncodeToString([]byte{1})},
		{"padding", hex.EncodeToString(badPaddingRaw)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := decryptChromeCookie(tc.raw, key[:], ""); err == nil {
				t.Fatal("expected decrypt error")
			}
		})
	}
	if _, err := decryptChromeCookie("00", []byte("short"), ""); err == nil {
		t.Fatal("expected cipher key error")
	}
	if got, err := decryptChromeCookie(encryptedCookie(t, key, []byte("value"), true), key, host); err != nil || got != "value" {
		t.Fatalf("cookie=%q err=%v", got, err)
	}
	if got, err := decryptChromeCookie(encryptedCookie(t, key, append(hashed[:], []byte("value")...), false), key, host); err != nil || got != "value" {
		t.Fatalf("hashed cookie=%q err=%v", got, err)
	}
	for _, value := range []string{"", "ok", "bad;cookie", "bad cookie", "bad\n"} {
		ok := validCookieValue(value)
		if ok != (value == "" || value == "ok") {
			t.Fatalf("validCookieValue(%q)=%v", value, ok)
		}
	}
}
func TestChromeCookieStoreErrors(t *testing.T) {
	oldCreate, oldWrite := createCookieTempDir, writeCookieStore
	t.Cleanup(func() { createCookieTempDir, writeCookieStore = oldCreate, oldWrite })
	home := t.TempDir()
	store := filepath.Join(home, "Library/Application Support/Google/Chrome/Default/Network/Cookies")
	if err := os.MkdirAll(filepath.Dir(store), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store, []byte("sqlite"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	oldHome, oldRoot := userHomeDir, openCookieRoot
	t.Cleanup(func() { userHomeDir, openCookieRoot = oldHome, oldRoot })
	userHomeDir = func() (string, error) { return "", errors.New("home failed") }
	if _, err := chromeCookiesMac(); err == nil {
		t.Fatal("expected home error")
	}
	userHomeDir = os.UserHomeDir
	openCookieRoot = func(string) (*os.Root, error) { return nil, errors.New("root failed") }
	if _, err := chromeCookiesMac(); err == nil {
		t.Fatal("expected root open error")
	}
	openCookieRoot = os.OpenRoot
	t.Setenv("PATH", t.TempDir())
	if _, err := chromeCookiesMac(); err == nil {
		t.Fatal("expected sqlite error")
	}
	if _, err := chromeSafeStorageKey(); err == nil {
		t.Fatal("expected security command error")
	}
	createCookieTempDir = func(string, string) (string, error) { return "", errors.New("temp failed") }
	if _, err := chromeCookiesMac(); err == nil {
		t.Fatal("expected temp creation error")
	}
	createCookieTempDir = oldCreate
	writeCookieStore = func(string, []byte, os.FileMode) error { return errors.New("write failed") }
	if _, err := chromeCookiesMac(); err == nil {
		t.Fatal("expected temp write error")
	}
	fileHome := t.TempDir() + "/home-file"
	if err := os.WriteFile(fileHome, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", fileHome)
	if _, err := chromeCookiesMac(); err == nil {
		t.Fatal("expected root error")
	}
	missingHome := t.TempDir()
	t.Setenv("HOME", missingHome)
	if _, err := chromeCookiesMac(); err == nil {
		t.Fatal("expected cookie store missing error")
	}
	oldDarwin := isDarwin
	isDarwin = true
	if _, err := CookiesFromBrowser("chrome"); err == nil {
		t.Fatal("expected browser cookie store error")
	}
	isDarwin = oldDarwin
	readHome := t.TempDir()
	readStore := filepath.Join(readHome, "Library/Application Support/Google/Chrome/Default/Network/Cookies")
	if err := os.MkdirAll(readStore, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", readHome)
	if _, err := chromeCookiesMac(); err == nil {
		t.Fatal("expected cookie store read error")
	}
}
