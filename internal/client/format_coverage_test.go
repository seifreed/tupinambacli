package client

import (
	"encoding/json"
	"strconv"
	"testing"
)

func TestFormatHelpersRejectBadPrices(t *testing.T) {
	for _, raw := range []string{`""`, `"."`, `"bad"`, `"NaN"`, `"+Inf"`, `"-1"`, `"-0.50"`, `"999999999999999999999999"`, `{}`, `null`} {
		if _, ok := PriceCents(Variant{Price: json.RawMessage(raw)}); ok {
			t.Fatalf("price %s accepted", raw)
		}
	}
	if _, ok := PriceCents(Variant{Price: json.RawMessage(`"1.25"`)}); !ok {
		t.Fatal("expected valid decimal price")
	}
}

func TestPriceCentsPreservesLargeDecimalPrecision(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("large decimal values require 64-bit cents")
	}
	tests := []struct {
		price string
		want  int
	}{
		{price: `"90071992547409.90"`, want: 9007199254740990},
		{price: `"90071992547409.91"`, want: 9007199254740991},
	}
	for _, tt := range tests {
		got, ok := PriceCents(Variant{Price: json.RawMessage(tt.price)})
		if !ok || got != tt.want {
			t.Errorf("PriceCents(%s) = %d, %v; want %d, true", tt.price, got, ok, tt.want)
		}
	}
}

func TestPriceCentsRejectsOverflowingDecimal(t *testing.T) {
	if _, ok := PriceCents(Variant{Price: json.RawMessage(`"92233720368547758.08"`)}); ok {
		t.Fatal("accepted decimal price greater than max int cents")
	}
}

func TestFormatTextAndSorting(t *testing.T) {
	if got := PlainText(`<p>Caf&eacute;</p> <b>en grano</b>`); got != "Café en grano" {
		t.Fatalf("plain text=%q", got)
	}
	products := []Product{{Title: "b"}, {Title: "a"}, {Title: "available", Variants: []Variant{{ID: 1, Available: true, Price: json.RawMessage(`"2.00"`)}}}}
	SortProducts(products)
	if products[0].Title != "available" || products[1].Title != "a" {
		t.Fatalf("sorted=%v", products)
	}
	SortProducts([]Product{
		{Title: "available", Variants: []Variant{{ID: 2, Available: true, Price: json.RawMessage(`"2.00"`)}}},
		{Title: "sold"},
	})
}
