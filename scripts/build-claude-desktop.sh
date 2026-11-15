#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd -P)"
target_os="${TUPINAMBA_GOOS:-$(go env GOOS)}"
target_arch="${TUPINAMBA_GOARCH:-$(go env GOARCH)}"
suffix=""
if [ "$target_os" = "windows" ]; then
	suffix=".exe"
fi

work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT
mkdir -p "$work_dir/server" "$ROOT/dist"
cp "$ROOT/packaging/claude-desktop/manifest.json" "$work_dir/manifest.json"
if [ "$target_os" = "windows" ]; then
	node - "$work_dir/manifest.json" <<'NODE'
const fs = require("fs");
const path = process.argv[2];
const manifest = JSON.parse(fs.readFileSync(path, "utf8"));
manifest.server.entry_point += ".exe";
manifest.server.mcp_config.command += ".exe";
fs.writeFileSync(path, JSON.stringify(manifest, null, 2) + "\n");
NODE
fi
cp "$ROOT/LICENSE" "$work_dir/LICENSE"
GOOS="$target_os" GOARCH="$target_arch" go build -o "$work_dir/server/tupinamba-mcp$suffix" ./cmd/tupinamba-mcp
npx --yes @anthropic-ai/mcpb@2.1.2 pack "$work_dir" "$ROOT/dist/tupinamba-shop-$target_os-$target_arch.mcpb"
