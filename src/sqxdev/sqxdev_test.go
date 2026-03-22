package sqxdev

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitTemplateGoAndBuild(t *testing.T) {
	root := t.TempDir()
	templateDir := filepath.Join(root, "demo")
	if err := InitTemplate("go", "demo", templateDir); err != nil {
		t.Fatalf("InitTemplate failed: %v", err)
	}

	for _, path := range []string{
		filepath.Join(templateDir, "main.go"),
		filepath.Join(templateDir, "sqx_runtime.go"),
		filepath.Join(templateDir, "go.mod"),
		filepath.Join(templateDir, "build.sh"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected generated file %s: %v", path, err)
		}
	}

	outPath := filepath.Join(root, "demo.sqx")
	if err := Compile("go", templateDir, outPath); err != nil {
		t.Fatalf("Compile(go) failed: %v", err)
	}

	output, err := exec.Command(outPath, "__sqx_manifest__").Output()
	if err != nil {
		t.Fatalf("running built SQX manifest failed: %v", err)
	}

	var manifest struct {
		Functions map[string]struct {
			Return string `json:"return"`
		} `json:"functions"`
	}
	if err := json.Unmarshal(output, &manifest); err != nil {
		t.Fatalf("manifest output was not valid JSON: %v", err)
	}
	if manifest.Functions["ping"].Return != "string" {
		t.Fatalf("expected ping return mode string, got %+v", manifest.Functions["ping"])
	}
}

func TestInitTemplateShellAndBuild(t *testing.T) {
	root := t.TempDir()
	templateDir := filepath.Join(root, "shell")
	if err := InitTemplate("shell", "demo", templateDir); err != nil {
		t.Fatalf("InitTemplate(shell) failed: %v", err)
	}

	srcPath := filepath.Join(templateDir, "demo.sqx")
	if _, err := os.Stat(srcPath); err != nil {
		t.Fatalf("expected generated shell module: %v", err)
	}

	outPath := filepath.Join(root, "demo-built.sqx")
	if err := Compile("shell", srcPath, outPath); err != nil {
		t.Fatalf("Compile(shell) failed: %v", err)
	}

	if fi, err := os.Stat(outPath); err != nil {
		t.Fatalf("expected shell output path: %v", err)
	} else if fi.Mode()&0o111 == 0 {
		t.Fatalf("expected shell output to be executable")
	}
}

func TestInitTemplateCAndCPP(t *testing.T) {
	root := t.TempDir()

	cDir := filepath.Join(root, "cplugin")
	if err := InitTemplate("c", "cplugin", cDir); err != nil {
		t.Fatalf("InitTemplate(c) failed: %v", err)
	}
	for _, path := range []string{
		filepath.Join(cDir, "main.c"),
		filepath.Join(cDir, "build.sh"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected generated C template file %s: %v", path, err)
		}
	}

	cppDir := filepath.Join(root, "cppplugin")
	if err := InitTemplate("cpp", "cppplugin", cppDir); err != nil {
		t.Fatalf("InitTemplate(cpp) failed: %v", err)
	}
	for _, path := range []string{
		filepath.Join(cppDir, "main.cpp"),
		filepath.Join(cppDir, "build.sh"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected generated C++ template file %s: %v", path, err)
		}
	}
}

func TestRunCompileAlias(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "demo.sqx")
	if err := os.WriteFile(src, []byte("#!/usr/bin/env bash\nprintf ok\n"), 0o755); err != nil {
		t.Fatalf("could not write source script: %v", err)
	}
	out := filepath.Join(root, "demo-built.sqx")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run([]string{"compile", "--lang", "shell", "--src", src, "--out", out}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%q", exitCode, stderr.String())
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("expected built output file: %v", err)
	}
	if !strings.Contains(stdout.String(), "Built SQX module:") {
		t.Fatalf("expected build message, got %q", stdout.String())
	}
}
