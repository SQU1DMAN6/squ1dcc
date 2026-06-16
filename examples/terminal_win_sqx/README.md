# terminal_win_sqx — Windows Terminal Control SQX Module

A low-level Windows terminal control module for SQU1DLang, written in C.
Provides direct access to the Windows Console API for advanced terminal rendering,
cursor control, color management, and raw input.

## Build

### With MinGW/GCC:
```bash
gcc -O2 -o terminal_win.sqx main.c
```

### With MSVC:
```bash
cl /O2 /Fe:terminal_win.sqx main.c
```

## Usage from SQU1DLang

### Loading the module
```squ1d
var term = pkg.load_sqx("path/to/terminal_win.sqx")
```

### Structured Return Contract
All functions return structured JSON via the SQX v1.9+ contract:
```json
{"ok": true, "value": ..., "error": null}
{"ok": false, "value": null, "error": "error message"}
```

Use the `<<` and `<<<` operators to extract error/ok fields:
```squ1d
var result = term.get_size()
if (<< result != null) {
    io.echo("Error: ", << result)
} el {
    var size = <<< result
    io.echo("Width: ", size.width, " Height: ", size.height)
}
```

## API Reference

### Output Functions

#### `term.write(text)`
Write text to the console using `WriteConsoleA`. Supports ANSI escape sequences
when `ENABLE_VIRTUAL_TERMINAL_PROCESSING` is active.

```squ1d
term.write("Hello, console!")
```

#### `term.writeln(text)`
Write text followed by a newline.

```squ1d
term.writeln("Hello, console!")
```

#### `term.write_at(x, y, text)`
Write text at a specific position in the console buffer.

```squ1d
term.write_at(10, 5, "Positioned text")
```

### Screen Management

#### `term.clear()`
Clear the console screen. Uses ANSI escape `\x1b[2J\x1b[H` with fallback to `cls`.

#### `term.get_size()`
Get the console window dimensions.
Returns: `{"width": 80, "height": 25}`

```squ1d
var size = <<< term.get_size()
var w = size.width
var h = size.height
```

#### `term.set_size(width, height)`
Set the console window size in character cells.

```squ1d
term.set_size(120, 40)
```

#### `term.set_title(title)`
Set the console window title.

```squ1d
term.set_title("My SQU1DLang Application")
```

#### `term.enter_alt_screen()`
Switch to the alternate screen buffer (used for TUI applications).

#### `term.exit_alt_screen()`
Return to the main screen buffer.

#### `term.scroll(rows)`
Scroll the console buffer by the given number of rows. Positive = scroll up,
negative = scroll down.

### Cursor Control

#### `term.move(x, y)`
Set the cursor position (0-based coordinates).

```squ1d
term.move(0, 0)  # Move to top-left
```

#### `term.cursor_pos()`
Get the current cursor position.
Returns: `{"x": 0, "y": 0}`

#### `term.show_cursor(show)`
Show or hide the cursor. Pass `true` to show, `false` to hide.

### Color and Style

#### `term.set_fg(color)`
Set the foreground text color. Accepts ANSI 256-color values (0-255).

```squ1d
term.set_fg(196)  # Bright red
```

Common VGA colors: 0=black, 1=red, 2=green, 3=yellow, 4=blue, 5=magenta,
6=cyan, 7=white, 8-15=bright variants.

#### `term.set_bg(color)`
Set the background color. Same color range as `set_fg`.

```squ1d
term.set_bg(232)  # Near-black
```

#### `term.reset_style()`
Reset all text formatting to default.

### Raw Mode

#### `term.enable_raw_mode()`
Enable raw console mode: disables line buffering and echo, enables
virtual terminal processing. Must be called before reading individual keys.

#### `term.disable_raw_mode()`
Restore the original console mode.

## Example: Simple TUI

```squ1d
# examples/terminal_demo.sqd
var term = pkg.load_sqx("terminal_win.sqx")

term.enable_raw_mode()
term.enter_alt_screen()
term.clear()
term.show_cursor(false)

# Draw a border
for (var x = 0; x < 80; x = x + 1) {
    term.write_at(x, 0, "#")
    term.write_at(x, 24, "#")
}
for (var y = 0; y < 25; y = y + 1) {
    term.write_at(0, y, "#")
    term.write_at(79, y, "#")
}

# Write centered text
term.move(35, 12)
term.set_fg(10)  # Bright green
term.write("Hello, TUI!")
term.reset_style()

# Wait for key press
var kb = pkg.load_sqx("keyboard_win.sqx")
kb.enable_raw_input()
kb.read_key()

# Cleanup
term.exit_alt_screen()
term.show_cursor(true)
term.disable_raw_mode()
```

## See Also

- `keyboard_win_sqx` — Low-level keyboard input module (sister library)
- `docs/TERMINAL_SQX.md` — Architecture overview