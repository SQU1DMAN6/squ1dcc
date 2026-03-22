#!/usr/bin/env bash
set -euo pipefail

OUT="${1:-hello_world.sqx}"
go build -trimpath -ldflags="-s -w" -o "$OUT" .
chmod +x "$OUT"
echo "Built $OUT"
