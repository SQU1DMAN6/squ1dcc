package vm

import (
	"fmt"
	"squ1d++/ast"
	"squ1d++/compiler"
	"squ1d++/lexer"
	"squ1d++/object"
	"squ1d++/parser"
	"testing"
)

func parse(input string) *ast.Program {
	l := lexer.New(input)
	p := parser.New(l)
	return p.ParseProgram()
}

func testIntegerObject(expected int64, actual object.Object) error {
	result, ok := actual.(*object.Integer)
	if !ok {
		return fmt.Errorf("Object is not Integer. Got %T (%+v)",
			actual, actual)
	}

	if result.Value != expected {
		return fmt.Errorf("Object has wrong value. Expected %d, got %d",
			expected, result.Value)
	}

	return nil
}

func testBooleanObject(expected bool, actual object.Object) error {
	result, ok := actual.(*object.Boolean)
	if !ok {
		return fmt.Errorf("Object is not Boolean. Got %T (%+v)",
			actual, actual)
	}

	if result.Value != expected {
		return fmt.Errorf("Object has wrong value. Expected %t, got %t",
			expected, result.Value)
	}

	return nil
}

func testStringObject(expected string, actual object.Object) error {
	result, ok := actual.(*object.String)
	if !ok {
		return fmt.Errorf("Object is not String. Got %T (%+v)", actual, actual)
	}

	if result.Value != expected {
		return fmt.Errorf("Object has wrong value. Expected %q, got %q",
			expected, result.Value)
	}

	return nil
}

func TestArrayLiterals(t *testing.T) {
	tests := []vmTestCase{
		{"[]", []int{}},
		{"[1, 2, 3]", []int{1, 2, 3}},
		{"[1 + 2, 3 * 4, 5 + 6]", []int{3, 12, 11}},
	}

	runVmTests(t, tests)
}

func TestHashLiterals(t *testing.T) {
	tests := []vmTestCase{
		{
			"{}", map[object.HashKey]int64{},
		},
		{
			"{1: 2, 2: 3}",
			map[object.HashKey]int64{
				(&object.Integer{Value: 1}).HashKey(): 2,
				(&object.Integer{Value: 2}).HashKey(): 3,
			},
		},
		{
			"{1 + 1: 2 * 2, 3 + 3: 4 * 4}",
			map[object.HashKey]int64{
				(&object.Integer{Value: 2}).HashKey(): 4,
				(&object.Integer{Value: 6}).HashKey(): 16,
			},
		},
	}

	runVmTests(t, tests)
}

func TestIndexExpressions(t *testing.T) {
	tests := []vmTestCase{
		{"[1, 2, 3][1]", 2},
		{"[1, 2, 3][0 + 2]", 3},
		{"[[1, 1, 1]][0][0]", 1},
		{"[][0]", Null},
		{"[1, 2, 3][99]", Null},
		{"[1][-1]", Null},
		{"{1: 1, 2: 2}[1]", 1},
		{"{1: 1, 2: 2}[2]", 2},
		{"{1: 1}[0]", Null},
		{"{}[0]", Null},
	}

	runVmTests(t, tests)
}

func runVmTests(t *testing.T, tests []vmTestCase) {
	t.Helper()

	for _, tt := range tests {
		program := parse(tt.input)

		comp := compiler.New()
		err := comp.Compile(program)
		if err != nil {
			t.Fatalf("Compiler error: %s", err)
		}

		vm := New(comp.Bytecode())
		err = vm.Run()
		if err != nil {
			t.Fatalf("Vm error: %s", err)
		}

		stackElem := vm.LastPoppedStackElem()

		testExpectedObject(t, tt.expected, stackElem)
	}
}

