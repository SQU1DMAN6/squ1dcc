#!/usr/bin/env bash
set -euo pipefail

OUT="${1:-sql.sqx}"
CACHE_ROOT="${TMPDIR:-/tmp}/squ1d_sqx_go_cache"
mkdir -p "$CACHE_ROOT"
GOCACHE_DIR="${GOCACHE:-$CACHE_ROOT}"
GOCACHE="$GOCACHE_DIR" go build -trimpath -ldflags="-s -w" -o "$OUT" .
chmod +x "$OUT"
echo "Built $OUT"
