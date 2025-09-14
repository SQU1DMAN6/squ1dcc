package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"squ1d++/repl"
)

func main() {
	user, err := user.Current()
	if err != nil {
		panic(err)
	}

	compileFlag := flag.Bool("compile", false, "Compile .sqd file to executable")
	outputFlag := flag.String("o", "", "Output executable name (default: same as input file)")
	flag.Parse()

	args := flag.Args()

	if *compileFlag {
		if len(args) == 0 {
			fmt.Fprintf(os.Stderr, "Error: No input file specified for compilation\n")
			fmt.Fprintf(os.Stderr, "Usage: %s -compile <input.sqd> [-o output]\n", os.Args[0])
			os.Exit(1)
		}
		
		inputFile := args[0]
		outputFile := *outputFlag
		if outputFile == "" {
			// Remove .sqd extension and add executable extension
			outputFile = strings.TrimSuffix(inputFile, ".sqd")
		}
		
		err := compileToExecutable(inputFile, outputFile)
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
			fmt.Fprintf(os.Stderr, "Error executing file %s: %v\n", filename, err)
			os.Exit(1)
		}
	} else {
		// Interactive REPL mode
		fmt.Printf("Hello %s! This is SQU1D++!\n", user.Username)
		repl.Start(os.Stdin, os.Stdout)
	}
}

func compileToExecutable(inputFile, outputFile string) error {
	// Read the input file
	content, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("could not read input file: %v", err)
	}

	// Create a temporary Go file that embeds the SQU1D++ code
	tempGoFile := "temp_squ1d_compiled.go"
	defer os.Remove(tempGoFile)

	goCode := fmt.Sprintf(`package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"squ1d++/compiler"
	"squ1d++/lexer"
	"squ1d++/object"
	"squ1d++/parser"
	"squ1d++/vm"
)

const squ1dCode = %q

func main() {
	// Create a new VM state
	constants := []object.Object{}
	globals := make([]object.Object, vm.GlobalsSize)
	symbolTable := compiler.NewSymbolTable()
	for i, v := range object.Builtins {
		symbolTable.DefineBuiltin(i, v.Name)
	}

	// Read the file line by line
	scanner := bufio.NewScanner(strings.NewReader(squ1dCode))
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines
		if line == "" {
			continue
		}

		// Parse and execute each line individually
		l := lexer.New(line)
		p := parser.New(l)

		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			fmt.Fprintf(os.Stderr, "ERROR on line %%d:\\n", lineNumber)
			for _, msg := range p.Errors() {
				fmt.Fprintf(os.Stderr, "\\t%%s\\n", msg)
			}
			os.Exit(1)
		}

		comp := compiler.NewWithState(symbolTable, constants)
		err := comp.Compile(program)
		if err != nil {
			fmt.Fprintf(os.Stderr, "COMPILATION ERROR on line %%d:\\n    %%s\\n", lineNumber, err)
			os.Exit(1)
		}

		code := comp.Bytecode()
		constants = code.Constants

		machine := vm.NewWithGlobalsStore(code, globals)

		err = machine.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "INSTRUCTIONS UNCLEAR on line %%d:\\n    %%s\\n", lineNumber, err)
			os.Exit(1)
		}

		// Print the result of each line
		lastPopped := machine.LastPoppedStackElem()
		if lastPopped != nil {
			fmt.Print(lastPopped.Inspect())
			fmt.Println()
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading code: %%v\\n", err)
		os.Exit(1)
	}
}
`, string(content))

	err = os.WriteFile(tempGoFile, []byte(goCode), 0644)
	if err != nil {
		return fmt.Errorf("could not write temporary Go file: %v", err)
	}

	// Compile the Go file to an executable
	cmd := exec.Command("go", "build", "-o", outputFile, tempGoFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("could not compile Go code: %v", err)
	}

	return nil
}
