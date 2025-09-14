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
)

const PROMPT = ">> "

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

	for {
		fmt.Fprintf(out, PROMPT)
		scanned := scanner.Scan()

		if !scanned {
			return
		}

		line := scanner.Text()
		l := lexer.New(line)
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
	io.WriteString(out, "ERROR:\n\t")
	for _, msg := range errors {
		io.WriteString(out, "\t"+msg+"\n")
	}
}

func ExecuteFile(filename string, out io.Writer) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("could not open file %s: %v", filename, err)
	}
	defer file.Close()

	// Read the entire file content
	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("could not read file %s: %v", filename, err)
	}

	// Create a new VM state for file execution
	constants := []object.Object{}
	globals := make([]object.Object, vm.GlobalsSize)
	symbolTable := compiler.NewSymbolTable()
	for i, v := range object.Builtins {
		symbolTable.DefineBuiltin(i, v.Name)
	}

	// Parse and execute the file content as one program
	l := lexer.New(string(content))
	p := parser.New(l)

	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		printParserErrors(out, p.Errors())
		return fmt.Errorf("parsing errors in file %s", filename)
	}

	comp := compiler.NewWithState(symbolTable, constants)
	err = comp.Compile(program)
	if err != nil {
		fmt.Fprintf(out, "COMPILATION ERROR:\n    %s\n", err)
		return fmt.Errorf("compilation error in file %s: %v", filename, err)
	}

	code := comp.Bytecode()
	constants = code.Constants

	machine := vm.NewWithGlobalsStore(code, globals)

	err = machine.Run()
	if err != nil {
		fmt.Fprintf(out, "INSTRUCTIONS UNCLEAR:\n    %s\n", err)
		return fmt.Errorf("runtime error in file %s: %v", filename, err)
	}

	// Print the last popped element if it exists
	lastPopped := machine.LastPoppedStackElem()
	if lastPopped != nil {
		io.WriteString(out, lastPopped.Inspect())
		io.WriteString(out, "\n")
	}

	return nil
}
