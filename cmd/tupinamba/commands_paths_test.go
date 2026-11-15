package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/seifreed/tupinambacli/internal/client"
)

func TestCatalogTextPaths(t *testing.T) {
	products := []client.Product{testProduct("Alpha", "alpha", true)}
	useTestClient(t, catalogHandler(products))
	file := t.TempDir() + "/items"
	if err := os.WriteFile(file, []byte("alpha\nmissing\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	totalFile := t.TempDir() + "/total-items"
	if err := os.WriteFile(totalFile, []byte("alpha\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		call func() error
		want string
	}{
		{"search", func() error { return cmdSearch([]string{"alpha"}) }, "Alpha"},
		{"batch", func() error { return cmdBatch([]string{"-f", file}) }, "no encontrado"},
		{"total", func() error { return cmdTotal([]string{"-f", totalFile}) }, "total:"},
		{"collections", func() error { return cmdCollections(nil) }, "Cafe"},
		{"collection json", func() error { return cmdCollections([]string{"--id", "cafe", "--json"}) }, "Alpha"},
		{"categories", func() error { return cmdCollections([]string{"--id", "cafe"}) }, "Alpha"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := captureStdout(t, tc.call)
			if err != nil || !strings.Contains(out, tc.want) {
				t.Fatalf("output=%q err=%v", out, err)
			}
		})
	}
}

func TestCatalogValidationPaths(t *testing.T) {
	file := t.TempDir() + "/items"
	if err := os.WriteFile(file, []byte("alpha 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	useTestClient(t, catalogHandler([]client.Product{testProduct("Alpha", "alpha", true)}))
	if err := cmdTotal([]string{"-f", file}); err == nil {
		t.Fatal("expected invalid quantity")
	}
	if err := cmdTotal([]string{"-f", file, "--max", "invalid"}); err == nil {
		t.Fatal("expected invalid maximum")
	}
	if err := os.WriteFile(file, []byte("alpha 1 extra\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cmdTotal([]string{"-f", file}); err == nil {
		t.Fatal("expected invalid line")
	}
	if err := os.WriteFile(file, []byte("alpha 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cmdTotal([]string{"-f", file, "--max", "0.01"}); err == nil {
		t.Fatal("expected total max error")
	}
	if err := cmdTotal([]string{"-f", file, "--max", "0"}); err == nil {
		t.Fatal("expected zero total max error")
	}
	t.Setenv("TUPINAMBA_MAX_EUR", "0")
	if err := cmdTotal([]string{"-f", file}); err == nil {
		t.Fatal("expected zero environment max error")
	}
	if err := cmdBatch([]string{"-f", t.TempDir() + "/missing"}); err == nil {
		t.Fatal("expected batch file error")
	}
	if err := cmdTotal([]string{"-f", t.TempDir() + "/missing"}); err == nil {
		t.Fatal("expected total file error")
	}
}

func TestCatalogAndCartHTTPFailures(t *testing.T) {
	useTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/products/empty.js" {
			jsonResponse(w, client.Product{Title: "Empty"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	if err := cmdSearch([]string{"anything"}); err == nil {
		t.Fatal("expected search error")
	}
	if err := cmdProduct([]string{"alpha"}); err == nil {
		t.Fatal("expected product error")
	}
	if err := cmdCollections(nil); err == nil {
		t.Fatal("expected collections error")
	}
	if err := cmdCollections([]string{"--id", "cafe"}); err == nil {
		t.Fatal("expected collection error")
	}
	file := t.TempDir() + "/items"
	if err := os.WriteFile(file, []byte("alpha\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cmdBatch([]string{"-f", file}); err == nil {
		t.Fatal("expected batch error")
	}
	if err := cmdTotal([]string{"-f", file}); err == nil {
		t.Fatal("expected total product error")
	}
	if err := cmdCart([]string{"get"}); err == nil {
		t.Fatal("expected cart error")
	}
	if err := cmdWhoami(nil); err == nil {
		t.Fatal("expected whoami error")
	}
}

func TestTotalFailurePaths(t *testing.T) {
	products := map[string]client.Product{
		"empty":  {Title: "Empty"},
		"huge":   {Title: "Huge", Variants: []client.Variant{{ID: 1, Available: true, Price: json.RawMessage(`"50000000000000000"`)}}},
		"first":  {Title: "First", Variants: []client.Variant{{ID: 1, Available: true, Price: json.RawMessage(`"50000000000000000"`)}}},
		"second": {Title: "Second", Variants: []client.Variant{{ID: 2, Available: true, Price: json.RawMessage(`"50000000000000000"`)}}},
	}
	useTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p, ok := products[strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/products/"), ".js")]; ok {
			jsonResponse(w, p)
			return
		}
		http.NotFound(w, r)
	}))
	for _, tc := range []struct {
		ref  string
		want string
	}{
		{"empty", "unavailable"},
		{"huge", "line total"},
	} {
		file := t.TempDir() + "/items"
		if err := os.WriteFile(file, []byte(tc.ref+" 2\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := cmdTotal([]string{"-f", file}); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("total %s err=%v", tc.ref, err)
		}
	}
	file := t.TempDir() + "/overflow"
	if err := os.WriteFile(file, []byte("first\nsecond\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cmdTotal([]string{"-f", file}); err == nil || !strings.Contains(err.Error(), "total exceeds") {
		t.Fatalf("expected total arithmetic error, got %v", err)
	}
}

func TestCheckoutAndWhoamiPaths(t *testing.T) {
	if err := cmdCheckout([]string{"--max", "invalid"}); err == nil {
		t.Fatal("expected invalid checkout maximum")
	}
	handler := func(body any) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) { jsonResponse(w, body) }
	}
	useTestClient(t, handler(map[string]any{
		"item_count": 3, "total_price": 350, "currency": "EUR",
		"items": []map[string]any{
			{"variant_id": 111, "quantity": 1},
			{"variant_id": 222, "quantity": 2},
		},
	}))
	out, err := captureStdout(t, func() error { return cmdCheckout(nil) })
	if err != nil || !strings.Contains(out, "ready") ||
		!strings.Contains(out, "/cart/111:1,222:2") ||
		!strings.Contains(out, "/cart/111:1,222:2?storefront=true") {
		t.Fatalf("checkout output=%q err=%v", out, err)
	}
	if err := cmdCheckout([]string{"--max", "1"}); err == nil {
		t.Fatal("expected checkout max error")
	}
	if err := cmdCheckout([]string{"--max", "0"}); err == nil {
		t.Fatal("expected zero checkout max error")
	}

	useTestClient(t, handler(map[string]any{
		"item_count": 1,
		"items":      []map[string]any{{"variant_id": 111, "quantity": 1}},
	}))
	out, err = captureStdout(t, func() error { return cmdWhoami(nil) })
	if err != nil || !strings.Contains(out, "cart: accessible (1 items)") || strings.Contains(out, "authenticated") {
		t.Fatalf("whoami output=%q err=%v", out, err)
	}
	useTestClient(t, handler(map[string]any{"item_count": "bad"}))
	if err := cmdWhoami(nil); err == nil {
		t.Fatal("expected whoami decode error")
	}
	useTestClient(t, handler(map[string]any{"item_count": -1, "total_price": 0}))
	if err := cmdCheckout(nil); err == nil {
		t.Fatal("expected invalid cart response")
	}
	useTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, "not-json")
	}))
	if err := cmdCheckout(nil); err == nil {
		t.Fatal("expected checkout decode error")
	}
	useTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	if err := cmdCheckout(nil); err == nil {
		t.Fatal("expected checkout HTTP error")
	}
}
