package shopmcp

import (
	"testing"
	"time"
)

func TestApplyRejectsUnknownStaleExpiredAndPriceChangedPlans(t *testing.T) {
	fixture := newShopFixture(t)
	service := New(fixture.client())
	session := connect(t, service)
	plan := output[PlanOutput](t, callTool(t, session, "plan_cart_change", planArgs("add", 1, "10.00")))
	unknown := map[string]any{"plan_id": "missing", "confirmation": "CONFIRM missing"}
	if r := callTool(t, session, "apply_cart_change", unknown); !r.IsError {
		t.Fatal("unknown plan accepted")
	}
	fixture.setCart(1, 500)
	args := map[string]any{"plan_id": plan.PlanID, "confirmation": plan.ConfirmationPhrase}
	if r := callTool(t, session, "apply_cart_change", args); !r.IsError {
		t.Fatal("stale cart plan accepted")
	}
	fixture.setCart(0, 0)
	plan = output[PlanOutput](t, callTool(t, session, "plan_cart_change", planArgs("add", 1, "10.00")))
	service.now = func() time.Time { return time.Now().Add(10 * time.Minute) }
	args = map[string]any{"plan_id": plan.PlanID, "confirmation": plan.ConfirmationPhrase}
	if r := callTool(t, session, "apply_cart_change", args); !r.IsError {
		t.Fatal("expired plan accepted")
	}
	service.now = time.Now
	plan = output[PlanOutput](t, callTool(t, session, "plan_cart_change", planArgs("add", 1, "10.00")))
	fixture.setPrice("6.00")
	args = map[string]any{"plan_id": plan.PlanID, "confirmation": plan.ConfirmationPhrase}
	if r := callTool(t, session, "apply_cart_change", args); !r.IsError {
		t.Fatal("stale price plan accepted")
	}
}
