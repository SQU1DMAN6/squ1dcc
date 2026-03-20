package builder

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"squ1d++/object"
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
