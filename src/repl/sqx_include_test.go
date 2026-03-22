package repl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecuteFileIncludesSQXPluginNamespace(t *testing.T) {
	root := t.TempDir()
	libDir := filepath.Join(root, "lib")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatalf("could not create lib directory: %v", err)
	}

	script := `#!/usr/bin/env bash
mode="$1"
shift
case "$mode" in
  imported) printf "123" ;;
  greet) printf "Hello %s" "$1" ;;
  sum) printf "%s" "$(($1 + $2))" ;;
  stats) printf '{"count": %s}' "$#" ;;
  *) echo "unknown mode: $mode" >&2; exit 2 ;;
esac
`
	scriptPath := filepath.Join(libDir, "tooling_plugin.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("could not write plugin script: %v", err)
	}

	manifest := `{
  "version": 1,
  "functions": {
    "importedFunction": { "exec": ["./tooling_plugin.sh", "imported"], "return": "int" },
    "sendGreeting": { "exec": ["./tooling_plugin.sh", "greet"], "append_args": true, "return": "string" },
    "sumTwo": { "exec": ["./tooling_plugin.sh", "sum"], "append_args": true, "return": "int" },
    "stats": { "exec": ["./tooling_plugin.sh", "stats"], "append_args": true, "return": "json" }
  }
}`
	if err := os.WriteFile(filepath.Join(libDir, "tooling.sqx"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("could not write sqx manifest: %v", err)
	}

	main := `pkg.include("lib/tooling.sqx", "tooling")
io.echo(tooling.importedFunction(), "\n")
io.echo(tooling.sendGreeting("Ana"), "\n")
io.echo(tooling.sumTwo(4, 6), "\n")
io.echo(tooling.stats(1, 2, 3).count, "\n")
`
	mainPath := filepath.Join(root, "main.sqd")
	if err := os.WriteFile(mainPath, []byte(main), 0o644); err != nil {
		t.Fatalf("could not write main file: %v", err)
	}

	var out strings.Builder
	if err := ExecuteFile(mainPath, &out); err != nil {
		t.Fatalf("ExecuteFile returned error: %v\noutput: %q", err, out.String())
	}

	got := out.String()
	for _, want := range []string{"123", "Hello Ana", "10", "3"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q, got: %q", want, got)
		}
	}
}

func TestExecuteFileIncludesSQXExecutableModule(t *testing.T) {
	root := t.TempDir()
	libDir := filepath.Join(root, "lib")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatalf("could not create lib directory: %v", err)
	}

	module := `#!/usr/bin/env bash
set -euo pipefail
command="${1:-}"
case "$command" in
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
    echo "unknown command: $command" >&2
    exit 2
    ;;
esac
`
	modulePath := filepath.Join(libDir, "tooling.sqx")
	if err := os.WriteFile(modulePath, []byte(module), 0o755); err != nil {
		t.Fatalf("could not write sqx module: %v", err)
	}

	main := `pkg.include("lib/tooling.sqx", "tooling")
io.echo(tooling.ping(), "\n")
io.echo(tooling.sumTwo(1, 4), "\n")
io.echo(tooling.stats(7, 8).count, "\n")
`
	mainPath := filepath.Join(root, "main.sqd")
	if err := os.WriteFile(mainPath, []byte(main), 0o644); err != nil {
		t.Fatalf("could not write main file: %v", err)
	}

	var out strings.Builder
	if err := ExecuteFile(mainPath, &out); err != nil {
		t.Fatalf("ExecuteFile returned error: %v\noutput: %q", err, out.String())
	}

	got := out.String()
	for _, want := range []string{"pong", "5", "2"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q, got: %q", want, got)
		}
	}
}
