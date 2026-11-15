package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestTotalUsesExactVariantReference(t *testing.T) {
	for _, tc := range []struct {
		name      string
		available bool
		price     string
		wantError string
		wantLine  string
	}{
		{name: "available", available: true, price: "5.00", wantLine: "[B] Multi x1 = 5.00€"},
		{name: "unavailable", price: "5.00", wantError: `variant "B" is unavailable`},
		{name: "invalid price", available: true, price: "invalid", wantError: `variant "B" has an invalid price`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			useTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/products/B.js" {
					http.NotFound(w, r)
					return
				}
				body := `{"title":"Multi","variants":[{"id":1,"sku":"A","available":true,"price":"4.00"},{"id":2,"sku":"B","available":` + strconv.FormatBool(tc.available) + `,"price":"` + tc.price + `"}]}`
				_, _ = w.Write([]byte(body))
			}))
			input := filepath.Join(t.TempDir(), "total.txt")
			if err := os.WriteFile(input, []byte("B\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			out, err := captureStdout(t, func() error { return cmdTotal([]string{"-f", input}) })
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("error=%v, want %q", err, tc.wantError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, tc.wantLine) || !strings.Contains(out, "total: 5.00€") {
				t.Fatalf("total ignored the exact variant reference: %s", out)
			}
		})
	}
}
