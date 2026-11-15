package shopmcp

import "testing"

func planArgs(operation string, quantity int, max string) map[string]any {
	return map[string]any{"operation": operation, "product_ref": "arabica", "variant_ref": "ARA", "quantity": quantity, "max_total_eur": max}
}

func TestCartChangeNeedsExactConfirmationAndCanRemove(t *testing.T) {
	fixture := newShopFixture(t)
	session := connect(t, New(fixture.client()))
	plan := output[PlanOutput](t, callTool(t, session, "plan_cart_change", planArgs("add", 2, "20.00")))
	if plan.QuantityAfter != 2 || plan.ProjectedTotal != "10.00€" || plan.ConfirmationPhrase != "CONFIRM "+plan.PlanID {
		t.Fatalf("plan: %#v", plan)
	}
	bad := map[string]any{"plan_id": plan.PlanID, "confirmation": "yes"}
	if r := callTool(t, session, "apply_cart_change", bad); !r.IsError {
		t.Fatal("wrong confirmation accepted")
	}
	good := map[string]any{"plan_id": plan.PlanID, "confirmation": plan.ConfirmationPhrase}
	applied := output[ApplyOutput](t, callTool(t, session, "apply_cart_change", good))
	if !applied.Verified || applied.Cart.ItemCount != 2 {
		t.Fatalf("apply: %#v", applied)
	}
	if r := callTool(t, session, "apply_cart_change", good); !r.IsError {
		t.Fatal("plan applied twice")
	}
	remove := output[PlanOutput](t, callTool(t, session, "plan_cart_change", planArgs("set", 0, "20.00")))
	removed := output[ApplyOutput](t, callTool(t, session, "apply_cart_change", map[string]any{"plan_id": remove.PlanID, "confirmation": remove.ConfirmationPhrase}))
	if !removed.Verified || !removed.Cart.Empty {
		t.Fatalf("remove: %#v", removed)
	}
}

func TestCartPlanRejectsInvalidAndOverBudgetInputs(t *testing.T) {
	session := connect(t, New(newShopFixture(t).client()))
	for _, args := range []map[string]any{planArgs("clear", 1, "20"), planArgs("add", 0, "20"), planArgs("set", -1, "20"), planArgs("set", 101, "20"), planArgs("add", 1, "invalid"), planArgs("add", 1, "4.99")} {
		if r := callTool(t, session, "plan_cart_change", args); !r.IsError {
			t.Errorf("accepted invalid plan %#v", args)
		}
	}
	args := planArgs("set", 0, "20")
	args["variant_ref"] = "OUT"
	if got := output[PlanOutput](t, callTool(t, session, "plan_cart_change", args)); got.QuantityAfter != 0 {
		t.Fatalf("unavailable variant removal: %#v", got)
	}
}
