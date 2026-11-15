package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/seifreed/tupinambacli/internal/client"
	"github.com/seifreed/tupinambacli/internal/money"
)

func cmdBatch(args []string) error {
	fs := newFlagSet("batch")
	file := fs.String("f", "-", "input file")
	output := addOutputFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if err := requireNoArgs(fs, "tupinamba batch -f <file|->"); err != nil {
		return err
	}
	lines, err := readLines(*file)
	if err != nil {
		return err
	}
	type row struct {
		Term    string          `json:"term"`
		Product *client.Product `json:"product,omitempty"`
	}
	var rows []row
	cl := newClient()
	var products []client.Product
	if len(lines) > 0 {
		products, err = cl.AllProducts()
		if err != nil {
			return err
		}
	}
	for _, term := range lines {
		hits := client.SearchProducts(products, term, 1)
		r := row{Term: term}
		if len(hits) > 0 {
			r.Product = &hits[0]
		}
		rows = append(rows, r)
	}
	if output.enabled() {
		return output.emit(rows)
	}
	for _, r := range rows {
		if r.Product == nil {
			fmt.Printf("%s -> no encontrado\n", displayText(r.Term))
			continue
		}
		fmt.Printf("%s -> ", displayText(r.Term))
		printProductLine(*r.Product)
	}
	return nil
}

func cmdTotal(args []string) error {
	fs := newFlagSet("total")
	file := fs.String("f", "-", "input file")
	output := addOutputFlags(fs)
	max := addMaxFlag(fs, "refuse total over EUR amount")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if err := requireNoArgs(fs, "tupinamba total -f <file|->"); err != nil {
		return err
	}
	maxCents, err := money.ParseEURCents(*max)
	if err != nil {
		return err
	}
	maxSet := strings.TrimSpace(*max) != ""
	lines, err := readLines(*file)
	if err != nil {
		return err
	}
	out := totalOut{}
	cl := newClient()
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) > 2 {
			return fmt.Errorf("invalid total line %q", line)
		}
		qty := 1
		if len(fields) == 2 {
			qty, err = strconv.Atoi(fields[1])
			if err != nil || qty <= 0 {
				return fmt.Errorf("quantity must be positive")
			}
		}
		p, err := cl.Product(fields[0])
		if err != nil {
			return err
		}
		v, exact := client.FindVariant(*p, fields[0])
		if exact {
			if !v.Available {
				return fmt.Errorf("variant %q is unavailable", fields[0])
			}
			if _, ok := client.PriceCents(v); !ok {
				return fmt.Errorf("variant %q has an invalid price", fields[0])
			}
		} else {
			var ok bool
			v, ok = client.BestVariant(*p)
			if !ok {
				return fmt.Errorf("%q is unavailable", p.Title)
			}
		}
		price, _ := client.PriceCents(v)
		cents, err := money.MultiplyCents(price, qty)
		if err != nil {
			return err
		}
		if out.TotalCents, err = money.AddCents(out.TotalCents, cents); err != nil {
			return err
		}
		out.Lines = append(out.Lines, lineTotal{
			Ref: fields[0], Title: p.Title, Quantity: qty,
			LineCents: cents, LineAmount: money.FormatEURCents(cents),
		})
	}
	out.Total = money.FormatEURCents(out.TotalCents)
	if maxSet && out.TotalCents > maxCents {
		return fmt.Errorf("total %s exceeds --max %s", out.Total, money.FormatEURCents(maxCents))
	}
	if output.enabled() {
		return output.emit(out)
	}
	for _, line := range out.Lines {
		fmt.Printf("[%s] %s x%d = %s\n", displayText(line.Ref), displayText(line.Title), line.Quantity, line.LineAmount)
	}
	fmt.Printf("total: %s\n", out.Total)
	return nil
}
