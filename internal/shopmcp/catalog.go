package shopmcp

import (
	"context"
	"errors"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/seifreed/tupinambacli/internal/client"
	"github.com/seifreed/tupinambacli/internal/money"
	"strings"
	"time"
)

func (s *Server) searchProducts(ctx context.Context, _ *mcp.CallToolRequest, in SearchInput) (toolResult *mcp.CallToolResult, output SearchOutput, resultErr error) {
	started := time.Now()
	defer func() { s.logTool(ctx, "search_products", started, resultErr) }()
	if strings.TrimSpace(in.Query) == "" || len([]rune(in.Query)) > 100 || in.Limit < 1 || in.Limit > maxSearchResults {
		return nil, SearchOutput{}, errors.New("query must be 1-100 characters and limit must be between 1 and 20")
	}
	products, err := s.shop.Search(in.Query, in.Limit)
	if err != nil {
		return nil, SearchOutput{}, err
	}
	return nil, SearchOutput{Products: productOutputs(products)}, nil
}

func (s *Server) getProduct(ctx context.Context, _ *mcp.CallToolRequest, in ProductInput) (toolResult *mcp.CallToolResult, output ProductOutput, resultErr error) {
	started := time.Now()
	defer func() { s.logTool(ctx, "get_product", started, resultErr) }()
	if strings.TrimSpace(in.Ref) == "" || len([]rune(in.Ref)) > 100 {
		return nil, ProductOutput{}, errors.New("ref must be 1-100 characters")
	}
	p, err := s.shop.Product(in.Ref)
	if err != nil {
		return nil, ProductOutput{}, err
	}
	return nil, productOutput(*p), nil
}

func (s *Server) cartStatus(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (toolResult *mcp.CallToolResult, output CartStatusOutput, resultErr error) {
	started := time.Now()
	defer func() { s.logTool(ctx, "cart_status", started, resultErr) }()
	cart, err := s.shop.CartSummary()
	if err != nil {
		return nil, CartStatusOutput{}, err
	}
	return nil, cartStatusOutput(s.shop, cart), nil
}

func (s *Server) calculateTotal(ctx context.Context, _ *mcp.CallToolRequest, in TotalInput) (toolResult *mcp.CallToolResult, output TotalOutput, resultErr error) {
	started := time.Now()
	defer func() { s.logTool(ctx, "calculate_total", started, resultErr) }()
	if len(in.Items) == 0 || len(in.Items) > 20 {
		return nil, TotalOutput{}, errors.New("items must contain between 1 and 20 lines")
	}
	maxCents, err := money.ParseEURCents(in.MaxTotalEUR)
	if err != nil {
		return nil, TotalOutput{}, fmt.Errorf("invalid max_total_eur: %w", err)
	}
	result := TotalOutput{Items: make([]TotalLineOutput, 0, len(in.Items)), MaxTotalCents: maxCents, MaxTotal: money.FormatEURCents(maxCents)}
	for _, item := range in.Items {
		if item.Quantity < 1 || item.Quantity > maxCartQuantity || strings.TrimSpace(item.ProductRef) == "" {
			return nil, TotalOutput{}, errors.New("each item needs a product reference and quantity from 1 to 100")
		}
		p, v, price, err := s.resolveVariant(item.ProductRef, item.VariantRef, true)
		if err != nil {
			return nil, TotalOutput{}, err
		}
		lineCents, err := money.MultiplyCents(price, item.Quantity)
		if err != nil {
			return nil, TotalOutput{}, err
		}
		result.TotalCents, err = money.AddCents(result.TotalCents, lineCents)
		if err != nil {
			return nil, TotalOutput{}, err
		}
		result.Items = append(result.Items, TotalLineOutput{Product: p.Title, Handle: p.Handle, SKU: v.SKU, Variant: v.Title, Quantity: item.Quantity, LineCents: lineCents, LineTotal: money.FormatEURCents(lineCents)})
	}
	result.Total = money.FormatEURCents(result.TotalCents)
	result.WithinBudget = result.TotalCents <= maxCents
	return nil, result, nil
}

func (s *Server) resolveVariant(ref, variantRef string, requireAvailable bool) (client.Product, client.Variant, int, error) {
	if strings.TrimSpace(ref) == "" || len([]rune(ref)) > 100 {
		return client.Product{}, client.Variant{}, 0, errors.New("product_ref must be 1-100 characters")
	}
	p, err := s.shop.Product(ref)
	if err != nil {
		return client.Product{}, client.Variant{}, 0, err
	}
	var v client.Variant
	if variantRef == "" {
		if direct, ok := client.FindVariant(*p, ref); ok {
			v = direct
		} else if best, ok := client.BestVariant(*p); ok {
			v = best
		} else {
			return client.Product{}, client.Variant{}, 0, fmt.Errorf("%q has no available variant", p.Title)
		}
	} else {
		var found bool
		v, found = client.FindVariant(*p, variantRef)
		if !found {
			return client.Product{}, client.Variant{}, 0, fmt.Errorf("variant %q not found in %q", variantRef, p.Title)
		}
	}
	if requireAvailable && !v.Available {
		return client.Product{}, client.Variant{}, 0, fmt.Errorf("variant %q is unavailable", variantRefOrID(v, variantRef))
	}
	price, valid := client.PriceCents(v)
	if !valid {
		return client.Product{}, client.Variant{}, 0, fmt.Errorf("variant %q has an invalid price", variantRefOrID(v, variantRef))
	}
	return *p, v, price, nil
}
