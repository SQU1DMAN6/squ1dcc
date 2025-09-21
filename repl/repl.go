package repl

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/user"
	"squ1d++/compiler"
	"squ1d++/lexer"
	"squ1d++/object"
	"squ1d++/parser"
	"squ1d++/vm"
	"strings"
)

const PROMPT = ">> "
const CONTINUATION_PROMPT = "==>  "

// readCompleteInput reads input until a complete statement is entered
func readCompleteInput(scanner *bufio.Scanner, out io.Writer) string {
	var input strings.Builder

	scanned := scanner.Scan()
	if !scanned {
		return ""
	}

	line := scanner.Text()
	input.WriteString(line)

	// Check if we need to continue reading (unmatched braces, parentheses, etc.)
	for needsContinuation(input.String()) {
		fmt.Fprintf(out, CONTINUATION_PROMPT)
		scanned := scanner.Scan()
		if !scanned {
			break
		}
		line = scanner.Text()
		input.WriteString("\n")
		input.WriteString(line)
	}

	return input.String()
}

func needsContinuation(line string) bool {
	openBraces := 0
	openParens := 0
	openBrackets := 0

	for _, char := range line {
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

	// Continue if we have unmatched delimiters
	return openBraces > 0 || openParens > 0 || openBrackets > 0
}

func Start(in io.Reader, out io.Writer) {
	// env := object.NewEnvironment()
	_, err := user.Current()
	if err != nil {
		panic(err)
	}
	scanner := bufio.NewScanner(in)
	constants := []object.Object{}
	globals := make([]object.Object, vm.GlobalsSize)
	symbolTable := compiler.NewSymbolTable()
	for i, v := range object.Builtins {
		symbolTable.DefineBuiltin(i, v.Name)
	}

	// Add class objects to globals
	classes := object.CreateClassObjects()
	builtinCount := len(object.Builtins)
	// Use the same order as the VM expects
	classNames := []string{"io", "type", "time", "os", "math", "string", "file", "pkg"}
	for _, className := range classNames {
		if classObj, ok := classes[className]; ok {
			symbolTable.DefineBuiltin(builtinCount, className)
			globals[builtinCount] = classObj
			builtinCount++
		}
	}

	for {
		fmt.Fprintf(out, PROMPT)

		// Read complete input (handling multi-line statements)
		input := readCompleteInput(scanner, out)
		if input == "" {
			return
		}

		// Simple include handling: include("path") or include("name")
		if incPath, ok := tryParseInclude(input); ok {
			if err := executeInclude(incPath, symbolTable, &constants, globals, out); err != nil {
				fmt.Fprintf(out, "Include error: %v\n", err)
			}
			continue
		}

		l := lexer.New(input)
		p := parser.New(l)

		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			printParserErrors(out, p.Errors())
			continue
		}

		comp := compiler.NewWithState(symbolTable, constants)
		err := comp.Compile(program)
		if err != nil {
			fmt.Fprintf(out, "COMPILATION ERROR:\n    %s\n", err)
			continue
		}

		code := comp.Bytecode()
		constants = code.Constants

		machine := vm.NewWithGlobalsStore(code, globals)

		err = machine.Run()
		if err != nil {
			fmt.Fprintf(out, "INSTRUCTIONS UNCLEAR:\n    %s\n", err)
			continue
		}

		lastPopped := machine.LastPoppedStackElem()
		if lastPopped != nil {
			io.WriteString(out, lastPopped.Inspect())
			io.WriteString(out, "\n")
		}
	}
}

func printParserErrors(out io.Writer, errors []string) {
	io.WriteString(out, "ERROR:\n\t\t\n")
	for _, msg := range errors {
		io.WriteString(out, "\t"+msg+"\n")
	}
}

func tryParseInclude(input string) (string, bool) {
	trimmed := strings.TrimSpace(input)
	if !strings.HasPrefix(trimmed, "include(") || !strings.HasSuffix(trimmed, ")") {
		return "", false
	}
	inside := strings.TrimSpace(trimmed[len("include(") : len(trimmed)-1])
	if len(inside) >= 2 && ((inside[0] == '"' && inside[len(inside)-1] == '"') || (inside[0] == '\'' && inside[len(inside)-1] == '\'')) {
		return inside[1 : len(inside)-1], true
	}
	return inside, true
}

