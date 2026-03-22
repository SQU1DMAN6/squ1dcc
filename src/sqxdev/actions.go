package sqxdev

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

type templateData struct {
	Name   string
	Module string
}

func InitTemplate(lang, name, outDir string) error {
	lang = strings.ToLower(strings.TrimSpace(lang))
	name = strings.TrimSpace(name)
	if name == "" {
		name = "plugin"
	}
	if outDir == "" {
		outDir = "."
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("could not create output directory: %w", err)
	}

	data := templateData{
		Name:   name,
		Module: normalizedModuleName(name),
	}

	switch lang {
	case "go", "golang":
		mainPath := filepath.Join(outDir, "main.go")
		runtimePath := filepath.Join(outDir, "sqx_runtime.go")
		goModPath := filepath.Join(outDir, "go.mod")
		buildPath := filepath.Join(outDir, "build.sh")

		if err := writeTemplateFile(mainPath, goMainTemplate, data, 0o644); err != nil {
			return err
		}
		if err := writeTemplateFile(runtimePath, goRuntimeTemplate, data, 0o644); err != nil {
			return err
		}
		if err := writeTemplateFile(goModPath, goModTemplate, data, 0o644); err != nil {
			return err
		}
		if err := writeTemplateFile(buildPath, goBuildTemplate, data, 0o755); err != nil {
			return err
		}
		return nil
	case "c":
		mainPath := filepath.Join(outDir, "main.c")
		buildPath := filepath.Join(outDir, "build.sh")
		if err := writeTemplateFile(mainPath, cMainTemplate, data, 0o644); err != nil {
			return err
		}
		if err := writeTemplateFile(buildPath, cBuildTemplate, data, 0o755); err != nil {
			return err
		}
		return nil
	case "cpp", "c++":
		mainPath := filepath.Join(outDir, "main.cpp")
		buildPath := filepath.Join(outDir, "build.sh")
		if err := writeTemplateFile(mainPath, cppMainTemplate, data, 0o644); err != nil {
			return err
		}
		if err := writeTemplateFile(buildPath, cppBuildTemplate, data, 0o755); err != nil {
			return err
		}
		return nil
	case "shell", "sh", "bash":
		modulePath := filepath.Join(outDir, name+".sqx")
		if err := writeTemplateFile(modulePath, shellTemplate, data, 0o755); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("unsupported template language %q", lang)
	}
}

func Compile(lang, src, out string) error {
	lang = strings.ToLower(strings.TrimSpace(lang))
	src = strings.TrimSpace(src)
	out = strings.TrimSpace(out)

	if src == "" {
		return fmt.Errorf("missing source path")
	}
	if out == "" {
		return fmt.Errorf("missing output path")
	}

	switch lang {
	case "go", "golang":
		absOut, err := filepath.Abs(out)
		if err != nil {
			return fmt.Errorf("could not resolve output path: %w", err)
		}

		buildArg := src
		workDir := ""
		if stat, err := os.Stat(src); err == nil {
			if stat.IsDir() {
				workDir = src
				buildArg = "."
			} else {
				workDir = filepath.Dir(src)
				buildArg = filepath.Base(src)
			}
		}

		cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", absOut, buildArg)
		if workDir != "" {
			cmd.Dir = workDir
		}
		if output, err := cmd.CombinedOutput(); err != nil {
			msg := strings.TrimSpace(string(output))
			if msg != "" {
				return fmt.Errorf("go build failed: %v: %s", err, msg)
			}
			return fmt.Errorf("go build failed: %w", err)
		}
		return nil
	case "c":
		return compileWithCandidates("c", [][]string{
			{"cc", "-O2", "-o", out, src},
			{"gcc", "-O2", "-o", out, src},
			{"clang", "-O2", "-o", out, src},
		})
	case "cpp", "c++":
		return compileWithCandidates("c++", [][]string{
			{"c++", "-O2", "-o", out, src},
			{"g++", "-O2", "-o", out, src},
			{"clang++", "-O2", "-o", out, src},
		})
	case "shell", "sh", "bash":
		content, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("could not read source script: %w", err)
		}
		if err := os.WriteFile(out, content, 0o755); err != nil {
			return fmt.Errorf("could not write output script: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported compile language %q", lang)
	}
}

