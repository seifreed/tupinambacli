package desktopmcp

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/seifreed/tupinambacli/internal/client"
)

func TestCatalogRemoteFailuresAndUntrustedOutputBounds(t *testing.T) {
	fixture := newShopFixture(t)
	session := connect(t, New(fixture.client()))
	fixture.setFail("/products.json", 1)
	if r := callTool(t, session, "search_products", map[string]any{"query": "coffee", "limit": 5}); !r.IsError {
		t.Fatal("catalog failure hidden")
	}
	fixture.setFail("/products/arabica.js", 1)
	fixture.setFail("/products.json", 1)
	if r := callTool(t, session, "get_product", map[string]any{"ref": "arabica"}); !r.IsError {
		t.Fatal("product failure hidden")
	}
	p := client.Product{Description: "<b>" + strings.Repeat("x", 1300) + "</b>", Variants: []client.Variant{{ID: 4, Title: "bad", Available: true, Price: json.RawMessage(`"oops"`)}}}
	got := productOutput(p)
	if len([]rune(got.Description)) != 1200 || got.Variants[0].Price != "" {
		t.Fatalf("unbounded/invalid product output: %#v", got)
	}
	fallback := productOutput(client.Product{BodyHTML: "<p>fallback</p>"})
	if fallback.Description != "fallback" {
		t.Fatalf("body fallback: %#v", fallback)
	}
}

func TestResolveVariantSelectionAndAvailability(t *testing.T) {
	fixture := newShopFixture(t)
	shop := fixture.client()
	product, err := shop.Product("10")
	if err != nil {
		t.Fatal(err)
	}
	if _, found := client.FindVariant(*product, "10"); found {
		t.Fatal("test product unexpectedly has variant 10")
	}
	service := New(shop)
	if _, _, _, err := service.resolveVariant("arabica", "OUT", true); err == nil {
		t.Fatal("unavailable variant accepted")
	}
	if _, _, _, err := service.resolveVariant("arabica", "", true); err != nil {
		t.Fatalf("best variant: %v", err)
	}
	fixture.price = "invalid"
	if _, _, _, err := service.resolveVariant("arabica", "", true); err == nil {
		t.Fatal("product without priced variants accepted")
	}
	if _, _, _, err := service.resolveVariant("arabica", "ARA", false); err == nil {
		t.Fatal("invalid price accepted")
	}
}

func TestVariantHelpersAndEnvironmentErrorLogging(t *testing.T) {
	fixture := newShopFixture(t)
	service := New(fixture.client())
	if _, _, _, err := service.resolveVariant("10", "", true); err != nil {
		t.Fatalf("best variant fallback: %v", err)
	}
	if _, _, _, err := service.resolveVariant("ARA", "", true); err != nil {
		t.Fatalf("direct variant reference: %v", err)
	}
	if got := variantRefOrID(client.Variant{ID: 42}, ""); got != "42" {
		t.Fatalf("variant id: %s", got)
	}
	t.Setenv("TUPINAMBA_ENV", "test")
	service.logTool(t.Context(), "search_products", time.Now(), errors.New("sensitive failure"))
}