func testExpectedObject(
	t *testing.T,
	expected interface{},
	actual object.Object,
) {
	t.Helper()

	switch expected := expected.(type) {
	case int:
		err := testIntegerObject(int64(expected), actual)
		if err != nil {
			t.Errorf("testIntegerObject failed: %s", err)
		}

	case bool:
		err := testBooleanObject(bool(expected), actual)
		if err != nil {
			t.Errorf("testBooleanObject failed: %s", err)
		}

	case string:
		err := testStringObject(expected, actual)
		if err != nil {
			t.Errorf("testStringObject failed: %s", err)
		}

	case []int:
		array, ok := actual.(*object.Array)
		if !ok {
			t.Errorf("Object is not Array: Got %T (%+v)", actual, actual)
			return
		}

		if len(array.Elements) != len(expected) {
			t.Errorf("Wrong number of elements. Expected %d, got %d",
				len(expected), len(array.Elements))
			return
		}

		for i, expectedElem := range expected {
			err := testIntegerObject(int64(expectedElem), array.Elements[i])
			if err != nil {
				t.Errorf("testIntegerObject failed: %s", err)
			}
		}

	case map[object.HashKey]int64:
		hash, ok := actual.(*object.Hash)
		if !ok {
			t.Errorf("Object is not Hash. Got %T (%+v)", actual, actual)
			return
		}

		if len(hash.Pairs) != len(expected) {
			t.Errorf("Hash has wrong number of Pairs. Expected %d, got %d",
				len(expected), len(hash.Pairs))
			return
		}

		for expectedKey, expectedValue := range expected {
			pair, ok := hash.Pairs[expectedKey]
			if !ok {
				t.Errorf("No pair for given key in Pairs")
			}

			err := testIntegerObject(expectedValue, pair.Value)
			if err != nil {
				t.Errorf("testIntegerObject failed: %s", err)
			}
		}

	case *object.Null:
		if actual != Null {
			t.Errorf("Object is not Null. Got %T (%+v)", actual, actual)
		}

	case *object.Error:
		errObj, ok := actual.(*object.Error)
		if !ok {
			t.Errorf("Object is not an Error. Got %T (%+v)", actual, actual)
			return
		}

		if errObj.Message != expected.Message {
			t.Errorf("Wrong error message. Expected %q, got %q",
				expected.Message, errObj.Message)
		}
	}
}

func TestIntegerArithmetic(t *testing.T) {
	tests := []vmTestCase{
		{"1", 1},
		{"2", 2},
		{"1 + 2", 3},
		{"1 - 2", -1},
		{"1 * 2", 2},
		{"4 / 2", 2},
		{"50 / 2 * 2 + 10 - 5", 55},
		{"5 + 5 + 5 + 5 - 10", 10},
		{"2 * 2 * 2 * 2 * 2", 32},
		{"5 * 2 + 10", 20},
		{"5 + 2 * 10", 25},
		{"5 * (2 + 10)", 60},
		{"-5", -5},
		{"-10", -10},
		{"-50 + 100 + -50", 0},
		{"(5 + 10 * 2 + 15 / 3) * 2 + -10", 50},
	}

	runVmTests(t, tests)
}

func TestFloatArithmetic(t *testing.T) {
	tests := []vmTestCase{
		{"'1", float64(1)},
		{"'1.123", float64(1.123)},
		{"'12.69", float64(12.69)},
		{"'1.40000", float64(1.4)},
		{"'1 + '1", float64(2)},
		{"'3 - '2", float64(1)},
		{"'2.5 * '2", float64(5)},
		{"'7 / '2", float64(3.5)},
	}

	runVmTests(t, tests)
}

func TestBooleanExpressions(t *testing.T) {
	tests := []vmTestCase{
		{"true", true},
		{"false", false},
		{"1 < 2", true},
		{"1 > 2", false},
		{"1 < 1", false},
		{"1 > 1", false},
		{"1 == 1", true},
		{"1 != 1", false},
		{"1 == 2", false},
		{"1 != 2", true},
		{"\"hello\" == \"hello\"", true},
		{"\"hello\" != \"hello\"", false},
		{"true == true", true},
		{"false == false", true},
		{"true == false", false},
		{"true != false", true},
		{"false != true", true},
		{"(1 < 2) == true", true},
		{"(1 < 2) == false", false},
		{"(1 > 2) == true", false},
		{"(1 > 2) == false", true},
		{"!true", false},
		{"!false", true},
		{"!5", false},
		{"!!true", true},
		{"!!false", false},
		{"!!5", true},
		{"!(if (false) { 5; })", true},
	}

	runVmTests(t, tests)
}