func compileWithCandidates(kind string, commands [][]string) error {
	var available []string
	var failures []string

	for _, cmdParts := range commands {
		if len(cmdParts) == 0 {
			continue
		}

		binary := cmdParts[0]
		resolved, err := exec.LookPath(binary)
		if err != nil {
			continue
		}
		available = append(available, binary)

		cmd := exec.Command(resolved, cmdParts[1:]...)
		output, err := cmd.CombinedOutput()
		if err == nil {
			return nil
		}

		msg := strings.TrimSpace(string(output))
		if msg == "" {
			failures = append(failures, fmt.Sprintf("%s: %v", binary, err))
		} else {
			failures = append(failures, fmt.Sprintf("%s: %v: %s", binary, err, msg))
		}
	}

	if len(available) == 0 {
		names := make([]string, 0, len(commands))
		for _, cmdParts := range commands {
			if len(cmdParts) > 0 {
				names = append(names, cmdParts[0])
			}
		}
		return fmt.Errorf("no %s compiler found (tried: %s)", kind, strings.Join(names, ", "))
	}

	return fmt.Errorf("%s build failed: %s", kind, strings.Join(failures, "; "))
}

func writeTemplateFile(path, tmpl string, data templateData, mode os.FileMode) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("refusing to overwrite existing file %q", path)
	}

	t, err := template.New(filepath.Base(path)).Parse(tmpl)
	if err != nil {
		return fmt.Errorf("template parse failed for %q: %w", path, err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return fmt.Errorf("could not create file %q: %w", path, err)
	}
	defer f.Close()

	if err := t.Execute(f, data); err != nil {
		return fmt.Errorf("template render failed for %q: %w", path, err)
	}
	return nil
}

func normalizedModuleName(name string) string {
	base := strings.ToLower(strings.TrimSpace(name))
	if base == "" {
		base = "plugin"
	}

	var b strings.Builder
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}

	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "plugin"
	}

	return "example.com/sqx/" + out
}

const goMainTemplate = `package main

import "os"

func main() {
	module := NewSQXModule("{{.Name}}")

	module.RegisterMany(
		// Add SQX methods here.
		// Name maps to <namespace>.<Name>() in SQU1DLang.
		// Handle receives CLI args as strings and returns a value/error.
		SQXMethod{
			Name:   "ping",
			Return: SQXReturnString,
			Handle: func(args []string) (interface{}, error) {
				if err := SQXRequireArgs(args, 0); err != nil {
					return nil, err
				}
				return "{{.Name}} pong", nil
			},
		},
	)

	os.Exit(module.Run(os.Args[1:], os.Stdout, os.Stderr))
}
`

const goBuildTemplate = `#!/usr/bin/env bash
set -euo pipefail

OUT="${1:-{{.Name}}.sqx}"
CACHE_ROOT="${TMPDIR:-/tmp}/squ1d_sqx_go_cache"
mkdir -p "$CACHE_ROOT"
GOCACHE_DIR="${GOCACHE:-$CACHE_ROOT}"
GOCACHE="$GOCACHE_DIR" go build -trimpath -ldflags="-s -w" -o "$OUT" .
chmod +x "$OUT"
echo "Built $OUT"
`

const goModTemplate = `module {{.Module}}

go 1.22
`

const shellTemplate = `#!/usr/bin/env bash
set -euo pipefail

command="${1:-}"

case "$command" in
  __sqx_manifest__)
    cat <<'JSON'
{"version":1,"functions":{"ping":{"return":"string"}}}
JSON
    ;;
  __sqx_call__)
    fn="${2:-}"
    shift 2 || true
    case "$fn" in
      # Add function handlers here.
      ping)
        printf "{{.Name}} pong"
        ;;
      *)
        echo "unknown SQX function: $fn" >&2
        exit 2
        ;;
    esac
    ;;
  *)
    echo "unknown SQX command: $command" >&2
    exit 2
    ;;
esac
`

