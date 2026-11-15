package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/seifreed/tupinambacli/internal/client"
	"github.com/seifreed/tupinambacli/internal/money"
)

func cmdCart(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: tupinamba cart get|add|set|clear")
	}
	cl := newClient()
	var res cartResult
	var err error
	switch args[0] {
	case "get":
		res, err = cartGet(cl, args[1:])
	case "add", "set":
		res, err = cartWrite(cl, args[0], args[1:])
	case "clear":
		res, err = cartClear(cl, args[1:])
	default:
		return fmt.Errorf("unknown cart command %q", args[0])
	}
	if err != nil {
		return err
	}
	if res.emitted {
		return nil
	}
	return emitJSON(res.raw)
}

type cartResult struct {
	raw     json.RawMessage
	emitted bool
}

func finishCart(raw json.RawMessage, err error, output outputFlags) (cartResult, error) {
	if err != nil {
		return cartResult{raw: raw}, err
	}
	safe, err := redactCartToken(raw)
	if err != nil {
		return cartResult{}, err
	}
	if output.enabled() {
		err = output.emit(safe)
		return cartResult{raw: safe, emitted: true}, err
	}
	return cartResult{raw: safe}, nil
}

func redactCartToken(raw json.RawMessage) (json.RawMessage, error) {
	var cart map[string]json.RawMessage
	if err := json.Unmarshal(raw, &cart); err != nil {
		return nil, err
	}
	if cart == nil {
		return nil, fmt.Errorf("invalid cart response")
	}
	delete(cart, "token")
	return json.Marshal(cart)
}

func cartGet(cl *client.Client, args []string) (cartResult, error) {
	fs := newFlagSet("cart get")
	output := addOutputFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return cartResult{}, err
	}
	if err := requireNoArgs(fs, "tupinamba cart get"); err != nil {
		return cartResult{}, err
	}
	raw, err := cl.Cart()
	return finishCart(raw, err, output)
}

func cartWrite(cl *client.Client, op string, args []string) (cartResult, error) {
	fs := newFlagSet("cart " + op)
	max := addMaxFlag(fs, "refuse line over EUR amount")
	variant := fs.String("variant", "", "variant id or SKU")
	output := addOutputFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return cartResult{}, err
	}
	maxCents, err := money.ParseEURCents(*max)
	if err != nil {
		return cartResult{}, err
	}
	maxSet := strings.TrimSpace(*max) != ""
	if fs.NArg() != 2 {
		return cartResult{}, fmt.Errorf("usage: tupinamba cart %s <handle|product_id|variant_id|sku> <qty> [--max eur]", op)
	}
	qty, err := strconv.Atoi(fs.Arg(1))
	if err != nil {
		return cartResult{}, err
	}
	writeVariant := *variant
	if maxSet && (op != "set" || qty != 0) {
		v, err := cl.ResolveVariant(fs.Arg(0), *variant)
		if err != nil {
			return cartResult{}, err
		}
		writeVariant = strconv.FormatInt(v.ID, 10)
		// ResolveVariant only returns variants with a valid price.
		price, _ := client.PriceCents(v)
		lineQuantity := qty
		if op == "add" {
			if qty <= 0 {
				return cartResult{}, fmt.Errorf("quantity must be positive")
			}
			existingQuantity, err := cl.CartVariantQuantity(v.ID)
			if err != nil {
				return cartResult{}, err
			}
			maxInt := int(^uint(0) >> 1)
			if existingQuantity > maxInt-lineQuantity {
				return cartResult{}, fmt.Errorf("line quantity exceeds supported range")
			}
			lineQuantity += existingQuantity
		}
		lineCents, err := money.MultiplyCents(price, lineQuantity)
		if err != nil {
			return cartResult{}, err
		}
		if lineCents > maxCents {
			return cartResult{}, fmt.Errorf("line total %s exceeds --max %s", money.FormatEURCents(lineCents), money.FormatEURCents(maxCents))
		}
	}
	var raw json.RawMessage
	if op == "add" {
		raw, err = cl.AddVariant(fs.Arg(0), writeVariant, qty)
	} else {
		raw, err = cl.SetVariant(fs.Arg(0), writeVariant, qty)
	}
	return finishCart(raw, err, output)
}

func cartClear(cl *client.Client, args []string) (cartResult, error) {
	fs := newFlagSet("cart clear")
	output := addOutputFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return cartResult{}, err
	}
	if err := requireNoArgs(fs, "tupinamba cart clear"); err != nil {
		return cartResult{}, err
	}
	raw, err := cl.Clear()
	return finishCart(raw, err, output)
}

func cmdCheckout(args []string) error {
	fs := newFlagSet("checkout")
	output := addOutputFlags(fs)
	max := addMaxFlag(fs, "refuse cart total over EUR amount")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	maxCents, err := money.ParseEURCents(*max)
	if err != nil {
		return err
	}
	maxSet := strings.TrimSpace(*max) != ""
	if fs.NArg() > 1 || (fs.NArg() == 1 && fs.Arg(0) != "status") {
		return fmt.Errorf("usage: tupinamba checkout [status]")
	}
	cl := newClient()
	cart, err := cl.CartSummary()
	if err != nil {
		return err
	}
	checkoutURL, browserCartURL := cl.CartURLs(cart)
	out := checkoutOut{
		ItemCount: cart.ItemCount, TotalCents: cart.TotalPrice, Total: money.FormatEURCents(cart.TotalPrice),
		Currency: cart.Currency, Empty: cart.ItemCount == 0, CanCheckout: cart.ItemCount > 0,
		CheckoutURL: checkoutURL, BrowserCartURL: browserCartURL,
	}
	if out.Empty {
		out.Reason = "empty_cart"
	}
	if maxSet && out.TotalCents > maxCents {
		return fmt.Errorf("cart total %s exceeds --max %s", out.Total, money.FormatEURCents(maxCents))
	}
	if output.enabled() {
		return output.emit(out)
	}
	fmt.Printf("items: %d\n", cart.ItemCount)
	fmt.Printf("total: %s %s\n", money.FormatEURCents(cart.TotalPrice), displayText(cart.Currency))
	fmt.Printf("status: %s\n", checkoutStatus(out.CanCheckout))
	fmt.Println("checkout: " + out.CheckoutURL)
	if out.BrowserCartURL != "" {
		fmt.Println("browser_cart: " + out.BrowserCartURL)
	}
	return nil
}

type checkoutOut struct {
	ItemCount      int    `json:"item_count"`
	TotalCents     int    `json:"total_cents"`
	Total          string `json:"total"`
	Currency       string `json:"currency"`
	Empty          bool   `json:"empty"`
	CanCheckout    bool   `json:"can_checkout"`
	Reason         string `json:"reason,omitempty"`
	CheckoutURL    string `json:"checkout_url"`
	BrowserCartURL string `json:"browser_cart_url,omitempty"`
}

func checkoutStatus(ok bool) string {
	if ok {
		return "ready"
	}
	return "empty_cart"
}