func TestStringExpressions(t *testing.T) {
	tests := []vmTestCase{
		{`"monkey"`, "monkey"},
		{`"mon" + "key"`, "monkey"},
		{`"mon" + "key" + "banana"`, "monkeybanana"},
	}

	runVmTests(t, tests)
}

func TestConditionals(t *testing.T) {
	tests := []vmTestCase{
		{"if (true) { 10 }", 10},
		{"if (true) { 10 } el { 20 }", 10},
		{"if (false) { 10 } el { 20 }", 20},
		{"if (1) { 10 }", 10},
		{"if (1 < 2) { 10 }", 10},
		{"if (1 < 2) { 10 } el { 20 }", 10},
		{"if (1 > 2) { 10 } el { 20 }", 20},
		{"if (1 > 2) { 10 }", Null},
		{"if (false) { 10 }", Null},
		{"if ((if (false) { 10 })) { 10 } el { 20 }", 20},
	}

	runVmTests(t, tests)
}

func TestGlobalLetStatements(t *testing.T) {
	tests := []vmTestCase{
		{"var one = 1; one", 1},
		{"var one = 1; var two = 2; one + two", 3},
		{"var one = 1; var two = one + one; one + two", 3},
	}

	runVmTests(t, tests)
}

func TestCallingFunctionsWithoutArguments(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `
			var fiveplusten = def() { 5 + 10; };
			fiveplusten()
			`,
			expected: 15,
		},
		{
			input: `
var one = def() { 1; };
var two = def() { 2; };
one() + two()
`,
			expected: 3,
		},
		{
			input: `
var a = def() { 1 };
var b = def() { a() + 1 };
var c = def() { b() + 1 };
c();
`,
			expected: 3,
		},
		{
			input: `
			var returnsone = def() { 1; };
			var returnsonereturner = def() { returnsone; };
			returnsonereturner()();
			`,
			expected: 1,
		},
	}

	runVmTests(t, tests)
}

// I see the file got some errors and I know you are here.
func TestFunctionsWithReturnStatement(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `
			var earlyExit = def() { return 99; 100; };
			earlyExit();
			`,
			expected: 99,
		},
		{
			input: `
			var earlyExit = def() { return 99; return 100; };
			earlyExit();
			`,
			expected: 99,
		},
	}

	runVmTests(t, tests)
}

func TestFunctionsWithoutReturnValue(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `
			var noreturn = def() {};
			noreturn();
			`,
			expected: Null,
		},
		{
			input: `
			var noreturn = def() {};
			var noreturntwo = def() { noreturn(); };
			noreturn();
			noreturntwo();
			`,
			expected: Null,
		},
	}

	runVmTests(t, tests)
}

func TestCallingFunctionsWithBindings(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `
			var one = def() {
				var one = 1;
				one;
			}
			one();
			`,
			expected: 1,
		},
		{
			input: `
var oneAndTwo = def() { var one = 1; var two = 2; one + two; };
oneAndTwo();
`,
			expected: 3,
		},
		{
			input: `
var oneAndTwo = def() { var one = 1; var two = 2; one + two; };
var threeAndFour = def() { var three = 3; var four = 4; three + four; };
oneAndTwo() + threeAndFour();
`,
			expected: 10,
		},
		{
			input: `
var firstFoobar = def() { var foobar = 50; foobar; };
var secondFoobar = def() { var foobar = 100; foobar; };
firstFoobar() + secondFoobar();
`,
			expected: 150,
		},
		{
			input: `
var globalSeed = 50;
var minusOne = def() {
var num = 1;
globalSeed - num;
}
var minusTwo = def() {
var num = 2;
globalSeed - num;
}
minusOne() + minusTwo();
`,
			expected: 97,
		},
	}

	runVmTests(t, tests)
}

func TestCallingFunctionsWithArgumentsAndBindings(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `
			var identity = def(a) { a; };
			identity(4);
			`,
			expected: 4,
		},
		{
			input: `
			var sum = def(a, b) { a + b; };
			sum(1, 2);
			`,
			expected: 3,
		},
		{
			input: `
var sum = def(a, b) {
	var c = a + b;
	c;
};
sum(1, 2);
`,
			expected: 3,
		},
		{
			input: `
var sum = def(a, b) {
	var c = a + b;
	c;
};
sum(1, 2) + sum(3, 4);`,
			expected: 10,
		},
		{
			input: `
var sum = def(a, b) {
	var c = a + b;
	c;
};
var outer = def() {
	sum(1, 2) + sum(3, 4);
};
outer();
`,
			expected: 10,
		},
	}

	runVmTests(t, tests)
}

