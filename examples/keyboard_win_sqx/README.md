# keyboard_win_sqx — Windows Low-Level Keyboard SQX Module

A low-level Windows keyboard input module for SQU1DLang, written in C.
Uses `ReadConsoleInput` and `GetAsyncKeyState` for raw key events with full
modifier support, simultaneous key detection, and advanced event handling.

## Build

### With MinGW/GCC:
```bash
gcc -O2 -o keyboard_win.sqx main.c
```

### With MSVC:
```bash
cl /O2 /Fe:keyboard_win.sqx main.c
```

## Usage from SQU1DLang

### Loading the module
```squ1d
var kb = pkg.load_sqx("path/to/keyboard_win.sqx")
```

### General Pattern
1. Call `kb.enable_raw_input()` once to activate raw input mode
2. Use `kb.read_key()` to block on a single key event
3. Or use `kb.poll_event()` to check for events without blocking
4. Call `kb.disable_raw_input()` when done

## API Reference

### Input Mode Control

#### `kb.enable_raw_input()`
Enable raw keyboard input mode. This sets the console to use `ReadConsoleInput`
for unfiltered key events, including mouse and window resize events.
Must be called before any read operation.

```squ1d
kb.enable_raw_input()
```

#### `kb.disable_raw_input()`
Restore the original console input mode.

```squ1d
kb.disable_raw_input()
```

### Reading Input

#### `kb.read_key()`
Read a single input event, blocking until one is available.
Returns a structured object describing the event.

**Returns (Keyboard Event):**
```json
{
    "type": "key",
    "key": "a",
    "char": "a",
    "pressed": true,
    "repeat_count": 1,
    "vk_code": 65,
    "modifiers": ["shift"],
    "ctrl": false,
    "alt": false,
    "shift": true,
    "caps_lock": false,
    "num_lock": true,
    "scroll_lock": false
}
```

**Returns (Mouse Event):**
```json
{
    "type": "mouse",
    "event": "mouse_move",
    "x": 42,
    "y": 13,
    "button": "left",
    "wheel": "none",
    "modifiers": []
}
```

**Returns (Resize Event):**
```json
{
    "type": "resize",
    "width": 120,
    "height": 40
}
```

**Returns (Focus Event):**
```json
{
    "type": "focus",
    "focused": true
}
```

#### `kb.read_keys()`
Read all pending input events (non-blocking).
Returns a JSON array of all events currently in the input buffer.

```squ1d
var events = kb.read_keys()
```

#### `kb.poll_event()`
Check if input events are available without blocking.
Returns `true` if events are pending, `false` otherwise.

```squ1d
while (kb.poll_event()) {
    var evt = kb.read_key()
    # Process event...
}
```

#### `kb.flush_input()`
Discard all pending input events from the buffer.

### Key State Queries

#### `kb.get_key_state(vk_code)`
Get the current pressed state of a specific virtual key code.
Returns: `{"key": "a", "vk_code": 65, "pressed": true}`

```squ1d
var state = kb.get_key_state(65)  # Check 'A' key
if (state.pressed) {
    io.echo("A is pressed")
}
```

Virtual key codes:
- `0x41`-`0x5A`: A-Z
- `0x30`-`0x39`: 0-9
- `VK_LEFT` (`0x25`), `VK_UP` (`0x26`), `VK_RIGHT` (`0x27`), `VK_DOWN` (`0x28`)
- `VK_RETURN` (`0x0D`), `VK_ESCAPE` (`0x1B`), `VK_SPACE` (`0x20`)
- `VK_SHIFT` (`0x10`), `VK_CONTROL` (`0x11`), `VK_MENU` (`0x12`)
- `VK_F1`-`VK_F24` (`0x70`-`0x87`)

#### `kb.get_pressed_keys()`
Get all currently pressed keys (asynchronous polling via `GetAsyncKeyState`).
Returns a JSON array of all keys currently held down.

```squ1d
var pressed = kb.get_pressed_keys()
# pressed = [{"key": "a", "vk_code": 65}, {"key": "ctrl", "vk_code": 17}]
```

