package object

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const sqxTypedArgPrefix = "__sqx_typed__:"

// SQXManifest is the JSON schema loaded from .sqx plugin files.
type SQXManifest struct {
	Version   int                        `json:"version"`
	Functions map[string]SQXFunctionSpec `json:"functions"`
}

// SQXFunctionSpec describes one exported plugin function.
// Example:
//
//	{
//	  "exec": ["./tooling_plugin.sh", "sum"],
//	  "append_args": true,
//	  "return": "int"
//	}
type SQXFunctionSpec struct {
	Exec       []string          `json:"exec"`
	AppendArgs bool              `json:"append_args"`
	Return     string            `json:"return"`
	Env        map[string]string `json:"env"`
}

// LoadSQXNamespace loads a .sqx plugin manifest and converts it to a hash
// namespace where each declared function is exposed as a callable builtin.
func LoadSQXNamespace(path string) (*Hash, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read SQX file %q: %w", path, err)
	}

	var manifest SQXManifest

	// Backward-compatible path: JSON command manifest.
	if err := json.Unmarshal(content, &manifest); err == nil {
		if len(manifest.Functions) > 0 {
			return loadSQXCommandManifest(path, manifest)
		}
		return nil, fmt.Errorf("SQX %q is JSON but declares no functions; provide functions or use an executable SQX module", path)
	}

	// Native module path: execute the .sqx itself as a plugin module.
	return loadSQXExecutableModule(path)
}

func loadSQXCommandManifest(path string, manifest SQXManifest) (*Hash, error) {
	baseDir := filepath.Dir(path)
	namespace := &Hash{Pairs: make(map[HashKey]HashPair)}

	for fnName, spec := range manifest.Functions {
		if len(spec.Exec) == 0 {
			return nil, fmt.Errorf("SQX function %q in %q has empty exec command", fnName, path)
		}

		fnNameCopy := fnName
		specCopy := spec

		builtin := &Builtin{
			Class:      "",
			Attributes: make(map[string]Object),
		}
		builtin.Fn = func(args ...Object) Object {
			out, runErr := runSQXFunction(baseDir, fnNameCopy, specCopy, args...)
			if runErr != nil {
				return &Error{Message: runErr.Error()}
			}
			return out
		}

		key := &String{Value: fnName}
		namespace.Pairs[key.HashKey()] = HashPair{Key: key, Value: builtin}
	}

	return namespace, nil
}

func loadSQXExecutableModule(path string) (*Hash, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("could not resolve SQX path %q: %w", path, err)
	}

	manifestOut, err := runSQXModule(absPath, "__sqx_manifest__")
	if err != nil {
		return nil, fmt.Errorf("could not load SQX module manifest from %q: %w", absPath, err)
	}

	var manifest SQXManifest
	if err := json.Unmarshal(manifestOut, &manifest); err != nil {
		return nil, fmt.Errorf("invalid SQX module manifest in %q: %w", absPath, err)
	}
	if len(manifest.Functions) == 0 {
		return nil, fmt.Errorf("SQX module %q has no functions", absPath)
	}

	// Attempt to create a session-aware loader
	loader, loaderErr := NewSQXSessionLoader(absPath, manifest)
	if loaderErr != nil && CurrentSessionMode != SessionModeLegacy {
		// If loader creation fails, fall back to process-per-call
		loader = nil
	}

	namespace := &Hash{Pairs: make(map[HashKey]HashPair)}
	for fnName, spec := range manifest.Functions {
		fnNameCopy := fnName
		specCopy := spec

		builtin := &Builtin{
			Class:      "",
			Attributes: make(map[string]Object),
		}
		builtin.Fn = func(args ...Object) Object {
			var out Object
			var runErr error

			// Use session loader if available
			if loader != nil && !loader.session.IsClosed() {
				out, runErr = loader.Call(fnNameCopy, specCopy.Return, args...)
			} else {
				out, runErr = runSQXModuleFunction(absPath, fnNameCopy, specCopy.Return, args...)
			}

			if runErr != nil {
				return &Error{Message: runErr.Error()}
			}
			return out
		}

		key := &String{Value: fnName}
		namespace.Pairs[key.HashKey()] = HashPair{Key: key, Value: builtin}
	}

	return namespace, nil
}

func runSQXModule(path string, args ...string) ([]byte, error) {
	cmd := exec.Command(path, args...)
	cmd.Dir = filepath.Dir(path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg != "" {
			return nil, fmt.Errorf("%v: %s", err, msg)
		}
		return nil, err
	}
	return output, nil
}

func runSQXModuleFunction(modulePath, fnName, returnMode string, args ...Object) (Object, error) {
	callArgs := []string{"__sqx_call__", fnName}
	for _, arg := range args {
		callArgs = append(callArgs, sqxArgToString(arg))
	}

	output, err := runSQXModule(modulePath, callArgs...)
	if err != nil {
		return nil, fmt.Errorf("SQX function %q failed: %w", fnName, err)
	}
	return parseSQXOutput(output, returnMode)
}

