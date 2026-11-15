package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func encodeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Errorf("encode test response: %v", err)
	}
}

func TestPriceCentsReadsShopifyShapes(t *testing.T) {
	if got, ok := PriceCents(Variant{Price: json.RawMessage(`"4.50"`)}); !ok || got != 450 {
		t.Fatalf("string price = %d, %v", got, ok)
	}
	if got, ok := PriceCents(Variant{Price: json.RawMessage(`2500`)}); !ok || got != 2500 {
		t.Fatalf("cent price = %d, %v", got, ok)
	}
}

func TestImageUnmarshalSupportsShopifyShapes(t *testing.T) {
	for _, raw := range []string{`"https://cdn.example/image.jpg"`, `{"src":"https://cdn.example/image.jpg"}`} {
		var image Image
		if err := json.Unmarshal([]byte(raw), &image); err != nil || image.Src != "https://cdn.example/image.jpg" {
			t.Fatalf("image from %s = %+v, err=%v", raw, image, err)
		}
	}
	var image Image
	if err := json.Unmarshal([]byte(`123`), &image); err == nil {
		t.Fatal("expected malformed image to fail")
	}
}

func TestFoldMatchesPlainCafeToAccent(t *testing.T) {
	if !strings.Contains(fold("Café en grano"), fold("cafe")) {
		t.Fatal("expected accent-insensitive match")
	}
}

func TestMergeCookieHeader(t *testing.T) {
	got := mergeCookieHeader("cart=old; keep=1", []*http.Cookie{
		{Name: "cart", Value: "new"},
		{Name: "extra", Value: "2"},
	})
	want := "cart=new; extra=2; keep=1"
	if got != want {
		t.Fatalf("cookie header = %q, want %q", got, want)
	}
}

func TestMergeCookieHeaderRemovesExpiredCookies(t *testing.T) {
	got := mergeCookieHeader("expired=old; date-expired=old; keep=1", []*http.Cookie{
		{Name: "expired", Value: "", MaxAge: -1},
		{Name: "date-expired", Value: "", Expires: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)},
	})
	if got != "keep=1" {
		t.Fatalf("cookie header = %q, want %q", got, "keep=1")
	}
}

func TestCartVariantQuantity(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cart.js" {
			http.NotFound(w, r)
			return
		}
		encodeJSON(t, w, map[string]any{
			"item_count": 6, "total_price": 2100, "currency": "EUR",
			"items": []CartLine{{VariantID: 10, Quantity: 2}, {VariantID: 11, Quantity: 1}, {VariantID: 10, Quantity: 3}},
		})
	}))
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}
	if got, err := c.CartVariantQuantity(10); err != nil || got != 5 {
		t.Fatalf("variant quantity = %d, %v", got, err)
	}
	if got, err := c.CartVariantQuantity(99); err != nil || got != 0 {
		t.Fatalf("missing variant quantity = %d, %v", got, err)
	}

	failed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer failed.Close()
	if _, err := (&Client{httpClient: failed.Client(), baseURL: failed.URL}).CartVariantQuantity(10); err == nil {
		t.Fatal("expected cart read error")
	}
}

func TestSecureRedirect(t *testing.T) {
	for _, tc := range []struct {
		name string
		url  string
		ok   bool
	}{
		{name: "official https", url: "https://capsulastupinamba.com/cart.js", ok: true},
		{name: "official https port", url: "https://capsulastupinamba.com:443/cart.js", ok: true},
		{name: "http downgrade", url: "http://capsulastupinamba.com/cart.js"},
		{name: "other host", url: "https://example.com/cart.js"},
		{name: "other port", url: "https://capsulastupinamba.com:8443/cart.js"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, tc.url, nil)
			if err != nil {
				t.Fatal(err)
			}
			got := secureRedirect(req, nil)
			if (got == nil) != tc.ok {
				t.Fatalf("secureRedirect(%s) = %v", tc.url, got)
			}
		})
	}
}

func TestBestVariantPicksCheapestAvailable(t *testing.T) {
	got, ok := BestVariant(Product{Variants: []Variant{
		{ID: 1, Available: false, Price: json.RawMessage(`"1.00"`)},
		{ID: 2, Available: true, Price: json.RawMessage(`"4.00"`)},
		{ID: 3, Available: true, Price: json.RawMessage(`"3.00"`)},
	}})
	if !ok || got.ID != 3 {
		t.Fatalf("variant = %+v, ok=%v", got, ok)
	}
}

func TestBestVariantSkipsInvalidPrice(t *testing.T) {
	got, ok := BestVariant(Product{Variants: []Variant{
		{ID: 1, Available: true, Price: json.RawMessage(`"not-a-price"`)},
		{ID: 2, Available: true, Price: json.RawMessage(`"3.00"`)},
	}})
	if !ok || got.ID != 2 {
		t.Fatalf("variant = %+v, ok=%v", got, ok)
	}
}

func TestSortProductsPrefersAvailableThenCheapest(t *testing.T) {
	products := []Product{
		{Title: "sold", Variants: []Variant{{ID: 1, Available: false, Price: json.RawMessage(`"1.00"`)}}},
		{Title: "expensive", Variants: []Variant{{ID: 2, Available: true, Price: json.RawMessage(`"9.00"`)}}},
		{Title: "cheap", Variants: []Variant{{ID: 3, Available: true, Price: json.RawMessage(`"2.00"`)}}},
	}
	SortProducts(products)
	if products[0].Title != "cheap" || products[2].Title != "sold" {
		t.Fatalf("order = %s, %s, %s", products[0].Title, products[1].Title, products[2].Title)
	}
}

func TestCollectionProductsSortsBeforeLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"products":[
			{"title":"sold","variants":[{"id":1,"available":false,"price":"1.00"}]},
			{"title":"cheap","variants":[{"id":2,"available":true,"price":"2.00"}]},
			{"title":"expensive","variants":[{"id":3,"available":true,"price":"9.00"}]}
		]}`))
	}))
	defer srv.Close()

	got, err := (&Client{httpClient: srv.Client(), baseURL: srv.URL}).CollectionProducts("cafe", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title != "cheap" {
		t.Fatalf("products = %+v", got)
	}
}

func TestResolveVariantRejectsInvalidPrice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"title":"broken",
			"variants":[{"id":1,"sku":"BROKEN","available":true,"price":"not-a-price"}]
		}`))
	}))
	defer srv.Close()

	if _, err := (&Client{httpClient: srv.Client(), baseURL: srv.URL}).ResolveVariant("broken", "BROKEN"); err == nil {
		t.Fatal("expected invalid price to be rejected")
	}
}
