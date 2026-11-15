package desktopmcp

import (
	"fmt"
	"github.com/seifreed/tupinambacli/internal/client"
	"github.com/seifreed/tupinambacli/internal/money"
	"time"
)

func productOutputs(products []client.Product) []ProductOutput {
	result := make([]ProductOutput, len(products))
	for i, product := range products {
		result[i] = productOutput(product)
	}
	return result
}

func productOutput(p client.Product) ProductOutput {
	description := client.PlainText(p.Description)
	if description == "" {
		description = client.PlainText(p.BodyHTML)
	}
	if runes := []rune(description); len(runes) > 1200 {
		description = string(runes[:1200])
	}
	result := ProductOutput{ID: p.ID, Handle: p.Handle, Title: p.Title, Type: p.Type, Description: description, Variants: make([]VariantOutput, 0, len(p.Variants))}
	for _, variant := range p.Variants {
		out := VariantOutput{ID: variant.ID, SKU: variant.SKU, Title: variant.Title, Available: variant.Available}
		if cents, ok := client.PriceCents(variant); ok {
			out.PriceCents = cents
			out.Price = money.FormatEURCents(cents)
		}
		result.Variants = append(result.Variants, out)
	}
	return result
}

func cartStatusOutput(shop *client.Client, cart client.CartSummary) CartStatusOutput {
	checkout, browser := shop.CartURLs(cart)
	items := make([]CartItemOutput, len(cart.Items))
	for i, item := range cart.Items {
		items[i] = CartItemOutput{VariantID: item.VariantID, Quantity: item.Quantity}
	}
	return CartStatusOutput{ItemCount: cart.ItemCount, TotalCents: cart.TotalPrice, Total: money.FormatEURCents(cart.TotalPrice), Currency: cart.Currency, Empty: cart.ItemCount == 0, CheckoutURL: checkout, BrowserCartURL: browser, Items: items}
}

func planOutput(plan cartPlan) PlanOutput {
	return PlanOutput{PlanID: plan.id, ConfirmationPhrase: "CONFIRM " + plan.id, Operation: plan.operation, Product: plan.productTitle, Handle: plan.handle, SKU: plan.sku, Variant: plan.variantTitle, VariantID: plan.variantID, QuantityBefore: plan.quantityBefore, QuantityAfter: plan.quantityAfter, LineTotalCents: plan.lineTotalCents, ProjectedTotalCents: plan.projectedCents, ProjectedTotal: money.FormatEURCents(plan.projectedCents), MaxTotalCents: plan.maxTotalCents, MaxTotal: money.FormatEURCents(plan.maxTotalCents), ExpiresAt: plan.expiresAt.UTC().Format(time.RFC3339)}
}

func variantRefOrID(v client.Variant, ref string) string {
	if ref != "" {
		return ref
	}
	return fmt.Sprint(v.ID)
}
