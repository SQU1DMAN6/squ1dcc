# Writing SQX Modules in C

## Overview

SQX (SQU1D Extension) modules are executable plugins that extend SQU1DLang with native functionality. Writing SQX modules in C gives you direct access to operating system APIs, maximum performance, and minimal memory footprint.

This guide covers writing C-based SQX modules using the SQX Runtime Library (`sqx_runtime.h`).

## Quick Start

### Minimal Example

```c
#include "sqx_runtime.h"

static void cmd_ping(int argc, char **argv) {
    (void)argc; (void)argv;
    sqx_write_string("pong");
}

SQX_BEGIN_MANIFEST
    SQX_REGISTER(ping, "string")
SQX_END_MANIFEST

int main(int argc, char **argv) {
    return sqx_main(argc, argv);
}
```

### Building

```bash
# With sqx_runtime.h in a known include path
gcc -O2 -I/path/to/include -o mymodule.sqx main.c

# With SQX_RUNTIME_DIR environment variable
SQX_RUNTIME_DIR=/path/to/include gcc -O2 -I"$SQX_RUNTIME_DIR" -o mymodule.sqx main.c
```

### Testing from SQU1DLang

```sqd
pkg.include("path/to/mymodule.sqx", "my")
var result = my.ping()
io.echo(result)  # Output: pong
```

## SQX Runtime Library

The `sqx_runtime.h` header provides everything needed to build a compliant SQX module:

### Function Registration

Use the `SQX_BEGIN_MANIFEST` / `SQX_END_MANIFEST` macros with `SQX_REGISTER`:

```c
SQX_BEGIN_MANIFEST
    SQX_REGISTER(function_name, "return_type")
    SQX_REGISTER(another_function, "structured")
SQX_END_MANIFEST
```

The macro creates a handler named `cmd_function_name` — your implementation must match this naming convention.

### Return Mode Types

| Return Mode | Description | Handler Calls |
|-------------|-------------|---------------|
| `"string"` | Plain string output | `sqx_write_string(val)` |
| `"structured"` | `{ok, value, error}` envelope | `sqx_write_*` functions |
| `"int"` | Integer value | `sqx_write_int(val)` |
| `"bool"` | Boolean value | `sqx_write_bool(val)` |
| `"json"` | Raw JSON output | `sqx_write_json(val)` |
| `"null"` | Null output | `sqx_write_null()` |
| `"auto"` | Auto-detect type | Any `sqx_write_*` |

### Structured Result Functions

All structured output functions write JSON to stdout in the format:
```json
{"ok":true,"value":... ,"error":null}
```

| Function | Description |
|----------|-------------|
| `sqx_write_structured(ok, value_json, error_msg)` | Write arbitrary structured result |
| `sqx_write_string(str)` | Success with string value (auto-escaped) |
| `sqx_write_int(val)` | Success with integer value |
| `sqx_write_bool(val)` | Success with boolean value |
| `sqx_write_null()` | Success with null value |
| `sqx_write_json(json_str)` | Success with raw JSON value |
| `sqx_write_json_object(body)` | Success with JSON object `{body}` |
| `sqx_write_array(body)` | Success with JSON array `[body]` |
| `sqx_write_error(msg)` | Error result (auto-escaped) |

### JSON String Escaping

`sqx_write_string()` and `sqx_write_error()` automatically escape JSON special characters. The underlying `sqx_escape_json()` function is also available for custom use.

## Session Mode

SQX modules can operate in persistent session mode, where the module process stays alive and communicates via JSON-over-stdin/stdout.

### How It Works

1. SQU1DCC starts the module with `--session` flag
2. Module enters the session message loop (`sqx_serve_session()`)
3. SQU1DCC sends JSON requests on stdin
4. Module processes and responds on stdout
5. Module stays alive until `{"cmd":"shutdown"}` is received

The session protocol is fully handled by `sqx_runtime.h` — no additional code needed.

### Request Format

```json
{"cmd":"call","fn":"function_name","args":["arg1","arg2"]}
{"cmd":"ping"}
{"cmd":"shutdown"}
```

### Response Format

```json
{"ok":true,"value":"result","error":null}
```

## Complete Example: File Statistics Module

