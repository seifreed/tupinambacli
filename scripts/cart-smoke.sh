#!/usr/bin/env bash
set -euo pipefail

BIN="${BIN:-./tupinamba}"
REF="${1:-E102800}"
MAX_EUR="${MAX_EUR:-5}"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/tupinamba-cart-smoke.XXXXXX")"
export TUPINAMBA_CONFIG_DIR="$TMP_DIR/config"

cleanup() {
	status=$?
	trap - EXIT
	if ! "$BIN" cart clear --json >"$TMP_DIR/cleanup.json"; then
		if [ "$status" -eq 0 ]; then
			status=1
		fi
	fi
	rm -rf "$TMP_DIR"
	exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

"$BIN" whoami >/dev/null
"$BIN" cart set "$REF" 1 --max "$MAX_EUR" --json >"$TMP_DIR/add.json"
"$BIN" cart get --json >"$TMP_DIR/cart.json"
grep -q '"item_count": 1' "$TMP_DIR/cart.json"
"$BIN" cart set "$REF" 0 --max "$MAX_EUR" --json >"$TMP_DIR/remove.json"
"$BIN" cart get --json >"$TMP_DIR/empty.json"
grep -q '"item_count": 0' "$TMP_DIR/empty.json"

echo "ok cart set/add/remove smoke for $REF"
