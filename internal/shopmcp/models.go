package shopmcp

import (
	"time"
)

const (
	maxSearchResults = 20
	maxCartQuantity  = 100
	maxPlanCount     = 16
	planLifetime     = 5 * time.Minute
)

type ProductInput struct {
	Ref string `json:"ref" jsonschema:"Product handle, product ID, or variant SKU"`
}

type ProductOutput struct {
	ID          int64           `json:"id"`
	Handle      string          `json:"handle"`
	Title       string          `json:"title"`
	Type        string          `json:"type,omitempty"`
	Description string          `json:"description,omitempty"`
	Variants    []VariantOutput `json:"variants"`
}

type VariantOutput struct {
	ID         int64  `json:"id"`
	SKU        string `json:"sku,omitempty"`
	Title      string `json:"title"`
	Available  bool   `json:"available"`
	PriceCents int    `json:"price_cents,omitempty"`
	Price      string `json:"price,omitempty"`
}

type SearchInput struct {
	Query string `json:"query" jsonschema:"Product search terms"`
	Limit int    `json:"limit" jsonschema:"Maximum results (1-20)"`
}

type CartStatusOutput struct {
	ItemCount      int              `json:"item_count"`
	TotalCents     int              `json:"total_cents"`
	Total          string           `json:"total"`
	Currency       string           `json:"currency"`
	Empty          bool             `json:"empty"`
	CheckoutURL    string           `json:"checkout_url"`
	BrowserCartURL string           `json:"browser_cart_url,omitempty"`
	Items          []CartItemOutput `json:"items"`
}

type CartItemOutput struct {
	VariantID int64 `json:"variant_id"`
	Quantity  int   `json:"quantity"`
}

type TotalInput struct {
	Items       []TotalItemInput `json:"items" jsonschema:"Products and quantities to price"`
	MaxTotalEUR string           `json:"max_total_eur" jsonschema:"Maximum product subtotal in EUR, for example 40.00"`
}

type TotalItemInput struct {
	ProductRef string `json:"product_ref" jsonschema:"Product handle or ID"`
	VariantRef string `json:"variant_ref,omitempty" jsonschema:"Optional variant ID or SKU"`
	Quantity   int    `json:"quantity" jsonschema:"Quantity from 1 to 100"`
}

type TotalLineOutput struct {
	Product   string `json:"product"`
	Handle    string `json:"handle"`
	SKU       string `json:"sku,omitempty"`
	Variant   string `json:"variant"`
	Quantity  int    `json:"quantity"`
	LineCents int    `json:"line_cents"`
	LineTotal string `json:"line_total"`
}

type TotalOutput struct {
	Items         []TotalLineOutput `json:"items"`
	TotalCents    int               `json:"total_cents"`
	Total         string            `json:"total"`
	MaxTotalCents int               `json:"max_total_cents"`
	MaxTotal      string            `json:"max_total"`
	WithinBudget  bool              `json:"within_budget"`
}

type PlanInput struct {
	Operation   string `json:"operation" jsonschema:"add or set"`
	ProductRef  string `json:"product_ref" jsonschema:"Product handle or ID"`
	VariantRef  string `json:"variant_ref,omitempty" jsonschema:"Optional variant ID or SKU"`
	Quantity    int    `json:"quantity" jsonschema:"For add, quantity to add; for set, target quantity (0 removes the variant)"`
	MaxTotalEUR string `json:"max_total_eur" jsonschema:"Maximum projected product subtotal in EUR"`
}

type PlanOutput struct {
	PlanID              string `json:"plan_id"`
	ConfirmationPhrase  string `json:"confirmation_phrase"`
	Operation           string `json:"operation"`
	Product             string `json:"product"`
	Handle              string `json:"handle"`
	SKU                 string `json:"sku,omitempty"`
	Variant             string `json:"variant"`
	VariantID           int64  `json:"variant_id"`
	QuantityBefore      int    `json:"quantity_before"`
	QuantityAfter       int    `json:"quantity_after"`
	LineTotalCents      int    `json:"line_total_cents"`
	ProjectedTotalCents int    `json:"projected_total_cents"`
	ProjectedTotal      string `json:"projected_total"`
	MaxTotalCents       int    `json:"max_total_cents"`
	MaxTotal            string `json:"max_total"`
	ExpiresAt           string `json:"expires_at"`
}

type ApplyInput struct {
	PlanID       string `json:"plan_id" jsonschema:"Plan returned by plan_cart_change"`
	Confirmation string `json:"confirmation" jsonschema:"Must exactly match the confirmation phrase shown in the plan after the user approves it"`
}

type ApplyOutput struct {
	Outcome          string           `json:"outcome"`
	Verified         bool             `json:"verified"`
	VerificationNote string           `json:"verification_note,omitempty"`
	Cart             CartStatusOutput `json:"cart"`
}

type cartPlan struct {
	id             string
	operation      string
	productRef     string
	variantID      int64
	variantRef     string
	productTitle   string
	handle         string
	variantTitle   string
	sku            string
	quantityBefore int
	quantityAfter  int
	baseTotalCents int
	maxTotalCents  int
	lineTotalCents int
	projectedCents int
	unitPriceCents int
	expiresAt      time.Time
}