```c
/*
 * fstats.sqx — File Statistics SQX Module
 *
 * BUILD:
 *   gcc -O2 -I/path/to/include -o fstats.sqx main.c
 */

#include "sqx_runtime.h"
#include <sys/stat.h>

/* count_lines(path) — Count lines in a file */
static void cmd_count_lines(int argc, char **argv) {
    if (argc < 1) {
        sqx_write_error("count_lines requires 1 argument (path)");
        return;
    }

    FILE *f = fopen(argv[0], "r");
    if (!f) {
        sqx_write_error("could not open file");
        return;
    }

    long count = 0;
    int ch;
    while ((ch = fgetc(f)) != EOF) {
        if (ch == '\n') count++;
    }
    fclose(f);

    sqx_write_int(count);
}

/* file_size(path) — Get file size in bytes */
static void cmd_file_size(int argc, char **argv) {
    if (argc < 1) {
        sqx_write_error("file_size requires 1 argument (path)");
        return;
    }

    struct stat st;
    if (stat(argv[0], &st) != 0) {
        sqx_write_error("could not stat file");
        return;
    }

    sqx_write_int((long long)st.st_size);
}

/* file_info(path) — Get file metadata as structured result */
static void cmd_file_info(int argc, char **argv) {
    if (argc < 1) {
        sqx_write_error("file_info requires 1 argument (path)");
        return;
    }

    struct stat st;
    if (stat(argv[0], &st) != 0) {
        sqx_write_error("could not stat file");
        return;
    }

    char buf[256];
    snprintf(buf, sizeof(buf),
        "\"size\":%lld,\"mode\":%o,\"is_dir\":%s",
        (long long)st.st_size,
        st.st_mode & 0777,
        S_ISDIR(st.st_mode) ? "true" : "false");
    sqx_write_json_object(buf);
}

SQX_BEGIN_MANIFEST
    SQX_REGISTER(count_lines, "int")
    SQX_REGISTER(file_size, "int")
    SQX_REGISTER(file_info, "structured")
SQX_END_MANIFEST

int main(int argc, char **argv) {
    return sqx_main(argc, argv);
}
```

## Windows-Specific Considerations

### Console API Modules

For Windows console modules, include `<windows.h>` before `sqx_runtime.h`:

```c
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include "sqx_runtime.h"
```

The `sqx_runtime.h` header already defines `WIN32_LEAN_AND_MEAN` and includes `<windows.h>`.

### Building on Windows

```batch
REM MSVC
cl /O2 /I"..\..\src\include" /Feterminal_win.sqx main.c

REM MinGW
gcc -O2 -I"../../src/include" -o terminal_win.sqx main.c
```

## Best Practices

### 1. Always Validate Arguments

```c
static void cmd_example(int argc, char **argv) {
    if (argc < 2) {
        sqx_write_error("example requires 2 arguments");
        return;
    }
    // argv[0], argv[1] are safe to use
}
```

### 2. Use Structured Returns for Fallible Operations

```c
static void cmd_risky(int argc, char **argv) {
    if (some_condition) {
        sqx_write_error("something went wrong");
        return;
    }
    sqx_write_string("success");
}
```

### 3. Use Void Return Type

SQX C handlers return `void`. All output is done via `sqx_write_*` functions.

### 4. Static State is Safe in Session Mode

Session mode keeps the process alive, so static variables persist across calls:

```c
static int call_count = 0;

static void cmd_count_calls(int argc, char **argv) {
    call_count++;
    sqx_write_int(call_count);
}
```

### 5. Include Session Mode Support

The `sqx_main()` dispatcher automatically handles session mode. Just ensure your `main()` calls `sqx_main()`.

## Troubleshooting

### Module not found by SQU1DCC

- Ensure the `.sqx` file is executable (`chmod +x module.sqx`)
- Ensure the module is in a path accessible to SQU1DCC
- Verify the manifest output: `./module.sqx __sqx_manifest__`

### Structured result parsing fails

- Check that JSON output is valid: `{"ok":true,"value":...,"error":null}`
- Ensure `ok` is a boolean (`true`/`false`), not a string
- Ensure `error` is `null` or a string, not missing or a different type

### Session mode not working

- Verify the module accepts `--session` flag
- Check that responses are written to stdout
- Ensure each response is on a single line

## Summary

Writing SQX modules in C is straightforward with the SQX Runtime Library:

1. Include `"sqx_runtime.h"`
2. Implement handler functions matching `cmd_<name>(int argc, char **argv)` pattern
3. Register functions with `SQX_REGISTER` inside `SQX_BEGIN_MANIFEST`/`SQX_END_MANIFEST`
4. Call `sqx_main(argc, argv)` from `main()`
5. Use `sqx_write_*` functions for structured output
6. Build and test with SQU1DCC