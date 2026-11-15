package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/seifreed/tupinambacli/internal/client"
)

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	err = fn()
	_ = w.Close()
	os.Stdout = old
	b, readErr := io.ReadAll(r)
	_ = r.Close()
	if err == nil {
		err = readErr
	}
	return string(b), err
}

func useTestClient(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	old := newClient
	newClient = func() *client.Client {
		return client.NewWithOptions(client.ClientOptions{StateStore: testStateStore{}, HTTPClient: srv.Client(), BaseURL: srv.URL})
	}
	t.Cleanup(func() {
		newClient = old
		srv.Close()
	})
	return srv
}

type testStateStore struct{}

func (testStateStore) LoadSession() (client.Session, error) { return client.Session{}, nil }
func (testStateStore) SaveSession(client.Session) error     { return nil }
func (testStateStore) LoadCatalogCache() (client.CatalogCache, error) {
	return client.CatalogCache{}, os.ErrNotExist
}
func (testStateStore) SaveCatalogCache(client.CatalogCache) error { return nil }

var _ client.StateStore = testStateStore{}

func jsonResponse(w http.ResponseWriter, v any) {
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func testProduct(title, handle string, available bool) client.Product {
	return client.Product{ID: 1, Title: title, Handle: handle, Variants: []client.Variant{{
		ID: 10, SKU: "SKU-1", Available: available, Price: json.RawMessage(`"3.50"`),
	}}}
}

func catalogHandler(products []client.Product) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/products.json":
			if r.URL.Query().Get("page") == "2" {
				jsonResponse(w, map[string]any{"products": []client.Product{}})
				return
			}
			jsonResponse(w, map[string]any{"products": products})
		case "/products/alpha.js":
			p := testProduct("Alpha", "alpha", true)
			p.Description = ""
			p.BodyHTML = "<p>description</p>"
			p.Variants = append(p.Variants, client.Variant{ID: 11, SKU: "SKU-2", Available: false, Price: json.RawMessage(`"4.00"`)})
			jsonResponse(w, p)
		case "/collections.json":
			jsonResponse(w, map[string]any{"collections": []client.Collection{{Handle: "cafe", Title: "Cafe", ProductsCount: 1}}})
		case "/collections/cafe/products.json":
			jsonResponse(w, map[string]any{"products": products[:1]})
		default:
			http.NotFound(w, r)
		}
	}
}

func TestCatalogCommands(t *testing.T) {
	products := []client.Product{testProduct("Alpha", "alpha", true), testProduct("Beta", "beta", false)}
	useTestClient(t, catalogHandler(products))
	for _, tc := range []struct {
		name string
		call func() error
		want string
	}{
		{"search json", func() error { return cmdSearch([]string{"alpha", "--limit", "1", "--json"}) }, "Alpha"},
		{"search toon", func() error { return cmdSearch([]string{"alpha", "--limit", "1", "--toon"}) }, "Alpha"},
		{"product json", func() error { return cmdProduct([]string{"alpha", "--json"}) }, "Alpha"},
		{"product human", func() error { return cmdProduct([]string{"alpha"}) }, "description"},
		{"collections json", func() error { return cmdCollections([]string{"--json"}) }, "Cafe"},
		{"categories human", func() error { return cmdCollections([]string{"--id", "cafe"}) }, "Alpha"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := captureStdout(t, tc.call)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, tc.want) {
				t.Fatalf("output %q lacks %q", out, tc.want)
			}
		})
	}
}

func TestPrintProductLine(t *testing.T) {
	for _, p := range []client.Product{
		{ID: 2, Title: "Empty"},
		testProduct("Available", "available", true),
		testProduct("Sold", "sold", false),
	} {
		out, err := captureStdout(t, func() error { printProductLine(p); return nil })
		if err != nil || out == "" {
			t.Fatalf("output=%q err=%v", out, err)
		}
	}
	if got := formatVariantPrice(client.Variant{Price: json.RawMessage(`"bad"`)}); got != "precio no disponible" {
		t.Fatalf("invalid variant price = %q", got)
	}
}
