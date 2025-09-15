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

```squ1d
'42
'-17
'1.123
'29.24837
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

SQU1DLang provides several built-in functions:

### `write(...args)`

Prints values to the console:

```squ1d
write("Hello, World!");
write("Value:", 42);
```

### `read([prompt])`

Reads input from the user:

```squ1d
var input = read();
var name = read("Enter your name: ");
```

### `cat(value)`

Returns the length of a string or array:

```squ1d
var len = cat("hello");        // Returns 5
var count = cat([1, 2, 3]);    // Returns 3
```

### `append(array, value)`

Adds an element to the end of an array:

```squ1d
var numbers = [1, 2, 3];
var extended = append(numbers, 4);  // Returns [1, 2, 3, 4]
```

### `tp(value)`

Returns the type of a value as a string:

```squ1d
tp(42);        // Returns "Integer"
tp("hello");   // Returns "String"
tp([1, 2]);    // Returns "Array"
tp({});        // Returns "Object"
tp(true);      // Returns "Boolean"
```

## Operators

### Arithmetic Operators

```squ1d
+   // Addition
-   // Subtraction
*   // Multiplication
/   // Division
```

### Comparison Operators

```squ1d
==  // Equal to
!=  // Not equal to
<   // Less than
>   // Greater than
```

### Logical Operators

```squ1d
!   // Logical NOT
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

## Getting Started

To run SQU1DLang code, use the REPL:

```bash
go run .
```

This will start an interactive session where you can type SQU1DLang code and see the results immediately.

---

_SQU1D++ SQU1DLang Compiler, version 1, written by Quan Thai._
