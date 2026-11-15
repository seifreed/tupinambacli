package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const BaseURL = "https://capsulastupinamba.com"
const maxHTTPResponseBytes = 16 << 20

type Client struct {
	httpClient *http.Client
	baseURL    string
	cookie     string
	cookieMu   sync.RWMutex
	store      StateStore
	initErr    error
}

type ClientOptions struct {
	StateStore StateStore
	HTTPClient *http.Client
	BaseURL    string
}

func New(store StateStore) *Client { return NewWithOptions(ClientOptions{StateStore: store}) }

func NewWithOptions(options ClientOptions) *Client {
	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second, CheckRedirect: secureRedirect}
	}
	baseURL := options.BaseURL
	if baseURL == "" {
		baseURL = BaseURL
	}
	c := &Client{httpClient: httpClient, baseURL: baseURL, store: options.StateStore}
	if options.StateStore == nil {
		c.initErr = errors.New("client state store is required")
		return c
	}
	s, err := options.StateStore.LoadSession()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		c.initErr = fmt.Errorf("load session: %w", err)
	} else if s.Cookie != "" {
		c.cookie = s.Cookie
	}
	return c
}

func secureRedirect(req *http.Request, _ []*http.Request) error {
	port := req.URL.Port()
	if req.URL.Scheme != "https" || !strings.EqualFold(req.URL.Hostname(), "capsulastupinamba.com") || port != "" && port != "443" {
		return fmt.Errorf("refusing insecure or cross-site redirect")
	}
	return nil
}

func (c *Client) doJSON(method, u string, body, out any) (result error) {
	if c.initErr != nil {
		return c.initErr
	}
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, u, r)
	if err != nil {
		return err
	}
	req.Header.Set("accept", "application/json")
	req.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36")
	if body != nil {
		req.Header.Set("content-type", "application/json")
	}
	c.cookieMu.RLock()
	cookie := c.cookie
	c.cookieMu.RUnlock()
	if cookie != "" {
		req.Header.Set("cookie", cookie)
	}
	started := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		errorCode := "transport"
		if errors.Is(err, context.DeadlineExceeded) {
			errorCode = "timeout"
		} else if errors.Is(err, context.Canceled) {
			errorCode = "canceled"
		}
		slog.Default().Error("external request failed", "component", "client", "operation", "http_request", "event_name", "http.client.failed", "method", method, "path", req.URL.Path, "duration_ms", time.Since(started).Milliseconds(), "error_type", fmt.Sprintf("%T", err), "error_code", errorCode)
		return err
	}
	status := resp.StatusCode
	errorCode := ""
	defer func() {
		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		} else if status >= 400 {
			level = slog.LevelWarn
		}
		outcome := "success"
		if result != nil {
			outcome = "error"
		}
		attrs := []any{"component", "client", "operation", "http_request", "event_name", "http.client.completed", "method", method, "path", req.URL.Path, "status_code", status, "duration_ms", time.Since(started).Milliseconds(), "outcome", outcome}
		if result != nil {
			attrs = append(attrs, "error_type", fmt.Sprintf("%T", result), "error_code", errorCode)
		}
		slog.Default().Log(context.Background(), level, "external request completed", attrs...)
	}()
	defer func() {
		if closeErr := resp.Body.Close(); result == nil {
			result = closeErr
			errorCode = "response_close"
		}
	}()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPResponseBytes+1))
	if err != nil {
		errorCode = "response_read"
		return err
	}
	if len(data) > maxHTTPResponseBytes {
		errorCode = "response_too_large"
		return fmt.Errorf("shopify response exceeds %d bytes", maxHTTPResponseBytes)
	}
	var sessionErr error
	if cookies := resp.Cookies(); len(cookies) > 0 {
		c.cookieMu.Lock()
		c.cookie = mergeCookieHeader(c.cookie, cookies)
		if c.store != nil {
			if err := c.store.SaveSession(Session{Cookie: c.cookie}); err != nil {
				errorCode = "session_persist"
				sessionErr = fmt.Errorf("save session: %w", err)
			}
		}
		c.cookieMu.Unlock()
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Remote response bodies are untrusted and may contain credentials or personal data.
		errorCode = "http_status"
		return fmt.Errorf("shopify HTTP %d", resp.StatusCode)
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			errorCode = "response_decode"
			return err
		}
	}
	return sessionErr
}
