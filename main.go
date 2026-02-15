package main

import (
	"flag"
	"fmt"
	"os"
	"os/user"
	"runtime"
	"squ1d++/builder"
	"squ1d++/object"
	"squ1d++/repl"
	"strings"
)

func main() {
	user, err := user.Current()
	if err != nil {
		panic(err)
	}

	compileFlag := flag.Bool("B", false, "Build .sqd file to executable")
	outputFlag := flag.String("o", "", "Output executable name (default: same as input file)")
	flag.Parse()

	args := flag.Args()

	if *compileFlag {
		if len(args) == 0 {
			fmt.Fprintf(os.Stderr, "Error: No input file specified for compilation\n")
			fmt.Fprintf(os.Stderr, "Usage: %s -B <input.sqd> [-o output]\n", os.Args[0])
			os.Exit(1)
		}

		inputFile := args[0]
		outputFile := *outputFlag
		if outputFile == "" {
			outputFile = strings.TrimSuffix(inputFile, ".sqd")
		}
		// Add .exe extension on Windows if not already present
		if !strings.HasSuffix(outputFile, ".exe") && runtime.GOOS == "windows" {
			outputFile = outputFile + ".exe"
		}

		err := builder.BuildStandalone(inputFile, outputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error compiling %s: %v\n", inputFile, err)
			os.Exit(1)
		}
		fmt.Printf("Successfully compiled %s to %s\n", inputFile, outputFile)
	} else if len(args) > 0 {
		// Execute file mode
		filename := args[0]
		err := repl.ExecuteFile(filename, os.Stdout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error executing file %s: %v\n\t", filename, err)
			os.Exit(1)
		}
	} else {
		// Interactive REPL mode
		fmt.Printf("Hello %s! This is the SQU1D++ SQU1DLang compiler, version 1.6.0 written by Quan Thai.\n", user.Username)
		fmt.Printf("Available classes: %s\n\n", object.ListDefinedClasses())
		repl.Start(os.Stdin, os.Stdout)
	}
}