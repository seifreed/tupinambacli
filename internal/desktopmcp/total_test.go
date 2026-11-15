package desktopmcp

import "testing"

func TestCalculateTotalAndValidation(t *testing.T) {
	session := connect(t, New(newShopFixture(t).client()))
	args := map[string]any{"items": []any{map[string]any{"product_ref": "arabica", "variant_ref": "ARA", "quantity": 2}}, "max_total_eur": "12.00"}
	total := output[TotalOutput](t, callTool(t, session, "calculate_total", args))
	if total.Total != "10.00€" || !total.WithinBudget || total.Items[0].SKU != "ARA" {
		t.Fatalf("total: %#v", total)
	}
	args["max_total_eur"] = "9.00"
	if got := output[TotalOutput](t, callTool(t, session, "calculate_total", args)); got.WithinBudget {
		t.Fatal("over budget marked within budget")
	}
	for _, invalid := range []map[string]any{
		{"items": []any{}, "max_total_eur": "10"},
		{"items": []any{map[string]any{"product_ref": "arabica", "quantity": 0}}, "max_total_eur": "10"},
		{"items": []any{map[string]any{"product_ref": "missing", "quantity": 1}}, "max_total_eur": "10"},
		{"items": []any{map[string]any{"product_ref": "arabica", "variant_ref": "OUT", "quantity": 1}}, "max_total_eur": "10"},
		{"items": []any{map[string]any{"product_ref": "arabica", "quantity": 1}}, "max_total_eur": "invalid"},
	} {
		if r := callTool(t, session, "calculate_total", invalid); !r.IsError {
			t.Errorf("accepted invalid total args %#v", invalid)
		}
	}
}
