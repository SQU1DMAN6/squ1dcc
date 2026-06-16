# SQX Session Mode Guide

## Overview

Session mode is a persistent process model for SQX modules. Instead of spawning a new process for every function call, the module starts once and stays alive, communicating via JSON messages over stdin/stdout.

This eliminates process spawn overhead, enables stateful modules, and dramatically improves performance for real-time applications.

## When to Use Session Mode

### ✅ Use Session Mode For:
- **Terminal applications** — screen rendering, cursor positioning, raw input
- **Editors** — vim/emacs-style modal editing with continuous input
- **Real-time UIs** — dashboards, animations, TUI frameworks
- **Stateful modules** — keyboard state, clipboard, window management
- **High-frequency calls** — any module called 10+ times per second
- **Batch operations** — multiple sequential calls to the same module

### ❌ Don't Need Session Mode For:
- One-off file operations
- Stateless computations (math, string manipulation)
- Simple queries
- Debugging/prototyping

## Performance Comparison

| Scenario | Process-per-Call | Session Mode | Improvement |
|----------|-----------------|--------------|-------------|
| Single call | 5-50ms | 1-5ms | 5-10x |
| 100 calls | 500-5000ms | 10-100ms | 50x |
| 1000 calls | 5-50s | 100-1000ms | 50x |
| Editor loop (60fps) | Unusable | Viable | ∞ |

## How Session Mode Works

### Session Lifecycle

```
SQU1DCC                          SQX Module
  │                                  │
  │── start module --session ──────>│
  │                                  │ (module enters message loop)
  │── {"cmd":"call","fn":"func",    │
  │    "args":["x"]}               │
  │───────────────────────────────>│
  │                                  │ (processes call)
  │── {"ok":true,"value":...}     │
  │<───────────────────────────────│
  │                                  │
  │── {"cmd":"ping"}              │
  │───────────────────────────────>│
  │── {"ok":true,"value":"pong"}  │
  │<───────────────────────────────│
  │                                  │
  │── {"cmd":"shutdown"}          │
  │───────────────────────────────>│ (module exits)
  │                                  │
```

### Message Flow

1. SQU1DCC starts the module with `--session` flag
2. Module enters `sqx_serve_session()` loop
3. For each call, SQU1DCC sends a JSON request
4. Module processes and responds
5. On `shutdown` or stdin EOF, module exits

## Configuration

### CLI Flag

```bash
squ1dcc -sqx-session=auto myfile.sqd    # Default: try sessions, fall back to legacy
squ1dcc -sqx-session=always myfile.sqd   # Force session mode (fail if unsupported)
squ1dcc -sqx-session=legacy myfile.sqd   # Never use sessions
```

### Mode Options

| Mode | Behavior |
|------|----------|
| `auto` (default) | Try session mode when module supports it; fall back to process-per-call |
| `always` | Require session mode; fail if module doesn't support it |
| `legacy` | Never use sessions; process-per-call only |

## State Management

### What State Persists

In session mode, module state persists between calls:

- Global/static variables
- Open file handles
- Console modes (raw mode, cursor position)
- Allocated memory
- Connection pools

### Example: Counter

```c
static int counter = 0;

static void cmd_get_count(int argc, char **argv) {
    (void)argc; (void)argv;
    sqx_write_int(counter);
}

static void cmd_increment(int argc, char **argv) {
    counter++;
    sqx_write_bool(1);
}
```

This counter persists across calls within the same session.

### Resetting State

Include a reset function for modules with mutable state:

```c
static void cmd_reset(int argc, char **argv) {
    (void)argc; (void)argv;
    counter = 0;
    sqx_write_bool(1);
}
```

## Session Pool

SQU1DCC provides a session pool for managing multiple modules:

```go
// Create a pool with max 3 concurrent sessions
pool := sqx.NewSessionPool(cfg, 3)

// Acquire a session
sess, _ := pool.Acquire()
result, _ := sess.Call("ping", nil)

// Return to pool
pool.Release(sess)

// Close all on exit
defer pool.CloseAll()
```

