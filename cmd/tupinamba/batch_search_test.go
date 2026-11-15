package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/seifreed/tupinambacli/internal/client"
)

func TestBatchLoadsCatalogOnce(t *testing.T) {
	var fetches atomic.Int32
	useTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/products.json" {
			http.NotFound(w, r)
			return
		}
		fetches.Add(1)
		jsonResponse(w, map[string]any{"products": []client.Product{
			testProduct("Alpha", "alpha", true), testProduct("Beta", "beta", true),
		}})
	}))
	input := filepath.Join(t.TempDir(), "batch.txt")
	if err := os.WriteFile(input, []byte("alpha\nbeta\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := captureStdout(t, func() error { return cmdBatch([]string{"-f", input}) })
	if err != nil {
		t.Fatal(err)
	}
	if fetches.Load() != 1 || !strings.Contains(out, "Alpha") || !strings.Contains(out, "Beta") {
		t.Fatalf("fetches=%d output=%s", fetches.Load(), out)
	}
}