const cMainTemplate = `#include <stdio.h>
#include <string.h>

static int handle_call(const char *fn, int argc, char **argv) {
	(void)argv;

	// Add SQX function handlers here.
	if (strcmp(fn, "ping") == 0) {
		if (argc != 0) {
			fprintf(stderr, "expected 0 argument(s), got %d\n", argc);
			return 2;
		}
		printf("{{.Name}} pong");
		return 0;
	}

	fprintf(stderr, "unknown SQX function: %s\n", fn);
	return 2;
}

int main(int argc, char **argv) {
	if (argc < 2) {
		fprintf(stderr, "usage: {{.Name}} <__sqx_manifest__|__sqx_call__>\n");
		return 2;
	}

	if (strcmp(argv[1], "__sqx_manifest__") == 0) {
		// Declare exported methods and return modes here.
		printf("{\"version\":1,\"functions\":{\"ping\":{\"return\":\"string\"}}}");
		return 0;
	}

	if (strcmp(argv[1], "__sqx_call__") == 0) {
		if (argc < 3) {
			fprintf(stderr, "missing SQX function name\n");
			return 2;
		}
		return handle_call(argv[2], argc - 3, argv + 3);
	}

	fprintf(stderr, "unknown SQX command: %s\n", argv[1]);
	return 2;
}
`

const cBuildTemplate = `#!/usr/bin/env bash
set -euo pipefail

OUT="${1:-{{.Name}}.sqx}"
cc -O2 -o "$OUT" main.c
chmod +x "$OUT"
echo "Built $OUT"
`

const cppMainTemplate = `#include <iostream>
#include <string>

static int handle_call(const std::string &fn, int argc, char **argv) {
	(void)argv;

	// Add SQX function handlers here.
	if (fn == "ping") {
		if (argc != 0) {
			std::cerr << "expected 0 argument(s), got " << argc << "\n";
			return 2;
		}
		std::cout << "{{.Name}} pong";
		return 0;
	}

	std::cerr << "unknown SQX function: " << fn << "\n";
	return 2;
}

int main(int argc, char **argv) {
	if (argc < 2) {
		std::cerr << "usage: {{.Name}} <__sqx_manifest__|__sqx_call__>\n";
		return 2;
	}

	const std::string command = argv[1];
	if (command == "__sqx_manifest__") {
		// Declare exported methods and return modes here.
		std::cout << "{\"version\":1,\"functions\":{\"ping\":{\"return\":\"string\"}}}";
		return 0;
	}

	if (command == "__sqx_call__") {
		if (argc < 3) {
			std::cerr << "missing SQX function name\n";
			return 2;
		}
		return handle_call(argv[2], argc - 3, argv + 3);
	}

	std::cerr << "unknown SQX command: " << command << "\n";
	return 2;
}
`

const cppBuildTemplate = `#!/usr/bin/env bash
set -euo pipefail

OUT="${1:-{{.Name}}.sqx}"
c++ -O2 -o "$OUT" main.cpp
chmod +x "$OUT"
echo "Built $OUT"
`

