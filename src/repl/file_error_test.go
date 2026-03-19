package repl

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

func TestExecuteFilePrintsHelpfulUndefinedVariableError(t *testing.T) {
	content := "var x = def() { return y }\nx()\n"
	f, err := ioutil.TempFile("", "test2-*.sqd")
	if err != nil {
		t.Fatalf("couldn't create temp file: %v", err)
	}
	defer os.Remove(f.Name())

	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("couldn't write temp file: %v", err)
	}
	f.Close()

	var out strings.Builder
	if err := ExecuteFile(f.Name(), &out); err != nil {
		// ExecuteFile may return an error; we still want to inspect output
	}

	o := out.String()
	if !strings.Contains(o, "ERROR:") {
		t.Fatalf("expected output to contain ERROR:, got: %q", o)
	}
	if !strings.Contains(o, "Undefined variable y") {
		t.Fatalf("expected output to mention 'Undefined variable y', got: %q", o)
	}
	if !strings.Contains(o, f.Name()) {
		t.Fatalf("expected output to contain filename %s, got: %q", f.Name(), o)
	}

	// Also verify it printed a line/column mention
	if !strings.Contains(o, "line") || !strings.Contains(o, "column") {
		t.Fatalf("expected output to contain line and column info, got: %q", o)
	}
}
