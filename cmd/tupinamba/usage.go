package main

import (
	"fmt"
	"os"
)

func usage() {
	fmt.Fprint(os.Stderr, `tupinamba - unofficial CLI for capsulastupinamba.com

USAGE:
  tupinamba search <term> [--limit N] [--json|--toon]
  tupinamba product <handle|product_id|variant_id|sku> [--json|--toon]
  tupinamba batch -f <file|-> [--json|--toon]
  tupinamba total -f <file|-> [--max eur] [--json|--toon]
  tupinamba collections|categories [--id handle] [--json|--toon]
  tupinamba login --from-browser chrome
  tupinamba import-har --file <file>
  tupinamba set-cookie --stdin
  tupinamba whoami
  tupinamba cart get|add|set|clear [--variant sku|id] [--max eur] [--json|--toon]
  tupinamba checkout [status] [--max eur] [--json|--toon]
  tupinamba mcp

ENV:
  TUPINAMBA_CONFIG_DIR    override ~/.tupinamba
  TUPINAMBA_MAX_EUR       default spending cap for cart add/set
  tupinamba version
`)
}
