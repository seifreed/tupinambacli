---
name: tupinamba-shop
description: >-
  Explore, compare, price, and prepare a capsulastupinamba.com cart with the
  local `tupinamba` CLI. Use for Tupinamba coffee, capsules, tea, latte, or
  accessory requests. It never places an order: the user completes checkout
  in the browser.
---

# Tupinamba Shop

Use the local `tupinamba` CLI to turn natural-language requests or photographed
lists into live products and, when requested, a prepared Shopify cart.

## Claude Desktop

When the `Tupinamba Shop` MCP extension is enabled in Claude Desktop, use its
tools instead of shell commands:

- Discover products with `search_products`, then inspect finalists with
  `get_product`.
- Inspect the existing cart with `cart_status` and price a proposal with
  `calculate_total`.
- Use `plan_cart_change` to show the exact product, variant, quantity change,
  projected subtotal, and confirmation phrase. Wait for the user's explicit
  approval of that plan before calling `apply_cart_change` with the exact
  phrase. Never infer approval from the initial shopping request.
- `set` quantity zero removes that variant. The Desktop tools do not place an
  order; the user reviews the returned browser cart URL and completes checkout.

If those MCP tools are unavailable, follow the CLI workflows below.

## Safety contract

1. Exploration, comparison, and totals are read-only. Before the first
   `cart add`, `cart set`, or `cart clear`, show the resolved cart diff and wait
   for explicit confirmation. A recommendation is not confirmation.
2. Before writing, establish destination and a hard product-subtotal cap. Check
   the proposed plan with `total --max`; after writing, check the whole cart
   with `checkout status --max`. `cart add/set --max` protects only the affected
   product line; `cart add` includes units of that variant already in the cart.
3. Inspect the existing cart first. Existing lines count toward the cap; never
   clear, replace, duplicate, or silently substitute them.
4. Refresh the relevant collection and resolve each finalist by **handle** with
   `product <handle>` just before totals and confirmation. The client may fall
   back to cached catalog data, so never promise stronger freshness than the
   CLI can prove.
5. Treat prices as tax-inclusive product subtotals. Shipping is provisional
   until the browser checkout; see `references/cli-reference.md` for the live
   policy caveat.
6. `checkout status` reports cart total, empty/non-empty state, and cart-specific
   browser URLs. Open `browser_cart_url` when the user must see the cart in
   their browser. It does not validate payment, address, shipping, or final
   stock. Never claim an order was placed.
7. Never print cookies, session data, or cart tokens. Use `whoami` for a safe
   cart-session check. The CLI removes the cart token from `cart` output.
8. Keep all customer-facing language centered on Tupinamba. Never name,
   compare with, or promote competing coffee or capsule brands.

## Route the request

- For discovery, preferences, quantities, budgets, substitutions, cart modes,
  or conversational feedback, read
  [references/shopping-assistance.md](references/shopping-assistance.md).
- For “premium”, “best”, or quality-based coffee recommendations, also read
  [references/quality-selection.md](references/quality-selection.md).
- For exact command shapes and operational caveats, read
  [references/cli-reference.md](references/cli-reference.md).

## Catalog workflow

The store exposes these collection handles: `cafe-en-grano`, `cafe-molido`,
`capsulas-compostables`, `piramidal`, `granel`, and `lattes`. Accessories are
best found with `search` or `product`. Do not apply capsule compatibility to
beans, ground coffee, tea, lattes, or accessories.

Discover broadly, then resolve finalists by handle:

```bash
tupinamba categories --id capsulas-compostables --limit 50 --json
tupinamba search intense --limit 5 --json
tupinamba product intense-compostable-home --json
```

For a pasted list, normalize one item per line and use `batch`. For a photo,
first show the extracted items and quantities so the user can correct them.

Read the product description for type, composition, format, capsule/serving
count, compatibility, SKU, variants, price, and availability. Use the handle
for final lookups and writes; show the SKU in the human-readable plan. For a
non-default variant, pass its SKU or id with `--variant` while keeping the
product handle as the main reference.

Compute the proposed subtotal:

```bash
printf 'adigio-compostable 2\npack-discovery 1\n' \
  | tupinamba total -f - --max 40 --json
```

Show product, handle/SKU, format, quantity, unit and line price, product
subtotal, provisional shipping, and any substitution. Revalidate, then wait
for confirmation.

## Cart workflow

Initialize or verify the cart session only when the user moves from planning
to a cart write:

```bash
tupinamba whoami
tupinamba checkout status --json
```

An anonymous Shopify cart is sufficient for a new plan. Use
`login --from-browser chrome` only when the user wants to reuse the browser's
existing cart. On non-macOS systems, use `import-har --file <file>` or
`set-cookie --stdin` for that reuse case. Cookie import is one-way: a new cart
cookie created by the CLI is not written back into Chrome.

After confirmation, prefer repeatable absolute updates:

```bash
tupinamba cart set adigio-compostable 2 --max 10
tupinamba cart set pack-discovery 1 --max 30
tupinamba checkout status --max 40 --json
```

Use `cart add` only for explicitly additive intent. Compare the resulting cart
with the approved diff and stop on any mismatch; never blindly retry a write.
To make the result visible in the user's browser, open the returned
`browser_cart_url`; do not open a generic `/checkout` URL.
