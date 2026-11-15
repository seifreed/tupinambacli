# Shopping Assistance

Use this guide for discovery, preferences, quantities, budgets, substitutions,
and cart intent.

## Conversation modes

For exploration, stay read-only. Compare two or three available candidates in
a compact table: product, profile, composition, format/count, price per unit or
capsule when known, availability, and evidence freshness. Do not ask for budget,
destination, authentication, or cart confirmation until the user wants a plan
that can be written.

For a purchase plan, reuse known context and ask only the next blocking question
when relevant: hard budget, quantity or daily consumption, destination, then
taste. State non-blocking assumptions as “He asumido que…” and let the user
correct them before totals.

Infer the occasion when useful:

- daily use: suitable repeatable format and duration;
- tasting: distinct profiles or an explicit variety pack;
- office: known serving count within budget;
- gift: variety and presentation only when product data supports it;
- replenishment: the latest valid proposal in this conversation, never assumed
  order history.

“Déjalo preparado” means keep a read-only plan. “Cancela la propuesta” removes
the pending plan and approval without touching the cart.

## Constraints and recommendation

Treat compatibility, product type/format, explicit composition or dietary
filters, hard budget, and exclusions as hard constraints. Never relax them
silently. Treat taste, quality, variety, value, and duration as objectives; use
the user's stated priority and explain the trade-off.

Match taste only to live description evidence: intensity/body/crema, elegance
or balance, roast, and named tasting notes. Treat caffeine, ingredients,
allergens, origin, certifications, compostability, and sustainability as facts
only when the product page states them. Say “no consta en la ficha” when absent;
do not infer medical or dietary safety.

Separate evidence from judgment:

- “La ficha indica”: concrete product facts;
- “Mi recomendación”: why they match the request;
- confidence: high for direct detailed evidence, medium for partial evidence,
  low for sparse descriptions or ties.

For compound requests such as “best under 20 EUR” or “premium for one month”,
compare a small number of valid plans. If none satisfies every objective, show
the closest plan and the smallest required compromise. Briefly explain material
rejections: unavailable, incompatible, wrong format, missing evidence, weaker
taste match, or over budget.

## Quantities, packs, and budget

Normalize the requested unit before calculation:

- capsule or serving: one drink;
- box/estuche: the count stated on its product page;
- pack: one purchasable bundle with explicitly listed contents;
- variety: a distinct profile, not a quantity.

“20 coffees” means 20 servings, not 20 boxes. Ask when the unit is ambiguous.
For packs, calculate price per capsule and compare equivalent boxes only when
counts and formats are known. Label this as value, not quality.

If daily consumption is known, show approximate duration from explicit serving
counts. Do not estimate servings for beans or ground coffee without a dosage
basis. Respect the hard product-subtotal cap, show provisional shipping
separately, and never add unwanted items merely to reach free shipping.

## Compatibility and substitutions

For capsules, discuss compatibility only when the user explicitly asks and use
generic wording without naming third-party brands. Report only compatibility
stated on the product page. Clearly label beans, ground coffee, tea, lattes,
and accessories as non-capsule products.

If a selection becomes unavailable, preserve hard constraints, inspect the same
collection, and offer the closest evidence-backed replacement with format,
quantity, SKU, and price difference. Ask before changing the plan. Never
silently swap 100% Arabica for a blend, a box for a pack, decaf for caffeinated,
or one machine system for another.

## Live state and approval

Follow the live-data boundaries in `cli-reference.md`: refresh the collection,
then resolve finalists with `product <handle>`. Describe the result as
best-effort current data because product lookup can fall back to cache.

If collection and product data disagree, the revalidated product page wins and
the discrepancy should be stated. Any change to product, price, availability,
variant, machine, format, quantity, budget, substitution, or other hard
constraint invalidates the pending plan and its confirmation.

Track accepted/rejected candidates and stated preferences only in the current
conversation. Positive or negative feedback adjusts later recommendations but
is neither permanent memory nor permission to write the cart.

## Cart intent and verification

- “recomiéndame”, “compara”, or “prepara”: read-only;
- “añade”: additive intent; use `cart add` only after confirmation;
- “pon” or “deja el carrito en”: target quantity; use `cart set`;
- “reemplaza mi carrito”: show current versus proposed state, confirm, then
  clear and set;
- “completa hasta X”: count the current subtotal and propose exact additions
  without filling unused budget arbitrarily;
- “repón lo anterior”: revalidate the latest conversational plan and compare
  its SKUs with current lines before deciding whether to top up or set exactly.

Before writing, show the exact diff, unit semantics, line prices, product
subtotal, and provisional shipping. After writing, compare the returned cart
with that approved diff and run `checkout status --max`. Never blindly retry
`cart add`; first establish whether the previous request succeeded.

Translate failures into action: reauthenticate on session failures, search
again for missing products, use the substitution flow for unavailable products,
rebuild plans that exceed `--max`, retry safe reads once on transient failures,
and stop for approval when cart state differs. Never automatically retry a
mutation.

## Response shape

End a proposal with the primary choice, one useful alternative, constraints
met, and material trade-offs. After a successful write, provide product,
handle/SKU, quantity, unit and line price, subtotal, provisional shipping, and
checkout URL. Redact secrets and state that payment has not occurred.
