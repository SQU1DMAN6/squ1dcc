# SQU1DLang Programming Language

SQU1DLang is a dynamic programming language with a clean syntax. This document provides a comprehensive guide to the language's syntax and features.

## Table of Contents

- [Basic Syntax](#basic-syntax)
- [Data Types](#data-types)
- [Variables](#variables)
- [Functions](#functions)
- [Control Flow](#control-flow)
- [Data Structures](#data-structures)
- [Built-in Functions](#built-in-functions)
- [Operators](#operators)
- [Examples](#examples)

## Basic Syntax

SQU1DLang uses a simple, expression-based syntax where functions are key.

## Comments

Comments in SQU1DLang use the hash sign (`#`), and can be terminated with another hash sign.

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

Functions are declared using the `def` keyword, and may be assigned to a variable if one wishes to call them later:

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

Note that a `while (true)` loop should have a break or exit statement to prevent a stack overflow.

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

io.write(y) # null
io.write(z) # 20
```

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

SQU1DLang provides several built-in functions organized into different categories:

### I/O Functions:

```squ1d
var input = io.read()
var output = io.write(input)
```

Returns values to the console:

```squ1d
io.write("Hello, World!");
io.write("Value:", 42);
```

Prints values to the console:

```squ1d
io.echo("Hello, World!");
io.echo("Value:", 42);
```

#### `io.read([prompt])`

Reads input from the user:

```squ1d
var input = io.read();
var name = io.read("Enter your name: ");
```

#### `array.cat(value)`

Returns the length of a string or array:

```squ1d
var len = array.cat("hello");        # Returns 5
var count = array.cat([1, 2, 3]);    # Returns 3
```

#### `array.append(array, value)`

Adds an element to the end of an array:

```squ1d
var numbers = [1, 2, 3];
var extended = array.append(numbers, 4);  # Returns [1, 2, 3, 4]
```

#### `type.tp(value)`

Returns the type of a value as a string:

```squ1d
type.tp(42);        # Returns "Integer"
type.tp("hello");   # Returns "String"
type.tp([1, 2]);    # Returns "Array"
type.tp({});        # Returns "Object"
type.tp(true);      # Returns "Boolean"
```

### Math Functions

#### `math.abs(value)`

Returns the absolute value of a number:

```squ1d
math.abs(-5)    # Returns 5
math.abs(3.14)  # Returns 3.14
```

#### `math.sqrt(value)`

Returns the square root of a number:

```squ1d
math.sqrt(16)
math.sqrt(2)
```

#### `math.pow(base, exponent)`

Returns base raised to the power of exponent:

```squ1d
io.write(pow(2, 3));  # Returns 8
io.write(pow(3, 2));  # Returns 9
```

#### `math.sin(value)`, `math.cos(value)`

Mathematical constants:

```squ1d
math.pi()   # Returns 3.141592653589793
math.e()    # Returns 2.718281828459045
```

### String Functions

#### `string.upper(string)`

Converts a string to uppercase:

```squ1d
string.upper("hello")  # Returns "HELLO"
```

#### `string.lower(string)`

Converts a string to lowercase:

```squ1d
string.lower("WORLD")  # Returns "world"
```

#### `string.trim(string)`

Removes whitespace from both ends of a string:

```squ1d
string.trim("  hello  ")  # Returns "hello"
```

### System Functions

#### `os.env(key)`

Gets an environment variable:

```squ1d
os.env("HOME")  # Returns your home directory
```

#### `os.exec(command)`

Executes a system command:

```squ1d
os.exec("echo hello")  # Returns "hello"
```

#### `os.exit(status)`

Exits the program

```squ1d
if (err) {
    os.exit(1) # Exit status 1
} el {
    os.exit(0) # Exit status 0
}
```

#### `time.sleep(seconds)`

Pauses execution for the specified number of seconds:

```squ1d
time.sleep(1);  # Sleep for 1 second
```

#### `time.now()`

Returns the current timestamp:

```squ1d
time.now()  # Returns current Unix timestamp
```

### Package Management

#### `pkg.create(name, description)`

Creates a new package:

```squ1d
pkg.create("mypackage", "A sample package");
```

#### `pkg.list()`

Lists all available packages:

```squ1d
pkg.list()
```

#### `pkg.remove(name)`

Removes a package:

```squ1d
pkg.remove("mypackage");
```

### Type Conversion

#### `type.i2fl(integer)`

Converts an integer to a float:

```squ1d
type.i2fl(42)  # Returns 42.0
```

#### `type.fl2i(float)`

Converts a float to an integer:

```squ1d
type.fl2i(3.14)  # Returns 3
```

### Keyboard Events

```squ1d
keyboard.listen()
```

Constantly listens for keyboard events, often used in a `while` loop, returning a string value when a key is pressed, and returning `null` otherwise.

```squ1d
keyboard.read()
```

Reads for a single keyboard event, can be used outside of `while` loops, but holds up the process until a key is pressed, returning a string value when a key is pressed.

```squ1d
keyboard.stop()
```

Stops listening for keyboard events.

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
    while (i < cat(arr)) {
        total = total + arr[i];
        i = i + 1;
    }
    return total;
}

var numbers = [1, 2, 3, 4, 5];
var total = sumArray(numbers);
io.write("Sum:", total);
```

### Hash Map Example

```squ1d
var student = {
    "name": "John",
    "grades": [85, 92, 78, 96],
    "active": true
};

io.write("Student:", student["name"]);
io.write("Average grade: ", student["grades"][0]);
```

### Interactive Program

```squ1d
io.write("Welcome to SQU1DLang Calculator!");
var num1 = io.read("Enter first number: ");
var num2 = io.read("Enter second number: ");
var sum = num1 + num2;
io.write("The sum is: ", sum);
```

### Keyboard event listener program

Example using keyboard.read():

```squ1d
var key = keyboard.read()

io.write(key + " pressed")
```

Example using keyboard.listen():

```squ1d
io.write("Press a key to start...")
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
io.write("Exiting...")
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
squ1d++
```

### Running Files

To execute a SQU1DLang file:

```bash
squ1d++ filename.sqd
```

### Compiling to Executable

To compile a SQU1DLang file to a standalone executable:

```bash
squ1d++ -B input.sqd -o output
```

This creates a standalone executable that doesn't require Go to run.

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

This is the recommended way to structure modular code. All functions defined in the included file will be accessible through the namespace using dot notation.

**Example library file (`lib/math_utils.sqd`):**

```squ1d
var add = def(a, b) { a + b };
var max = def(arr) { 
    var m = arr[0];
    var i = 1;
    while (i < len(arr)) {
        if (arr[i] > m) { m = arr[i]; };
        i = i + 1;
    };
    m
};
```

The include system searches in the current directory, `./lib/` directory, and user's package cache.

---

_SQU1D++ SQU1DLang Compiler, version 1, written by Quan Thai._
