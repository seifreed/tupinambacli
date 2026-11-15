package shopmcp

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/seifreed/tupinambacli/internal/money"
)

func (s *Server) planCartChange(ctx context.Context, _ *mcp.CallToolRequest, in PlanInput) (toolResult *mcp.CallToolResult, output PlanOutput, resultErr error) {
	started := time.Now()
	defer func() { s.logTool(ctx, "plan_cart_change", started, resultErr) }()
	if in.Operation != "add" && in.Operation != "set" {
		return nil, PlanOutput{}, errors.New("operation must be add or set")
	}
	if in.Operation == "add" && (in.Quantity < 1 || in.Quantity > maxCartQuantity) || in.Operation == "set" && (in.Quantity < 0 || in.Quantity > maxCartQuantity) {
		return nil, PlanOutput{}, fmt.Errorf("quantity is outside the allowed range 0-%d", maxCartQuantity)
	}
	maxCents, err := money.ParseEURCents(in.MaxTotalEUR)
	if err != nil {
		return nil, PlanOutput{}, fmt.Errorf("invalid max_total_eur: %w", err)
	}
	p, v, price, err := s.resolveVariant(in.ProductRef, in.VariantRef, in.Quantity != 0)
	if err != nil {
		return nil, PlanOutput{}, err
	}
	cart, err := s.shop.CartSummary()
	if err != nil {
		return nil, PlanOutput{}, err
	}
	before := 0
	for _, line := range cart.Items {
		if line.VariantID == v.ID {
			before += line.Quantity
		}
	}
	after := in.Quantity
	if in.Operation == "add" {
		if before > maxCartQuantity-in.Quantity {
			return nil, PlanOutput{}, fmt.Errorf("variant quantity exceeds the allowed maximum of %d", maxCartQuantity)
		}
		after += before
	}
	oldLineCents, err := money.MultiplyCents(price, before)
	if err != nil {
		return nil, PlanOutput{}, err
	}
	lineCents, err := money.MultiplyCents(price, after)
	if err != nil {
		return nil, PlanOutput{}, err
	}
	projected := cart.TotalPrice - oldLineCents
	if projected < 0 {
		return nil, PlanOutput{}, errors.New("cart total is inconsistent with the current product price; refresh the cart and retry")
	}
	projected, err = money.AddCents(projected, lineCents)
	if err != nil {
		return nil, PlanOutput{}, err
	}
	if projected > maxCents {
		return nil, PlanOutput{}, fmt.Errorf("projected cart total %s exceeds max_total_eur %s", money.FormatEURCents(projected), money.FormatEURCents(maxCents))
	}
	var idBytes [8]byte
	if _, err := s.random(idBytes[:]); err != nil {
		return nil, PlanOutput{}, errors.New("could not create a cart plan")
	}
	now := s.now()
	plan := cartPlan{id: hex.EncodeToString(idBytes[:]), operation: in.Operation, productRef: p.Handle, variantID: v.ID, variantRef: fmt.Sprint(v.ID), productTitle: p.Title, handle: p.Handle, variantTitle: v.Title, sku: v.SKU, quantityBefore: before, quantityAfter: after, baseTotalCents: cart.TotalPrice, maxTotalCents: maxCents, lineTotalCents: lineCents, projectedCents: projected, unitPriceCents: price, expiresAt: now.Add(planLifetime)}
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, pending := range s.plans {
		if !now.Before(pending.expiresAt) {
			delete(s.plans, key)
		}
	}
	if len(s.plans) >= maxPlanCount {
		return nil, PlanOutput{}, errors.New("too many pending cart plans; apply or wait for one to expire")
	}
	s.plans[plan.id] = plan
	return nil, planOutput(plan), nil
}
