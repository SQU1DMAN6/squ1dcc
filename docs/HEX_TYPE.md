# Hex Data Type in SQU1D++

## Overview

The `Hex` data type is a whole number type used for representing hexadecimal values. It functions similarly to the `Integer` type but is specifically designed for hexadecimal notation, making it convenient for working with byte values, color codes, memory addresses, and other hex-based data.

## Syntax

### Hex Literals

Hex literals are defined using the `0x` prefix followed by hexadecimal digits (0-9, a-f, A-F):

```sqd
var byte_val = 0xFF;        # 255 in decimal
var color = 0xABCDEF;       # Full RGB color code
var short_val = 0x10;       # 16 in decimal
var zero = 0x0;             # Zero
```

### Case Insensitivity

Hex digits can be uppercase or lowercase:

```sqd
var a = 0xFF;               # Uppercase
var b = 0xff;               # Lowercase
var c = 0xAB;               # Mixed (valid)
```

Both representations are equivalent.

## Display Format

Hex values are displayed in lowercase hex notation with the `0x` prefix:

```sqd
var x = 0xFF;
io.echo(x);                 # Output: 0xff

var neg = -0x20;
io.echo(neg);               # Output: -0x20
```

## Operations

### Arithmetic Operations

Hex values support all arithmetic operations similar to integers. When operations involve hex + hex, the result is returned as a hex value. When hex is mixed with integers, the result type depends on the operation context:

```sqd
var a = 0x10;
var b = 0x20;

var add = a + b;            # 0x30
var sub = b - a;            # 0x10
var mul = 0x4 * 0x3;        # 0xc
var div = 0x100 / 0x10;     # 0x10
var mod = 0x17 % 0x5;       # 0x2
```

### Mixed Type Arithmetic

Hex values automatically convert to integers when operated with integers:

```sqd
var hex_val = 0x20;         # 32 in hex
var int_val = 16;

var result = hex_val + int_val;  # 48 (integer)
io.echo(result);             # Output: 48
```

### Negation

Hex values can be negated:

```sqd
var x = 0xFF;
var neg_x = -x;              # -0xff
```

## Type Checking

Use `type.tp()` to check if a value is of type Hex:

```sqd
var hex_val = 0xFF;
var int_val = 255;

io.echo(type.tp(hex_val));   # Output: Hex
io.echo(type.tp(int_val));   # Output: Integer
```

## Type Conversion

### Hex to Integer

Convert a Hex value to an Integer using `type.h2i()`:

```sqd
var hex_num = 0xAB;
var int_num = type.h2i(hex_num);
io.echo(int_num);            # Output: 171
```

### Integer to Hex

Convert an Integer to Hex using `type.hex()`:

```sqd
var int_val = 255;
var hex_val = type.hex(int_val);
io.echo(hex_val);            # Output: 0xff
```

### Array of Hex to String

Convert an array of hex values (or integers) to a string using `type.hex2s()`. This is useful for converting byte arrays to strings:

```sqd
var data = [0x48, 0x65, 0x6C, 0x6C, 0x6F];
var text = type.hex2s(data);
io.echo(text);               # Output: Hello
```

You can also mix hex and integer values in the array:

```sqd
var mixed = [0x50, 105, 0x7A, 122, 0x61];
var result = type.hex2s(mixed);
io.echo(result);             # Output: Pizza
```

## Division Behavior

Like integers, hex division rounds down to the nearest whole number:

```sqd
var a = 0x10;   # 16
var b = 0x3;    # 3

var result = a / b;          # 0x5 (16 / 3 = 5.33... → 5)
```

## Examples

### Working with Byte Values

```sqd
var byte_value = 0xFF;       # Maximum byte value (255)
var half = byte_value / 0x2; # 0x7f (127)
```

### Converting Color Codes

```sqd
var rgb_color = 0xFFFF00;    # Yellow in RGB hex
var red = (rgb_color >> 16) & 0xFF;
var green = (rgb_color >> 8) & 0xFF;
var blue = rgb_color & 0xFF;
```

### Creating Byte Strings

```sqd
var message_hex = [0x48, 0x65, 0x6C, 0x6C, 0x6F, 0x20, 0x57, 0x6F, 0x72, 0x6C, 0x64];
var message = type.hex2s(message_hex);
io.echo(message);            # Output: Hello World
```

## Automatic Type Conversion

Hex values automatically convert to integers when necessary in operations, allowing seamless mixing of numeric types in your code.

## Implementation Details

- **Range**: Same as Integer (-2^63 to 2^63-1)
- **Storage**: 64-bit signed integer
- **Display Format**: Lowercase hexadecimal (0x prefix)
- **Division**: Rounds down (integer division)
- **Hashing**: Supports use as hash map keys

## Notes

- Hex literals must start with `0x` (lowercase x)
- Hex values are case-insensitive in the source code but display in lowercase
- Operations between hex and hex return hex; operations with integers return integers
- Hex is useful for code clarity when working with low-level data, but operationally identical to integers with identical numeric values
