package browserauth

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/seifreed/tupinambacli/internal/cookieheader"
)

const chromeCookieQuery = `select host_key, name, value, hex(encrypted_value) from cookies where host_key in ('capsulastupinamba.com', '.capsulastupinamba.com');`

var isDarwin = runtime.GOOS == "darwin"
var userHomeDir = os.UserHomeDir
var openCookieRoot = os.OpenRoot

var createCookieTempDir = os.MkdirTemp
var writeCookieStore = os.WriteFile

func CookiesFromBrowser(browser string) (session string, result error) {
	started := time.Now()
	defer func() {
		level, outcome := slog.LevelInfo, "success"
		attrs := []any{"component", "client", "operation", "browser_cookie_import", "event_name", "auth.browser_cookie_import.completed", "duration_ms", time.Since(started).Milliseconds()}
		if result != nil {
			level, outcome = slog.LevelWarn, "error"
			attrs = append(attrs, "error_type", fmt.Sprintf("%T", result))
		}
		attrs = append(attrs, "outcome", outcome, "cookie_count", len(cookieheader.Parse(session)))
		slog.Default().Log(context.Background(), level, "browser cookie import completed", attrs...)
	}()
	browser = strings.ToLower(strings.TrimSpace(browser))
	if browser == "" {
		browser = "chrome"
	}
	if browser != "chrome" {
		return "", fmt.Errorf("only chrome is supported")
	}
	if !isDarwin {
		return "", fmt.Errorf("chrome cookie import currently supports macOS only; use set-cookie --stdin or import-har")
	}
	cookies, err := chromeCookiesMac()
	if err != nil {
		return "", err
	}
	if len(cookies) == 0 {
		return "", fmt.Errorf("no Tupinamba cookies found in %s", browser)
	}
	return cookieheader.Format(cookies), nil
}

func chromeCookiesMac() (cookies map[string]string, result error) {
	home, err := userHomeDir()
	if err != nil {
		return nil, err
	}
	root, err := openCookieRoot(home)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := root.Close(); result == nil {
			result = closeErr
		}
	}()
	src, err := firstExisting(root,
		"Library/Application Support/Google/Chrome/Default/Network/Cookies",
		"Library/Application Support/Google/Chrome/Default/Cookies",
	)
	if err != nil {
		return nil, err
	}
	data, err := root.ReadFile(src)
	if err != nil {
		return nil, err
	}
	tmpDir, err := createCookieTempDir("", "tupinamba-chrome-cookies-*")
	if err != nil {
		return nil, err
	}
	defer func() {
		if cleanupErr := os.RemoveAll(tmpDir); result == nil {
			result = cleanupErr
		}
	}()
	if err := writeCookieStore(filepath.Join(tmpDir, "cookies.db"), data, 0o600); err != nil {
		return nil, err
	}
	cmd := exec.Command("sqlite3", "-tabs", "cookies.db", chromeCookieQuery)
	cmd.Dir = tmpDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("read Chrome cookies with sqlite3: %w", err)
	}
	var key []byte
	keyAttempted, decryptFailures := false, 0
	cookies = map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 4 {
			continue
		}
		val := fields[2]
		if val == "" && fields[3] != "" {
			if !keyAttempted {
				keyAttempted = true
				var err error
				key, err = chromeSafeStorageKey()
				if err != nil {
					slog.Warn("Chrome cookie decryption key unavailable", "component", "client", "operation", "browser_cookie_import", "event_name", "auth.browser_cookie_key.unavailable", "error_type", fmt.Sprintf("%T", err))
				}
			}
			if key != nil {
				var err error
				val, err = decryptChromeCookie(fields[3], key, fields[0])
				if err != nil {
					decryptFailures++
				}
			}
		}
		if val != "" && validCookieValue(val) {
			cookies[fields[1]] = val
		}
	}
	if decryptFailures > 0 {
		slog.Warn("Chrome cookies could not be decrypted", "component", "client", "operation", "browser_cookie_import", "event_name", "auth.browser_cookie_decryption.failed", "cookie_count", decryptFailures)
	}
	return cookies, nil
}

func firstExisting(root *os.Root, paths ...string) (string, error) {
	for _, path := range paths {
		if _, err := root.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("chrome cookie store not found")
}

func chromeSafeStorageKey() ([]byte, error) {
	// Chrome's macOS Safe Storage format requires PBKDF2-HMAC-SHA1 for compatibility.
	out, err := exec.Command("security", "find-generic-password", "-w", "-s", "Chrome Safe Storage").Output()
	if err != nil {
		return nil, err
	}
	return pbkdf2.Key(sha1.New, string(bytes.TrimSpace(out)), []byte("saltysalt"), 1003, 16)
}

func decryptChromeCookie(hexValue string, key []byte, host string) (string, error) {
	raw, err := hex.DecodeString(hexValue)
	if err != nil {
		return "", err
	}
	if len(raw) > 3 && string(raw[:3]) == "v10" {
		raw = raw[3:]
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	if len(raw) == 0 || len(raw)%block.BlockSize() != 0 {
		return "", fmt.Errorf("invalid encrypted cookie length")
	}
	out := make([]byte, len(raw))
	cipher.NewCBCDecrypter(block, bytes.Repeat([]byte(" "), block.BlockSize())).CryptBlocks(out, raw)
	pad := int(out[len(out)-1])
	if pad <= 0 || pad > block.BlockSize() || pad > len(out) {
		return "", fmt.Errorf("invalid cookie padding")
	}
	plain := out[:len(out)-pad]
	if len(plain) > sha256.Size {
		sum := sha256.Sum256([]byte(host))
		if bytes.Equal(plain[:sha256.Size], sum[:]) {
			plain = plain[sha256.Size:]
		}
	}
	return string(plain), nil
}

func validCookieValue(s string) bool {
	for _, r := range s {
		if r < 0x21 || r > 0x7e || r == ';' {
			return false
		}
	}
	return true
}
