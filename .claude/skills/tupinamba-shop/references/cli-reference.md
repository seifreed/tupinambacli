# `tupinamba` CLI Reference

All data goes to stdout; errors go to stderr. Exit codes: `0` success, `1`
runtime error, `2` usage error.

## Commands

| Command | Purpose |
|---|---|
| `search <term> --limit N --json --toon` | Search title, description, type, and tags. Results prefer available, cheaper products. |
| `product <handle\|product_id\|variant_id\|sku> --json --toon` | Show product details and variants. A handle attempts the direct product endpoint first. |
| `batch -f <file\|-> --json --toon` | Resolve one search term per input line. |
| `total -f <file\|-> --max EUR --json --toon` | Sum `<ref> [qty]` lines and enforce a product-subtotal cap. |
| `collections --json --toon` | List Shopify collections. |
| `categories --id <handle> --limit N --json --toon` | List products in a collection. |
| `login --from-browser chrome` | Import a macOS Chrome cart session for reuse. |
| `import-har --file <file>` | Import a Tupinamba Cookie header from a HAR. |
| `set-cookie --stdin` | Save a raw Cookie header without exposing it in shell history. |
| `whoami` | Verify that the cart session can be read; it does not identify a customer account. |
| `cart get --json --toon` | Read the cart; the CLI redacts its token from output. |
| `cart add <ref> <qty> --max EUR` | Add quantity; cap the resulting variant line, including its existing quantity. |
| `cart set <ref> <qty> --max EUR` | Set absolute quantity and cap that variant line; use `0` to remove. |
| `cart clear` | Empty the cart after explicit confirmation. |
| `checkout status --max EUR --json --toon` | Show total, item count, status, a cart-specific checkout URL, and `browser_cart_url`. |

Product references may be handles, product ids, variant ids, or SKUs. Use the
handle for live final lookup and writes. For a selected non-default variant,
use `--variant <sku|id>`.

## Live-data boundaries

`search` and SKU/id fallback lookups may use `catalog.json`, cached for up to
10 minutes. Collection listing is fetched directly. `product <handle>` tries
the live Shopify product endpoint first, but silently falls back to catalog
lookup if that request fails. Compare both sources and describe freshness as
best-effort, not guaranteed.

`total` resolves every reference again, but references other than handles may
come from cache. It rejects unavailable products and non-positive quantities.

`cart set` is idempotent; `cart add` is additive. A line-level `--max` does not
protect the whole cart; for `cart add` it includes the existing quantity of the
selected variant. `checkout status --max` checks the whole current product
subtotal, but `can_checkout` only means the cart is non-empty.
Opening `browser_cart_url` transfers that resolved cart into the browser.
Importing browser cookies alone does not copy later CLI cookie updates back.

## Shipping and environment

The store currently ships to Peninsular Spain and the Balearic Islands. Its
policy lists 6 EUR and 12 EUR respectively below the free-shipping threshold.
The banner says free shipping “from 30 EUR”, while the policy says orders
“over 30 EUR”; do not rely on exactly 30 EUR. Shipping is finalized only after
the address is entered in browser checkout.

The live policy currently estimates delivery in 24-48 hours depending on its
13:00 cutoff and names OXPERTA as courier. Re-read the policy before quoting
delivery details; `checkout status` does not provide them.

`TUPINAMBA_CONFIG_DIR` overrides the config directory.
`TUPINAMBA_MAX_EUR` sets the default cap for cart, total, and checkout commands.
