package include

import (
	"fmt"
	"os"
	"path/filepath"
	"squ1d++/compiler"
	"squ1d++/lexer"
	"squ1d++/object"
	"squ1d++/parser"
	"squ1d++/vm"
	"strings"
)

type Loader struct {
	loadedFiles map[string]bool
	searchPaths []string
}

func NewLoader() *Loader {
	homeDir, _ := os.UserHomeDir()
	packageDir := filepath.Join(homeDir, ".cache", "squ1dlang")

	return &Loader{
		loadedFiles: make(map[string]bool),
		searchPaths: []string{".", "./lib", "./packages", packageDir},
	}
}

// AddSearchPath adds a directory to the search path for packages
func (l *Loader) AddSearchPath(path string) {
	l.searchPaths = append(l.searchPaths, path)
}

// LoadFile loads and executes a SQU1D++ file
func (l *Loader) LoadFile(filename string, symbolTable *compiler.SymbolTable, constants interface{}, globals interface{}) (interface{}, error) {
	// Resolve the file path
	resolvedPath, err := l.resolvePath(filename)
	if err != nil {
		return nil, err
	}

	// Check if already loaded to prevent circular includes
	if l.loadedFiles[resolvedPath] {
		return constants, nil
	}

	// Mark as loaded
	l.loadedFiles[resolvedPath] = true

	// Read file content
	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("could not read file '%s': %v", resolvedPath, err)
	}

	// Parse and execute the file
	return l.executeContent(string(content), symbolTable, constants, globals)
}

// resolvePath resolves a file path, checking search paths and extensions
func (l *Loader) resolvePath(filename string) (string, error) {
	// If it's an absolute path or already has .sqd extension, use as-is
	if filepath.IsAbs(filename) || strings.HasSuffix(filename, ".sqd") {
		if _, err := os.Stat(filename); err == nil {
			return filename, nil
		}
		return "", fmt.Errorf("file '%s' not found", filename)
	}

	// Try to find the file in search paths
	for _, searchPath := range l.searchPaths {
		// Try with .sqd extension
		fullPath := filepath.Join(searchPath, filename+".sqd")
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath, nil
		}

		// Try as directory with __init__.sqd
		initPath := filepath.Join(searchPath, filename, "__init__.sqd")
		if _, err := os.Stat(initPath); err == nil {
			return initPath, nil
		}
	}

	return "", fmt.Errorf("file or package '%s' not found in search paths", filename)
}

// executeContent parses and executes SQU1D++ code content
func (l *Loader) executeContent(content string, symbolTable *compiler.SymbolTable, constants interface{}, globals interface{}) (interface{}, error) {
	// Parse the content
	lex := lexer.New(content)
	parser := parser.New(lex)
	program := parser.ParseProgram()

	if len(parser.Errors()) != 0 {
		return nil, fmt.Errorf("parsing errors: %v", parser.Errors())
	}

	// Compile the program
	comp := compiler.NewWithState(symbolTable, constants.([]object.Object))
	err := comp.Compile(program)
	if err != nil {
		return nil, fmt.Errorf("compilation error: %v", err)
	}

	// Execute the program
	code := comp.Bytecode()
	machine := vm.NewWithGlobalsStore(code, globals.([]object.Object))

	err = machine.Run()
	if err != nil {
		return nil, fmt.Errorf("runtime error: %v", err)
	}

	return code.Constants, nil
}

// GlobalLoader is the global loader instance
var GlobalLoader = NewLoader()
