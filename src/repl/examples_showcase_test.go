package repl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPluginsShellShowcase(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", "examples", "plugins_shell"))
	modulePath := filepath.Join(root, "lib", "tooling.sqx")
	if err := os.Chmod(modulePath, 0o755); err != nil {
		t.Fatalf("could not chmod shell module: %v", err)
	}

	mainPath := filepath.Join(root, "main.sqd")
	var out strings.Builder
	if err := ExecuteFile(mainPath, &out); err != nil {
		t.Fatalf("ExecuteFile failed: %v\noutput: %q", err, out.String())
	}

	got := out.String()
	for _, want := range []string{
		"ping:  shell pong",
		"sum:  12",
		"stats count:  3",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q, got: %q", want, got)
		}
	}
}

func TestPluginsSQLShowcase(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", "examples", "plugins_test"))
	modulePath := filepath.Join(root, "lib", "sql.sqx")
	if err := os.Chmod(modulePath, 0o755); err != nil {
		t.Fatalf("could not chmod sql module: %v", err)
	}

	mainPath := filepath.Join(root, "main.sqd")
	var out strings.Builder
	if err := ExecuteFile(mainPath, &out); err != nil {
		t.Fatalf("ExecuteFile failed: %v\noutput: %q", err, out.String())
	}

	got := out.String()
	for _, want := range []string{
		"ping:  sql module pong",
		"row count:  3",
		"first user:  Ada",
		"select literal:  1",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q, got: %q", want, got)
		}
	}
}
