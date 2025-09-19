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

Floats can be written with or without quotes:

```squ1d
'42.5      # Quoted float
42.5       # Unquoted float
'-17.25    # Negative quoted float
-17.25     # Negative unquoted float
'1.123     # Quoted decimal
1.123      # Unquoted decimal
'29.24837  # Quoted float
29.24837   # Unquoted float
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

Variable names must start with a letter or underscore and can contain letters, digits, and underscores.

## Functions

### Function Declaration

Functions are declared using the `def` keyword, and may be assigned to a variable:

```squ1d
def(x, y) {
    return x + y;
}
```

### Function Calls

```squ1d
var result = add(5, 3);
```

### Anonymous Functions

```squ1d
def(x, y) {
    return x * y;
}();
```

### Higher-Order Functions

Functions are first-class objects and can be passed as arguments:

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
    write("Positive");
} el {
    write("Negative or zero");
}
```

Note: The `else` keyword is shortened to `el` in SQU1DLang.

### Conditional Expressions

```squ1d
var message = if (x > 0) { "Positive" } el { "Negative" };
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

### Core Functions

#### `write(...args)`

Prints values to the console:

```squ1d
write("Hello, World!");
write("Value:", 42);
```

#### `read([prompt])`

Reads input from the user:

```squ1d
var input = read();
var name = read("Enter your name: ");
```

#### `cat(value)`

Returns the length of a string or array:

```squ1d
var len = cat("hello");        # Returns 5
var count = cat([1, 2, 3]);    # Returns 3
```

#### `append(array, value)`

Adds an element to the end of an array:

```squ1d
var numbers = [1, 2, 3];
var extended = append(numbers, 4);  # Returns [1, 2, 3, 4]
```

#### `tp(value)`

Returns the type of a value as a string:

```squ1d
tp(42);        # Returns "Integer"
tp("hello");   # Returns "String"
tp([1, 2]);    # Returns "Array"
tp({});        # Returns "Object"
tp(true);      # Returns "Boolean"
```

### Math Functions

#### `abs(value)`

Returns the absolute value of a number:

```squ1d
write(abs(-5));    # Returns 5
write(abs(3.14));  # Returns 3.14
```

#### `sqrt(value)`

Returns the square root of a number:

```squ1d
write(sqrt(16));   # Returns 4
write(sqrt(2));    # Returns 1.4142135623730951
```

#### `pow(base, exponent)`

Returns base raised to the power of exponent:

```squ1d
write(pow(2, 3));  # Returns 8
write(pow(3, 2));  # Returns 9
```

#### `sin(value)`, `cos(value)`

Trigonometric functions:

```squ1d
write(sin(0));     # Returns 0
write(cos(0));     # Returns 1
```

#### `pi`, `e`

Mathematical constants:

```squ1d
write(pi);         # Returns 3.141592653589793
write(e);          # Returns 2.718281828459045
```

### String Functions

#### `upper(string)`

Converts a string to uppercase:

```squ1d
write(upper("hello"));  # Returns "HELLO"
```

#### `lower(string)`

Converts a string to lowercase:

```squ1d
write(lower("WORLD"));  # Returns "world"
```

#### `trim(string)`

Removes whitespace from both ends of a string:

```squ1d
write(trim("  hello  "));  # Returns "hello"
```

### System Functions

#### `env(key)`

Gets an environment variable:

```squ1d
write(env("HOME"));  # Returns your home directory
```

#### `exec(command)`

Executes a system command:

```squ1d
write(exec("echo hello"));  # Returns "hello"
```

#### `sleep(seconds)`

Pauses execution for the specified number of seconds:

```squ1d
sleep(1);  # Sleep for 1 second
```

#### `now()`

Returns the current timestamp:

```squ1d
write(now());  # Returns current Unix timestamp
```

### Package Management

#### `pkg_create(name, description)`

Creates a new package:

```squ1d
pkg_create("mypackage", "A sample package");
```

#### `pkg_list()`

Lists all available packages:

```squ1d
write(pkg_list());
```

#### `pkg_remove(name)`

Removes a package:

```squ1d
pkg_remove("mypackage");
```

### Type Conversion

#### `i2fl(integer)`

Converts an integer to a float:

```squ1d
write(i2fl(42));  # Returns 42.0
```

#### `fl2i(float)`

Converts a float to an integer:

```squ1d
write(fl2i(3.14));  # Returns 3
```

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
def sumArray(arr) {
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
write("Sum:", total);
```

### Hash Map Example

```squ1d
var student = {
    "name": "John",
    "grades": [85, 92, 78, 96],
    "active": true
};

write("Student:", student["name"]);
write("Average grade: ", student["grades"][0]);
```

### Interactive Program

```squ1d
write("Welcome to SQU1DLang Calculator!");
var num1 = read("Enter first number: ");
var num2 = read("Enter second number: ");
var sum = num1 + num2;
write("The sum is: ", sum);
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

SQU1DLang is implemented as a complete compiler with the following components:

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
go run .
```

### Running Files

To execute a SQU1DLang file:

```bash
go run . filename.sqd
```

### Compiling to Executable

To compile a SQU1DLang file to a standalone executable:

```bash
go run . -B input.sqd -o output
```

This creates a standalone executable that doesn't require Go to run.

### Package Management

SQU1DLang includes a built-in package management system:

```squ1d
# Create a new package
pkg_create("mypackage", "A sample package");

# List available packages
write(pkg_list());

# Remove a package
pkg_remove("mypackage");
```

### File Includes

You can include other SQU1DLang files using the `include()` function:

```squ1d
include("library.sqd");
```

The include system searches in the current directory, `lib/` directory, and user's package cache.

---

_SQU1D++ SQU1DLang Compiler, version 1, written by Quan Thai._