### Event Waiting

#### `kb.wait_key(vk_code1, vk_code2, ...)`
Wait for a specific key or combination. Blocks until one of the specified
keys is pressed. Returns the matching key event.

```squ1d
# Wait for Escape or Enter
var evt = kb.wait_key(0x1B, 0x0D)
io.echo("Pressed: ", evt.key)
```

#### `kb.bind(key, callback_name)`
Register a binding for use in a SQU1DLang event loop. Note that the actual
callback dispatch must be implemented in SQU1DLang.

```squ1d
var binding = kb.bind("65", "on_key_a")
# Use binding.bound_key and binding.callback in your event loop
```

## Complete Example: Real-time Key Viewer

```squ1d
# key_viewer.sqd
var kb = pkg.load_sqx("keyboard_win.sqx")
var term = pkg.load_sqx("terminal_win.sqx")

kb.enable_raw_input()
term.enable_raw_mode()
term.clear()

term.write("Real-time Key Viewer - Press Ctrl+C to exit\n")
term.write("==========================================\n\n")

while (true) {
    var evt = kb.read_key()
    
    if (evt.type == "key" and evt.pressed) {
        term.clear()
        term.write_at(0, 3, "Key:       ")
        term.write_at(11, 3, evt.key)
        
        if (evt.char != "") {
            term.write_at(0, 4, "Char:      ")
            term.write_at(11, 4, evt.char)
        }
        
        term.write_at(0, 5, "VK Code:   ")
        term.write_at(11, 5, evt.vk_code)
        
        term.write_at(0, 6, "Modifiers: ")
        if (evt.shift) { term.write("shift ") }
        if (evt.ctrl)  { term.write("ctrl ") }
        if (evt.alt)   { term.write("alt ") }
        
        # Exit on Ctrl+C (VK_CANCEL or Ctrl key combos)
        if (evt.vk_code == 0x43 and evt.ctrl) {
            break
        }
    }
}

kb.disable_raw_input()
term.disable_raw_mode()
term.clear()
io.echo("Exited.\n")
```

## Key Names Reference

| Key | String | VK Code |
|-----|--------|---------|
| A-Z | `a`-`z` | 0x41-0x5A |
| 0-9 | `0`-`9` | 0x30-0x39 |
| Enter | `enter` | 0x0D |
| Escape | `escape` | 0x1B |
| Tab | `tab` | 0x09 |
| Space | `space` | 0x20 |
| Backspace | `backspace` | 0x08 |
| Delete | `delete` | 0x2E |
| Insert | `insert` | 0x2D |
| Home | `home` | 0x24 |
| End | `end` | 0x23 |
| Page Up | `page_up` | 0x21 |
| Page Down | `page_down` | 0x22 |
| Up Arrow | `up` | 0x26 |
| Down Arrow | `down` | 0x28 |
| Left Arrow | `left` | 0x25 |
| Right Arrow | `right` | 0x27 |
| Shift | `shift` | 0x10 |
| Ctrl | `ctrl` | 0x11 |
| Alt | `alt` | 0x12 |
| F1-F12 | `f1`-`f12` | 0x70-0x7B |
| Caps Lock | `caps_lock` | 0x14 |
| Num Lock | `num_lock` | 0x90 |
| Scroll Lock | `scroll_lock` | 0x91 |
| Print Screen | `print_screen` | 0x2C |
| Pause | `pause` | 0x13 |
| Left Win | `lwin` | 0x5B |
| Right Win | `rwin` | 0x5C |
| Apps/Menu | `apps` | 0x5D |

## Integration with terminal_win_sqx

The keyboard module is designed to complement `terminal_win_sqx` for building
TUIs. The recommended workflow:

1. Load both modules
2. Enable raw input and terminal raw mode
3. Enter alternate screen buffer
4. Render using terminal functions
5. Read input using keyboard functions
6. Clean up on exit

## See Also

- `terminal_win_sqx` — Windows Terminal Control module (sister library)
- Built-in `keyboard` class — Simpler keyboard API for basic use cases