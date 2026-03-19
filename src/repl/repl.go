package repl

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"squ1d++/ast"
	"squ1d++/compiler"
	"squ1d++/evaluator"
	"squ1d++/lexer"
	"squ1d++/object"
	"squ1d++/parser"
	"squ1d++/vm"
	"strings"
)

const PROMPT = ">> "
const CONTINUATION_PROMPT = " > "

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
	// Ensure builtins write to the REPL output writer so tests can capture prints.
	object.OutWriter = out
	env := object.NewEnvironment()
	_, err := user.Current()
	if err != nil {
		panic(err)
	}
	scanner := bufio.NewScanner(in)

	classes := object.CreateClassObjects()
	for name, obj := range classes {
		env.Set(name, obj)
	}

	for {
		fmt.Fprintf(out, PROMPT)

		// Read complete input (handling multi-line statements)
		input := readCompleteInput(scanner, out)
		if input == "" {
			continue
		}

		// Simple include handling: include("path") or include("name")
		if incPath, ok := tryParseInclude(input); ok {
			if err := executeInclude(incPath, env, out); err != nil {
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

		evaluated := evaluator.Eval(program, env)
		if evaluated != nil {
			if evaluated.Type() != object.NULL_OBJ {
				io.WriteString(out, evaluated.Inspect())
				io.WriteString(out, "\n")
			}
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

func executeInclude(path string, env *object.Environment, out io.Writer) error {
	candidates := []string{path}
	if !strings.HasSuffix(path, ".sqd") {
		candidates = append(candidates, "lib/"+path+".sqd")
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, home+"/.squ1dlang/packages/"+path+"/main.sqd")
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

	evaluated := evaluator.Eval(program, env)
	if evaluated != nil && evaluated.Type() == object.ERROR_OBJ {
		return fmt.Errorf("runtime error in include %s: %s", chosen, evaluated.Inspect())
	}
	return nil
}

func ExecuteFile(filename string, out io.Writer) error {
	// Ensure builtins write to the provided writer so file execution prints
	// are captured by callers (tests, CLI, etc.).
	object.OutWriter = out
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

	// We'll compile & run the file one complete statement at a time,
	// preserving compiler state and globals between statements (like main.go).

	// Build an initial symbol table and register builtins and class names
	symbolTable := compiler.NewSymbolTable()
	for i, v := range object.Builtins {
		symbolTable.DefineBuiltin(i, v.Name)
	}

	// Register class objects in the symbol table and pre-seed globals
	globals := make([]object.Object, vm.GlobalsSize)
	classes := object.CreateClassObjects()
	classNames := []string{"io", "type", "time", "os", "math", "string", "file", "pkg", "array", "sys", "keyboard"}
	for _, className := range classNames {
		if classObj, ok := classes[className]; ok {
			sym := symbolTable.Define(className)
			globals[sym.Index] = classObj
		}
	}

	constants := []object.Object{}

	// Read the file and execute complete statements
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	var currentStatement strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		if len(strings.TrimSpace(line)) == 0 {
			continue
		}

		currentStatement.WriteString(line)

		if !needsContinuation(currentStatement.String()) {
			stmt := currentStatement.String()
			currentStatement.Reset()

			// Handle include inline (keeps existing include behavior)
			if incPath, ok := tryParseInclude(stmt); ok {
				if err := executeInclude(incPath, object.NewEnvironment(), out); err != nil {
					fmt.Fprintf(out, "Include error: %v\n", err)
					return err
				}
				continue
			}

			l := lexer.New(stmt)
			p := parser.New(l)
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				printParserErrors(out, p.Errors())
				return fmt.Errorf("Parsing errors in file %s:\t%v\n", filename, p.Errors())
			}

			// Compile the current statement only into a temporary compiler
			// that shares the global symbol table and current constants.
			tmp := compiler.NewWithState(symbolTable, constants)
			if err := tmp.Compile(program); err != nil {
				return fmt.Errorf("Compilation error in file %s: %v", filename, err)
			}

			// Seed any undefined globals discovered during this statement's
			// compilation so runtime accesses will produce Error objects with
			// file/line info instead of causing unexpected instant exits.
			for idx, e := range tmp.UndefinedGlobals() {
				if e == nil {
					continue
				}

				// If the current statement is a `suppress` wrapping a
				// let-declaration, avoid seeding undefined globals that
				// originated on that same line. This prevents suppressed
				// definitions from causing immediate prints at definition
				// time; the entries will be seeded before the next
				// statement execution (the loop seeds every iteration).
				if ss, ok := program.Statements[0].(*ast.SuppressStatement); ok {
					if ls, ok2 := ss.Statement.(*ast.LetStatement); ok2 {
						if e.Line == ls.Token.Line {
							continue
						}
					}
				}

				if e.Filename == "" {
					e.Filename = filename
				}
				globals[idx] = e
			}

			bytecode := tmp.Bytecode()
			constants = bytecode.Constants

			machine := vm.NewWithGlobalsStore(bytecode, globals)
			if err := machine.Run(); err != nil {
				io.WriteString(out, err.Error()+"\n")
				return err
			}

			// Check if the result is an IncludeDirective
			if last := machine.LastPoppedStackElem(); last != nil {
				if directive, ok := last.(*object.IncludeDirective); ok {
					// Handle the inclusion by parsing and executing the file
					if err := executeIncludeDirective(directive, symbolTable, &constants, globals, filename, out); err != nil {
						fmt.Fprintf(out, "Include error: %v\n", err)
						return err
					}
				} else if last.Type() != object.NULL_OBJ {
					io.WriteString(out, last.Inspect()+"\n")
				}
			}
		} else {
			currentStatement.WriteString("\n")
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("Error reading file %s: %v", filename, err)
	}

	// Handle any remaining statement
	if currentStatement.Len() > 0 {
		stmt := currentStatement.String()
		if incPath, ok := tryParseInclude(stmt); ok {
			if err := executeInclude(incPath, object.NewEnvironment(), out); err != nil {
				fmt.Fprintf(out, "Include error: %v\n", err)
				return err
			}
			return nil
		}

		l := lexer.New(stmt)
		p := parser.New(l)
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			printParserErrors(out, p.Errors())
			return fmt.Errorf("Parsing errors in file %s: %v", filename, p.Errors())
		}

		// Final statement: compile only this statement into a temporary
		// compiler so we don't rerun previously-compiled code.
		tmp := compiler.NewWithState(symbolTable, constants)
		if err := tmp.Compile(program); err != nil {
			return fmt.Errorf("Compilation error in file %s: %v", filename, err)
		}

		bytecode := tmp.Bytecode()
		machine := vm.NewWithGlobalsStore(bytecode, globals)
		if err := machine.Run(); err != nil {
			io.WriteString(out, err.Error()+"\n")
			return err
		}
		if last := machine.LastPoppedStackElem(); last != nil {
			if last.Type() != object.NULL_OBJ {
				io.WriteString(out, last.Inspect()+"\n")
			}
		}
	}

	return nil
}

// executeIncludeDirective handles pkg.include() directives by loading and evaluating a file
// and registering its functions in the symbol table and globals
func executeIncludeDirective(directive *object.IncludeDirective, symbolTable *compiler.SymbolTable, constants *[]object.Object, globals []object.Object, caller string, out io.Writer) error {
	// Resolve the include filename relative to the caller and common locations
	// Normalize path separators in the directive filename so relative joins work
	normalized := filepath.Clean(strings.ReplaceAll(directive.Filename, "\\", string(os.PathSeparator)))
	candidates := []string{normalized}
	// Try relative to caller's directory
	if caller != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(caller), normalized))
		candidates = append(candidates, filepath.Join(filepath.Dir(caller), "lib", normalized))
	}
	candidates = append(candidates, filepath.Join("lib", normalized))

	var content []byte
	var err error
	var chosen string
	for _, c := range candidates {
		if fi, statErr := os.Stat(c); statErr == nil && !fi.IsDir() {
			chosen = c
			break
		}
	}
	if chosen == "" {
		return fmt.Errorf("Failed to read include file '%s': file not found", directive.Filename)
	}

	content, err = os.ReadFile(chosen)
	if err != nil {
		return fmt.Errorf("Failed to read include file '%s': %v", chosen, err)
	}

	// Parse the file
	l := lexer.New(string(content))
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) != 0 {
		return fmt.Errorf("Parse errors in '%s': %v", directive.Filename, p.Errors())
	}

	// Create a new environment for the included file
	// Seed it with all class objects and builtins
	includeEnv := object.NewEnvironment()
	classes := object.CreateClassObjects()
	for className, classObj := range classes {
		includeEnv.Set(className, classObj)
	}

	// Evaluate the program in this new environment
	evalResult := evaluator.Eval(program, includeEnv)
	if evalResult != nil && evalResult.Type() == object.ERROR_OBJ {
		return fmt.Errorf("Evaluation error in '%s': %v", directive.Filename, evalResult)
	}

	// Extract all functions and variables from the environment and create a namespace Hash
	nsHash := &object.Hash{Pairs: make(map[object.HashKey]object.HashPair)}

	storeContents := includeEnv.GetStore()
	for name, obj := range storeContents {
		// Skip built-in classes - only include user-defined objects
		if _, isBuiltin := classes[name]; isBuiltin {
			continue
		}

		// Include Functions and Builtins (match REPL behavior)
		switch obj.(type) {
		case *object.Function, *object.Builtin, *object.Closure:
			key := &object.String{Value: name}
			nsHash.Pairs[key.HashKey()] = object.HashPair{Key: key, Value: obj}
		}
	}

	// Register the namespace as a variable in the symbol table and globals
	ns := symbolTable.Define(directive.Namespace)
	if ns.Index < len(globals) {
		globals[ns.Index] = nsHash
	}

	// Also register the namespace globally so sys.list() can find it
	object.RegisterNamespace(directive.Namespace, nsHash)

	return nil
}
