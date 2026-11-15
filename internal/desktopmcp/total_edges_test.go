package desktopmcp

import "testing"

func TestCalculateTotalDetectsMoneyOverflow(t *testing.T) {
	fixture := newShopFixture(t)
	fixture.price = "46116860184273879.04"
	session := connect(t, New(fixture.client()))
	args := map[string]any{"items": []any{
		map[string]any{"product_ref": "arabica", "variant_ref": "ARA", "quantity": 1},
		map[string]any{"product_ref": "arabica", "variant_ref": "ARA", "quantity": 1},
	}, "max_total_eur": "92233720368547758.07"}
	if r := callTool(t, session, "calculate_total", args); !r.IsError {
		t.Fatal("total overflow accepted")
	}
}

func TestCalculateTotalDetectsLineOverflow(t *testing.T) {
	fixture := newShopFixture(t)
	fixture.price = "92233720368547758.07"
	session := connect(t, New(fixture.client()))
	args := map[string]any{"items": []any{map[string]any{"product_ref": "arabica", "variant_ref": "ARA", "quantity": 2}}, "max_total_eur": "92233720368547758.07"}
	if r := callTool(t, session, "calculate_total", args); !r.IsError {
		t.Fatal("line overflow accepted")
	}
}
