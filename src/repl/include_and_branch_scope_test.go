package repl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecuteFileProcessesAllPkgIncludesInIfBlock(t *testing.T) {
	root := t.TempDir()
	libDir := filepath.Join(root, "lib")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatalf("could not create lib dir: %v", err)
	}

	sortLib := "sort >> (arr) { return arr }\n"
	if err := os.WriteFile(filepath.Join(libDir, "sort.sqd"), []byte(sortLib), 0o644); err != nil {
		t.Fatalf("could not write sort library: %v", err)
	}

	readerLib := `get >> (mode) {
    if (mode == 1 and mode != 0) {
        return [3, 1, 2]
    }
    return []
}
`
	if err := os.WriteFile(filepath.Join(libDir, "reader.sqd"), []byte(readerLib), 0o644); err != nil {
		t.Fatalf("could not write reader library: %v", err)
	}

	main := `if (os.iRuntime("os") == "windows") {
    pkg.include("lib\\sort.sqd", "sort")
    pkg.include("lib\\reader.sqd", "reader")
} el {
    pkg.include("lib/sort.sqd", "sort")
    pkg.include("lib/reader.sqd", "reader")
}
var sorted = sort.sort(reader.get(1))
io.echo(sorted, "\n")
`

	mainPath := filepath.Join(root, "main.sqd")
	if err := os.WriteFile(mainPath, []byte(main), 0o644); err != nil {
		t.Fatalf("could not write main file: %v", err)
	}

	var out strings.Builder
	if err := ExecuteFile(mainPath, &out); err != nil {
		t.Fatalf("ExecuteFile returned error: %v\noutput: %q", err, out.String())
	}

	if !strings.Contains(out.String(), "[3, 1, 2]") {
		t.Fatalf("expected output to contain sorted array, got: %q", out.String())
	}
}

func TestExecuteFileRedeclareVarAcrossIfBranchesUsesSameGlobal(t *testing.T) {
	root := t.TempDir()
	main := `var mode = 1
if (mode == 1) {
    var x = [1, 2, 3]
} elif (mode == 2) {
    var x = [9]
}
io.echo(array.cat(x), "\n")
`

	mainPath := filepath.Join(root, "main.sqd")
	if err := os.WriteFile(mainPath, []byte(main), 0o644); err != nil {
		t.Fatalf("could not write main file: %v", err)
	}

	var out strings.Builder
	if err := ExecuteFile(mainPath, &out); err != nil {
		t.Fatalf("ExecuteFile returned error: %v\noutput: %q", err, out.String())
	}

	if strings.Contains(out.String(), "ERROR:") {
		t.Fatalf("expected no error output, got: %q", out.String())
	}
	if !strings.Contains(out.String(), "3") {
		t.Fatalf("expected output to include array length 3, got: %q", out.String())
	}
}
