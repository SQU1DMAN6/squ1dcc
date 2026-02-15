package bytecode

import (
	"encoding/binary"
	"fmt"
	"io"
	"squ1d++/code"
	"squ1d++/object"
)

// Format version for bytecode compatibility checking
const VERSION = 1

// Package represents a compiled SQU1D++ package that can be serialized
type Package struct {
	Version      int
	Instructions code.Instructions
	Constants    []object.Object
}

// Serialize writes a Package to bytecode format
func (p *Package) Serialize(w io.Writer) error {
	// Write version
	if err := binary.Write(w, binary.LittleEndian, int32(p.Version)); err != nil {
		return fmt.Errorf("failed to write version: %v", err)
	}

	// Write instructions length and data
	if err := binary.Write(w, binary.LittleEndian, int32(len(p.Instructions))); err != nil {
		return fmt.Errorf("failed to write instructions length: %v", err)
	}
	if _, err := w.Write(p.Instructions); err != nil {
		return fmt.Errorf("failed to write instructions: %v", err)
	}

	// Write constants length
	if err := binary.Write(w, binary.LittleEndian, int32(len(p.Constants))); err != nil {
		return fmt.Errorf("failed to write constants length: %v", err)
	}

	// Write each constant
	for i, constant := range p.Constants {
		if err := serializeConstant(w, constant); err != nil {
			return fmt.Errorf("failed to serialize constant %d: %v", i, err)
		}
	}

	return nil
}

// Deserialize reads a Package from bytecode format
func Deserialize(r io.Reader) (*Package, error) {
	pkg := &Package{}

	// Read version
	var version int32
	if err := binary.Read(r, binary.LittleEndian, &version); err != nil {
		return nil, fmt.Errorf("failed to read version: %v", err)
	}
	pkg.Version = int(version)

	if pkg.Version != VERSION {
		return nil, fmt.Errorf("unsupported bytecode version: %d (expected %d)", pkg.Version, VERSION)
	}

	// Read instructions
	var insLen int32
	if err := binary.Read(r, binary.LittleEndian, &insLen); err != nil {
		return nil, fmt.Errorf("failed to read instructions length: %v", err)
	}
	instructions := make([]byte, insLen)
	if _, err := io.ReadFull(r, instructions); err != nil {
		return nil, fmt.Errorf("failed to read instructions: %v", err)
	}
	pkg.Instructions = instructions

	// Read constants
	var constLen int32
	if err := binary.Read(r, binary.LittleEndian, &constLen); err != nil {
		return nil, fmt.Errorf("failed to read constants length: %v", err)
	}
	pkg.Constants = make([]object.Object, constLen)
	for i := 0; i < int(constLen); i++ {
		const_, err := deserializeConstant(r)
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize constant %d: %v", i, err)
		}
		pkg.Constants[i] = const_
	}

	return pkg, nil
}

// Constant type markers for serialization
const (
	constTypeNil        = 0
	constTypeInt        = 1
	constTypeFloat      = 2
	constTypeString     = 3
	constTypeBool       = 4
	constTypeArray      = 5
	constTypeHash       = 6
	constTypeCompiledFn = 7
)

