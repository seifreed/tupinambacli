package main

import (
	"flag"
	"testing"
)

func TestCommandFlagParseErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"search", func() error { return cmdSearch([]string{"--unknown"}) }},
		{"product", func() error { return cmdProduct([]string{"--unknown"}) }},
		{"collections", func() error { return cmdCollections([]string{"--unknown"}) }},
		{"login", func() error { return cmdLogin([]string{"--unknown"}) }},
		{"set-cookie", func() error { return cmdSetCookie([]string{"--unknown"}) }},
		{"import-har", func() error { return cmdImportHAR([]string{"--unknown"}) }},
		{"batch", func() error { return cmdBatch([]string{"--unknown"}) }},
		{"total", func() error { return cmdTotal([]string{"--unknown"}) }},
		{"cart get", func() error { _, err := cartGet(nil, []string{"--unknown"}); return err }},
		{"cart add", func() error { _, err := cartWrite(nil, "add", []string{"--unknown"}); return err }},
		{"cart clear", func() error { _, err := cartClear(nil, []string{"--unknown"}); return err }},
		{"checkout", func() error { return cmdCheckout([]string{"--unknown"}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); err == nil {
				t.Fatal("expected flag parsing error")
			}
		})
	}
}

func TestFlagOrdering(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Bool("json", false, "")
	if got := reorderArgs(fs, []string{"--json=x"}); len(got) != 2 || got[0] != "--json=x" {
		t.Fatalf("reordered equals args=%v", got)
	}
	got := reorderArgs(fs, []string{"x", "--json", "--", "--literal", "-2"})
	if len(got) != 5 || got[0] != "--json" || got[1] != "--" || got[2] != "x" || got[3] != "--literal" {
		t.Fatalf("reordered args=%v", got)
	}
}
