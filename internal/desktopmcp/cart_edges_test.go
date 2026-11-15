package desktopmcp

import (
	"errors"
	"math"
	"testing"
	"time"
)

func TestPlanRejectsCartLimitsAndExpiresOldPlans(t *testing.T) {
	fixture := newShopFixture(t)
	service := New(fixture.client())
	session := connect(t, service)
	fixture.setCart(100, 50000)
	if r := callTool(t, session, "plan_cart_change", planArgs("add", 1, "999999")); !r.IsError {
		t.Fatal("quantity cap ignored")
	}
	fixture.setCart(1, 0)
	if r := callTool(t, session, "plan_cart_change", planArgs("add", 1, "999999")); !r.IsError {
		t.Fatal("inconsistent cart total accepted")
	}
	fixture.setCart(0, 0)
	clock := time.Now()
	service.now = func() time.Time { return clock }
	counter := byte(0)
	service.random = func(dst []byte) (int, error) {
		counter++
		for i := range dst {
			dst[i] = counter
		}
		return len(dst), nil
	}
	for range maxPlanCount {
		if r := callTool(t, session, "plan_cart_change", planArgs("add", 1, "999999")); r.IsError {
			t.Fatalf("plan %d failed: %v", counter, r.Content)
		}
	}
	if r := callTool(t, session, "plan_cart_change", planArgs("add", 1, "999999")); !r.IsError {
		t.Fatal("pending plan limit ignored")
	}
	clock = clock.Add(planLifetime + time.Second)
	if r := callTool(t, session, "plan_cart_change", planArgs("add", 1, "999999")); r.IsError {
		t.Fatal("expired plans were not pruned")
	}
}

func TestPlanPropagatesNetworkMoneyAndRandomFailures(t *testing.T) {
	fixture := newShopFixture(t)
	session := connect(t, New(fixture.client()))
	fixture.setFail("/products/arabica.js", 1)
	fixture.setFail("/products.json", 1)
	if r := callTool(t, session, "plan_cart_change", planArgs("add", 1, "999999")); !r.IsError {
		t.Fatal("product fetch failure hidden")
	}
	fixture.price = "92233720368547758.07"
	if r := callTool(t, session, "plan_cart_change", planArgs("add", 2, "92233720368547758.07")); !r.IsError {
		t.Fatal("multiplication overflow accepted")
	}
	fixture.price = "5.00"
	fixture.setCart(1, math.MaxInt)
	args := planArgs("set", 2, "92233720368547758.07")
	if r := callTool(t, session, "plan_cart_change", args); !r.IsError {
		t.Fatal("total overflow accepted")
	}
	fixture.setCart(0, 0)
	service := New(fixture.client())
	service.random = func([]byte) (int, error) { return 0, errors.New("secret randomness failure") }
	if r := callTool(t, connect(t, service), "plan_cart_change", planArgs("add", 1, "10.00")); !r.IsError {
		t.Fatal("random source failure hidden")
	}
}

func TestApplyReportsPostWriteVerificationFailures(t *testing.T) {
	fixture := newShopFixture(t)
	session := connect(t, New(fixture.client()))
	plan := output[PlanOutput](t, callTool(t, session, "plan_cart_change", planArgs("add", 1, "10.00")))
	fixture.failAfterWrite = true
	res := output[ApplyOutput](t, callTool(t, session, "apply_cart_change", map[string]any{"plan_id": plan.PlanID, "confirmation": plan.ConfirmationPhrase}))
	if res.Outcome != "applied_unverified" || res.Verified {
		t.Fatalf("post-write failure: %#v", res)
	}
	plan = output[PlanOutput](t, callTool(t, session, "plan_cart_change", planArgs("add", 1, "10.00")))
	fixture.overBudgetAfterWrite = true
	res = output[ApplyOutput](t, callTool(t, session, "apply_cart_change", map[string]any{"plan_id": plan.PlanID, "confirmation": plan.ConfirmationPhrase}))
	if res.Outcome != "applied_unverified" || res.Verified {
		t.Fatalf("unmatched result: %#v", res)
	}
}

func TestPlanHandlesCartReadFailureAndExistingQuantityOverflow(t *testing.T) {
	fixture := newShopFixture(t)
	session := connect(t, New(fixture.client()))
	fixture.badCart = true
	if r := callTool(t, session, "plan_cart_change", planArgs("add", 1, "20")); !r.IsError {
		t.Fatal("malformed cart accepted")
	}
	fixture.badCart = false
	fixture.price = "92233720368547758.07"
	fixture.setCart(2, math.MaxInt)
	if r := callTool(t, session, "plan_cart_change", planArgs("set", 1, "92233720368547758.07")); !r.IsError {
		t.Fatal("existing line overflow accepted")
	}
}

func TestApplyHandlesProductAndCartWriteErrors(t *testing.T) {
	fixture := newShopFixture(t)
	session := connect(t, New(fixture.client()))
	plan := output[PlanOutput](t, callTool(t, session, "plan_cart_change", planArgs("add", 1, "10")))
	fixture.price = "bad"
	args := map[string]any{"plan_id": plan.PlanID, "confirmation": plan.ConfirmationPhrase}
	if r := callTool(t, session, "apply_cart_change", args); !r.IsError {
		t.Fatal("invalid updated product accepted")
	}
	fixture.price = "5.00"
	plan = output[PlanOutput](t, callTool(t, session, "plan_cart_change", planArgs("add", 1, "10")))
	fixture.setFail("/cart/add.js", 1)
	args = map[string]any{"plan_id": plan.PlanID, "confirmation": plan.ConfirmationPhrase}
	if r := callTool(t, session, "apply_cart_change", args); !r.IsError {
		t.Fatal("cart write error hidden")
	}
}
