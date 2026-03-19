package repl

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

func TestSuppressLetDoesNotPrintDefinitionError(t *testing.T) {
	content := "suppress var x = def() { return NonExistentVariable }\nvar z = << x()\ntype.tp(z)\nio.echo(z)\n"
	f, err := ioutil.TempFile("", "suppress-*.sqd")
	if err != nil {
		t.Fatalf("couldn't create temp file: %v", err)
	}
	defer os.Remove(f.Name())

	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("couldn't write temp file: %v", err)
	}
	f.Close()

	var out strings.Builder
	_ = ExecuteFile(f.Name(), &out)

	o := out.String()
	// Expected: no immediate error printed at definition time. The error should
	// only appear once when io.echo(z) executes.
	if strings.Count(o, "Undefined variable") != 1 {
		t.Fatalf("expected exactly 1 Undefined variable print, got: %q", o)
	}

	if !strings.Contains(o, "Error") {
		t.Fatalf("expected type.tp(z) to print 'Error', got: %q", o)
	}
}

func TestBlockDirectiveExitsOnError(t *testing.T) {
	content := "block var x = def() { return NonExistentVariable }\n"
	f, err := ioutil.TempFile("", "block-*.sqd")
	if err != nil {
		t.Fatalf("couldn't create temp file: %v", err)
	}
	defer os.Remove(f.Name())

	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("couldn't write temp file: %v", err)
	}
	f.Close()

	var out strings.Builder
	err = ExecuteFile(f.Name(), &out)
	if err == nil {
		t.Fatalf("expected ExecuteFile to return an error (exit) for block directive")
	}
	if !strings.Contains(out.String(), "Undefined variable") {
		t.Fatalf("expected output to include undefined variable message, got: %q", out.String())
	}
}
