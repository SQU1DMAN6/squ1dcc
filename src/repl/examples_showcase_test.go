package repl

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func skipIfMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Skipf("example artifact missing: %s (%v)", path, err)
	}
}

func TestPluginsShellShowcase(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", "examples", "plugins_shell"))
	modulePath := filepath.Join(root, "lib", "tooling.sqx")
	skipIfMissing(t, modulePath)
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
	skipIfMissing(t, modulePath)
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

func TestHelloWorldSQXShowcase(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", "examples", "hello_world_sqx"))
	skipIfMissing(t, filepath.Join(root, "showcase.sqd"))
	buildDir := filepath.Join(root, "lib")
	buildCmd := exec.Command("bash", "build.sh")
	buildCmd.Dir = buildDir
	buildCmd.Env = append(os.Environ(), "GOCACHE=/tmp/go-build")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build.sh failed: %v\noutput: %s", err, string(output))
	}

	modulePath := filepath.Join(buildDir, "hello_world.sqx")
	if err := os.Chmod(modulePath, 0o755); err != nil {
		t.Fatalf("could not chmod SQX module: %v", err)
	}

	mainPath := filepath.Join(root, "showcase.sqd")
	var out strings.Builder
	if err := ExecuteFile(mainPath, &out); err != nil {
		t.Fatalf("ExecuteFile failed: %v\noutput: %q", err, out.String())
	}

	got := out.String()
	for _, want := range []string{
		"execute:  Hello, World! This message is from SQX.",
		"bool:  true",
		"add:  8",
		"sumArray:  10",
		"countKeys:  2",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q, got: %q", want, got)
		}
	}
}