func runSQXFunction(baseDir, fnName string, spec SQXFunctionSpec, args ...Object) (Object, error) {
	absBaseDir := baseDir
	if resolvedBaseDir, err := filepath.Abs(baseDir); err == nil {
		absBaseDir = resolvedBaseDir
	}

	cmdParts := append([]string{}, spec.Exec...)
	if len(cmdParts) == 0 {
		return nil, fmt.Errorf("SQX function %q has no executable", fnName)
	}

	// Resolve relative executable paths against the .sqx file directory.
	if strings.Contains(cmdParts[0], string(filepath.Separator)) && !filepath.IsAbs(cmdParts[0]) {
		cmdParts[0] = filepath.Join(absBaseDir, cmdParts[0])
	}

	if spec.AppendArgs {
		for _, arg := range args {
			cmdParts = append(cmdParts, sqxArgToString(arg))
		}
	}

	cmd := exec.Command(cmdParts[0], cmdParts[1:]...)
	cmd.Dir = absBaseDir
	if len(spec.Env) > 0 {
		env := os.Environ()
		for k, v := range spec.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg != "" {
			return nil, fmt.Errorf("SQX function %q failed: %v: %s", fnName, err, msg)
		}
		return nil, fmt.Errorf("SQX function %q failed: %v", fnName, err)
	}

	return parseSQXOutput(output, spec.Return)
}

func parseSQXOutput(output []byte, returnMode string) (Object, error) {
	mode := strings.ToLower(strings.TrimSpace(returnMode))
	raw := string(output)
	trimmed := strings.TrimSpace(raw)

	switch mode {
	case "", "auto":
		return parseSQXAuto(trimmed)
	case "string":
		return &String{Value: trimmed}, nil
	case "raw":
		return &String{Value: raw}, nil
	case "int", "integer":
		v, err := strconv.ParseInt(trimmed, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("expected integer output, got %q", trimmed)
		}
		return &Integer{Value: v}, nil
	case "float":
		v, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return nil, fmt.Errorf("expected float output, got %q", trimmed)
		}
		return &Float{Value: v}, nil
	case "bool", "boolean":
		v, err := strconv.ParseBool(strings.ToLower(trimmed))
		if err != nil {
			return nil, fmt.Errorf("expected boolean output, got %q", trimmed)
		}
		return &Boolean{Value: v}, nil
	case "null":
		return &Null{}, nil
	case "json":
		var v interface{}
		if err := json.Unmarshal([]byte(trimmed), &v); err != nil {
			return nil, fmt.Errorf("expected JSON output, got %q", trimmed)
		}
		return sqxJSONToObject(v), nil
	case "structured":
		return parseSQXStructured(trimmed)
	default:
		return nil, fmt.Errorf("unknown SQX return mode %q", returnMode)
	}
}

// parseSQXStructured parses a StructuredResult JSON envelope:
//
//	{"ok":true,"value":...,"error":null}
//	{"ok":false,"value":null,"error":"message"}
//
// Returns a Hash with keys: "ok" (Boolean), "value" (any), "error" (String|Null).
//
// STRICT RULES:
// - "ok" must be a real JSON boolean, never a string.
// - "error" must be a JSON string or null, never a string containing the word "null".
// - "value" must be a valid JSON type, never a string pretending to be another type.
func parseSQXStructured(trimmed string) (Object, error) {
	// First, parse into raw interface to validate type integrity
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		return nil, fmt.Errorf("expected structured JSON object, got invalid JSON: %q", trimmed)
	}

	// Validate that "ok" field exists and is a bool
	okRaw, okExists := raw["ok"]
	if !okExists {
		return nil, fmt.Errorf("structured result missing required 'ok' field: %q", trimmed)
	}
	okVal, okIsBool := okRaw.(bool)
	if !okIsBool {
		return nil, fmt.Errorf("structured result 'ok' must be a boolean, got %T: %q", okRaw, trimmed)
	}

	// Validate that "value" field exists
	if _, valueExists := raw["value"]; !valueExists {
		return nil, fmt.Errorf("structured result missing required 'value' field: %q", trimmed)
	}

	// Validate that "error" field exists
	errRaw, errExists := raw["error"]
	if !errExists {
		return nil, fmt.Errorf("structured result missing required 'error' field: %q", trimmed)
	}

	// Error field must be string or null
	if errRaw != nil {
		if _, errIsString := errRaw.(string); !errIsString {
			return nil, fmt.Errorf("structured result 'error' must be string or null, got %T: %q", errRaw, trimmed)
		}
	}

	// Now parse properly into the struct
	var sr struct {
		Ok    bool            `json:"ok"`
		Value json.RawMessage `json:"value"`
		Error string          `json:"error"`
	}
	if err := json.Unmarshal([]byte(trimmed), &sr); err != nil {
		return nil, fmt.Errorf("expected structured output, got %q", trimmed)
	}

	// Cross-validate: if ok is false, error should be non-empty; if ok is true, error should be empty
	if sr.Ok && sr.Error != "" {
		return nil, fmt.Errorf("structured result inconsistency: ok=true but error is non-empty: %q", trimmed)
	}
	if !sr.Ok && sr.Error == "" {
		return nil, fmt.Errorf("structured result inconsistency: ok=false but error is empty: %q", trimmed)
	}

	pairs := make(map[HashKey]HashPair)

	// ok field — always a real boolean
	okKey := &String{Value: "ok"}
	pairs[okKey.HashKey()] = HashPair{Key: okKey, Value: &Boolean{Value: okVal}}

	// value field — must be valid JSON
	valueKey := &String{Value: "value"}
	if sr.Value != nil && len(sr.Value) > 0 {
		var v interface{}
		if err := json.Unmarshal(sr.Value, &v); err == nil {
			pairs[valueKey.HashKey()] = HashPair{Key: valueKey, Value: sqxJSONToObject(v)}
		} else {
			// Raw value is not valid JSON — this is an error condition
			return nil, fmt.Errorf("structured result 'value' is not valid JSON: %q", string(sr.Value))
		}
	} else {
		pairs[valueKey.HashKey()] = HashPair{Key: valueKey, Value: &Null{}}
	}

	// error field — always string or null
	errorKey := &String{Value: "error"}
	if errRaw != nil && errRaw.(string) != "" {
		pairs[errorKey.HashKey()] = HashPair{Key: errorKey, Value: &String{Value: errRaw.(string)}}
	} else {
		pairs[errorKey.HashKey()] = HashPair{Key: errorKey, Value: &Null{}}
	}

	return NewHash(pairs), nil
}

