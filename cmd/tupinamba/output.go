package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"unicode"

	toon "github.com/toon-format/toon-go"
)

type outputFlags struct {
	json *bool
	toon *bool
}

type lineTotal struct {
	Ref        string `json:"ref"`
	Title      string `json:"title"`
	Quantity   int    `json:"quantity"`
	LineCents  int    `json:"line_cents"`
	LineAmount string `json:"line_amount"`
}

type totalOut struct {
	Lines      []lineTotal `json:"lines"`
	TotalCents int         `json:"total_cents"`
	Total      string      `json:"total"`
}

var marshalTOON = func(v any) (string, error) { return toon.MarshalString(v) }

func addOutputFlags(fs *flag.FlagSet) outputFlags {
	return outputFlags{
		json: fs.Bool("json", false, "emit JSON"),
		toon: fs.Bool("toon", false, "emit TOON"),
	}
}

func (o outputFlags) enabled() bool { return *o.json || *o.toon }

func (o outputFlags) emit(v any) error {
	if *o.toon {
		return emitTOON(v)
	}
	return emitJSON(v)
}

func emitJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

func emitTOON(v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return err
	}
	s, err := marshalTOON(generic)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(os.Stdout, s)
	return err
}

func addMaxFlag(fs *flag.FlagSet, usage string) *string {
	return fs.String("max", strings.TrimSpace(os.Getenv("TUPINAMBA_MAX_EUR")), usage)
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	if n < 0 {
		n = 0
	}
	return string(runes[:n]) + "..."
}

func displayText(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}
