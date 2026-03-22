package sqx

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type ReturnMode string

const (
	ReturnAuto   ReturnMode = "auto"
	ReturnString ReturnMode = "string"
	ReturnRaw    ReturnMode = "raw"
	ReturnInt    ReturnMode = "int"
	ReturnFloat  ReturnMode = "float"
	ReturnBool   ReturnMode = "bool"
	ReturnNull   ReturnMode = "null"
	ReturnJSON   ReturnMode = "json"
)

type Handler func(args []string) (interface{}, error)

type Method struct {
	Name   string
	Return ReturnMode
	Handle Handler
}

type Module struct {
	name    string
	methods map[string]Method
}

type manifest struct {
	Version   int                     `json:"version"`
	Functions map[string]manifestFunc `json:"functions"`
}

type manifestFunc struct {
	Return string `json:"return"`
}

func NewModule(name string) *Module {
	return &Module{
		name:    name,
		methods: make(map[string]Method),
	}
}

func (m *Module) Register(method Method) {
	name := strings.TrimSpace(method.Name)
	if name == "" || method.Handle == nil {
		return
	}
	if method.Return == "" {
		method.Return = ReturnAuto
	}
	method.Name = name
	m.methods[name] = method
}

func (m *Module) RegisterMany(methods ...Method) {
	for _, method := range methods {
		m.Register(method)
	}
}

func (m *Module) Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintf(stderr, "usage: %s <__sqx_manifest__|__sqx_call__>\n", m.name)
		return 2
	}

	switch args[0] {
	case "__sqx_manifest__":
		return m.writeManifest(stdout, stderr)
	case "__sqx_call__":
		return m.call(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown SQX command: %s\n", args[0])
		return 2
	}
}

func (m *Module) writeManifest(stdout, stderr io.Writer) int {
	out := manifest{
		Version:   1,
		Functions: make(map[string]manifestFunc, len(m.methods)),
	}
	for name, method := range m.methods {
		out.Functions[name] = manifestFunc{Return: string(method.Return)}
	}

	data, err := json.Marshal(out)
	if err != nil {
		fmt.Fprintf(stderr, "could not encode SQX manifest: %v\n", err)
		return 1
	}

	if _, err := stdout.Write(data); err != nil {
		fmt.Fprintf(stderr, "could not write SQX manifest: %v\n", err)
		return 1
	}
	return 0
}

func (m *Module) call(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "missing SQX function name")
		return 2
	}

	fnName := args[0]
	method, ok := m.methods[fnName]
	if !ok {
		fmt.Fprintf(stderr, "unknown SQX function: %s\n", fnName)
		return 2
	}

	result, err := method.Handle(args[1:])
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}

	if err := writeResult(stdout, method.Return, result); err != nil {
		fmt.Fprintf(stderr, "SQX result encode failed for %s: %v\n", fnName, err)
		return 1
	}
	return 0
}

func writeResult(out io.Writer, mode ReturnMode, value interface{}) error {
	switch strings.ToLower(strings.TrimSpace(string(mode))) {
	case "", string(ReturnAuto):
		return writeAuto(out, value)
	case string(ReturnString):
		_, err := io.WriteString(out, fmt.Sprint(value))
		return err
	case string(ReturnRaw):
		if bytes, ok := value.([]byte); ok {
			_, err := out.Write(bytes)
			return err
		}
		_, err := io.WriteString(out, fmt.Sprint(value))
		return err
	case string(ReturnInt):
		intVal, err := coerceInt64(value)
		if err != nil {
			return err
		}
		_, err = io.WriteString(out, strconv.FormatInt(intVal, 10))
		return err
	case string(ReturnFloat):
		floatVal, err := coerceFloat64(value)
		if err != nil {
			return err
		}
		_, err = io.WriteString(out, strconv.FormatFloat(floatVal, 'f', -1, 64))
		return err
	case string(ReturnBool):
		boolVal, err := coerceBool(value)
		if err != nil {
			return err
		}
		if boolVal {
			_, err = io.WriteString(out, "true")
		} else {
			_, err = io.WriteString(out, "false")
		}
		return err
	case string(ReturnNull):
		return nil
	case string(ReturnJSON):
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		_, err = out.Write(data)
		return err
	default:
		return fmt.Errorf("unknown return mode %q", mode)
	}
}

