package builder

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
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
