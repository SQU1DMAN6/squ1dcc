package object

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSQXNamespaceExecutableModule(t *testing.T) {
	root := t.TempDir()
	modulePath := filepath.Join(root, "tooling.sqx")

	module := `#!/usr/bin/env bash
set -euo pipefail
cmd="${1:-}"
case "$cmd" in
  __sqx_manifest__)
    printf '{"version":1,"functions":{"ping":{"return":"string"},"sumTwo":{"return":"int"},"stats":{"return":"json"}}}'
    ;;
  __sqx_call__)
    fn="${2:-}"
    shift 2 || true
    case "$fn" in
      ping) printf "pong" ;;
      sumTwo) printf "%s" "$(($1 + $2))" ;;
      stats) printf '{"count": %s}' "$#" ;;
      *) echo "unknown fn: $fn" >&2; exit 2 ;;
    esac
    ;;
  *)
    echo "unknown cmd: $cmd" >&2
    exit 2
    ;;
esac
`
	if err := os.WriteFile(modulePath, []byte(module), 0o755); err != nil {
		t.Fatalf("could not write module: %v", err)
	}

	ns, err := LoadSQXNamespace(modulePath)
	if err != nil {
		t.Fatalf("LoadSQXNamespace returned error: %v", err)
	}

	ping := mustSQXModuleBuiltin(t, ns, "ping")
	pingObj := ping.Fn()
	gotPing, ok := pingObj.(*String)
	if !ok || gotPing.Value != "pong" {
		t.Fatalf("unexpected ping result: %T (%v)", pingObj, pingObj)
	}

	sum := mustSQXModuleBuiltin(t, ns, "sumTwo")
	sumObj := sum.Fn(&Integer{Value: 2}, &Integer{Value: 9})
	gotSum, ok := sumObj.(*Integer)
	if !ok || gotSum.Value != 11 {
		t.Fatalf("unexpected sum result: %T (%v)", sumObj, sumObj)
	}

	stats := mustSQXModuleBuiltin(t, ns, "stats")
	statsObj := stats.Fn(&Integer{Value: 1}, &Integer{Value: 2}, &Integer{Value: 3})
	statsHash, ok := statsObj.(*Hash)
	if !ok {
		t.Fatalf("expected hash result for stats, got %T (%v)", statsObj, statsObj)
	}

	countObj, ok := sqxModuleHashLookup(statsHash, "count")
	if !ok {
		t.Fatalf("expected stats.count in hash, got %v", statsHash)
	}
	count, ok := countObj.(*Integer)
	if !ok || count.Value != 3 {
		t.Fatalf("expected stats.count=3, got %T (%v)", countObj, countObj)
	}
}

func mustSQXModuleBuiltin(t *testing.T, ns *Hash, name string) *Builtin {
	t.Helper()
	obj, ok := sqxModuleHashLookup(ns, name)
	if !ok {
		t.Fatalf("namespace missing function %q", name)
	}
	b, ok := obj.(*Builtin)
	if !ok {
		t.Fatalf("namespace value %q is not builtin, got %T", name, obj)
	}
	return b
}

func sqxModuleHashLookup(h *Hash, key string) (Object, bool) {
	hashKey := (&String{Value: key}).HashKey()
	pair, ok := h.Pairs[hashKey]
	if !ok {
		return nil, false
	}
	return pair.Value, true
}
