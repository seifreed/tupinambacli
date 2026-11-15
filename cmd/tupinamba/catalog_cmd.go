package main

import (
	"fmt"
	"strings"

	"github.com/seifreed/tupinambacli/internal/client"
	"github.com/seifreed/tupinambacli/internal/money"
)

func cmdSearch(args []string) error {
	fs := newFlagSet("search")
	limit := fs.Int("limit", 10, "max results")
	output := addOutputFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	products, err := newClient().Search(strings.Join(fs.Args(), " "), *limit)
	if err != nil {
		return err
	}
	if output.enabled() {
		return output.emit(products)
	}
	for _, p := range products {
		printProductLine(p)
	}
	return nil
}

func cmdProduct(args []string) error {
	fs := newFlagSet("product")
	output := addOutputFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: tupinamba product <handle|product_id|variant_id|sku>")
	}
	p, err := newClient().Product(fs.Arg(0))
	if err != nil {
		return err
	}
	if output.enabled() {
		return output.emit(p)
	}
	printProductLine(*p)
	if len(p.Variants) > 1 {
		for _, v := range p.Variants {
			status := ""
			if !v.Available {
				status = " (agotado)"
			}
			fmt.Printf("  variant [%s/%d] %s - %s%s\n", displayText(v.SKU), v.ID, displayText(v.Title), formatVariantPrice(v), status)
		}
	}
	desc := client.PlainText(p.Description)
	if desc == "" {
		desc = client.PlainText(p.BodyHTML)
	}
	if desc != "" {
		fmt.Println("  " + displayText(truncate(desc, 500)))
	}
	return nil
}

func cmdCollections(args []string) error {
	fs := newFlagSet("collections")
	id := fs.String("id", "", "collection handle")
	limit := fs.Int("limit", 20, "max products with --id")
	output := addOutputFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if err := requireNoArgs(fs, "tupinamba collections|categories [--id handle]"); err != nil {
		return err
	}
	cl := newClient()
	if *id != "" {
		products, err := cl.CollectionProducts(*id, *limit)
		if err != nil {
			return err
		}
		if output.enabled() {
			return output.emit(products)
		}
		for _, p := range products {
			printProductLine(p)
		}
		return nil
	}
	collections, err := cl.Collections()
	if err != nil {
		return err
	}
	if output.enabled() {
		return output.emit(collections)
	}
	for _, c := range collections {
		fmt.Printf("[%s] %s (%d)\n", displayText(c.Handle), displayText(c.Title), c.ProductsCount)
	}
	return nil
}

func printProductLine(p client.Product) {
	if len(p.Variants) == 0 {
		fmt.Printf("[%d] %s\n", p.ID, displayText(p.Title))
		return
	}
	v, ok := client.BestVariant(p)
	if !ok {
		v = p.Variants[0]
	}
	fmt.Printf("[%s/%d] %s - %s", displayText(v.SKU), v.ID, displayText(p.Title), formatVariantPrice(v))
	if !v.Available {
		fmt.Print(" (agotado)")
	}
	fmt.Println()
}

func formatVariantPrice(v client.Variant) string {
	if cents, ok := client.PriceCents(v); ok {
		return money.FormatEURCents(cents)
	}
	return "precio no disponible"
}
