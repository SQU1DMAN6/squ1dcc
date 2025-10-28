package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
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
			fmt.Fprintf(os.Stderr, "Error executing file %s: %v\n\t", filename, err)
			os.Exit(1)
		}
	} else {
		// Interactive REPL mode
		fmt.Printf("Hello %s! This is the SQU1D++ SQU1DLang compiler, version 1 written by Quan Thai.\n", user.Username)
		fmt.Printf("Available classes: %s\n\n", object.ListDefinedClasses())
		repl.Start(os.Stdin, os.Stdout)
	}
}

// isCompleteStatement checks if the input represents a complete statement
func isCompleteStatement(input string) bool {
	openBraces := 0
	openParens := 0
	openBrackets := 0

	for _, char := range input {
		switch char {
		case '{':
			openBraces++
		case '}':
			openBraces--
		case '(':
			openParens++
		case ')':
			openParens--
		case '[':
			openBrackets++
		case ']':
			openBrackets--
		}
	}

	// Statement is complete if all delimiters are matched
	return openBraces == 0 && openParens == 0 && openBrackets == 0
}

func compileToExecutable(inputFile, outputFile string) error {
	// Read the input file
	content, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("Could not read input file: %v", err)
	}

	// Expand include("...") statements by inlining their content before embedding
	expandIncludes := func(code string) (string, error) {
		var out []string
		lines := strings.Split(code, "\n")
		for _, raw := range lines {
			line := strings.TrimSpace(raw)
			if strings.HasPrefix(line, "include(") && strings.HasSuffix(line, ")") {
				inside := strings.TrimSpace(line[len("include(") : len(line)-1])
				if len(inside) >= 2 && ((inside[0] == '"' && inside[len(inside)-1] == '"') || (inside[0] == '\'' && inside[len(inside)-1] == '\'')) {
					inside = inside[1 : len(inside)-1]
				}
				candidates := []string{inside}
				if !strings.HasSuffix(inside, ".sqd") {
					candidates = append(candidates, filepath.Join("lib", inside+".sqd"))
				}
				if home, err := os.UserHomeDir(); err == nil {
					candidates = append(candidates, filepath.Join(home, ".cache", "squ1dlang", inside, "main.sqd"))
				}
				var chosen string
				for _, c := range candidates {
					if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
						chosen = c
						break
					}
				}
				if chosen == "" {
					return "", fmt.Errorf("Module or file not found: %s", inside)
				}
				data, err := os.ReadFile(chosen)
				if err != nil {
					return "", fmt.Errorf("Could not read %s: %v", chosen, err)
				}
				out = append(out, string(data))
				continue
			}
			out = append(out, raw)
		}
		return strings.Join(out, "\n"), nil
	}

	expanded, err := expandIncludes(string(content))
	if err != nil {
		return fmt.Errorf("Include error: %v", err)
	}

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

// isCompleteStatement checks if the input represents a complete statement
func isCompleteStatement(input string) bool {
	openBraces := 0
	openParens := 0
	openBrackets := 0

	for _, char := range input {
		switch char {
		case '{':
			openBraces++
		case '}':
			openBraces--
		case '(':
			openParens++
		case ')':
			openParens--
		case '[':
			openBrackets++
		case ']':
			openBrackets--
		}
	}

	// Statement is complete if all delimiters are matched
	return openBraces == 0 && openParens == 0 && openBrackets == 0
}

func main() {
	// Create a new VM state
	constants := []object.Object{}
	globals := make([]object.Object, vm.GlobalsSize)
	symbolTable := compiler.NewSymbolTable()
	for i, v := range object.Builtins {
		symbolTable.DefineBuiltin(i, v.Name)
	}

	// Register class objects (math, os, time, string, etc.)
	classes := object.CreateClassObjects()
	for name, classObj := range classes {
    	sym := symbolTable.Define(name)
    	globals[sym.Index] = classObj
    }

	// Create a single compiler instance to maintain state across lines
	comp := compiler.NewWithState(symbolTable, constants)

	// Read the file and group lines into complete statements
	scanner := bufio.NewScanner(strings.NewReader(squ1dCode))
	lineNumber := 0
	var currentStatement strings.Builder

	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		currentStatement.WriteString(line)
		currentStatement.WriteString("\n")

		// Check if we have a complete statement
		if isCompleteStatement(currentStatement.String()) {
			statement := strings.TrimSpace(currentStatement.String())
			if statement != "" {
				// Parse and execute the complete statement
				l := lexer.New(statement)
				p := parser.New(l)

				program := p.ParseProgram()
				if len(p.Errors()) != 0 {
					fmt.Fprintf(os.Stderr, "ERROR on line %%d:\\n", lineNumber)
					for _, msg := range p.Errors() {
						fmt.Fprintf(os.Stderr, "\\t%%s\\n", msg)
					}
					os.Exit(1)
				}

				// Compile the program using the persistent compiler
				err := comp.Compile(program)
				if err != nil {
					fmt.Fprintf(os.Stderr, "COMPILATION ERROR on line %%d:\\n    %%s\\n", lineNumber, err)
					os.Exit(1)
				}

				// Get the current bytecode and update constants
				code := comp.Bytecode()
				constants = code.Constants

				// Create a new VM for this statement's execution
				machine := vm.NewWithGlobalsStore(code, globals)

				err = machine.Run()
				if err != nil {
					fmt.Fprintf(os.Stderr, "INSTRUCTIONS UNCLEAR on line %%d:\\n    %%s\\n", lineNumber, err)
					os.Exit(1)
				}

				// Print the result of each statement
				lastPopped := machine.LastPoppedStackElem()
				if lastPopped != nil {
					fmt.Print(lastPopped.Inspect())
					fmt.Println()
				}
			}

			// Reset for next statement
			currentStatement.Reset()
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading code: %%v\\n", err)
		os.Exit(1)
	}
}
`, expanded)

	err = os.WriteFile(tempGoFile, []byte(goCode), 0644)
	if err != nil {
		return fmt.Errorf("Could not write temporary Go file: %v", err)
	}

	// Compile the Go file to an executable
	cmd := exec.Command("go", "build", "-o", outputFile, tempGoFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("Could not compile code: %v", err)
	}

	return nil
}