package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type CartSummary struct {
	TotalPrice int        `json:"total_price"`
	ItemCount  int        `json:"item_count"`
	Currency   string     `json:"currency"`
	Items      []CartLine `json:"items"`
}

type CartLine struct {
	VariantID int64 `json:"variant_id"`
	Quantity  int   `json:"quantity"`
}

func (c *Client) Cart() (json.RawMessage, error) {
	var raw json.RawMessage
	return raw, c.doJSON(http.MethodGet, c.baseURL+"/cart.js", nil, &raw)
}

func (c *Client) CartSummary() (CartSummary, error) {
	raw, err := c.Cart()
	if err != nil {
		return CartSummary{}, err
	}
	var cart CartSummary
	if err := json.Unmarshal(raw, &cart); err != nil {
		return CartSummary{}, err
	}
	if cart.TotalPrice < 0 || cart.ItemCount < 0 {
		return CartSummary{}, fmt.Errorf("invalid cart response")
	}
	count := 0
	for _, item := range cart.Items {
		if item.VariantID <= 0 || item.Quantity <= 0 || item.Quantity > int(^uint(0)>>1)-count {
			return CartSummary{}, fmt.Errorf("invalid cart item")
		}
		count += item.Quantity
	}
	if count != cart.ItemCount {
		return CartSummary{}, fmt.Errorf("cart item count does not match lines")
	}
	return cart, nil
}

func (c *Client) CartVariantQuantity(variantID int64) (int, error) {
	cart, err := c.CartSummary()
	if err != nil {
		return 0, err
	}
	quantity := 0
	for _, item := range cart.Items {
		if item.VariantID == variantID {
			quantity += item.Quantity
		}
	}
	return quantity, nil
}

func (c *Client) CartURLs(cart CartSummary) (string, string) {
	if cart.ItemCount == 0 {
		return c.baseURL + "/checkout", ""
	}
	lines := make([]string, len(cart.Items))
	for i, item := range cart.Items {
		lines[i] = strconv.FormatInt(item.VariantID, 10) + ":" + strconv.Itoa(item.Quantity)
	}
	checkout := c.baseURL + "/cart/" + strings.Join(lines, ",")
	return checkout, checkout + "?storefront=true"
}

func (c *Client) AddVariant(ref, variantRef string, qty int) (json.RawMessage, error) {
	if qty <= 0 {
		return nil, fmt.Errorf("quantity must be positive")
	}
	v, err := c.ResolveVariant(ref, variantRef)
	if err != nil {
		return nil, err
	}
	var raw json.RawMessage
	return raw, c.doJSON(http.MethodPost, c.baseURL+"/cart/add.js", map[string]any{"id": strconv.FormatInt(v.ID, 10), "quantity": qty}, &raw)
}

func (c *Client) SetVariant(ref, variantRef string, qty int) (json.RawMessage, error) {
	if qty < 0 {
		return nil, fmt.Errorf("quantity must be >= 0")
	}
	v, err := c.resolveVariant(ref, variantRef, qty != 0)
	if err != nil {
		return nil, err
	}
	var raw json.RawMessage
	return raw, c.doJSON(http.MethodPost, c.baseURL+"/cart/update.js", map[string]any{
		"updates": map[string]int{strconv.FormatInt(v.ID, 10): qty},
	}, &raw)
}

func (c *Client) Clear() (json.RawMessage, error) {
	var raw json.RawMessage
	return raw, c.doJSON(http.MethodPost, c.baseURL+"/cart/clear.js", map[string]any{}, &raw)
}

func (c *Client) ResolveVariant(ref, variantRef string) (Variant, error) {
	return c.resolveVariant(ref, variantRef, true)
}

func (c *Client) resolveVariant(ref, variantRef string, requireAvailable bool) (Variant, error) {
	p, err := c.Product(ref)
	if err != nil {
		return Variant{}, err
	}
	if variantRef == "" {
		if _, ok := FindVariant(*p, ref); ok {
			variantRef = ref
		}
	}
	if variantRef != "" {
		for _, v := range p.Variants {
			if strconv.FormatInt(v.ID, 10) == variantRef || strings.EqualFold(v.SKU, variantRef) {
				if requireAvailable && !v.Available {
					return Variant{}, fmt.Errorf("variant %q is unavailable", variantRef)
				}
				if requireAvailable {
					if _, ok := PriceCents(v); !ok {
						return Variant{}, fmt.Errorf("variant %q has an invalid price", variantRef)
					}
				}
				return v, nil
			}
		}
		return Variant{}, fmt.Errorf("variant %q not found in %q", variantRef, p.Title)
	}
	if v, ok := BestVariant(*p); ok {
		return v, nil
	}
	if !requireAvailable && len(p.Variants) > 0 {
		return p.Variants[0], nil
	}
	return Variant{}, fmt.Errorf("%q is unavailable", p.Title)
}
