package vm

import (
	"squ1d++/compiler"
	"squ1d++/lexer"
	"squ1d++/object"
	"squ1d++/parser"
	"testing"
)

func TestErrorPipeAssignsErrorToVariable(t *testing.T) {
	input := `var x = def() { return y }; var z = << x(); z`

	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	comp := compiler.New()
	if err := comp.Compile(prog); err != nil {
		t.Fatalf("compiler error: %s", err)
	}

	vm := New(comp.Bytecode())
	if err := vm.Run(); err != nil {
		t.Fatalf("vm error: %s", err)
	}

	last := vm.LastPoppedStackElem()
	_, ok := last.(*object.Error)
	if !ok {
		t.Fatalf("expected last popped to be Error, got %T (%+v)", last, last)
	}
}

func TestUnblockLetSwallowsErrorInVM(t *testing.T) {
	// Unblock without error pipe should swallow errors and assign null
	input := `var func = def() { return y }; unblock var x = func(); x`

	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	comp := compiler.New()
	if err := comp.Compile(prog); err != nil {
		t.Fatalf("compiler error: %s", err)
	}

	vm := New(comp.Bytecode())
	if err := vm.Run(); err != nil {
		t.Fatalf("vm error: %s", err)
	}

	last := vm.LastPoppedStackElem()
	if last != Null {
		t.Fatalf("expected last popped to be Null after unblock, got %T (%+v)", last, last)
	}
}

func TestUnblockWithErrorPipeAssignsErrorInVM(t *testing.T) {
	// When both unblock and error pipe are present, error pipe takes precedence
	input := `var func = def() { return y }; unblock var y = << func(); y`

	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	comp := compiler.New()
	if err := comp.Compile(prog); err != nil {
		t.Fatalf("compiler error: %s", err)
	}

	vm := New(comp.Bytecode())
	if err := vm.Run(); err != nil {
		t.Fatalf("vm error: %s", err)
	}

	last := vm.LastPoppedStackElem()
	_, ok := last.(*object.Error)
	if !ok {
		t.Fatalf("expected last popped to be Error, got %T (%+v)", last, last)
	}
}