func serializeConstant(w io.Writer, obj object.Object) error {
	switch obj := obj.(type) {
	case *object.Null:
		return binary.Write(w, binary.LittleEndian, int8(constTypeNil))

	case *object.Integer:
		if err := binary.Write(w, binary.LittleEndian, int8(constTypeInt)); err != nil {
			return err
		}
		return binary.Write(w, binary.LittleEndian, obj.Value)

	case *object.Float:
		if err := binary.Write(w, binary.LittleEndian, int8(constTypeFloat)); err != nil {
			return err
		}
		return binary.Write(w, binary.LittleEndian, obj.Value)

	case *object.String:
		if err := binary.Write(w, binary.LittleEndian, int8(constTypeString)); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, int32(len(obj.Value))); err != nil {
			return err
		}
		_, err := w.Write([]byte(obj.Value))
		return err

	case *object.Boolean:
		if err := binary.Write(w, binary.LittleEndian, int8(constTypeBool)); err != nil {
			return err
		}
		return binary.Write(w, binary.LittleEndian, obj.Value)

	case *object.Array:
		if err := binary.Write(w, binary.LittleEndian, int8(constTypeArray)); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, int32(len(obj.Elements))); err != nil {
			return err
		}
		for _, elem := range obj.Elements {
			if err := serializeConstant(w, elem); err != nil {
				return err
			}
		}
		return nil

	case *object.Hash:
		if err := binary.Write(w, binary.LittleEndian, int8(constTypeHash)); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, int32(len(obj.Pairs))); err != nil {
			return err
		}
		for _, pair := range obj.Pairs {
			// Serialize key
			if err := serializeConstant(w, pair.Key); err != nil {
				return err
			}
			// Serialize value
			if err := serializeConstant(w, pair.Value); err != nil {
				return err
			}
		}
		return nil

	case *object.CompiledFunction:
		if err := binary.Write(w, binary.LittleEndian, int8(constTypeCompiledFn)); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, int32(len(obj.Instructions))); err != nil {
			return err
		}
		if _, err := w.Write(obj.Instructions); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, int32(obj.NumLocals)); err != nil {
			return err
		}
		return binary.Write(w, binary.LittleEndian, int32(obj.NumParameters))

	default:
		return fmt.Errorf("cannot serialize object type: %T", obj)
	}
}

func deserializeConstant(r io.Reader) (object.Object, error) {
	var typeMarker int8
	if err := binary.Read(r, binary.LittleEndian, &typeMarker); err != nil {
		return nil, err
	}

	switch typeMarker {
	case constTypeNil:
		return &object.Null{}, nil

	case constTypeInt:
		var val int64
		if err := binary.Read(r, binary.LittleEndian, &val); err != nil {
			return nil, err
		}
		return &object.Integer{Value: val}, nil

	case constTypeFloat:
		var val float64
		if err := binary.Read(r, binary.LittleEndian, &val); err != nil {
			return nil, err
		}
		return &object.Float{Value: val}, nil

	case constTypeString:
		var len int32
		if err := binary.Read(r, binary.LittleEndian, &len); err != nil {
			return nil, err
		}
		data := make([]byte, len)
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, err
		}
		return &object.String{Value: string(data)}, nil

	case constTypeBool:
		var val bool
		if err := binary.Read(r, binary.LittleEndian, &val); err != nil {
			return nil, err
		}
		return &object.Boolean{Value: val}, nil

	case constTypeArray:
		var len int32
		if err := binary.Read(r, binary.LittleEndian, &len); err != nil {
			return nil, err
		}
		elements := make([]object.Object, len)
		for i := 0; i < int(len); i++ {
			elem, err := deserializeConstant(r)
			if err != nil {
				return nil, err
			}
			elements[i] = elem
		}
		return &object.Array{Elements: elements}, nil

	case constTypeHash:
		var len int32
		if err := binary.Read(r, binary.LittleEndian, &len); err != nil {
			return nil, err
		}
		pairs := make(map[object.HashKey]object.HashPair)
		for i := 0; i < int(len); i++ {
			key, err := deserializeConstant(r)
			if err != nil {
				return nil, err
			}
			value, err := deserializeConstant(r)
			if err != nil {
				return nil, err
			}
			hashable, ok := key.(object.Hashable)
			if !ok {
				return nil, fmt.Errorf("unhashable key type: %T", key)
			}
			pairs[hashable.HashKey()] = object.HashPair{Key: key, Value: value}
		}
		return &object.Hash{Pairs: pairs}, nil

	case constTypeCompiledFn:
		var insLen int32
		if err := binary.Read(r, binary.LittleEndian, &insLen); err != nil {
			return nil, err
		}
		instructions := make([]byte, insLen)
		if _, err := io.ReadFull(r, instructions); err != nil {
			return nil, err
		}
		var numLocals, numParams int32
		if err := binary.Read(r, binary.LittleEndian, &numLocals); err != nil {
			return nil, err
		}
		if err := binary.Read(r, binary.LittleEndian, &numParams); err != nil {
			return nil, err
		}
		return &object.CompiledFunction{
			Instructions:  instructions,
			NumLocals:     int(numLocals),
			NumParameters: int(numParams),
		}, nil

	default:
		return nil, fmt.Errorf("unknown constant type marker: %d", typeMarker)
	}
}
