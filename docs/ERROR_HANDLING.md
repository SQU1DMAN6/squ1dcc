# SQU1DLang Error Handling Standard

## Overview

The SQU1DLang error handling standard provides a consistent, predictable approach to handling errors across the language. All fallible functions (those that can fail) must follow a specific return pattern.

## Standard Return Pattern

All fallible functions **MUST** return the following structure:

```sqd
{
    "ok": boolean,
    "value": any,
    "error": string | null
}
```

### Fields

- **`ok`**: Boolean indicating whether the operation succeeded (true) or failed (false)
- **`value`**: The successful result if `ok == true`, or `null` if `ok == false`
- **`error`**: Error message string if `ok == false`, or `null` if `ok == true`

## Recommended Usage Patterns

### Method 1: NOT RECOMMENDED - Unsafe

**DO NOT USE** - This pattern has critical flaws:

```sqd
if ((<< divide(51, 3)) == null) {
    # PROBLEM 1: divide() is called TWICE (once in <<, once below)
    # PROBLEM 2: Mixes execution with error extraction
    io.write(divide(51, 3).value)
}
```

**Issues:**
- Function is executed twice
- Unclear control flow
- Inconsistent state between checks
- Hard to debug

### Method 2: RECOMMENDED - Clean Standard

✓ **USE THIS** - Single execution, clear semantics:

```sqd
var result = divide(51, 3)

if (result.ok) {
    io.echo("Success: ")
    io.echo(result.value)
    io.echo("\n")
} el {
    io.echo("Error: ")
    io.echo(result.error)
    io.echo("\n")
}
```

**Advantages:**
- Single execution
- Explicit control flow
- Predictable debugging behavior
- Canonical approach
- Clear, readable code

### Method 3: ACCEPTABLE - Shorthand

**USE THIS IF** - Result already exists:

```sqd
var result = divide(51, 3)

if (<< result == null && <<< result == true) {
    # No error occurred
    io.echo(result.value)
    io.echo("\n")
} el {
    # Error occurred
    io.echo(result.error)
    io.echo("\n")
}
```

**Advantages:**
- Safe because result already exists (no re-execution)
- Shorthand for error checking
- Optional syntactic sugar on top of Method 2

## Enforcement Rules

### RULE 1: Never Execute Functions Inside `<<`

WRONG:
```sqd
if ((<< divide(x, y)) == null) {
    # Function executed here
}
```

CORRECT:
```sqd
var result = divide(x, y)
if ((<< result) == null) {
    # Safe, result already computed
}
```

### RULE 2: Use `ok` Field for Logic Decisions

WRONG:
```sqd
if (result.error == null) {
    # Not idiomatic
}
```

✓ CORRECT:
```sqd
if (result.ok) {
    # Clear and idiomatic
}
```

### RULE 3: Avoid Duplicate Execution

WRONG:
```sqd
if (someFunction().ok) {
    var value = someFunction().value  # Executed twice!
}
```

CORRECT:
```sqd
var result = someFunction()
if (result.ok) {
    var value = result.value  # Single execution
}
```

## Implementation Examples

### Simple Function with Error Handling

```sqd
divide >> (a, b) {
    if (b == 0) {
        return {
            "ok": false,
            "value": null,
            "error": "Division by zero"
        }
    }
    
    return {
        "ok": true,
        "value": a / b,
        "error": null
    }
}

# Usage
var result = divide(10, 0)
if (result.ok) {
    io.echo("Result: ")
    io.echo(result.value)
} el {
    io.echo("Error: ")
    io.echo(result.error)
}
```

### Validation Function

```sqd
validatePositive >> (x) {
    if (x < 0) {
        return {
            "ok": false,
            "value": null,
            "error": "Value must be positive"
        }
    }
    
    return {
        "ok": true,
        "value": x,
        "error": null
    }
}
```

### Chaining Operations

```sqd
var result1 = divide(100, 5)
if (result1.ok) {
    var result2 = divide(50, result1.value)
    if (result2.ok) {
        io.echo("Final result: ")
        io.echo(result2.value)
    } el {
        io.echo("Second operation failed")
    }
} el {
    io.echo("First operation failed")
}
```

## Error Extraction Operators

### `<<` Operator (Error Pipe)

Extracts the error value from a result object. Returns the error if present, or NULL if no error.

```sqd
var result = divide(10, 0)

if (<< result == null) {
    # No error
} el {
    # Error occurred
}
```

### `<<<` Operator (OK Pipe)

Will extract the `ok` field value for convenience:

```sqd
var result = divide(10, 2)

if (<<< result) {
    # ok == true
} el {
    # ok == false
}
```

## Testing

See `examples/error_handling/main.sqd` for comprehensive test cases demonstrating:
- Method 2 (Recommended)
- Method 3 (Acceptable)
- Error cases
- Chained operations
- Multiple error scenarios

Run tests with:
```bash
./squ1dcc examples/error_handling/main.sqd
```

## Summary

- **Method 2** is the **canonical approach** - use it by default
- **Method 3** is acceptable shorthand for checking existing results
- **Method 1** must be avoided completely
- All fallible functions return `{ok, value, error}`
- Single execution, predictable flow, explicit error handling
- Consistent across all SQU1DLang codebases