const goRuntimeTemplate = `package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type SQXReturnMode string

const (
	sqxTypedArgPrefix            = "__sqx_typed__:"
	SQXReturnAuto   SQXReturnMode = "auto"
	SQXReturnString SQXReturnMode = "string"
	SQXReturnRaw    SQXReturnMode = "raw"
	SQXReturnInt    SQXReturnMode = "int"
	SQXReturnFloat  SQXReturnMode = "float"
	SQXReturnBool   SQXReturnMode = "bool"
	SQXReturnNull   SQXReturnMode = "null"
	SQXReturnJSON   SQXReturnMode = "json"
)

type SQXHandler func(args []string) (interface{}, error)

type SQXMethod struct {
	Name   string
	Return SQXReturnMode
	Handle SQXHandler
}

type SQXModule struct {
	name    string
	methods map[string]SQXMethod
}

func NewSQXModule(name string) *SQXModule {
	return &SQXModule{
		name:    name,
		methods: make(map[string]SQXMethod),
	}
}

func (m *SQXModule) Register(method SQXMethod) {
	name := strings.TrimSpace(method.Name)
	if name == "" || method.Handle == nil {
		return
	}
	if method.Return == "" {
		method.Return = SQXReturnAuto
	}
	method.Name = name
	m.methods[name] = method
}

func (m *SQXModule) RegisterMany(methods ...SQXMethod) {
	for _, method := range methods {
		m.Register(method)
	}
}

func (m *SQXModule) Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintf(stderr, "usage: %s <__sqx_manifest__|__sqx_call__>\n", m.name)
		return 2
	}

	switch args[0] {
	case "__sqx_manifest__":
		return m.writeManifest(stdout, stderr)
	case "__sqx_call__":
		return m.call(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown SQX command: %s\n", args[0])
		return 2
	}
}

func (m *SQXModule) writeManifest(stdout, stderr io.Writer) int {
	type manifestFunc struct {
		Return string ` + "`json:\"return\"`" + `
	}
	type manifest struct {
		Version   int                     ` + "`json:\"version\"`" + `
		Functions map[string]manifestFunc ` + "`json:\"functions\"`" + `
	}

	out := manifest{
		Version:   1,
		Functions: make(map[string]manifestFunc, len(m.methods)),
	}
	for name, method := range m.methods {
		out.Functions[name] = manifestFunc{Return: string(method.Return)}
	}

	data, err := json.Marshal(out)
	if err != nil {
		fmt.Fprintf(stderr, "could not encode SQX manifest: %v\n", err)
		return 1
	}
	if _, err := stdout.Write(data); err != nil {
		fmt.Fprintf(stderr, "could not write SQX manifest: %v\n", err)
		return 1
	}
	return 0
}

func (m *SQXModule) call(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "missing SQX function name")
		return 2
	}

	method, ok := m.methods[args[0]]
	if !ok {
		fmt.Fprintf(stderr, "unknown SQX function: %s\n", args[0])
		return 2
	}

	result, err := method.Handle(args[1:])
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}

	if err := writeSQXResult(stdout, method.Return, result); err != nil {
		fmt.Fprintf(stderr, "SQX result encode failed: %v\n", err)
		return 1
	}
	return 0
}

func writeSQXResult(out io.Writer, mode SQXReturnMode, value interface{}) error {
	switch strings.ToLower(strings.TrimSpace(string(mode))) {
	case "", string(SQXReturnAuto):
		return writeSQXAuto(out, value)
	case string(SQXReturnString):
		_, err := io.WriteString(out, fmt.Sprint(value))
		return err
	case string(SQXReturnRaw):
		if bytes, ok := value.([]byte); ok {
			_, err := out.Write(bytes)
			return err
		}
		_, err := io.WriteString(out, fmt.Sprint(value))
		return err
	case string(SQXReturnInt):
		i, err := sqxToInt64(value)
		if err != nil {
			return err
		}
		_, err = io.WriteString(out, strconv.FormatInt(i, 10))
		return err
	case string(SQXReturnFloat):
		f, err := sqxToFloat64(value)
		if err != nil {
			return err
		}
		_, err = io.WriteString(out, strconv.FormatFloat(f, 'f', -1, 64))
		return err
	case string(SQXReturnBool):
		b, err := sqxToBool(value)
		if err != nil {
			return err
		}
		if b {
			_, err = io.WriteString(out, "true")
		} else {
			_, err = io.WriteString(out, "false")
		}
		return err
	case string(SQXReturnNull):
		return nil
	case string(SQXReturnJSON):
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		_, err = out.Write(data)
		return err
	default:
		return fmt.Errorf("unknown SQX return mode %q", mode)
	}
}

func writeSQXAuto(out io.Writer, value interface{}) error {
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		_, err := io.WriteString(out, v)
		return err
	case []byte:
		_, err := out.Write(v)
		return err
	case bool:
		if v {
			_, err := io.WriteString(out, "true")
			return err
		}
		_, err := io.WriteString(out, "false")
		return err
	case int:
		_, err := io.WriteString(out, strconv.Itoa(v))
		return err
	case int8, int16, int32, int64:
		i, err := sqxToInt64(v)
		if err != nil {
			return err
		}
		_, err = io.WriteString(out, strconv.FormatInt(i, 10))
		return err
	case uint, uint8, uint16, uint32, uint64:
		i, err := sqxToInt64(v)
		if err != nil {
			return err
		}
		_, err = io.WriteString(out, strconv.FormatInt(i, 10))
		return err
	case float32, float64:
		f, err := sqxToFloat64(v)
		if err != nil {
			return err
		}
		_, err = io.WriteString(out, strconv.FormatFloat(f, 'f', -1, 64))
		return err
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return err
		}
		_, err = out.Write(data)
		return err
	}
}

func sqxToInt64(v interface{}) (int64, error) {
	switch n := v.(type) {
	case int:
		return int64(n), nil
	case int8:
		return int64(n), nil
	case int16:
		return int64(n), nil
	case int32:
		return int64(n), nil
	case int64:
		return n, nil
	case uint:
		return int64(n), nil
	case uint8:
		return int64(n), nil
	case uint16:
		return int64(n), nil
	case uint32:
		return int64(n), nil
	case uint64:
		return int64(n), nil
	case float32:
		return int64(n), nil
	case float64:
		return int64(n), nil
	case string:
		return strconv.ParseInt(strings.TrimSpace(n), 10, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to int", v)
	}
}

func sqxToFloat64(v interface{}) (float64, error) {
	switch n := v.(type) {
	case int:
		return float64(n), nil
	case int8:
		return float64(n), nil
	case int16:
		return float64(n), nil
	case int32:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case uint:
		return float64(n), nil
	case uint8:
		return float64(n), nil
	case uint16:
		return float64(n), nil
	case uint32:
		return float64(n), nil
	case uint64:
		return float64(n), nil
	case float32:
		return float64(n), nil
	case float64:
		return n, nil
	case string:
		return strconv.ParseFloat(strings.TrimSpace(n), 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to float", v)
	}
}

func sqxToBool(v interface{}) (bool, error) {
	switch b := v.(type) {
	case bool:
		return b, nil
	case string:
		return strconv.ParseBool(strings.TrimSpace(strings.ToLower(b)))
	default:
		return false, fmt.Errorf("cannot convert %T to bool", v)
	}
}

func SQXRequireArgs(args []string, expected int) error {
	if len(args) != expected {
		return fmt.Errorf("expected %d argument(s), got %d", expected, len(args))
	}
	return nil
}

func SQXDecodeArg(arg string) (interface{}, error) {
	if !strings.HasPrefix(arg, sqxTypedArgPrefix) {
		return arg, nil
	}
	payload := strings.TrimPrefix(arg, sqxTypedArgPrefix)
	var decoded interface{}
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		return nil, fmt.Errorf("failed to decode typed SQX argument: %w", err)
	}
	return decoded, nil
}

func SQXArgAny(args []string, index int) (interface{}, error) {
	if index < 0 || index >= len(args) {
		return nil, fmt.Errorf("missing argument at index %d", index)
	}
	return SQXDecodeArg(args[index])
}

func SQXArgString(args []string, index int) (string, error) {
	if index < 0 || index >= len(args) {
		return "", fmt.Errorf("missing argument at index %d", index)
	}

	decoded, err := SQXDecodeArg(args[index])
	if err != nil {
		return "", err
	}

	switch v := decoded.(type) {
	case string:
		return v, nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(v), nil
	case nil:
		return "", nil
	default:
		return fmt.Sprint(v), nil
	}
}

func SQXArgBool(args []string, index int) (bool, error) {
	if index < 0 || index >= len(args) {
		return false, fmt.Errorf("missing argument at index %d", index)
	}

	decoded, err := SQXDecodeArg(args[index])
	if err != nil {
		return false, err
	}

	switch v := decoded.(type) {
	case bool:
		return v, nil
	case string:
		if v == "true" {
			return true, nil
		} else if v == "false" {
			return false, nil
		}
		return false, fmt.Errorf("incorrect value provided")
	case float64:
		return v != 0, nil
	default:
		return false, fmt.Errorf("incorrect value provided")
	}
}

func SQXArgInt(args []string, index int) (int64, error) {
	decoded, err := SQXArgAny(args, index)
	if err != nil {
		return 0, err
	}

	switch v := decoded.(type) {
	case float64:
		return int64(v), nil
	case string:
		i, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("argument %d must be int: %w", index, err)
		}
		return i, nil
	default:
		return 0, fmt.Errorf("argument %d must be int", index)
	}
}
`
