package builder

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"squ1d++/bytecode"
	"squ1d++/compiler"
	"squ1d++/lexer"
	"squ1d++/parser"
	"strings"
)

// BuildStandalone compiles a SQU1D++ file to a standalone executable
// that requires no external dependencies or project files
func BuildStandalone(inputFile, outputFile string) error {
	// Read and compile the source code
	source, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("could not read input file: %v", err)
	}

	baseDir := filepath.Dir(inputFile)

	// Expand includes inline
	expandedCode, err := expandIncludes(string(source), baseDir)
	if err != nil {
		return fmt.Errorf("include expansion error: %v", err)
	}

	// Pre-process pkg.include() calls to build namespace objects
	// This would evaluate included files and prepare globals.
	// For now, we keep pkg.include() for REPL mode only
	modifiedCode, err := processPkgIncludes(expandedCode, baseDir)
	if err != nil {
		return fmt.Errorf("include processing error: %v", err)
	}

	// Parse and compile the modified code
	compiledCode, err := compileSourceWithNamespaces(modifiedCode)
	if err != nil {
		return fmt.Errorf("compilation error: %v", err)
	}

	// Serialize bytecode
	pkg := &bytecode.Package{
		Version:      bytecode.VERSION,
		Instructions: compiledCode.Instructions,
		Constants:    compiledCode.Constants,
	}

	// Create bytecode data
	tempFile := filepath.Join(filepath.Dir(outputFile), ".squ1d_temp.byc")
	bcFile, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("could not create bytecode file: %v", err)
	}
	defer os.Remove(tempFile)
	defer bcFile.Close()

	if err := pkg.Serialize(bcFile); err != nil {
		return fmt.Errorf("could not serialize bytecode: %v", err)
	}

	bcFile.Close()

	// Read bytecode as hex for embedding
	bcData, err := os.ReadFile(tempFile)
	if err != nil {
		return fmt.Errorf("could not read bytecode: %v", err)
	}

	// Generate standalone Go runtime
	generatedGo := fmt.Sprintf(`package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"squ1d++/bytecode"
	"squ1d++/vm"
	"squ1d++/object"
	"squ1d++/compiler"
)

const bytecodeHex = %q

func main() {
	// Decode bytecode
	bcData, err := hex.DecodeString(bytecodeHex)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Internal error: invalid bytecode: %%v\n", err)
		os.Exit(1)
	}

	// Deserialize bytecode package
	pkg, err := bytecode.Deserialize(bytes.NewReader(bcData))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Internal error: could not load program: %%v\n", err)
		os.Exit(1)
	}

	// Initialize VM state
	globals := make([]object.Object, vm.GlobalsSize)
	symbolTable := compiler.NewSymbolTable()
	
	// Register builtins
	for i, v := range object.Builtins {
		symbolTable.DefineBuiltin(i, v.Name)
	}

	// Register class objects
	classes := object.CreateClassObjects()
	for name, classObj := range classes {
		sym := symbolTable.Define(name)
		globals[sym.Index] = classObj
	}

	// Create and run VM
	machine := vm.NewWithGlobalsStore(&compiler.Bytecode{
		Instructions: pkg.Instructions,
		Constants:    pkg.Constants,
	}, globals)

	if err := machine.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Runtime error: %%s\n", err)
		os.Exit(1)
	}

	// Print result if any
	lastPopped := machine.LastPoppedStackElem()
	if lastPopped != nil && lastPopped.Type() != object.NULL_OBJ {
		fmt.Println(lastPopped.Inspect())
	}
}
`, hex.EncodeToString(bcData))

	// Get the project root (where go.mod exists)
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("could not get working directory: %v", err)
	}

	// Create a temporary Go file to compile (in a temp dir)
	buildDir, err := os.MkdirTemp("", "squ1d_build_")
	if err != nil {
		return fmt.Errorf("could not create build directory: %v", err)
	}
	defer os.RemoveAll(buildDir)

	tempGoFile := filepath.Join(buildDir, "main.go")

	if err := os.WriteFile(tempGoFile, []byte(generatedGo), 0644); err != nil {
		return fmt.Errorf("could not write generated code: %v", err)
	}

	// Get absolute path for output
	absOutputFile, err := filepath.Abs(outputFile)
	if err != nil {
		return fmt.Errorf("could not resolve output path: %v", err)
	}

	// Compile with go build from the PROJECT ROOT directory
	// This ensures go.mod is found and can resolve packages
	cmd := exec.Command("go", "build", "-o", absOutputFile, tempGoFile)
	cmd.Dir = projectRoot // Run from project root so go.mod is found
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("compilation failed: %v", err)
	}

	return nil
}

