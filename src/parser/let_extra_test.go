package parser

import (
	"squ1d++/ast"
	"squ1d++/lexer"
	"testing"
)

func TestUnblockLetStatement(t *testing.T) {
	input := "unblock var x = 5;"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. Got %d", len(program.Statements))
	}

	stmt := program.Statements[0]
	letStmt, ok := stmt.(*ast.LetStatement)
	if !ok {
		t.Fatalf("stmt not *ast.LetStatement. Got %T", stmt)
	}

	if !letStmt.Unblock {
		t.Errorf("expected Unblock to be true")
	}

	// value should be 5
	if il, ok := letStmt.Value.(*ast.IntegerLiteral); !ok || il.Value != 5 {
		t.Fatalf("letStmt.Value not integer 5. Got %T", letStmt.Value)
	}
}

func TestErrorPipeLetStatement(t *testing.T) {
	input := "var y = << func();"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. Got %d", len(program.Statements))
	}

	stmt := program.Statements[0]
	letStmt, ok := stmt.(*ast.LetStatement)
	if !ok {
		t.Fatalf("stmt not *ast.LetStatement. Got %T", stmt)
	}

	if !letStmt.ErrorPipe {
		t.Errorf("expected ErrorPipe to be true")
	}

	_, ok = letStmt.Value.(*ast.CallExpression)
	if !ok {
		t.Errorf("expected Value to be CallExpression; got %T", letStmt.Value)
	}
}
