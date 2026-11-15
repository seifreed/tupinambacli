package client

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAllProductsUsesFreshCache(t *testing.T) {
	t.Setenv("TUPINAMBA_CONFIG_DIR", t.TempDir())
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(previous)
	want := []Product{{Title: "Cached"}}
	store := &memoryStateStore{catalog: CatalogCache{SavedAt: time.Now(), Products: want}, hasCatalog: true}
	got, err := (&Client{httpClient: &http.Client{}, baseURL: BaseURL, store: store}).AllProducts()
	if err != nil || len(got) != 1 || got[0].Title != "Cached" {
		t.Fatalf("products=%v err=%v", got, err)
	}
	if !strings.Contains(logs.String(), `"event_name":"catalog.cache.hit"`) {
		t.Fatalf("missing cache hit event: %s", logs.String())
	}
}

func TestCatalogPaginationAndMatching(t *testing.T) {
	page := make([]Product, 250)
	for i := range page {
		page[i] = Product{ID: int64(i + 1), Title: "Product"}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/products.json":
			if r.URL.Query().Get("page") == "1" {
				encodeJSON(t, w, map[string]any{"products": page})
			} else {
				encodeJSON(t, w, map[string]any{"products": []Product{{ID: 999}}})
			}
		case "/collections.json":
			if r.URL.Query().Get("page") == "1" {
				encodeJSON(t, w, map[string]any{"collections": make([]Collection, 250)})
			} else {
				encodeJSON(t, w, map[string]any{"collections": []Collection{{Title: "last"}}})
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}
	products, err := c.AllProducts()
	if err != nil || len(products) != 251 {
		t.Fatalf("products=%d err=%v", len(products), err)
	}
	collections, err := c.Collections()
	if err != nil || len(collections) != 251 {
		t.Fatalf("collections=%d err=%v", len(collections), err)
	}
	if _, complete, err := c.productPages("/products.json", 1); err == nil || complete {
		t.Fatalf("complete=%v err=%v", complete, err)
	}
	if productMatches(products[0], "1") == false || productMatches(Product{Variants: []Variant{{ID: 4, SKU: "SKU"}}}, "SKU") == false {
		t.Fatal("expected product matching")
	}
	if firstNonEmpty("", "ok") != "ok" {
		t.Fatal("expected fallback value")
	}
}

func TestCatalogPaginationLimitReturnsError(t *testing.T) {
	page := make([]Product, 250)
	collections := make([]Collection, 250)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/products.json", "/collections/foo/products.json":
			encodeJSON(t, w, map[string]any{"products": page})
		case "/collections.json":
			encodeJSON(t, w, map[string]any{"collections": collections})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}
	if products, err := c.AllProducts(); err == nil || products != nil {
		t.Fatalf("expected incomplete product catalog error, products=%d err=%v", len(products), err)
	}
	if products, err := c.CollectionProducts("foo", 1); err == nil || products != nil {
		t.Fatalf("expected incomplete collection products error, products=%d err=%v", len(products), err)
	}
	if items, err := c.Collections(); err == nil || items != nil {
		t.Fatalf("expected incomplete collections error, collections=%d err=%v", len(items), err)
	}
}

func TestCatalogPaginationAcceptsExactPageLimit(t *testing.T) {
	page := make([]Product, 250)
	collections := make([]Collection, 250)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastPage := r.URL.Query().Get("page") == "11"
		switch r.URL.Path {
		case "/products.json", "/collections/foo/products.json":
			products := page
			if lastPage {
				products = nil
			}
			encodeJSON(t, w, map[string]any{"products": products})
		case "/collections.json":
			items := collections
			if lastPage {
				items = nil
			}
			encodeJSON(t, w, map[string]any{"collections": items})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}
	if products, err := c.AllProducts(); err != nil || len(products) != 2500 {
		t.Fatalf("expected exact-limit product catalog, products=%d err=%v", len(products), err)
	}
	if products, err := c.CollectionProducts("foo", 0); err != nil || len(products) != 2500 {
		t.Fatalf("expected exact-limit collection products, products=%d err=%v", len(products), err)
	}
	if items, err := c.Collections(); err != nil || len(items) != 2500 {
		t.Fatalf("expected exact-limit collections, collections=%d err=%v", len(items), err)
	}
}

func TestCatalogPaginationSentinelFailureIsReturned(t *testing.T) {
	page := make([]Product, 250)
	collections := make([]Collection, 250)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "11" {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		switch r.URL.Path {
		case "/products.json":
			encodeJSON(t, w, map[string]any{"products": page})
		case "/collections.json":
			encodeJSON(t, w, map[string]any{"collections": collections})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}
	if _, err := c.AllProducts(); err == nil {
		t.Fatal("expected product sentinel request failure")
	}
	if _, err := c.Collections(); err == nil {
		t.Fatal("expected collections sentinel request failure")
	}
}

func TestCatalogErrorAndOfficialSavePaths(t *testing.T) {
	t.Setenv("TUPINAMBA_CONFIG_DIR", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}
	if _, err := c.Search("x", 1); err == nil {
		t.Fatal("expected search error")
	}
	if _, err := c.CollectionProducts("bad", 1); err == nil {
		t.Fatal("expected collection product error")
	}
	if _, err := c.Collections(); err == nil {
		t.Fatal("expected collections error")
	}
	store := &memoryStateStore{}
	official := &Client{httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"products":[]}`))}, nil
	})}, baseURL: BaseURL, store: store}
	if _, err := official.AllProducts(); err != nil {
		t.Fatal(err)
	}
	if !store.hasCatalog {
		t.Fatal("official catalog response was not cached")
	}
	all := make([]Collection, 250)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		encodeJSON(t, w, map[string]any{"collections": all})
	}))
	defer server.Close()
	if got, err := (&Client{httpClient: server.Client(), baseURL: server.URL}).Collections(); err == nil || got != nil {
		t.Fatalf("expected collection pagination limit error, collections=%d err=%v", len(got), err)
	}
}
