package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/seifreed/tupinambacli/internal/client"
)

func cmdLogin(args []string) error {
	fs := newFlagSet("login")
	browser := fs.String("from-browser", "chrome", "chrome")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if err := requireNoArgs(fs, "tupinamba login --from-browser chrome"); err != nil {
		return err
	}
	cookie, err := cookiesFromBrowser(*browser)
	if err != nil {
		return err
	}
	return saveSession(client.Session{Cookie: cookie}, "browser")
}

func cmdSetCookie(args []string) error {
	fs := newFlagSet("set-cookie")
	stdin := fs.Bool("stdin", false, "read cookie from stdin")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if !*stdin || fs.NArg() != 0 {
		return fmt.Errorf("usage: tupinamba set-cookie --stdin")
	}
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	cookie := strings.TrimSpace(string(b))
	if cookie == "" {
		return fmt.Errorf("usage: tupinamba set-cookie --stdin")
	}
	return saveSession(client.Session{Cookie: cookie}, "stdin")
}

func cmdImportHAR(args []string) error {
	fs := newFlagSet("import-har")
	file := fs.String("file", "", "HAR file")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if *file == "" || fs.NArg() != 0 {
		return fmt.Errorf("usage: tupinamba import-har --file <file>")
	}
	started := time.Now()
	b, err := readPathFile(*file)
	logInputRead("auth.har.read", "file", len(b), time.Since(started), err)
	if err != nil {
		return err
	}
	cookie, err := cookieFromHAR(b)
	if err != nil {
		return err
	}
	return saveSession(client.Session{Cookie: cookie}, "har")
}

func saveSession(s client.Session, source string) error {
	if err := (fileStateStore{}).SaveSession(s); err != nil {
		slog.Default().Error("session save failed", "component", "auth", "operation", "session_save", "event_name", "auth.session.save_failed", "source", source, "error_type", fmt.Sprintf("%T", err))
		return err
	}
	slog.Default().Info("session saved", "component", "auth", "operation", "session_save", "event_name", "auth.session.saved", "source", source)
	fmt.Println("session saved")
	return nil
}

func cmdWhoami(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: tupinamba whoami")
	}
	cart, err := newClient().CartSummary()
	if err != nil {
		return err
	}
	fmt.Printf("cart: accessible (%d items)\n", cart.ItemCount)
	return nil
}

func cookieFromHAR(b []byte) (string, error) {
	var har struct {
		Log struct {
			Entries []struct {
				Request struct {
					URL     string `json:"url"`
					Headers []struct {
						Name  string `json:"name"`
						Value string `json:"value"`
					} `json:"headers"`
				} `json:"request"`
			} `json:"entries"`
		} `json:"log"`
	}
	if err := json.Unmarshal(b, &har); err != nil {
		return "", err
	}
	for _, e := range har.Log.Entries {
		u, err := url.Parse(e.Request.URL)
		if err != nil || !strings.EqualFold(u.Scheme, "https") || !isTupinambaHost(u.Hostname()) {
			continue
		}
		for _, h := range e.Request.Headers {
			if strings.EqualFold(h.Name, "cookie") && h.Value != "" {
				return h.Value, nil
			}
		}
	}
	return "", fmt.Errorf("no Tupinamba Cookie header found in HAR")
}

func isTupinambaHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	want := strings.ToLower(strings.TrimPrefix(client.BaseURL, "https://"))
	return host == want || strings.HasSuffix(host, "."+want)
}
