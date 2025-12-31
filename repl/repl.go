package repl

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/user"
	"squ1d++/evaluator"
	"squ1d++/lexer"
	"squ1d++/object"
	"squ1d++/parser"
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

	// Create a new environment for file execution
	env := object.NewEnvironment()
	classes := object.CreateClassObjects()
	for name, obj := range classes {
		env.Set(name, obj)
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
				if err := executeInclude(incPath, env, out); err != nil {
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

			evaluated := evaluator.Eval(program, env)
			if evaluated != nil {
				if evaluated.Type() == object.ERROR_OBJ {
					if errObj, ok := evaluated.(*object.Error); ok {
						if errObj.Filename == "" {
							errObj.Filename = filename
						}
						if errObj.Line == 0 {
							errObj.Line = 1
						}
						if errObj.Column == 0 {
							errObj.Column = 1
						}
						io.WriteString(out, errObj.Inspect())
						io.WriteString(out, "\n")
					}
					return fmt.Errorf("Runtime error in file %s:\t%s\n", filename, evaluated.Inspect())
				}
				if evaluated.Type() != object.NULL_OBJ {
					io.WriteString(out, evaluated.Inspect())
					io.WriteString(out, "\n")
				}
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
			if err := executeInclude(incPath, env, out); err != nil {
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

		evaluated := evaluator.Eval(program, env)
		if evaluated != nil {
			if evaluated.Type() == object.ERROR_OBJ {
				if errObj, ok := evaluated.(*object.Error); ok {
					if errObj.Filename == "" {
						errObj.Filename = filename
					}
					if errObj.Line == 0 {
						errObj.Line = 1
					}
					if errObj.Column == 0 {
						errObj.Column = 1
					}
					io.WriteString(out, errObj.Inspect())
					io.WriteString(out, "\n")
				}
				return fmt.Errorf("Runtime error in file %s: %s", filename, evaluated.Inspect())
			}
			if evaluated.Type() != object.NULL_OBJ {
				io.WriteString(out, evaluated.Inspect())
				io.WriteString(out, "\n")
			}
		}
	}

	return nil
}