func TestCallingFunctionsWithWrongArguments(t *testing.T) {
	tests := []vmTestCase{
		{
			input:    `def() { 1; }(1)`,
			expected: `Wrong number of arguments. Expected 0, got 1`,
		},
		{
			input:    `def(a) { a; }();`,
			expected: `Wrong number of arguments. Expected 1, got 0`,
		},
		{
			input:    `def(a, b) { a + b; }(1);`,
			expected: `Wrong number of arguments. Expected 2, got 1`,
		},
	}

	for _, tt := range tests {
		program := parse(tt.input)

		comp := compiler.New()
		err := comp.Compile(program)
		if err != nil {
			t.Fatalf("Compiler error: %s", err)
		}

		vm := New(comp.Bytecode())
		err = vm.Run()
		if err == nil {
			t.Fatalf("Expected VM error but resulted in none.")
		}

		if err.Error() != tt.expected {
			t.Fatalf("Wrong VM error: Expected %q, got %q",
				tt.expected, err)
		}
	}
}

func TestClosures(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `
			var newClosure = def(a) {
				def() { a; };
			};
			var closure = newClosure(99)
			closure();
			`,
			expected: 99,
		},
		{
			input: `
			var newAdder = def(a, b) {
				def(c) { a + b + c };
			}
			var adder = newAdder(1, 2);
			adder(8);
			`,
			expected: 11,
		},
		{
			input: `
			var newAdder = def(a, b) {
				var c = a + b
				def(d) { c + d }
			}
			var adder = newAdder(1, 2)
			adder(8)
			`,
			expected: 11,
		},
		{
			input: `
var newAdderOuter = def(a, b) {
var c = a + b;
def(d) {
var e = d + c;
def(f) { e + f; };
};
};
var newAdderInner = newAdderOuter(1, 2)
var adder = newAdderInner(3);
adder(8);
`,
			expected: 14,
		},
		{
			input: `
var a = 1;
var newAdderOuter = def(b) {
def(c) {
def(d) { a + b + c + d };
};
};
var newAdderInner = newAdderOuter(2)
var adder = newAdderInner(3);
adder(8);
`,
			expected: 14,
		},
		{
			input: `
var newClosure = def(a, b) {
var one = def() { a; };
var two = def() { b; };
def() { one() + two(); };
};
var closure = newClosure(9, 90);
closure();
`,
			expected: 99,
		},
	}

	runVmTests(t, tests)
}

func TestRecursiveFunctions(t *testing.T) {
	tests := []vmTestCase{
		{
			input: `
			var countdown = def(x) {
				if (x == 0) {
					return 0;
				} el {
					countdown(x - 1)
				}
			}
			countdown(9);
			`,
			expected: 0,
		},
		{
			input: `
			var wrapper = def() {
				var countdown = def(x) {
					if (x == 0) {
						return 0
					} el {
						countdown(x - 1)
					}
				}
				countdown(9)
			}
			wrapper()
			`,
			expected: 0,
		},
	}

	runVmTests(t, tests)
}

func TestBuiltinFunctions(t *testing.T) {
	tests := []vmTestCase{
		{`cat("")`, 0},
		{`cat("four")`, 4},
		{`cat("Hello, World!")`, 13},
		{`cat([1, 2, 3])`, 3},
		{`cat([])`, 0},
		{
			`cat("one", "two")`,
			&object.Error{
				Message: "Wrong number of arguments. Expected 1, got 2",
			},
		},
		{
			`cat(1)`,
			&object.Error{
				Message: "Argument 0 to `cat` is not supported, got INTEGER",
			},
		},
		{
			`append([], 1)`, []int{1},
		},
		{
			`append(1, 1)`,
			&object.Error{
				Message: "Argument 0 to `append` must be ARRAY, got INTEGER",
			},
		},
	}

	runVmTests(t, tests)
}

type vmTestCase struct {
	input    string
	expected interface{}
}