// compileSource parses and compiles SQU1D++ source code
func compileSource(source string) (*compiler.Bytecode, error) {
	l := lexer.New(source)
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		return nil, fmt.Errorf("parse error: %v", p.Errors())
	}

	comp := compiler.New()
	if err := comp.Compile(program); err != nil {
		return nil, err
	}

	return comp.Bytecode(), nil
}

// expandIncludes recursively expands pkg.include() calls
func expandIncludes(code string, baseDir string) (string, error) {
	var result []string
	scanner := bufio.NewScanner(strings.NewReader(code))

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Check for pkg.include() calls
		if strings.Contains(trimmed, "pkg.include(") {
			// Extract filename from include
			// Simple parser for pkg.include("filename") or pkg.include("filename", "namespace")
			startIdx := strings.Index(trimmed, `"`)
			if startIdx == -1 {
				result = append(result, line)
				continue
			}

			endIdx := strings.Index(trimmed[startIdx+1:], `"`)
			if endIdx == -1 {
				result = append(result, line)
				continue
			}

			filename := trimmed[startIdx+1 : startIdx+1+endIdx]

			// Try to find the file
			candidates := []string{
				filename,
				filepath.Join(baseDir, filename),
				filepath.Join("lib", filename),
			}

			var found string
			for _, candidate := range candidates {
				if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
					found = candidate
					break
				}
			}

			if found == "" {
				// If not found, keep the original line (runtime will handle it)
				result = append(result, line)
				continue
			}

			// Read and recursively expand the included file
			includedCode, err := os.ReadFile(found)
			if err != nil {
				return "", fmt.Errorf("could not read include file %s: %v", found, err)
			}

			expandedInclude, err := expandIncludes(string(includedCode), filepath.Dir(found))
			if err != nil {
				return "", fmt.Errorf("error expanding include %s: %v", found, err)
			}

			// Check if a namespace was provided in the include call
			// Look for a second string argument after the filename
			rest := trimmed[startIdx+1+endIdx+1:]
			ns := ""
			if idx := strings.Index(rest, `"`); idx != -1 {
				idx2 := strings.Index(rest[idx+1:], `"`)
				if idx2 != -1 {
					ns = rest[idx+1 : idx+1+idx2]
				}
			}

			if ns == "" {
				// No namespace requested — inline the expanded include
				result = append(result, expandedInclude)
				continue
			}

			// Build a namespaced wrapper that executes the included file in a local
			// function scope and returns an object/hash containing exported symbols.
			exported := findTopLevelVars(expandedInclude)

			wrapper := "var " + ns + " = (def() {\n"
			wrapper += expandedInclude + "\n"
			wrapper += "return {"
			for i, name := range exported {
				if i > 0 {
					wrapper += ","
				}
				wrapper += name + ": " + name
			}
			wrapper += "}\n})()"

			result = append(result, wrapper)
			continue
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n"), scanner.Err()
}

// processPkgIncludes extracts pkg.include() directives and tracks imported libraries
func processPkgIncludes(code string, baseDir string) (string, error) {
	_ = baseDir // used for resolving library paths

	// For now, in standalone builds we inline pkg.include() calls
	// The bytecode will contain the expanded code, so pkg.include()
	// namespaces won't be available at runtime in compiled mode
	// (They work in REPL/evaluation mode)
	return code, nil
}

// compileSourceWithNamespaces compiles code
func compileSourceWithNamespaces(source string) (*compiler.Bytecode, error) {
	l := lexer.New(source)
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		return nil, fmt.Errorf("parse error: %v", p.Errors())
	}

	comp := compiler.New()

	if err := comp.Compile(program); err != nil {
		return nil, err
	}

	return comp.Bytecode(), nil
}

// findLibrary searches for a library file in standard locations
func findLibrary(libPath string, baseDir string) string {
	candidates := []string{
		libPath,
		filepath.Join(baseDir, libPath),
		filepath.Join("lib", libPath),
		filepath.Join("libraries", libPath),
	}

	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}

// findTopLevelVars returns a list of top-level variable names declared in the
// provided SQU1D++ source. It looks for lines like `var name =`.
func findTopLevelVars(src string) []string {
	re := regexp.MustCompile(`(?m)^\s*var\s+([A-Za-z_][A-Za-z0-9_]*)\s*=`)
	matches := re.FindAllStringSubmatch(src, -1)
	names := make([]string, 0, len(matches))
	seen := make(map[string]bool)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		name := m[1]
		if !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}
	return names
}
