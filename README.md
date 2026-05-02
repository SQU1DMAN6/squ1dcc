# SQU1DLang Programming Language

SQU1DLang is a dynamic programming language that runs on the SQU1D++ compiler + VM runtime. This document is a practical guide to the currently implemented syntax and features.

## Table of Contents

- [Basic Syntax](#basic-syntax)
- [Data Types](#data-types)
- [Variables](#variables)
- [Functions](#functions)
- [Control Flow](#control-flow)
- [Data Structures](#data-structures)
- [Built-in Functions](#built-in-functions)
- [Operators](#operators)
- [Language Features](#language-features)
- [Compiler Architecture](#compiler-architecture)
- [Getting Started](#getting-started)

## Basic Syntax

SQU1DLang uses a simple, expression-based syntax where functions are key.

## Comments

Comments use `# ... #` delimiters.

```squ1d
# this is a comment #
var x = 1
```

### Statement Termination

Statements can be terminated with semicolons (`;`), but they are optional in many contexts.

## Data Types

SQU1DLang supports the following primitive data types:

### Integers

```squ1d
42
-17
0
```

### Floats

Floats can be written and automatically detected if they have a decimal point:

```squ1d
42.37
-17.25
1.123
29.24837
```

### Hex

Hex (hexadecimal) values are whole numbers prefixed with `0x`. They are useful for working with byte values, color codes, and other hex-based data:

```squ1d
0xFF        # 255 in decimal
0xABCDEF    # RGB color code
0x10        # 16 in decimal
0x0         # Zero
```

Hex values are displayed in lowercase hex notation (e.g., `0xff`). For more details, see [HEX_TYPE.md](docs/HEX_TYPE.md).

### Booleans

```squ1d
true
false
```

### Strings

```squ1d
"Hello, World!"
""
'Hello, World!'
`Hello, World!`
"I said \"Hello\""
'I said \'Hello\''
`I said \`Hello\``
`"I said 'Hello'"`
```

### Null

```squ1d
null
```

## Variables

Variables are declared using the `var` keyword:

```squ1d
var x = 42;
var name = "SQU1DLang";
var isActive = true;
```

And can be reassigned using `=`:

```squ1d
x = x + 1;
y = "Hello";
isActive = false;
```

Variable names must start with a letter or underscore and can contain letters, digits, and underscores.

## Suppression

The `suppress` keyword can be used to silence the output of a command in SQU1DLang, but still evaluating it:

```squ1d
suppress x = type.tp(y)
suppress 123 + 456
```

## Functions

### Function Declaration

Functions can be declared in two supported forms:

```squ1d
var add = def(a, b) {
    return a + b
}

addNamed >> (a, b) {
    return a + b
}
```

### Function Calls

```squ1d
var result = add(5, 3);
subtract(10, 4);
```

### Anonymous Functions

```squ1d
def(x, y) {
    return x * y;
}(3, 4);
```

### Higher-Order Functions

Functions are first-class objects and can be passed as arguments if one wishes to call them later:

```squ1d
var apply = def(fn, x, y) {
    return fn(x, y);
}

var result = apply(add, 10, 20);
```

## Control Flow

### If-Else Statements

```squ1d
if (x > 0) {
    "Positive";
} el {
    "Negative or zero";
}
```

### Conditional Expressions

```squ1d
var message = if (x > 0) { "Positive" } el { "Negative" };
```

### For loops

```squ1d
var new_array = [1, 2, 3, 4, 5]

for (var i = 0; i < array.cat(new_array); i = i + 1) {
        io.echo(new_array[i])
}
```

### While loops

While loops can be written in two ways:

1. Using condition

```squ1d
var new_array = [1, 2, 3, 4, 5]
var i = 0

while (i < array.cat(new_array)) {
    io.echo(new_array[i], "\n")
    suppress i = i + 1
}
```

2. Using `true` condition and `break` statement

```squ1d
var new_array = [1, 2, 3, 4, 5]
var i = 0

while (true) {
    suppress if (i >= array.cat(new_array)) {
        break
    }
    suppress io.echo(new_array[i], "\n")
    suppress i = i + 1
}

```

Note that `while (true)` loops should still have an exit condition. Runtime guards (`SysMaxInstructionCount`, `SysMaxLoopIterations`) stop runaway loops, but explicit termination is recommended.

## Error Handling

### Unblock

The `unblock` keyword allows the code to continue executing even if a function returns an error.

```squ1d
unblock var x = def() { return y }

io.echo("Hi")
```

### Error Pipe

The error pipe keyword (`<<`) is used to retrieve the error returned by a function instead of its value.

```squ1d
unblock var func = def() { return y }
unblock var y = << func()
io.echo(y)
```

If the function executes successfully, the error pipe returns `null`.

```squ1d
var func = def() { return 20 }
var y = << func()
var z = func()

io.echo(y) # null
io.echo(z) # 20
```

## Performance

SQU1DLang performance is evaluated with a compiler/VM pipeline (lexer, parser, compiler, bytecode VM), which is more comparable to JITed runtimes like LuaJIT or Graal.

- Loop handling is now bound-checked by `SysMaxInstructionCount` and `SysMaxLoopIterations` to avoid runaway `while true` cycles and stack overflow.
- Object allocation uses memory pooling in `object.NewArray` / `object.NewHash` and reuse via `ReleaseArray` / `ReleaseHash`.
- Evaluator-style AST interpretation is being migrated to compiled VM execution in REPL for better throughput and predictability.

Compared with C++ and Rust:

- C++/Rust has raw native speed and manual control of memory; SQU1DLang offers managed heap and GC-friendly object reuse with safety checks.
- SQU1DLang has lower latency for quick scripting, and compile+VM avoids interpreter overhead.
- For heavy numeric loops, build `-B` to embed and avoid runtime lexer/parser overhead.

## Data Structures

### Arrays

Arrays are created using square brackets:

```squ1d
var numbers = [1, 2, 3, 4, 5];
var mixed = [1, "hello", true];
var empty = [];
```

### Array Indexing

```squ1d
var first = numbers[0];
var last = numbers[4];
```

### Hash Maps (Objects)

Hash maps are created using curly braces with key-value pairs:

```squ1d
var person = {
    "name": "Alice",
    "age": 30,
    "active": true
};
```

### Hash Map Access

```squ1d
var name = person["name"];
var age = person["age"];
```

## Built-in Functions

Built-ins are class-scoped and accessed with dot notation.

### `io`

- `io.read([prompt])` reads input and auto-parses to `Integer`, `Float`, or `String`.
- `io.write(...)` returns a single joined `String` (it does not print by itself).
- `io.echo(...)` prints to output.

### `type`

- `type.tp`, `type.i2fl`, `type.fl2i`, `type.s2i`, `type.s2fl`, `type.d2s`

### `math`

- `math.abs`, `math.sqrt`, `math.pow`, `math.rand`, `math.sin`, `math.cos`, `math.pi`, `math.e`

### `time`

- `time.now`, `time.sleep`

### `os`

- `os.env`, `os.exec`, `os.exit`, `os.iRuntime`

### `string`

- `string.upper`, `string.lower`, `string.trim`, `string.sepr`

### `array`

- `array.append`, `array.pop`, `array.remove`, `array.cat`, `array.join`

### `file`

- `file.read`, `file.write`

### `pkg`

- `pkg.include(path)` returns file contents as `String`.
- `pkg.include(path, namespace)` imports top-level functions under `namespace`.
- `pkg.create`, `pkg.list`, `pkg.remove`

### `sys`

- `sys.gc`, `sys.set_overflow_size`, `sys.get_overflow_size`, `sys.list`

### `keyboard`

- `keyboard.read`, `keyboard.listen`, `keyboard.stop`, `keyboard.on`, `keyboard.off`

## Operators

### Arithmetic Operators

```squ1d
+   # Addition
-   # Subtraction
*   # Multiplication
/   # Division
%   # Modulo (remainder)
```

### Comparison Operators

```squ1d
==  # Equal to
!=  # Not equal to
<   # Less than
>   # Greater than
<=  # Less than or equal to
>=  # Greater than or equal to
```

### Logical Operators

```squ1d
!   # Logical NOT
and # Logical AND
or  # Logical OR
```

### Assignment Operator

```squ1d
=   # Assignment (for variable reassignment)
```

### String Concatenation

```squ1d
var greeting = "Hello" + " " + "World";
```

### Array Processing

```squ1d
var sumArray = def (arr) {
    var total = 0;
    var i = 0;
    while (i < array.cat(arr)) {
        total = total + arr[i];
        i = i + 1;
    }
    return total;
}

var numbers = [1, 2, 3, 4, 5];
var total = sumArray(numbers);
io.echo("Sum:", total);
```

### Hash Map Example

```squ1d
var student = {
    "name": "John",
    "grades": [85, 92, 78, 96],
    "active": true
};

io.echo("Student:", student["name"]);
io.echo("Average grade: ", student["grades"][0]);
```

### Interactive Program

```squ1d
io.echo("Welcome to SQU1DLang Calculator!");
var num1 = io.read("Enter first number: ");
var num2 = io.read("Enter second number: ");
var sum = num1 + num2;
io.echo("The sum is: ", sum);
```

### Keyboard event listener program

Example using keyboard.read():

```squ1d
var key = keyboard.read()

io.echo(key + " pressed")
```

Example using keyboard.listen():

```squ1d
io.echo("Press a key to start...")
var key = null

while (true) {
    suppress key = keyboard.listen()

    if (type.tp(key) == "String") {
        io.echo(key + " pressed\r\n")
    }

    suppress if (key == "KeyQ" or key == "KeyCtrl+C") {
        break
    } el {
        continue
    }
}

suppress keyboard.stop()
io.echo("Exiting...")
os.exit(0)
```

## Language Features

- **Dynamic Typing**: Variables can hold values of any type
- **First-class Functions**: Functions can be assigned to variables and passed as arguments
- **Closures**: Functions capture their lexical environment
- **Garbage Collection**: Automatic memory management
- **REPL Support**: Interactive read-eval-print loop for testing code
- **Bytecode Compilation**: Code is compiled to bytecode for efficient execution
- **Virtual Machine**: Custom VM for executing compiled bytecode
- **Package System**: Built-in package management and module system
- **File Includes**: Support for including other SQU1DLang files
- **Standalone Executables**: Compile to native executables

## Compiler Architecture

SQU1DLang is implemented as a complete compiler called Squ1d++ with the following components:

- **Lexer**: Tokenizes source code into tokens
- **Parser**: Builds an Abstract Syntax Tree (AST) from tokens
- **Compiler**: Compiles AST to bytecode instructions
- **Virtual Machine**: Executes bytecode instructions
- **Object System**: Runtime object representation and garbage collection
- **Built-in Functions**: Extensive library of built-in functions
- **Package Manager**: Built-in package creation and management

## Getting Started

### Interactive REPL

To start an interactive session where you can type SQU1DLang code and see the results immediately:

```bash
squ1dcc
```

### Running Files

To execute a SQU1DLang file:

```bash
squ1dcc filename.sqd
```

### Compiling to Executable

To compile a SQU1DLang file to a standalone executable:

```bash
squ1dcc -B input.sqd -o output
```

This creates a standalone executable that doesn't require Go to run.
The produced binary embeds the runtime of the `squ1dcc` binary used during build.

### Package Management

SQU1DLang includes a built-in package management system:

```squ1d
# Create a new package
pkg.create("mypackage", "A sample package");

# List available packages
pkg.list()

# Remove a package
pkg.remove("mypackage");
```

### File Includes

You can include other SQU1DLang files using the `pkg.include()` function. There are two modes:

#### Including with Return Value

Returns the file contents as a string:

```squ1d
var content = pkg.include("library.sqd");
```

#### Including with Namespace (Recommended for Libraries)

Include functions from another file and access them via a namespace:

```squ1d
pkg.include("lib/math_utils.sqd", "math");
var result = math.add(5, 10);
var maxVal = math.max([1, 5, 3, 9]);
```

This is the recommended way to structure modular code. Top-level functions in the included file (both `var fn = def(...)` and `fn >> (...)`) are exported through the namespace via dot notation.

**Example library file (`lib/math_utils.sqd`):**

```squ1d
var add = def(a, b) { a + b };
var max = def(arr) { 
    var m = arr[0];
    var i = 1;
    while (i < array.cat(arr)) {
        if (arr[i] > m) { m = arr[i]; };
        i = i + 1;
    };
    m
};
```

For `pkg.include(path, namespace)`, include resolution checks the provided path, then paths relative to the caller (including caller `lib/`), then `./lib/`.

### SQX Plugins (Extensions)

You can also include `.sqx` plugin manifests with the same namespace form:

```squ1d
pkg.include("lib/tooling.sqx", "tooling");
var sum = tooling.sumTwo(2, 3);
```

An `.sqx` module is executed as an external native module. The runtime asks it for
its exported functions, then calls those functions via a small CLI protocol.

When a module is loaded, SQU1D++ runs:
- `<module>.sqx __sqx_manifest__`

The module must print a JSON manifest like:

```json
{
  "version": 1,
  "functions": {
    "sumTwo": {
      "return": "int"
    }
  }
}
```

When a function is called, SQU1D++ runs:
- `<module>.sqx __sqx_call__ <functionName> <arg1> <arg2> ...`

The module writes the result to stdout.

Supported function fields:
- `return`: output parsing mode (`auto`, `string`, `raw`, `int`, `float`, `bool`, `null`, `json`).

Typed SQX arguments:
- Runtime now forwards arrays/objects as typed payloads for executable `.sqx` modules.
- In Go SQX templates/runtime helpers, use `SQXArgAny`, `SQXArgBool`, `SQXArgInt`, and `SQXArgString` to decode non-string values safely.

Backward compatibility:
- JSON command manifests are still supported (`exec`, `append_args`, `env`), but the preferred model is a native executable `.sqx` module.

Binary-only SQX workflow (no SQU1DCC source required):

```bash
# Generate a standalone SQX template
squ1dcc sqx init --lang go --name myplugin --out ./myplugin

# Build it into native machine code (.sqx executable)
squ1dcc sqx build --lang go --src ./myplugin --out ./myplugin.sqx

# Alias command:
squ1dcc sqx compile --lang go --src ./myplugin --out ./myplugin.sqx
```

The generated Go template includes a local SQX runtime helper file, so plugin
authors do not need to import `squ1d++/...` packages.

`sqx init` supports: `go`, `c`, `cpp`, `shell`.
`sqx build` supports: `go`, `c`, `cpp`, `shell`.

For standalone builds, namespaced `.sqx` includes are rewritten to `pkg.load_sqx("<absolute-path>")` during include expansion.

### Runtime Note for Included Functions

Namespace imports from `pkg.include(path, namespace)` currently use the evaluator compatibility path for imported function bodies. Most language features work as expected, but advanced control-flow behavior can differ from fully compiled top-level code in some edge cases.

---

_SQU1D++ SQU1DLang Compiler, version 1.8.0, written by Quan Thai._
