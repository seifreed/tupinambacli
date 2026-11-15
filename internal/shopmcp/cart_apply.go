package shopmcp

import (
	"context"
	"errors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"time"
)

func (s *Server) applyCartChange(ctx context.Context, _ *mcp.CallToolRequest, in ApplyInput) (toolResult *mcp.CallToolResult, output ApplyOutput, resultErr error) {
	started := time.Now()
	defer func() { s.logTool(ctx, "apply_cart_change", started, resultErr) }()
	s.apply.Lock()
	defer s.apply.Unlock()
	s.mu.Lock()
	plan, ok := s.plans[in.PlanID]
	if !ok {
		s.mu.Unlock()
		return nil, ApplyOutput{}, errors.New("cart plan not found or already used; create a new plan")
	}
	if !s.now().Before(plan.expiresAt) {
		delete(s.plans, in.PlanID)
		s.mu.Unlock()
		return nil, ApplyOutput{}, errors.New("cart plan expired; create a new plan")
	}
	if in.Confirmation != "CONFIRM "+plan.id {
		s.mu.Unlock()
		return nil, ApplyOutput{}, errors.New("confirmation does not match the plan")
	}
	delete(s.plans, in.PlanID)
	s.mu.Unlock()

	cart, err := s.shop.CartSummary()
	if err != nil {
		return nil, ApplyOutput{}, err
	}
	current := 0
	for _, line := range cart.Items {
		if line.VariantID == plan.variantID {
			current += line.Quantity
		}
	}
	if cart.TotalPrice != plan.baseTotalCents || current != plan.quantityBefore {
		return nil, ApplyOutput{}, errors.New("cart changed after planning; inspect it and create a new plan")
	}
	_, variant, price, err := s.resolveVariant(plan.productRef, plan.variantRef, plan.quantityAfter != 0)
	if err != nil {
		return nil, ApplyOutput{}, err
	}
	if variant.ID != plan.variantID || price != plan.unitPriceCents {
		return nil, ApplyOutput{}, errors.New("product price or variant changed after planning; create a new plan")
	}
	if plan.operation == "add" {
		_, err = s.shop.AddVariant(plan.productRef, plan.variantRef, plan.quantityAfter-plan.quantityBefore)
	} else {
		_, err = s.shop.SetVariant(plan.productRef, plan.variantRef, plan.quantityAfter)
	}
	if err != nil {
		return nil, ApplyOutput{}, err
	}
	updated, err := s.shop.CartSummary()
	if err != nil {
		return nil, ApplyOutput{Outcome: "applied_unverified", VerificationNote: "The change was sent, but the resulting cart could not be verified."}, nil
	}
	verifiedQuantity := 0
	for _, line := range updated.Items {
		if line.VariantID == plan.variantID {
			verifiedQuantity += line.Quantity
		}
	}
	if verifiedQuantity != plan.quantityAfter || updated.TotalPrice > plan.maxTotalCents {
		return nil, ApplyOutput{Outcome: "applied_unverified", VerificationNote: "The resulting quantity or subtotal differs from the approved plan; inspect the cart before retrying."}, nil
	}
	return nil, ApplyOutput{Outcome: "applied", Verified: true, Cart: cartStatusOutput(s.shop, updated)}, nil
}
