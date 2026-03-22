package sqx

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestModuleRunManifestAndCall(t *testing.T) {
	m := NewModule("demo")
	m.RegisterMany(
		Method{
			Name:   "ping",
			Return: ReturnString,
			Handle: func(args []string) (interface{}, error) {
				if err := RequireArgs(args, 0); err != nil {
					return nil, err
				}
				return "pong", nil
			},
		},
		Method{
			Name:   "sum",
			Return: ReturnInt,
			Handle: func(args []string) (interface{}, error) {
				a, err := ArgInt(args, 0)
				if err != nil {
					return nil, err
				}
				b, err := ArgInt(args, 1)
				if err != nil {
					return nil, err
				}
				return a + b, nil
			},
		},
	)

	var out bytes.Buffer
	var errOut bytes.Buffer
	if code := m.Run([]string{"__sqx_manifest__"}, &out, &errOut); code != 0 {
		t.Fatalf("manifest returned code %d stderr=%q", code, errOut.String())
	}

	var manifest struct {
		Functions map[string]struct {
			Return string `json:"return"`
		} `json:"functions"`
	}
	if err := json.Unmarshal(out.Bytes(), &manifest); err != nil {
		t.Fatalf("manifest was not valid json: %v (%q)", err, out.String())
	}
	if manifest.Functions["ping"].Return != "string" {
		t.Fatalf("unexpected ping return mode: %+v", manifest.Functions["ping"])
	}
	if manifest.Functions["sum"].Return != "int" {
		t.Fatalf("unexpected sum return mode: %+v", manifest.Functions["sum"])
	}

	out.Reset()
	errOut.Reset()
	if code := m.Run([]string{"__sqx_call__", "sum", "2", "9"}, &out, &errOut); code != 0 {
		t.Fatalf("call returned code %d stderr=%q", code, errOut.String())
	}
	if strings.TrimSpace(out.String()) != "11" {
		t.Fatalf("expected sum output 11, got %q", out.String())
	}
}

func TestModuleRunUnknownFunction(t *testing.T) {
	m := NewModule("demo")
	var out bytes.Buffer
	var errOut bytes.Buffer
	code := m.Run([]string{"__sqx_call__", "missing"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("expected code 2 for unknown function, got %d", code)
	}
	if !strings.Contains(errOut.String(), "unknown SQX function") {
		t.Fatalf("unexpected stderr: %q", errOut.String())
	}
}
