package shopmcp

import "testing"

func TestRemoteFailuresRemainObservable(t *testing.T) {
	fixture := newShopFixture(t)
	session := connect(t, New(fixture.client()))
	fixture.setFail("/cart.js", 1)
	if r := callTool(t, session, "cart_status", map[string]any{}); !r.IsError {
		t.Fatal("HTTP error hidden")
	}
	fixture.badCart = true
	if r := callTool(t, session, "cart_status", map[string]any{}); !r.IsError {
		t.Fatal("malformed cart accepted")
	}
	fixture.badCart = false
	plan := output[PlanOutput](t, callTool(t, session, "plan_cart_change", planArgs("add", 1, "10.00")))
	fixture.setFail("/cart.js", 1)
	args := map[string]any{"plan_id": plan.PlanID, "confirmation": plan.ConfirmationPhrase}
	if r := callTool(t, session, "apply_cart_change", args); !r.IsError {
		t.Fatal("write proceeded without cart pre-check")
	}
}
