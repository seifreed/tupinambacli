package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func (errorReader) Close() error             { return nil }

func TestNewAndDoJSON(t *testing.T) {
	store := &memoryStateStore{session: Session{Cookie: "old=1"}}
	c := New(store)
	if c.cookie != "old=1" || c.httpClient.Timeout == 0 {
		t.Fatalf("client=%+v", c)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok" {
			w.Header().Add("set-cookie", "new=2")
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		if r.URL.Path == "/bad" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("bad response"))
			return
		}
		if r.URL.Path == "/expire" {
			w.Header().Add("set-cookie", "old=; Max-Age=0")
			_, _ = w.Write([]byte(`{}`))
			return
		}
		_, _ = w.Write([]byte("not-json"))
	}))
	defer srv.Close()
	c = &Client{httpClient: srv.Client(), baseURL: srv.URL, cookie: "old=1", store: store}
	var out map[string]bool
	if err := c.doJSON(http.MethodGet, srv.URL+"/ok", nil, &out); err != nil || !out["ok"] || c.cookie != "new=2; old=1" {
		t.Fatalf("out=%v cookie=%q err=%v", out, c.cookie, err)
	}
	if err := c.doJSON(http.MethodPost, srv.URL+"/ok", map[string]string{"x": "y"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := c.doJSON(http.MethodGet, srv.URL+"/bad", nil, nil); err == nil {
		t.Fatal("expected HTTP error")
	}
	if err := c.doJSON(http.MethodGet, srv.URL+"/invalid", nil, &out); err == nil {
		t.Fatal("expected JSON error")
	}
	store.sessionSaveErr = errTestStore
	if err := c.doJSON(http.MethodGet, srv.URL+"/ok", nil, &out); !errors.Is(err, errTestStore) || !strings.Contains(err.Error(), "save session") {
		t.Fatalf("expected session persistence error, got %v", err)
	}
	store.sessionSaveErr = nil
	if err := c.doJSON(http.MethodGet, srv.URL+"/expire", nil, nil); err != nil || c.cookie != "new=2" {
		t.Fatalf("expired cookie was retained: cookie=%q err=%v", c.cookie, err)
	}
	if err := c.doJSON(http.MethodGet, "://bad", nil, nil); err == nil {
		t.Fatal("expected request error")
	}
	if err := c.doJSON(http.MethodPost, srv.URL, make(chan int), nil); err == nil {
		t.Fatal("expected body marshal error")
	}
	broken := &Client{httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("transport failed")
	})}}
	if err := broken.doJSON(http.MethodGet, srv.URL, nil, nil); err == nil {
		t.Fatal("expected transport error")
	}
	readFail := &Client{httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: errorReader{}}, nil
	})}}
	if err := readFail.doJSON(http.MethodGet, srv.URL, nil, nil); err == nil {
		t.Fatal("expected response read error")
	}
}

func TestDoJSONLogsSafeRequestMetadata(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	defer slog.SetDefault(previous)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("secret-response-body"))
	}))
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL, cookie: "session=secret-cookie"}
	if err := c.doJSON(http.MethodGet, srv.URL+"/cart.js?token=secret-query", nil, nil); err == nil {
		t.Fatal("expected HTTP error")
	}
	got := logs.String()
	for _, secret := range []string{"secret-response-body", "secret-cookie", "secret-query"} {
		if strings.Contains(got, secret) {
			t.Fatalf("log contains sensitive value %q: %s", secret, got)
		}
	}
	if !strings.Contains(got, `"event_name":"http.client.completed"`) || !strings.Contains(got, `"operation":"http_request"`) || !strings.Contains(got, `"status_code":400`) || !strings.Contains(got, `"outcome":"error"`) || !strings.Contains(got, `"error_code":"http_status"`) {
		t.Fatalf("missing request event: %s", got)
	}
	logs.Reset()
	malformed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("not-json")) }))
	defer malformed.Close()
	var output map[string]any
	if err := (&Client{httpClient: malformed.Client()}).doJSON(http.MethodGet, malformed.URL, nil, &output); err == nil {
		t.Fatal("expected JSON decode error")
	}
	if !strings.Contains(logs.String(), `"outcome":"error"`) || !strings.Contains(logs.String(), `"error_type":"*json.SyntaxError"`) || !strings.Contains(logs.String(), `"error_code":"response_decode"`) {
		t.Fatalf("missing decode failure details: %s", logs.String())
	}
	logs.Reset()
	timeout := &Client{httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, context.DeadlineExceeded
	})}}
	if err := timeout.doJSON(http.MethodGet, "https://example.com", nil, nil); err == nil {
		t.Fatal("expected timeout")
	}
	if !strings.Contains(logs.String(), `"error_code":"timeout"`) {
		t.Fatalf("missing timeout code: %s", logs.String())
	}
}

