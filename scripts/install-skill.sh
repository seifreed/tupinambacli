#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd -P)"
SRC="$ROOT/.claude/skills/tupinamba-shop"
SKILLS_DIR="${CLAUDE_SKILLS_DIR:-$HOME/.claude/skills}"

if [ ! -d "$SRC" ]; then
	echo "skill source not found: $SRC" >&2
	exit 1
fi

mkdir -p "$SKILLS_DIR"
SKILLS_DIR="$(cd "$SKILLS_DIR" && pwd -P)"
DST="$SKILLS_DIR/tupinamba-shop"
DST_REAL="$DST"
if [ -d "$DST" ]; then
	DST_REAL="$(cd "$DST" && pwd -P)"
fi
if [ "$SRC" = "$DST_REAL" ]; then
	echo "skill source and destination are the same directory" >&2
	exit 1
fi

rm -rf "$DST"
cp -R "$SRC" "$DST"
echo "installed $DST"
