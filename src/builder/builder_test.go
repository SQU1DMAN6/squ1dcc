package builder

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"squ1d++/object"
	"strings"
	"testing"

	"squ1d++/vm"
)

func TestBuildStandaloneWithEmbeddedBinary(t *testing.T) {
	// Build the compiler itself to serve as source binary.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	sourceBin := filepath.Join(cwd, "squ1dpp")
	if err := exec.Command("go", "build", "-o", sourceBin, "../main.go").Run(); err != nil {
		// we expect to run this test from the repository root under builder/, so adjust path
		if err := exec.Command("go", "build", "-o", sourceBin, "../main.go").Run(); err != nil {
			t.Skipf("could not build squ1dpp from builder test environment: %v", err)
		}
	}
	defer os.Remove(sourceBin)

	os.Setenv("SQU1D_SOURCE_BINARY", sourceBin)
	defer os.Unsetenv("SQU1D_SOURCE_BINARY")

	inputFile := filepath.Join(os.TempDir(), "embedded_test_input.sqd")
	if err := os.WriteFile(inputFile, []byte("1 + 2"), 0644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(inputFile)

	outputFile := filepath.Join(os.TempDir(), "embedded_test_out")
	defer os.Remove(outputFile)

	if err := BuildStandalone(inputFile, outputFile); err != nil {
		t.Fatalf("BuildStandalone returned error: %v", err)
	}

	out, err := exec.Command(outputFile).CombinedOutput()
	if err != nil {
		t.Fatalf("running output failed: %v output=%q", err, out)
	}

	if got := string(bytes.TrimSpace(out)); got != "3" {
		t.Fatalf("unexpected output: got %q, want %q", got, "3")
	}
}

func TestExpandIncludesExportsTopLevelFunctionDefinitions(t *testing.T) {
	root := t.TempDir()
	libDir := filepath.Join(root, "lib")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatalf("could not create lib directory: %v", err)
	}

	// Shorthand function definition (`name >> (...)`) is lowered by parser to a
	// let-style statement; export detection must include it.
	libSource := "sort >> (x) { return x }\n"
	if err := os.WriteFile(filepath.Join(libDir, "sort.sqd"), []byte(libSource), 0o644); err != nil {
		t.Fatalf("could not write library file: %v", err)
	}

	mainSource := "pkg.include(\"lib/sort.sqd\", \"sort\")\nsort.sort(42)\n"

	expanded, err := expandIncludes(mainSource, root)
	if err != nil {
		t.Fatalf("expandIncludes returned error: %v", err)
	}

	bc, err := compileSourceWithNamespaces(expanded)
	if err != nil {
		t.Fatalf("compileSourceWithNamespaces returned error: %v\nexpanded source:\n%s", err, expanded)
	}

	machine := vm.New(bc)
	if err := machine.Run(); err != nil {
		t.Fatalf("VM runtime error: %v\nexpanded source:\n%s", err, expanded)
	}

	got, ok := machine.LastPoppedStackElem().(*object.Integer)
	if !ok {
		t.Fatalf("expected integer result, got %T (%v)", machine.LastPoppedStackElem(), machine.LastPoppedStackElem())
	}
	if got.Value != 42 {
		t.Fatalf("expected 42, got %d", got.Value)
	}
}