func TestDoJSONRejectsOversizedResponse(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	defer slog.SetDefault(previous)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"ok":true}`+strings.Repeat(" ", 16<<20))
	}))
	defer srv.Close()
	var out map[string]bool
	if err := (&Client{httpClient: srv.Client()}).doJSON(http.MethodGet, srv.URL, nil, &out); err == nil {
		t.Fatal("accepted a response larger than the configured limit")
	}
	if !strings.Contains(logs.String(), `"error_code":"response_too_large"`) {
		t.Fatalf("missing oversized response event: %s", logs.String())
	}
}

func TestNewRejectsCorruptSession(t *testing.T) {
	store := &memoryStateStore{sessionLoadErr: errTestStore}
	if _, err := New(store).Cart(); !errors.Is(err, errTestStore) || !strings.Contains(err.Error(), "load session") {
		t.Fatalf("expected session load error, got %v", err)
	}
}

func TestCatalogClientPaths(t *testing.T) {
	products := []Product{{ID: 7, Title: "Café", Handle: "cafe", Tags: []string{"coffee"}, Variants: []Variant{{ID: 8, SKU: "SKU", Available: true, Price: json.RawMessage(`"2.00"`)}}}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/products/cafe.js":
			_, _ = w.Write([]byte(`{"title":"Café","tags":["coffee"],"images":["https://cdn.example/cafe.jpg"],"variants":[]}`))
		case strings.HasPrefix(r.URL.Path, "/products/"):
			http.NotFound(w, r)
		case r.URL.Path == "/products.json":
			encodeJSON(t, w, map[string]any{"products": products})
		case r.URL.Path == "/collections.json":
			encodeJSON(t, w, map[string]any{"collections": []Collection{{Title: "Cafe"}}})
		case strings.Contains(r.URL.Path, "/collections/"):
			encodeJSON(t, w, map[string]any{"products": products})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}
	for _, term := range []string{"", "cafe", "coffee", "missing"} {
		got, err := c.Search(term, 1)
		if err != nil {
			t.Fatal(err)
		}
		if term != "missing" && len(got) != 1 {
			t.Fatalf("term=%q products=%v", term, got)
		}
	}
	if got, err := c.Product("cafe"); err != nil || got.Type != "coffee" || len(got.Images) != 1 || got.Images[0].Src != "https://cdn.example/cafe.jpg" {
		t.Fatalf("product=%+v err=%v", got, err)
	}
	for _, ref := range []string{"7", "cafe", "8", "SKU"} {
		if _, err := c.Product(ref); err != nil {
			t.Fatalf("ref=%q err=%v", ref, err)
		}
	}
	if _, err := c.Product("unknown"); err == nil {
		t.Fatal("expected product not found")
	}
	if _, err := c.CollectionProducts("cafe", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Collections(); err != nil {
		t.Fatal(err)
	}
	if got := limitProducts(products, 0); len(got) != 1 {
		t.Fatal("unexpected limit")
	}
	if got := productTypeFromTags(nil); got != "" || productTypeFromTags([]string{"x"}) != "x" {
		t.Fatal("unexpected product type")
	}
}