func executeInclude(path string, symbolTable *compiler.SymbolTable, constants *[]object.Object, globals []object.Object, out io.Writer) error {
	candidates := []string{path}
	if !strings.HasSuffix(path, ".sqd") {
		candidates = append(candidates, "lib/"+path+".sqd")
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, home+"/.squ1dlang/packages/"+path+"/__init__.sqd")
	}

	var chosen string
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			chosen = c
			break
		}
	}
	if chosen == "" {
		return fmt.Errorf("module or file not found: %s", path)
	}

	data, err := os.ReadFile(chosen)
	if err != nil {
		return fmt.Errorf("could not read %s: %v", chosen, err)
	}

	// Parse and execute as a whole unit to preserve statements across lines
	l := lexer.New(string(data))
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		printParserErrors(out, p.Errors())
		return fmt.Errorf("parse errors in include %s", chosen)
	}

	comp := compiler.NewWithState(symbolTable, *constants)
	if err := comp.Compile(program); err != nil {
		return fmt.Errorf("compile error in include %s: %v", chosen, err)
	}
	bytecode := comp.Bytecode()
	*constants = bytecode.Constants
	m := vm.NewWithGlobalsStore(bytecode, globals)
	if err := m.Run(); err != nil {
		return fmt.Errorf("runtime error in include %s: %v", chosen, err)
	}
	return nil
}

func ExecuteFile(filename string, out io.Writer) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("Could not open file %s: %v", filename, err)
	}
	defer file.Close()

	// Read the entire file content
	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("Could not read file %s: %v", filename, err)
	}

	// Create a new VM state for file execution
	constants := []object.Object{}
	globals := make([]object.Object, vm.GlobalsSize)
	symbolTable := compiler.NewSymbolTable()
	for i, v := range object.Builtins {
		symbolTable.DefineBuiltin(i, v.Name)
	}

	// Add class objects to globals
	classes := object.CreateClassObjects()
	builtinCount := len(object.Builtins)
	// Use the same order as the VM expects
	classNames := []string{"io", "type", "time", "os", "math", "string", "file", "pkg"}
	for _, className := range classNames {
		if classObj, ok := classes[className]; ok {
			symbolTable.DefineBuiltin(builtinCount, className)
			globals[builtinCount] = classObj
			builtinCount++
		}
	}

	// Process the file content line by line to capture all outputs
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	var currentInput strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines (whitespace is ignored)
		if len(strings.TrimSpace(line)) == 0 {
			continue
		}

		currentInput.WriteString(line)

		// Check if we have a complete statement
		if !needsContinuation(currentInput.String()) {
			// We have a complete statement, execute it
			input := currentInput.String()
			currentInput.Reset()

			// Handle include inline
			if incPath, ok := tryParseInclude(input); ok {
				if err := executeInclude(incPath, symbolTable, &constants, globals, out); err != nil {
					fmt.Fprintf(out, "Include error: %v\n", err)
					return err
				}
				continue
			}

			l := lexer.New(input)
			p := parser.New(l)

			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				printParserErrors(out, p.Errors())
				return fmt.Errorf("Parsing errors in file %s:\t%v\n", filename, p.Errors())
			}

			comp := compiler.NewWithState(symbolTable, constants)
			err := comp.Compile(program)
			if err != nil {
				fmt.Fprintf(out, "COMPILATION ERROR:\n    %s\n", err)
				return fmt.Errorf("In file %s:\t%v\n", filename, err)
			}

			code := comp.Bytecode()
			constants = code.Constants

			machine := vm.NewWithGlobalsStore(code, globals)

			err = machine.Run()
			if err != nil {
				fmt.Fprintf(out, "INSTRUCTIONS UNCLEAR:\n    %s\n", err)
				return fmt.Errorf("Runtime error in file %s:\t%v\n", filename, err)
			}

			// Print the result of this statement
			lastPopped := machine.LastPoppedStackElem()
			if lastPopped != nil {
				io.WriteString(out, lastPopped.Inspect())
				io.WriteString(out, "\n")
			}
		} else {
			// Need more input for this statement, add a newline
			currentInput.WriteString("\n")
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("Error reading file %s: %v", filename, err)
	}

	// Handle any remaining input
	if currentInput.Len() > 0 {
		input := currentInput.String()

		// Handle include inline
		if incPath, ok := tryParseInclude(input); ok {
			if err := executeInclude(incPath, symbolTable, &constants, globals, out); err != nil {
				fmt.Fprintf(out, "Include error: %v\n", err)
				return err
			}
			return nil
		}

		l := lexer.New(input)
		p := parser.New(l)

		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			printParserErrors(out, p.Errors())
			return fmt.Errorf("Parsing errors in file %s: %v", filename, p.Errors())
		}

		comp := compiler.NewWithState(symbolTable, constants)
		err := comp.Compile(program)
		if err != nil {
			fmt.Fprintf(out, "COMPILATION ERROR:\n    %s\n", err)
			return fmt.Errorf("In file %s: %v", filename, err)
		}

		code := comp.Bytecode()
		constants = code.Constants

		machine := vm.NewWithGlobalsStore(code, globals)

		err = machine.Run()
		if err != nil {
			fmt.Fprintf(out, "INSTRUCTIONS UNCLEAR:\n    %s\n", err)
			return fmt.Errorf("Runtime error in file %s: %v", filename, err)
		}

		// Print the result of this statement
		lastPopped := machine.LastPoppedStackElem()
		if lastPopped != nil {
			io.WriteString(out, lastPopped.Inspect())
			io.WriteString(out, "\n")
		}
	}

	return nil
}