func TestExpandIncludesRewritesSQXNamespaceInclude(t *testing.T) {
	root := t.TempDir()
	libDir := filepath.Join(root, "lib")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatalf("could not create lib directory: %v", err)
	}

	script := `#!/usr/bin/env bash
mode="$1"
shift
case "$mode" in
  sum) printf "%s" "$(($1 + $2))" ;;
  *) echo "unknown mode: $mode" >&2; exit 2 ;;
esac
`
	if err := os.WriteFile(filepath.Join(libDir, "tooling_plugin.sh"), []byte(script), 0o755); err != nil {
		t.Fatalf("could not write plugin script: %v", err)
	}

	manifest := `{
  "version": 1,
  "functions": {
    "sumTwo": { "exec": ["./tooling_plugin.sh", "sum"], "append_args": true, "return": "int" }
  }
}`
	if err := os.WriteFile(filepath.Join(libDir, "tooling.sqx"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("could not write sqx manifest: %v", err)
	}

	mainSource := "pkg.include(\"lib/tooling.sqx\", \"tooling\")\ntooling.sumTwo(3, 4)\n"
	expanded, err := expandIncludes(mainSource, root)
	if err != nil {
		t.Fatalf("expandIncludes returned error: %v", err)
	}

	if !strings.Contains(expanded, "pkg.load_sqx(") {
		t.Fatalf("expected expanded source to contain pkg.load_sqx rewrite, got:\n%s", expanded)
	}

	re := regexp.MustCompile(`pkg\.load_sqx\("([^"]+)"\)`)
	match := re.FindStringSubmatch(expanded)
	if len(match) != 2 {
		t.Fatalf("could not parse load_sqx path from expanded source:\n%s", expanded)
	}
	if !filepath.IsAbs(match[1]) {
		t.Fatalf("expected load_sqx path to be absolute, got %q", match[1])
	}

	bc, err := compileSourceWithNamespaces(expanded)
	if err != nil {
		t.Fatalf("compileSourceWithNamespaces returned error: %v\nexpanded source:\n%s", err, expanded)
	}

	machine := vm.New(bc)
	if err := machine.Run(); err != nil {
		t.Fatalf("VM runtime error: %v\nexpanded source:\n%s", err, expanded)
	}

	got, ok := machine.LastPoppedStackElem().(*object.Integer)
	if !ok {
		t.Fatalf("expected integer result, got %T (%v)", machine.LastPoppedStackElem(), machine.LastPoppedStackElem())
	}
	if got.Value != 7 {
		t.Fatalf("expected 7, got %d", got.Value)
	}
}

func TestExpandIncludesSQXExecutableModuleRunsInVM(t *testing.T) {
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
    printf '{"version":1,"functions":{"sumTwo":{"return":"int"}}}'
    ;;
  __sqx_call__)
    fn="${2:-}"
    shift 2 || true
    case "$fn" in
      sumTwo) printf "%s" "$(($1 + $2))" ;;
      *) echo "unknown fn: $fn" >&2; exit 2 ;;
    esac
    ;;
  *)
    echo "unknown command: $command" >&2
    exit 2
    ;;
esac
`
	if err := os.WriteFile(filepath.Join(libDir, "tooling.sqx"), []byte(module), 0o755); err != nil {
		t.Fatalf("could not write sqx module: %v", err)
	}

	mainSource := "pkg.include(\"lib/tooling.sqx\", \"tooling\")\n" +
		"tooling.sumTwo(9, 4)\n"

	expanded, err := expandIncludes(mainSource, root)
	if err != nil {
		t.Fatalf("expandIncludes returned error: %v", err)
	}

	if !strings.Contains(expanded, "pkg.load_sqx(") {
		t.Fatalf("expected expanded source to contain pkg.load_sqx rewrite, got:\n%s", expanded)
	}

	bc, err := compileSourceWithNamespaces(expanded)
	if err != nil {
		t.Fatalf("compileSourceWithNamespaces returned error: %v\nexpanded source:\n%s", err, expanded)
	}

	machine := vm.New(bc)
	if err := machine.Run(); err != nil {
		t.Fatalf("VM runtime error: %v\nexpanded source:\n%s", err, expanded)
	}

	got, ok := machine.LastPoppedStackElem().(*object.Integer)
	if !ok {
		t.Fatalf("expected integer result, got %T (%v)", machine.LastPoppedStackElem(), machine.LastPoppedStackElem())
	}
	if got.Value != 13 {
		t.Fatalf("expected 13, got %d", got.Value)
	}
}
