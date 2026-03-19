package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"os/user"
	"runtime"
	"squ1d++/builder"
	"squ1d++/bytecode"
	"squ1d++/compiler"
	"squ1d++/object"
	"squ1d++/repl"
	"squ1d++/vm"
	"strings"
)

const embeddedMarker = "SQU1D++EMBED"

func main() {
	if ran, err := tryRunEmbedded(); ran {
		if err != nil {
			fmt.Fprintf(os.Stderr, "Runtime error: %v\n", err)
			os.Exit(1)
		}
		return
	}

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
		fmt.Printf("Hello %s! This is the SQU1D++ SQU1DLang compiler, version 1.7.0 written by Quan Thai.\n", user.Username)
		fmt.Printf("Available classes: %s\n\n", object.ListDefinedClasses())
		repl.Start(os.Stdin, os.Stdout)
	}
}

func tryRunEmbedded() (bool, error) {
	exe, err := os.Executable()
	if err != nil {
		return false, nil
	}

	data, err := os.ReadFile(exe)
	if err != nil {
		return false, nil
	}

	if len(data) < 8+len(embeddedMarker) {
		return false, nil
	}

	markerStart := len(data) - len(embeddedMarker) - 8
	if string(data[markerStart+8:]) != embeddedMarker {
		return false, nil
	}

	bcLen := int(binary.LittleEndian.Uint64(data[markerStart : markerStart+8]))
	payloadStart := markerStart - bcLen
	if payloadStart < 0 {
		return false, fmt.Errorf("invalid embedded bytecode payload length")
	}

	bcData := data[payloadStart:markerStart]
	pkg, err := bytecode.Deserialize(bytes.NewReader(bcData))
	if err != nil {
		return true, err
	}

	return true, runEmbeddedBytecode(pkg)
}

func runEmbeddedBytecode(pkg *bytecode.Package) error {
	globals := make([]object.Object, vm.GlobalsSize)
	symbolTable := compiler.NewSymbolTable()

	for i, v := range object.Builtins {
		symbolTable.DefineBuiltin(i, v.Name)
	}

	classes := object.CreateClassObjects()
	for name, classObj := range classes {
		sym := symbolTable.Define(name)
		globals[sym.Index] = classObj
	}

	machine := vm.NewWithGlobalsStore(&compiler.Bytecode{
		Instructions: pkg.Instructions,
		Constants:    pkg.Constants,
	}, globals)

	if err := machine.Run(); err != nil {
		return err
	}

	lastPopped := machine.LastPoppedStackElem()
	if lastPopped != nil && lastPopped.Type() != object.NULL_OBJ {
		fmt.Println(lastPopped.Inspect())
	}

	return nil
}
