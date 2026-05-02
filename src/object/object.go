package object

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"squ1d++/ast"
	"squ1d++/code"
	"strings"
	"sync"
)

// Tunable runtime limits. Users can override these by adjusting these globals
// before executing long-running programs.
var (
	SysMaxStackSize        = 65536
	SysMaxLoopIterations   = 1000000
	SysMaxInstructionCount = 10000000
)

var arrayPool = sync.Pool{New: func() interface{} { return &Array{} }}
var hashPool = sync.Pool{New: func() interface{} { return &Hash{Pairs: make(map[HashKey]HashPair)} }}

type BuiltinFunction func(args ...Object) Object

type ObjectType string

const (
	NULL_OBJ              = "NULL"
	ERROR_OBJ             = "ERROR"
	INTEGER_OBJ           = "INTEGER"
	FLOAT_OBJ             = "FLOAT"
	HEX_OBJ               = "HEX"
	BOOLEAN_OBJ           = "BOOLEAN"
	STRING_OBJ            = "STRING"
	RETURN_VALUE_OBJ      = "RETURN_VALUE"
	FUNCTION_OBJ          = "FUNCTION"
	BUILTIN_OBJ           = "BUILTIN"
	ARRAY_OBJ             = "ARRAY"
	HASH_OBJ              = "HASH"
	COMPILED_FUNCTION_OBJ = "COMPILED_FUNCTION_OBJ"
	CLOSURE_OBJ           = "CLOSURE"
	INCLUDE_DIRECTIVE_OBJ = "INCLUDE_DIRECTIVE"
)

type HashKey struct {
	Type  ObjectType
	Value uint64
}

type Hashable interface {
	HashKey() HashKey
}

type Object interface {
	Type() ObjectType
	Inspect() string
}

type Integer struct {
	Value int64
}

func (i *Integer) Type() ObjectType { return INTEGER_OBJ }
func (i *Integer) Inspect() string  { return fmt.Sprintf("%d", i.Value) }
func (i *Integer) HashKey() HashKey {
	return HashKey{Type: i.Type(), Value: uint64(i.Value)}
}

type Float struct {
	Value float64
}

func (f *Float) Type() ObjectType { return FLOAT_OBJ }
func (f *Float) Inspect() string {
	// Remove trailing zeros and unnecessary decimal point
	str := fmt.Sprintf("%.10f", f.Value)
	str = strings.TrimRight(str, "0")
	str = strings.TrimRight(str, ".")
	return str
}
func (f *Float) HashKey() HashKey {
	return HashKey{Type: f.Type(), Value: uint64(f.Value)}
}

type Hex struct {
	Value int64
}

func (h *Hex) Type() ObjectType { return HEX_OBJ }
func (h *Hex) Inspect() string {
	if h.Value < 0 {
		return fmt.Sprintf("-0x%x", -h.Value)
	}
	return fmt.Sprintf("0x%x", h.Value)
}
func (h *Hex) HashKey() HashKey {
	return HashKey{Type: h.Type(), Value: uint64(h.Value)}
}

type Boolean struct {
	Value bool
}

func (b *Boolean) Type() ObjectType { return BOOLEAN_OBJ }
func (b *Boolean) Inspect() string  { return fmt.Sprintf("%t", b.Value) }
func (b *Boolean) HashKey() HashKey {
	var value uint64

	if b.Value {
		value = 1
	} else {
		value = 0
	}

	return HashKey{Type: b.Type(), Value: value}
}

type Null struct{}

func (n *Null) Type() ObjectType { return NULL_OBJ }
func (n *Null) Inspect() string  { return "null" }

type ReturnValue struct {
	Value Object
}

func (rv *ReturnValue) Type() ObjectType { return RETURN_VALUE_OBJ }
func (rv *ReturnValue) Inspect() string  { return rv.Value.Inspect() }

type Error struct {
	Message   string
	Filename  string
	Line      int
	Column    int
	Traceback []string // Stack frames for error context
}

func (e *Error) Type() ObjectType { return ERROR_OBJ }
func (e *Error) Inspect() string {
	var out bytes.Buffer

	if e.Filename != "" {
		out.WriteString(fmt.Sprintf("ERROR: %s, line %d, column %d: %s\n", e.Filename, e.Line, e.Column, e.Message))
	} else {
		out.WriteString("ERROR: " + e.Message + "\n")
	}

	// Add traceback if available
	if len(e.Traceback) > 0 {
		out.WriteString("\nTraceback:\n")
		for _, frame := range e.Traceback {
			out.WriteString(fmt.Sprintf("  %s\n", frame))
		}
	}

	return strings.TrimRight(out.String(), "\n")
}

