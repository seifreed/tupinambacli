package client

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func cartTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/products/alpha.js":
			_, _ = w.Write([]byte(`{"title":"Alpha","variants":[{"id":4,"sku":"A","available":true,"price":"2.00"},{"id":5,"sku":"B","available":false,"price":"3.00"}]}`))
		case "/cart.js", "/cart/add.js", "/cart/update.js", "/cart/clear.js":
			encodeJSON(t, w, map[string]any{
				"item_count": 1, "total_price": 200, "currency": "EUR",
				"items": []map[string]any{{"variant_id": 4, "quantity": 1}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestCartClientOperations(t *testing.T) {
	srv := cartTestServer(t)
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}
	if raw, err := c.Cart(); err != nil || len(raw) == 0 {
		t.Fatalf("cart raw=%s err=%v", raw, err)
	}
	cart, err := c.CartSummary()
	if err != nil || cart.ItemCount != 1 {
		t.Fatalf("cart=%+v err=%v", cart, err)
	}
	checkout, browser := c.CartURLs(cart)
	if !strings.Contains(checkout, "/cart/4:1") || browser != checkout+"?storefront=true" {
		t.Fatalf("checkout=%q browser=%q", checkout, browser)
	}
	if checkout, browser := c.CartURLs(CartSummary{}); checkout != srv.URL+"/checkout" || browser != "" {
		t.Fatalf("empty checkout=%q browser=%q", checkout, browser)
	}
	if _, err := c.AddVariant("alpha", "", 0); err == nil {
		t.Fatal("expected add quantity error")
	}
	if _, err := c.AddVariant("alpha", "A", 2); err != nil {
		t.Fatal(err)
	}
	if _, err := c.AddVariant("missing", "", 1); err == nil {
		t.Fatal("expected add variant resolution error")
	}
	if _, err := c.SetVariant("alpha", "B", 1); err == nil {
		t.Fatal("expected unavailable variant")
	}
	if _, err := c.SetVariant("alpha", "B", 0); err != nil {
		t.Fatalf("expected removal of unavailable variant to succeed: %v", err)
	}
	if _, err := c.SetVariant("alpha", "A", -1); err == nil {
		t.Fatal("expected negative set quantity error")
	}
	if _, err := c.SetVariant("alpha", "A", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Clear(); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ResolveVariant("alpha", "missing"); err == nil {
		t.Fatal("expected missing variant error")
	}
	if _, err := c.ResolveVariant("missing", ""); err == nil {
		t.Fatal("expected missing product error")
	}
	if _, err := c.ResolveVariant("alpha", ""); err != nil {
		t.Fatal(err)
	}
}

func TestCartSummaryRejectsInconsistentLines(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	for _, tc := range []struct {
		name   string
		body   string
		status int
	}{
		{name: "invalid JSON", body: "{"},
		{name: "wrong field type", body: `{"item_count":"bad"}`},
		{name: "HTTP error", body: `{"error":"unavailable"}`, status: http.StatusBadRequest},
		{name: "negative total", body: `{"total_price":-1}`},
		{name: "negative item count", body: `{"item_count":-1}`},
		{name: "invalid variant", body: `{"items":[{"variant_id":0,"quantity":1}]}`},
		{name: "invalid quantity", body: `{"items":[{"variant_id":4,"quantity":0}]}`},
		{name: "quantity overflow", body: fmt.Sprintf(`{"items":[{"variant_id":4,"quantity":%d},{"variant_id":5,"quantity":1}]}`, maxInt)},
		{name: "item count mismatch", body: `{"item_count":2,"items":[{"variant_id":4,"quantity":1}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tc.status != 0 {
					w.WriteHeader(tc.status)
				}
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			c := &Client{httpClient: srv.Client(), baseURL: srv.URL}
			if _, err := c.CartSummary(); err == nil {
				t.Fatal("expected invalid cart to fail")
			}
		})
	}
}

func TestResolveVariantUnavailableAndInvalid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"title":"Broken","variants":[{"id":1,"sku":"bad","available":true,"price":"not-number"}]}`))
	}))
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}
	if _, err := c.ResolveVariant("broken", ""); err == nil {
		t.Fatal("expected unavailable product")
	}
	if _, err := c.ResolveVariant("broken", "bad"); err == nil {
		t.Fatal("expected invalid variant price")
	}
}

func TestSetVariantZeroResolvesUnavailableProductWithoutPrice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/products/broken.js" {
			_, _ = w.Write([]byte(`{"title":"Broken","variants":[{"id":2,"sku":"bad","available":false,"price":"invalid"}]}`))
			return
		}
		if r.URL.Path == "/cart/update.js" {
			_, _ = w.Write([]byte(`{}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}
	if _, err := c.SetVariant("broken", "", 0); err != nil {
		t.Fatalf("expected removal of unavailable product variant to succeed: %v", err)
	}
}
