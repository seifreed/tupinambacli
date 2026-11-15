package client

import (
	"encoding/json"
	"html"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/seifreed/tupinambacli/internal/money"
)

var htmlTags = regexp.MustCompile(`<[^>]+>`)

var accentFolder = strings.NewReplacer(
	"á", "a", "à", "a", "ä", "a", "â", "a",
	"é", "e", "è", "e", "ë", "e", "ê", "e",
	"í", "i", "ì", "i", "ï", "i", "î", "i",
	"ó", "o", "ò", "o", "ö", "o", "ô", "o",
	"ú", "u", "ù", "u", "ü", "u", "û", "u",
	"ç", "c", "ñ", "n",
)

func PriceCents(v Variant) (int, bool) {
	var n *int
	if json.Unmarshal(v.Price, &n) == nil {
		if n == nil {
			return 0, false
		}
		return *n, *n >= 0
	}
	var s string
	if json.Unmarshal(v.Price, &s) != nil {
		return 0, false
	}
	if s == "" || strings.TrimSpace(s) != s {
		return 0, false
	}
	cents, err := money.ParseEURCents(s)
	return cents, err == nil
}

func BestVariant(p Product) (Variant, bool) {
	var best Variant
	bestPrice := 0
	for _, v := range p.Variants {
		if !v.Available {
			continue
		}
		price, ok := PriceCents(v)
		if !ok {
			continue
		}
		if best.ID == 0 || price < bestPrice {
			best = v
			bestPrice = price
		}
	}
	return best, best.ID != 0
}

func FindVariant(p Product, ref string) (Variant, bool) {
	if ref == "" {
		return Variant{}, false
	}
	for _, v := range p.Variants {
		if strconv.FormatInt(v.ID, 10) == ref || strings.EqualFold(v.SKU, ref) {
			return v, true
		}
	}
	return Variant{}, false
}

func SortProducts(products []Product) {
	slices.SortStableFunc(products, func(a, b Product) int {
		av, aok := BestVariant(a)
		bv, bok := BestVariant(b)
		if aok != bok {
			if aok {
				return -1
			}
			return 1
		}
		if aok {
			ap, _ := PriceCents(av)
			bp, _ := PriceCents(bv)
			if ap != bp {
				return ap - bp
			}
		}
		return strings.Compare(fold(a.Title), fold(b.Title))
	})
}

func PlainText(s string) string {
	return strings.Join(strings.Fields(html.UnescapeString(htmlTags.ReplaceAllString(s, " "))), " ")
}

func searchText(p Product) string {
	return p.Title + " " + p.BodyHTML + " " + p.Description + " " + p.Type + " " + strings.Join(p.Tags, " ")
}

func fold(s string) string {
	return accentFolder.Replace(strings.ToLower(s))
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
