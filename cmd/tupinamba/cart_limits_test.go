package main

import (
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func TestCartAddMaximumIncludesExistingVariantQuantity(t *testing.T) {
	var writes atomic.Int32
	useTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/products/alpha.js":
			jsonResponse(w, testProduct("Alpha", "alpha", true))
		case "/cart.js":
			jsonResponse(w, map[string]any{
				"item_count": 2, "total_price": 700, "currency": "EUR",
				"items": []map[string]any{{"variant_id": 10, "quantity": 2}},
			})
		case "/cart/add.js":
			writes.Add(1)
			jsonResponse(w, map[string]any{"item_count": 3, "total_price": 1050, "currency": "EUR"})
		default:
			http.NotFound(w, r)
		}
	}))
	if err := cmdCart([]string{"add", "alpha", "1", "--max", "10"}); err == nil {
		t.Fatal("expected resulting line to exceed maximum")
	}
	if writes.Load() != 0 {
		t.Fatal("cart add request was sent despite resulting line exceeding maximum")
	}
	if err := cmdCart([]string{"add", "alpha", "0", "--max", "10"}); err == nil || !strings.Contains(err.Error(), "quantity must be positive") {
		t.Fatalf("expected positive quantity validation, got %v", err)
	}
	maxQuantity := strconv.Itoa(int(^uint(0) >> 1))
	if err := cmdCart([]string{"add", "alpha", maxQuantity, "--max", "1"}); err == nil || !strings.Contains(err.Error(), "line quantity") {
		t.Fatalf("expected line quantity overflow, got %v", err)
	}
}

func TestCartAddMaximumRequiresReadableCart(t *testing.T) {
	useTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/products/alpha.js" {
			jsonResponse(w, testProduct("Alpha", "alpha", true))
			return
		}
		if r.URL.Path == "/cart.js" {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		http.NotFound(w, r)
	}))
	if err := cmdCart([]string{"add", "alpha", "1", "--max", "10"}); err == nil {
		t.Fatal("expected cart read failure")
	}
}

func TestCartSetZeroRemovesUnavailableVariantWithMaximum(t *testing.T) {
	var writes atomic.Int32
	useTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/products/alpha.js":
			jsonResponse(w, testProduct("Alpha", "alpha", false))
		case "/cart/update.js":
			writes.Add(1)
			jsonResponse(w, map[string]any{"item_count": 0, "total_price": 0, "currency": "EUR"})
		default:
			http.NotFound(w, r)
		}
	}))
	if err := cmdCart([]string{"set", "alpha", "0", "--max", "0"}); err != nil {
		t.Fatalf("expected unavailable variant removal to succeed: %v", err)
	}
	if writes.Load() != 1 {
		t.Fatal("expected one cart update request")
	}
}
