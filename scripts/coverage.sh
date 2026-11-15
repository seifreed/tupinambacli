#!/usr/bin/env bash
set -euo pipefail

coverage_file="$(mktemp)"
trap 'rm -f "$coverage_file"' EXIT

go test -coverprofile="$coverage_file" ./...
report="$(go tool cover -func="$coverage_file")"
printf '%s\n' "$report"

total="$(printf '%s\n' "$report" | awk '$1 == "total:" {print $3}')"
if [[ "$total" != "100.0%" ]]; then
	echo "project coverage is $total; expected 100.0%" >&2
	exit 1
fi
