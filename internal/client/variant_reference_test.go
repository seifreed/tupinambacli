package client

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFindVariantRequiresReference(t *testing.T) {
	if _, ok := FindVariant(Product{Variants: []Variant{{ID: 1}}}, ""); ok {
		t.Fatal("empty reference unexpectedly matched a variant")
	}
}

func TestResolveVariantBySKU(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		product := `{
			"title":"multi",
			"variants":[
				{"id":1,"sku":"A","available":true,"price":"4.00"},
				{"id":2,"sku":"B","available":true,"price":"5.00"}
			]
		}`
		if r.URL.Path == "/products.json" {
			_, _ = w.Write([]byte(`{"products":[` + product + `]}`))
			return
		}
		_, _ = w.Write([]byte(product))
	}))
	defer srv.Close()
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}
	if got, err := c.ResolveVariant("multi", "B"); err != nil || got.ID != 2 {
		t.Fatalf("explicit variant = %+v, err=%v", got, err)
	}
	for _, ref := range []string{"B", "2"} {
		if got, err := c.ResolveVariant(ref, ""); err != nil || got.ID != 2 {
			t.Fatalf("variant ref %q = %+v, err=%v", ref, got, err)
		}
	}
}
