package evaluator

import (
	"squ1d++/lexer"
	"squ1d++/object"
	"squ1d++/parser"
	"testing"
)

func TestUnblockLetSwallowsError(t *testing.T) {
	input := `var func = def() { return y }
	unblock var x = func()
	5`

	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	env := object.NewEnvironment()
	evaluated := Eval(program, env)

	if evaluated == nil {
		t.Fatalf("expected a result, got nil")
	}

	if evaluated.Type() != object.INTEGER_OBJ {
		t.Fatalf("expected integer result, got %s", evaluated.Type())
	}

	if evaluated.(*object.Integer).Value != 5 {
		t.Fatalf("expected 5, got %d", evaluated.(*object.Integer).Value)
	}

	val, ok := env.Get("x")
	if !ok {
		t.Fatalf("expected variable x to be set")
	}

	if val.Type() != object.NULL_OBJ {
		t.Fatalf("expected x to be null, got %s", val.Type())
	}
}

func TestErrorPipeAssignsError(t *testing.T) {
	input := `var func = def() { return y }
	unblock var y = << func()
	5`

	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	env := object.NewEnvironment()
	evaluated := Eval(program, env)

	if evaluated == nil {
		t.Fatalf("expected a result, got nil")
	}

	if evaluated.Type() != object.INTEGER_OBJ {
		t.Fatalf("expected integer result, got %s", evaluated.Type())
	}

	if evaluated.(*object.Integer).Value != 5 {
		t.Fatalf("expected 5, got %d", evaluated.(*object.Integer).Value)
	}

	val, ok := env.Get("y")
	if !ok {
		t.Fatalf("expected variable y to be set")
	}

	if val.Type() != object.ERROR_OBJ {
		t.Fatalf("expected y to be Error object, got %s", val.Type())
	}
}