type Function struct {
	Parameters []*ast.Identifier
	Body       *ast.BlockStatement
	Env        *Environment
}

func (f *Function) Type() ObjectType { return FUNCTION_OBJ }
func (f *Function) Inspect() string {
	var out bytes.Buffer

	params := []string{}
	for _, p := range f.Parameters {
		params = append(params, p.String())
	}

	out.WriteString("fn")
	out.WriteString("(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") {\n")
	out.WriteString(f.Body.String())
	out.WriteString("\n}")

	return out.String()
}

type String struct {
	Value string
}

func (s *String) Type() ObjectType { return STRING_OBJ }
func (s *String) Inspect() string  { return s.Value }
func (s *String) HashKey() HashKey {
	h := fnv.New64a()
	h.Write([]byte(s.Value))

	return HashKey{Type: s.Type(), Value: h.Sum64()}
}

type Builtin struct {
	Fn         BuiltinFunction
	Class      string
	Attributes map[string]Object
}

func (b *Builtin) Type() ObjectType { return BUILTIN_OBJ }
func (b *Builtin) Inspect() string  { return "FUNCTION" }

type Array struct {
	Elements []Object
}

func (ao *Array) Type() ObjectType { return ARRAY_OBJ }
func (ao *Array) Inspect() string {
	var out bytes.Buffer

	elements := []string{}
	for _, e := range ao.Elements {
		elements = append(elements, e.Inspect())
	}

	out.WriteString("[")
	out.WriteString(strings.Join(elements, ", "))
	out.WriteString("]")

	return out.String()
}

type HashPair struct {
	Key   Object
	Value Object
}

type Hash struct {
	Pairs map[HashKey]HashPair
}

func (h *Hash) Type() ObjectType { return HASH_OBJ }
func (h *Hash) Inspect() string {
	var out bytes.Buffer

	pairs := []string{}
	for _, pair := range h.Pairs {
		pairs = append(pairs, fmt.Sprintf("%s: %s",
			pair.Key.Inspect(), pair.Value.Inspect()))
	}

	out.WriteString("{")
	out.WriteString(strings.Join(pairs, ", "))
	out.WriteString("}")

	return out.String()
}

func NewArray(elements []Object) *Array {
	a := arrayPool.Get().(*Array)
	if cap(a.Elements) >= len(elements) {
		a.Elements = a.Elements[:len(elements)]
		copy(a.Elements, elements)
	} else {
		a.Elements = make([]Object, len(elements))
		copy(a.Elements, elements)
	}
	return a
}

func ReleaseArray(a *Array) {
	if a == nil {
		return
	}
	for i := range a.Elements {
		a.Elements[i] = nil
	}
	a.Elements = nil
	arrayPool.Put(a)
}

func NewHash(pairs map[HashKey]HashPair) *Hash {
	h := hashPool.Get().(*Hash)
	if h.Pairs == nil {
		h.Pairs = make(map[HashKey]HashPair)
	}
	for k := range h.Pairs {
		delete(h.Pairs, k)
	}
	for k, v := range pairs {
		h.Pairs[k] = v
	}
	return h
}

func ReleaseHash(h *Hash) {
	if h == nil || h.Pairs == nil {
		return
	}
	for k := range h.Pairs {
		delete(h.Pairs, k)
	}
	hashPool.Put(h)
}

type CompiledFunction struct {
	Instructions  code.Instructions
	NumLocals     int
	NumParameters int
	Name          string
}

func (cf *CompiledFunction) Type() ObjectType { return COMPILED_FUNCTION_OBJ }
func (cf *CompiledFunction) Inspect() string {
	return fmt.Sprintf("CompiledFunction[%p]", cf)
}

type Closure struct {
	Fn   *CompiledFunction
	Free []Object
}

func (c *Closure) Type() ObjectType { return CLOSURE_OBJ }
func (c *Closure) Inspect() string {
	return fmt.Sprintf("Closure[%p]", c)
}

type IncludeDirective struct {
	Namespace string
	Filename  string
	Functions *Hash
}

func (id *IncludeDirective) Type() ObjectType { return INCLUDE_DIRECTIVE_OBJ }
func (id *IncludeDirective) Inspect() string {
	return fmt.Sprintf("IncludeDirective[%s from %s]", id.Namespace, id.Filename)
}