func parseSQXAuto(trimmed string) (Object, error) {
	if trimmed == "" {
		return &String{Value: ""}, nil
	}

	if i, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		return &Integer{Value: i}, nil
	}

	if f, err := strconv.ParseFloat(trimmed, 64); err == nil {
		return &Float{Value: f}, nil
	}

	if b, err := strconv.ParseBool(strings.ToLower(trimmed)); err == nil {
		return &Boolean{Value: b}, nil
	}

	var jsonVal interface{}
	if err := json.Unmarshal([]byte(trimmed), &jsonVal); err == nil {
		return sqxJSONToObject(jsonVal), nil
	}

	return &String{Value: trimmed}, nil
}

func sqxJSONToObject(v interface{}) Object {
	switch val := v.(type) {
	case nil:
		return &Null{}
	case bool:
		return &Boolean{Value: val}
	case float64:
		// Keep whole-number JSON values as Integer for ergonomic scripting.
		if float64(int64(val)) == val {
			return &Integer{Value: int64(val)}
		}
		return &Float{Value: val}
	case string:
		return &String{Value: val}
	case []interface{}:
		elements := make([]Object, len(val))
		for i, item := range val {
			elements[i] = sqxJSONToObject(item)
		}
		return NewArray(elements)
	case map[string]interface{}:
		pairs := make(map[HashKey]HashPair)
		for k, item := range val {
			key := &String{Value: k}
			pairs[key.HashKey()] = HashPair{Key: key, Value: sqxJSONToObject(item)}
		}
		return NewHash(pairs)
	default:
		return &String{Value: fmt.Sprintf("%v", val)}
	}
}

func sqxObjectToNative(arg Object) interface{} {
	switch v := arg.(type) {
	case *String:
		return v.Value
	case *Integer:
		return v.Value
	case *Float:
		return v.Value
	case *Boolean:
		return v.Value
	case *Null:
		return nil
	case *Array:
		out := make([]interface{}, len(v.Elements))
		for i, el := range v.Elements {
			out[i] = sqxObjectToNative(el)
		}
		return out
	case *Hash:
		out := make(map[string]interface{}, len(v.Pairs))
		for _, pair := range v.Pairs {
			k, ok := pair.Key.(*String)
			if !ok {
				continue
			}
			out[k.Value] = sqxObjectToNative(pair.Value)
		}
		return out
	default:
		return arg.Inspect()
	}
}

func sqxArgToString(arg Object) string {
	switch v := arg.(type) {
	case *String:
		return v.Value
	case *Integer:
		return strconv.FormatInt(v.Value, 10)
	case *Float:
		return strconv.FormatFloat(v.Value, 'f', -1, 64)
	case *Boolean:
		return strconv.FormatBool(v.Value)
	case *Null:
		return ""
	case *Array, *Hash:
		native := sqxObjectToNative(arg)
		encoded, err := json.Marshal(native)
		if err == nil {
			return sqxTypedArgPrefix + string(encoded)
		}
		return arg.Inspect()
	default:
		return arg.Inspect()
	}
}
