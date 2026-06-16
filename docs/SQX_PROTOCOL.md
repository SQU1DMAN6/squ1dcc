# SQX Protocol Specification v1.9

## Overview

SQX (SQU1D Extension) is a structured runtime extension protocol for SQU1DLang. It enables native code (C, Go, C++, etc.) to be called as if it were a built-in SQU1DLang function.

## Protocol Layers

SQX operates at three conceptual layers:

```
Layer A: SQX Protocol
  - Command parsing
  - Message format
  - Routing

Layer B: Runtime Modules
  - terminal.sqx
  - fs.sqx
  - net.sqx

Layer C: Platform Backend
  - Windows console API
  - Linux termios
  - macOS terminal handling
```

## Execution Modes

### Mode 1: Legacy (process-per-call)

The simplest mode. Each function call spawns a new process.

**Flow:**
1. SQU1DCC calls `module.sqx __sqx_manifest__` to discover functions
2. For each function call: `module.sqx __sqx_call__ <fn_name> [args...]`
3. Module writes result to stdout and exits
4. SQU1DCC reads stdout and parses the result

**Manifest format:**
```json
{"version":1,"functions":{"ping":{"return":"string"},"write":{"return":"structured"}}}
```

**Call format:**
```
$ ./module.sqx __sqx_call__ ping
$ ./module.sqx __sqx_call__ write "Hello World"
$ ./module.sqx __sqx_call__ add 3 4
```

### Mode 2: Session Mode (persistent process)

**Flow:**
1. SQU1DCC starts: `module.sqx --session`
2. Module stays alive, reading JSON requests from stdin
3. Module writes JSON responses to stdout
4. Session persists until `{"cmd":"shutdown"}` is received

**Request format:**
```json
{"cmd":"call","fn":"function_name","args":["arg1","arg2"]}
{"cmd":"ping"}
{"cmd":"shutdown"}
```

**Response format:**
```json
{"ok":true,"value":"result","error":null}
{"ok":false,"value":null,"error":"error message"}
```

### Mode 3: Native Lib Mode (future)

Shared library loading with direct function calls. Not yet implemented.

## Message Format

All SQX messages use JSON with a strict schema.

### Standard Response Envelope

Every SQX function MUST return this structure:

```json
{
    "ok": <boolean>,
    "value": <any valid JSON type>,
    "error": <string | null>
}
```

**Strict Rules:**

| Field | Type | Rule |
|-------|------|------|
| `ok` | boolean | Must be `true` or `false` (real JSON booleans, NOT strings) |
| `value` | any valid JSON | Must be valid JSON; never a string pretending to be another type |
| `error` | string or null | Must be `null` on success, a string on failure |

**Valid examples:**
```json
{"ok":true,"value":42,"error":null}
{"ok":true,"value":"hello","error":null}
{"ok":true,"value":{"width":80,"height":25},"error":null}
{"ok":true,"value":[1,2,3],"error":null}
{"ok":false,"value":null,"error":"division by zero"}
```

**Invalid examples (MUST NOT be used):**
```json
{"ok":"true","value":42,"error":null}        // WRONG: ok is a string
{"ok":true,"value":"42","error":null}        // WRONG: value is string-pretending-int
{"ok":false,"error":"fail"}                  // WRONG: missing value field
{"ok":true,"value":42}                       // WRONG: missing error field
```

### Type Mapping

When SQX responses are parsed into SQU1DLang objects:

| JSON Type | SQU1DLang Type |
|-----------|----------------|
| `null` | `Null` |
| `true`/`false` | `Boolean` |
| number (integer) | `Integer` |
| number (float) | `Float` |
| string | `String` |
| array | `Array` |
| object | `Hash` |

## Return Modes

The manifest declares the return mode for each function:

| Return Mode | Behavior | SQU1DLang Receives |
|-------------|----------|-------------------|
| `"auto"` | Auto-detect type | Parsed value |
| `"string"` | Raw string output | `String` |
| `"raw"` | Unaltered output | `String` |
| `"int"` / `"integer"` | Integer output | `Integer` |
| `"float"` | Float output | `Float` |
| `"bool"` / `"boolean"` | Boolean output | `Boolean` |
| `"null"` | Null output | `Null` |
| `"json"` | JSON-parsed output | Parsed object |
| `"structured"` | `{ok, value, error}` envelope | `Hash` with `ok`/`value`/`error` keys |

### Structured Mode (Recommended)

Structured mode wraps results in the standard envelope:

```json
{"ok":true,"value":42,"error":null}
```

In SQU1DLang, use the error pipe operators:

```sqd
var result = mymodule.some_function()
if (result.ok) {
    io.echo(result.value)
} el {
    io.echo(result.error)
}
```

## Typed Arguments

SQX supports typed arguments for complex data types. Arguments are prefixed with `__sqx_typed__:` followed by JSON:

```
__sqx_typed__:[1,2,3]
__sqx_typed__:{"key":"value"}
__sqx_typed__:true
```

This is automatically handled by SQU1DCC when passing arrays, hashes, or booleans to SQX functions.

## Error Pipe Alignment

The `<<` and `<<<` operators extract fields from structured results:

```
<< result    → extracts result.error (null if success)
<<< result   → extracts result.ok (boolean)
```

Example:
```sqd
var result = term.read_key()
if (<<< result) {
    # ok is true — success
    io.echo(result.value.key)
} el {
    # ok is false — error
    io.echo("Error: " + (<< result))
}
```

## Security Permissions (Future)

SQX modules SHOULD declare required capabilities:

```json
{
    "version": 1,
    "functions": { ... },
    "permissions": {
        "filesystem": true,
        "network": false,
        "terminal": true
    }
}
```

SQU1DCC should enforce capability restrictions based on these declarations.

## Error Handling

### Structured Errors

All errors in structured mode use the standard envelope:

```json
{"ok":false,"value":null,"error":"descriptive error message"}
```

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Runtime error |
| 2 | Usage error / unknown function |

## Performance Considerations

### Session Mode Recommended For:

- Terminal/UI operations (frequent calls)
- Stateful modules (keyboard state, cursor position)
- Real-time loops (editors, animations)
- Any sequence of 10+ calls

### Process-per-Call Acceptable For:

- One-off operations
- Stateless computations
- Debugging/testing
- Simple plugins

### Estimated Call Costs

| Mode | Latency | Throughput |
|------|---------|------------|
| Process-per-call | ~5-50ms | ~20-200 calls/sec |
| Session mode | ~0.1-1ms | ~1,000-10,000 calls/sec |
| Native lib (future) | ~0.001-0.01ms | ~100,000-1,000,000 calls/sec |

## Protocol Extensions

### Streaming Output (Future)

SQX should support chunked/streaming output format:

```json
{"type":"frame","data":"..."}
{"type":"complete","value":42,"error":null}
```

### Capability Discovery (Future)

Modules can advertise capabilities:

```json
{"cmd":"capabilities"}
{"type":"capabilities","session":true,"streaming":false,"max_args":256}
```

## Summary

The SQX protocol transforms CLI plugins into a structured, typed runtime extension system:

1. **Use structured mode** for all new modules
2. **Use session mode** for performance-sensitive applications
3. **Always return valid envelopes** with correct types
4. **Validate arguments** before processing
5. **Handle errors gracefully** with descriptive messages