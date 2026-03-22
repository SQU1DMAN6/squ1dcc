package sqxdev

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stdout)
		return 0
	}

	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	case "init":
		return runInit(args[1:], stdout, stderr)
	case "build", "compile":
		return runBuild(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "Unknown sqx command %q\n\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func runInit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("sqx init", flag.ContinueOnError)
	fs.SetOutput(stderr)

	lang := fs.String("lang", "go", "Template language: go, c, cpp, shell")
	name := fs.String("name", "plugin", "Plugin/module name")
	out := fs.String("out", ".", "Output directory")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if err := InitTemplate(*lang, *name, *out); err != nil {
		fmt.Fprintf(stderr, "sqx init failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "SQX template created in %s\n", *out)
	return 0
}

func runBuild(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("sqx build", flag.ContinueOnError)
	fs.SetOutput(stderr)

	lang := fs.String("lang", "go", "Source language: go, c, cpp, shell")
	src := fs.String("src", "", "Source file/directory")
	out := fs.String("out", "", "Output .sqx executable path")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *src == "" {
		if fs.NArg() > 0 {
			*src = fs.Arg(0)
		}
	}
	if *out == "" {
		if fs.NArg() > 1 {
			*out = fs.Arg(1)
		}
	}

	if *src == "" || *out == "" {
		fmt.Fprintln(stderr, "sqx build requires --src and --out (or positional src out)")
		return 2
	}

	if err := Compile(*lang, *src, *out); err != nil {
		fmt.Fprintf(stderr, "sqx build failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "Built SQX module: %s\n", *out)
	return 0
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "SQX Developer Tools")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  squ1dcc sqx init  --lang go --name sql --out ./sql_plugin")
	fmt.Fprintln(w, "  squ1dcc sqx build --lang go --src ./sql_plugin --out ./sql.sqx")
	fmt.Fprintln(w, "  squ1dcc sqx compile --lang c --src ./plugin/main.c --out ./plugin.sqx")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  init   Generate SQX starter templates")
	fmt.Fprintln(w, "  build  Compile SQX source into a native executable module")
	fmt.Fprintln(w, "  compile Alias of build")
}
