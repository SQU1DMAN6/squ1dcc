package sqx

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestServePing tests the Serve method's handling of the "ping" command.
func TestServePing(t *testing.T) {
	m := NewModule("test")
	m.Register(Method{
		Name:   "echo",
		Return: ReturnString,
		Handle: func(args []string) (interface{}, error) {
			if len(args) == 0 {
				return "", nil
			}
			return args[0], nil
		},
	})

	var out bytes.Buffer
	var errOut bytes.Buffer

	// Simulate session mode
	code := m.Run([]string{"--session"}, &out, &errOut)
	// When no stdin is provided, Serve returns immediately (EOF)
	if code != 0 {
		t.Fatalf("expected code 0, got %d; stderr=%q", code, errOut.String())
	}
}

// TestServeJSONProtocol tests the Serve method with JSON requests.
func TestServeJSONProtocol(t *testing.T) {
	m := NewModule("test")
	m.Register(Method{
		Name:   "echo",
		Return: ReturnString,
		Handle: func(args []string) (interface{}, error) {
			if len(args) == 0 {
				return "", nil
			}
			return args[0], nil
		},
	})
	m.Register(Method{
		Name:   "fail",
		Return: ReturnStructured,
		Handle: func(args []string) (interface{}, error) {
			return nil, nil
		},
	})

	var out bytes.Buffer
	input := strings.NewReader(`{"cmd":"call","fn":"echo","args":["hello"]}
{"cmd":"ping"}
{"cmd":"call","fn":"nonexistent","args":[]}
{"cmd":"shutdown"}
`)

	code := m.Serve(input, &out)
	if code != 0 {
		t.Fatalf("expected Serve code 0, got %d", code)
	}

	// Verify we got valid JSON responses
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 response lines, got %d: %q", len(lines), out.String())
	}

	// First response: echo result
	var r1 struct {
		Ok    bool        `json:"ok"`
		Value interface{} `json:"value"`
		Error string      `json:"error"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &r1); err != nil {
		t.Fatalf("invalid JSON response 1: %v: %q", err, lines[0])
	}
	if !r1.Ok {
		t.Fatalf("expected ok=true, got ok=%v", r1.Ok)
	}

	// Second response: ping
	var r2 struct {
		Ok    bool   `json:"ok"`
		Value string `json:"value"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &r2); err != nil {
		t.Fatalf("invalid JSON response 2: %v: %q", err, lines[1])
	}
	if r2.Value != "pong" {
		t.Fatalf("expected ping response 'pong', got %q", r2.Value)
	}

	// Third response: nonexistent function error
	var r3 struct {
		Ok    bool   `json:"ok"`
		Value string `json:"value"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(lines[2]), &r3); err != nil {
		t.Fatalf("invalid JSON response 3: %v: %q", err, lines[2])
	}
	if r3.Ok {
		t.Fatalf("expected ok=false for nonexistent function, got ok=true")
	}
	if !strings.Contains(r3.Error, "nonexistent") {
		t.Fatalf("expected error about nonexistent, got %q", r3.Error)
	}
}

// TestStructuredReturnMode tests the structured return contract.
func TestStructuredReturnMode(t *testing.T) {
	m := NewModule("test")
	m.Register(Method{
		Name:   "divide",
		Return: ReturnStructured,
		Handle: func(args []string) (interface{}, error) {
			if err := RequireArgs(args, 2); err != nil {
				return nil, err
			}
			a, err := ArgInt(args, 0)
			if err != nil {
				return nil, err
			}
			b, err := ArgInt(args, 1)
			if err != nil {
				return nil, err
			}
			return a / b, nil
		},
	})

	var out bytes.Buffer
	input := strings.NewReader(`{"cmd":"call","fn":"divide","args":["10","3"]}
{"cmd":"shutdown"}
`)
	code := m.Serve(input, &out)
	if code != 0 {
		t.Fatalf("expected Serve code 0, got %d", code)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) < 1 {
		t.Fatalf("expected at least 1 response, got %d", len(lines))
	}

	var result struct {
		Ok    bool   `json:"ok"`
		Value int    `json:"value"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &result); err != nil {
		t.Fatalf("invalid JSON response: %v: %q", err, lines[0])
	}
	if !result.Ok {
		t.Fatalf("expected ok=true, got %+v", result)
	}
	if result.Value != 3 {
		t.Fatalf("expected 10/3=3, got %d", result.Value)
	}
}

// TestStructuredCallViaCLI tests calling a structured-return function via CLI.
func TestStructuredCallViaCLI(t *testing.T) {
	m := NewModule("test")
	m.Register(Method{
		Name:   "greet",
		Return: ReturnStructured,
		Handle: func(args []string) (interface{}, error) {
			if len(args) == 0 {
				return nil, nil
			}
			result := map[string]string{"message": "Hello, " + args[0] + "!"}
			return result, nil
		},
	})

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := m.Run([]string{"__sqx_call__", "greet", "World"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", code, errOut.String())
	}

	var result struct {
		Ok    bool `json:"ok"`
		Value struct {
			Message string `json:"message"`
		} `json:"value"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON structured output: %v: %q", err, out.String())
	}
	if !result.Ok {
		t.Fatalf("expected ok=true, got %+v", result)
	}
	if result.Value.Message != "Hello, World!" {
		t.Fatalf("expected 'Hello, World!', got %q", result.Value.Message)
	}
}

// TestSessionPool tests the SessionPool basic operations.
func TestSessionPool(t *testing.T) {
	pool := NewSessionPool(SessionConfig{
		Path: "nonexistent.sqx",
	}, 2)

	// Acquire should fail gracefully since the module doesn't exist
	_, err := pool.Acquire()
	if err == nil {
		t.Fatalf("expected error for nonexistent module, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Fatalf("expected error about nonexistent, got %v", err)
	}

	// CloseAll should not panic
	pool.CloseAll()
}