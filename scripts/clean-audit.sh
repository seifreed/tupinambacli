#!/usr/bin/env bash
set -euo pipefail

max_lines="${MAX_GO_FILE_LINES:-220}"
bad=0

while IFS= read -r file; do
	lines="$(wc -l <"$file" | tr -d ' ')"
	if [ "$lines" -gt "$max_lines" ]; then
		echo "$file has $lines lines; max is $max_lines" >&2
		bad=1
	fi
done < <(find cmd internal -name '*.go' -type f | sort)

[ "$bad" -eq 0 ]
make check
go run honnef.co/go/tools/cmd/staticcheck@latest ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
# Chrome's documented macOS Safe Storage format mandates PBKDF2-HMAC-SHA1; exclude only the weak-hash rule for this compatibility use.
go run github.com/securego/gosec/v2/cmd/gosec@latest -fmt=text -exclude=G505 ./...
echo "clean-audit ok"