### Pool Benefits

- Reuses sessions across calls
- Limits concurrent resource usage
- Auto-closes dead sessions
- Graceful fallback on failure

## Writing Session-Compatible Modules

### C Modules (using sqx_runtime.h)

Session mode is automatic. Call `sqx_main()` from `main()`:

```c
#include "sqx_runtime.h"

// Your handlers...

SQX_BEGIN_MANIFEST
    SQX_REGISTER(your_func, "structured")
SQX_END_MANIFEST

int main(int argc, char **argv) {
    return sqx_main(argc, argv);
}
```

The `sqx_main()` function handles `--session` automatically.

### Go Modules (using sqx package)

Session mode is built into the Go SQX runtime:

```go
func main() {
    module := NewSQXModule("mymodule")
    module.RegisterMany(
        SQXMethod{
            Name:   "ping",
            Return: SQXReturnString,
            Handle: func(args []string) (interface{}, error) {
                return "pong", nil
            },
        },
    )

    if len(os.Args) > 1 && os.Args[1] == "--session" {
        os.Exit(module.Serve(os.Stdin, os.Stdout))
    }
    os.Exit(module.Run(os.Args[1:], os.Stdout, os.Stderr))
}
```

## Troubleshooting

### Session won't start

**Symptoms:**
- `"SQX session did not respond to ping"` error
- Module starts but immediately exits

**Causes & fixes:**
1. Module doesn't support `--session` flag
   - Add session mode support (see C/Go examples above)
2. Module crashes on startup
   - Run manually: `./module.sqx --session`
   - Check stderr output
3. Ping timeout
   - Increase timeout in `SessionConfig`
   - Ensure module writes response promptly

### Session disconnects

**Symptoms:**
- Calls start failing mid-session
- Process disappears

**Causes & fixes:**
1. Module crashed internally
   - Check for segfaults/unhandled errors
   - Add error handling in module code
2. Stdin/stdout pipe broken
   - Ensure JSON messages are correctly formatted
   - Check for extra newlines or truncated messages
3. Module called shutdown
   - Remove accidental shutdown calls

### Performance issues

**Symptoms:**
- Calls are slower than expected
- High CPU usage

**Causes & fixes:**
1. JSON parsing overhead
   - Minimize large JSON payloads
   - Use compact JSON (no unnecessary whitespace)
2. WinAPI calls on every function
   - Cache console handles
   - Batch operations where possible
3. Multiple sessions for same module
   - Use session pool to reuse sessions
   - Set appropriate pool size

## Best Practices

### 1. Always Support Fallback

Design modules to work in both session and legacy mode:

```c
int main(int argc, char **argv) {
    return sqx_main(argc, argv); // Handles both modes
}
```

### 2. Handle Session Gracefully

On shutdown, release resources:

```go
func (m *Module) cleanup() {
    // Close handles, free memory, etc.
}
```

### 3. Use Heartbeat/Ping

Sessions respond to `ping` for health checks. No additional code needed.

### 4. Validate State

Include a state validation function:

```c
static void cmd_validate_state(int argc, char **argv) {
    if (!g_initialized) {
        sqx_write_error("module not initialized");
        return;
    }
    sqx_write_bool(1);
}
```

### 5. Monitor Session Lifecycle

Log session events for debugging:

```go
log.Printf("Session started for %s", modulePath)
defer log.Printf("Session ended for %s", modulePath)
```

## Summary

Session mode transforms SQX from a simple plugin system into a high-performance runtime extension protocol:

- **10-50x faster** than process-per-call
- **Enables real-time applications** (editors, TUIs)
- **Maintains state** across calls
- **Automatic** with sqx_runtime.h and Go SQX runtime
- **Configurable** per-module or globally

When building new SQX modules, always include session mode support. The performance benefits are substantial and the implementation cost is near zero with the SQX runtime library.