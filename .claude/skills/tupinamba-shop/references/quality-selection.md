# Quality-Based Selection

Use this guide when the user asks for the best, most premium, or highest-quality
coffee capsules.

## Evidence-first comparison

Start from the live capsule collection, then inspect each finalist by handle:

```bash
tupinamba categories --id capsulas-compostables --limit 50 --json
tupinamba product <handle> --json
```

Do not use `search premium` as a ranking; search is textual and results are
ordered by availability and price. Exclude other product types unless requested
and distinguish individual boxes from packs.

Base the recommendation on evidence actually present in each product page:

1. composition and named origin;
2. concrete tasting notes, body, crema, acidity, and balance;
3. roast and preparation details;
4. explicit quality positioning such as “premium” or “gourmet”.

Composition is not an absolute score: 100% Arabica may suit elegance and
aromatic detail, while an Arabica/Robusta blend may better suit body and crema.
Marketing adjectives are supporting evidence, not proof. Price, pack size,
discount, compatibility, availability, and sustainability are not quality
scores.

Collection copy and product pages can disagree. Prefer the revalidated product
page and state the discrepancy. If evidence does not separate candidates, call
it a tie instead of inventing precision.

Present the top candidate and one useful alternative with evidence, confidence,
format, capsule count, SKU, live price, and availability. If quantity or pack
intent is unclear, resolve it before preparing a cart plan.

