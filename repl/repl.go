package repl

import (
	"bufio"
	"fmt"
	"io"
	"os/user"
	"squ1d++/compiler"
	"squ1d++/lexer"
	"squ1d++/parser"
	"squ1d++/vm"
	"squ1d++/object"
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
			fmt.Fprintf(out, "COMPILATION GOT FLIPPED:\n    %s\n", err)
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
		io.WriteString(out, lastPopped.Inspect())
		io.WriteString(out, "\n")
	}
}

func printParserErrors(out io.Writer, errors []string) {
	io.WriteString(out, "ERROR:\n\t")
	for _, msg := range errors {
		io.WriteString(out, "\t"+msg+"\n")
	}
}