func writeAuto(out io.Writer, value interface{}) error {
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		_, err := io.WriteString(out, v)
		return err
	case []byte:
		_, err := out.Write(v)
		return err
	case bool:
		if v {
			_, err := io.WriteString(out, "true")
			return err
		}
		_, err := io.WriteString(out, "false")
		return err
	case int:
		_, err := io.WriteString(out, strconv.Itoa(v))
		return err
	case int8, int16, int32, int64:
		i, err := coerceInt64(v)
		if err != nil {
			return err
		}
		_, err = io.WriteString(out, strconv.FormatInt(i, 10))
		return err
	case uint, uint8, uint16, uint32, uint64:
		i, err := coerceInt64(v)
		if err != nil {
			return err
		}
		_, err = io.WriteString(out, strconv.FormatInt(i, 10))
		return err
	case float32, float64:
		f, err := coerceFloat64(v)
		if err != nil {
			return err
		}
		_, err = io.WriteString(out, strconv.FormatFloat(f, 'f', -1, 64))
		return err
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return err
		}
		_, err = out.Write(data)
		return err
	}
}

func coerceInt64(v interface{}) (int64, error) {
	switch n := v.(type) {
	case int:
		return int64(n), nil
	case int8:
		return int64(n), nil
	case int16:
		return int64(n), nil
	case int32:
		return int64(n), nil
	case int64:
		return n, nil
	case uint:
		return int64(n), nil
	case uint8:
		return int64(n), nil
	case uint16:
		return int64(n), nil
	case uint32:
		return int64(n), nil
	case uint64:
		return int64(n), nil
	case float32:
		return int64(n), nil
	case float64:
		return int64(n), nil
	case string:
		i, err := strconv.ParseInt(strings.TrimSpace(n), 10, 64)
		if err != nil {
			return 0, err
		}
		return i, nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int", v)
	}
}

func coerceFloat64(v interface{}) (float64, error) {
	switch n := v.(type) {
	case int:
		return float64(n), nil
	case int8:
		return float64(n), nil
	case int16:
		return float64(n), nil
	case int32:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case uint:
		return float64(n), nil
	case uint8:
		return float64(n), nil
	case uint16:
		return float64(n), nil
	case uint32:
		return float64(n), nil
	case uint64:
		return float64(n), nil
	case float32:
		return float64(n), nil
	case float64:
		return n, nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		if err != nil {
			return 0, err
		}
		return f, nil
	default:
		return 0, fmt.Errorf("cannot convert %T to float", v)
	}
}

func coerceBool(v interface{}) (bool, error) {
	switch b := v.(type) {
	case bool:
		return b, nil
	case string:
		out, err := strconv.ParseBool(strings.TrimSpace(strings.ToLower(b)))
		if err != nil {
			return false, err
		}
		return out, nil
	default:
		return false, fmt.Errorf("cannot convert %T to bool", v)
	}
}

func RequireArgs(args []string, expected int) error {
	if len(args) != expected {
		return fmt.Errorf("expected %d argument(s), got %d", expected, len(args))
	}
	return nil
}

func ArgString(args []string, index int) (string, error) {
	if index < 0 || index >= len(args) {
		return "", fmt.Errorf("missing argument at index %d", index)
	}
	return args[index], nil
}

func ArgInt(args []string, index int) (int64, error) {
	value, err := ArgString(args, index)
	if err != nil {
		return 0, err
	}
	i, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("argument %d must be int: %w", index, err)
	}
	return i, nil
}
