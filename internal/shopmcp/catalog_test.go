package shopmcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestCatalogToolsAndCartOutputAreSafe(t *testing.T) {
	fixture := newShopFixture(t)
	session := connect(t, New(fixture.client()))
	want := map[string]bool{"search_products": false, "get_product": false, "cart_status": false, "calculate_total": false, "plan_cart_change": false, "apply_cart_change": false}
	for tool, err := range session.Tools(context.Background(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("missing tool %q", name)
		}
	}
	products := output[[]ProductOutput](t, callTool(t, session, "search_products", map[string]any{"query": "arabica", "limit": 5}))
	if len(products) != 1 || products[0].Handle != "arabica" {
		t.Fatalf("products: %#v", products)
	}
	product := output[ProductOutput](t, callTool(t, session, "get_product", map[string]any{"ref": "arabica"}))
	if product.Description != "Single origin coffee" || len(product.Variants) != 2 {
		t.Fatalf("product: %#v", product)
	}
	cart := output[CartStatusOutput](t, callTool(t, session, "cart_status", map[string]any{}))
	b, _ := json.Marshal(cart)
	if !cart.Empty || strings.Contains(string(b), "token") || strings.Contains(string(b), "must-not-leak") {
		t.Fatalf("unsafe cart: %s", b)
	}
}

func TestReadToolsRejectInvalidInputs(t *testing.T) {
	session := connect(t, New(newShopFixture(t).client()))
	for _, tc := range []struct {
		name string
		args map[string]any
	}{
		{"search_products", map[string]any{"query": " ", "limit": 3}},
		{"search_products", map[string]any{"query": strings.Repeat("x", 101), "limit": 3}},
		{"search_products", map[string]any{"query": "coffee", "limit": 21}},
		{"get_product", map[string]any{"ref": " "}},
	} {
		if r := callTool(t, session, tc.name, tc.args); !r.IsError {
			t.Errorf("accepted invalid input: %#v", tc.args)
		}
	}
}

func TestResolveVariantRejectsUnknownReferences(t *testing.T) {
	service := New(newShopFixture(t).client())
	for _, ref := range []string{"missing", strings.Repeat("x", 101)} {
		if _, _, _, err := service.resolveVariant(ref, "", true); err == nil {
			t.Errorf("accepted product ref %q", ref)
		}
	}
	if _, _, _, err := service.resolveVariant("arabica", "missing", true); err == nil {
		t.Fatal("accepted unknown variant")
	}
}
