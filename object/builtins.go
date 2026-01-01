package object

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"squ1d++/pkg"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

// Keyboard event system state
type KeyboardEvent struct {
	Key       string
	Timestamp time.Time
}

type KeyboardListener struct {
	keys     []string
	callback Object // Store actual function object or string
	id       string
}

var (
	keyboardListeners = make(map[string]*KeyboardListener)
	keyboardEvents    = make(chan KeyboardEvent, 100)
	keyboardMutex     sync.RWMutex
	rawModeActive     = false
	originalTermios   *term.State
	keyboardActive    = false
	pendingCallbacks  []Object
)

// OutWriter is the writer used by builtins for printing side-effects (e.g., io.echo).
// It defaults to os.Stdout but can be overridden by callers (like the REPL) so
// tests and embedded runners can capture output.
var OutWriter io.Writer = os.Stdout

// termios structure for raw mode

// enableRawMode switches terminal to raw mode for immediate key detection
func enableRawMode() error {
	if rawModeActive {
		return nil
	}

	fd := int(os.Stdin.Fd())

	// Check if stdin is a terminal
	if !term.IsTerminal(fd) {
		return fmt.Errorf("stdin is not a terminal")
	}

	// Use golang.org/x/term for cross-platform terminal handling
	state, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("failed to enable raw mode: %v", err)
	}

	originalTermios = state
	rawModeActive = true
	return nil
}

// disableRawMode restores terminal to original state
func disableRawMode() error {
	if !rawModeActive || originalTermios == nil {
		return nil
	}

	fd := int(os.Stdin.Fd())
	err := term.Restore(fd, originalTermios)
	if err != nil {
		return fmt.Errorf("failed to restore terminal: %v", err)
	}

	rawModeActive = false
	return nil
}

// readKey reads a single keypress in raw mode
func readKey() ([]byte, error) {
	buf := make([]byte, 4)
	n, err := os.Stdin.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

// normalizeKeyName converts raw key bytes to standardized key names
func normalizeKeyName(keyBytes []byte) string {
	if len(keyBytes) == 0 {
		return ""
	}

	// Handle special keys
	switch {
	case len(keyBytes) == 1:
		switch keyBytes[0] {
		case 3:
			return "KeyCtrl+C"
		case 4:
			return "KeyCtrl+D"
		case 5:
			return "KeyCtrl+E"
		case 17:
			return "KeyCtrl+Q"
		case 27:
			return "KeyEscape"
		case 13:
			return "KeyEnter"
		case 32:
			return "KeySpace"
		case 127:
			return "KeyBackspace"
		default:
			if keyBytes[0] >= 32 && keyBytes[0] < 127 {
				// Regular printable character
				char := strings.ToUpper(string(keyBytes[0]))
				return "Key" + char
			}
			return fmt.Sprintf("Key%d", keyBytes[0])
		}
	case len(keyBytes) == 3 && keyBytes[0] == 27 && keyBytes[1] == 91:
		// Arrow keys and function keys
		switch keyBytes[2] {
		case 65:
			return "KeyUp"
		case 66:
			return "KeyDown"
		case 67:
			return "KeyRight"
		case 68:
			return "KeyLeft"
		}
	}

	// Default case for unrecognized sequences
	return fmt.Sprintf("KeyUnknown_%v", keyBytes)
}

// checkKeyMatch checks if pressed key matches any of the target keys
func checkKeyMatch(pressedKey string, targetKeys []string) bool {
	for _, target := range targetKeys {
		if pressedKey == target {
			return true
		}
		// Also check for combinations like "KeyCtrl+E"
		if strings.Contains(pressedKey, "+") && strings.Contains(target, "+") {
			if pressedKey == target {
				return true
			}
		}
	}
	return false
}

// startKeyboardListener starts a background goroutine to listen for keyboard input
func startKeyboardListener() {
	if keyboardActive {
		return
	}

	go func() {
		keyboardActive = true
		defer func() { keyboardActive = false }()

		for keyboardActive {
			if !rawModeActive {
				time.Sleep(50 * time.Millisecond)
				continue
			}

			keyBytes, err := readKey()
			if err != nil {
				continue
			}

			keyName := normalizeKeyName(keyBytes)
			event := KeyboardEvent{
				Key:       keyName,
				Timestamp: time.Now(),
			}

			// Try to send event to channel, but don't block
			select {
			case keyboardEvents <- event:
			default:
				// Channel full, skip this event
			}
		}
	}()
}

// stopKeyboardListener stops the background keyboard listener
func stopKeyboardListener() {
	keyboardActive = false
	disableRawMode()
}

// processKeyboardEvents processes pending keyboard events and returns triggered events
func processKeyboardEvents() []KeyboardEvent {
	var triggeredEvents []KeyboardEvent

	// Process all pending events in the channel
	for {
		select {
		case event := <-keyboardEvents:
			// Check if this event matches any registered listeners
			keyboardMutex.RLock()
			for _, listener := range keyboardListeners {
				if checkKeyMatch(event.Key, listener.keys) {
					triggeredEvents = append(triggeredEvents, event)
					// Store the callback to be executed during update()
					if listener.callback != nil {
						pendingCallbacks = append(pendingCallbacks, listener.callback)
					}
				}
			}
			keyboardMutex.RUnlock()
		default:
			// No more events
			return triggeredEvents
		}
	}
}

func createBuiltin(fn BuiltinFunction, class string) *Builtin {
	return &Builtin{
		Fn:         fn,
		Class:      class,
		Attributes: make(map[string]Object),
	}
}

var Builtins = []struct {
	Name    string
	Builtin *Builtin
}{
	{
		"tp",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expected 1, got %d", len(args))
			}

			switch args[0].(type) {
			case *Array:
				return &String{Value: "Array"}
			case *String:
				return &String{Value: "String"}
			case *Hash:
				return &String{Value: "Object"}
			case *Integer:
				return &String{Value: "Integer"}
			case *Float:
				return &String{Value: "Float"}
			case *Boolean:
				return &String{Value: "Boolean"}
			case *Builtin:
				return &String{Value: "Builtin"}
			case *Function:
				return &String{Value: "Function"}
			case *Error:
				return &String{Value: "Error"}
			default:
				return &String{Value: "Null"}
			}
		}, "type"),
	},

	{
		"i2fl",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expected 1, got %d", len(args))
			}

			if args[0].Type() == FLOAT_OBJ {
				return args[0]
			}

			int, ok := args[0].(*Integer)
			if !ok {
				return newError("Argument 0 to `i2fl` must be INTEGER, got %s", args[0].Type())
			}

			return &Float{Value: float64(int.Value)}
		}, "type"),
	},
	{
		"fl2i",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expected 1, got %d", len(args))
			}

			if args[0].Type() == INTEGER_OBJ {
				return args[0]
			}

			fl, ok := args[0].(*Float)
			if !ok {
				return newError("Argument 0 to `fl2i` must be FLOAT, got %s", args[0].Type())
			}

			return &Integer{Value: int64(fl.Value)}
		}, "type"),
	},
	{
		"s2i",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expected 1, got %d", len(args))
			}

			if args[0].Type() == INTEGER_OBJ {
				return args[0]
			}

			strInteger, ok := args[0].(*String)
			if !ok {
				return newError("Argument 0 to `s2i` must be STRING, got %s", args[0].Type())
			}

			numInteger, err := strconv.ParseInt(strInteger.Value, 10, 0)
			if err != nil {
				return newError("Failed to convert string to integer: %s", err)
			}

			return &Integer{Value: numInteger}
		}, "type"),
	},
	{
		"s2fl",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expeceted 1, got %d", len(args))
			}

			if args[0].Type() == FLOAT_OBJ {
				return args[0]
			}

			stringFloat, ok := args[0].(*String)
			if !ok {
				return newError("Argument 0 to `s2fl` must be STRING, got %s", args[0].Type())
			}

			numFloat, err := strconv.ParseFloat(stringFloat.Value, 64)
			if err != nil {
				return newError("Failed to convert string to integer: %s", err)
			}

			return &Float{Value: numFloat}
		}, "type"),
	},
	{
		"d2s",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expected 1, got %d", len(args))
			}

			var stringValue string

			switch v := args[0].(type) {
			case *Integer:
				stringValue = fmt.Sprint(v.Value)
			case *Float:
				stringValue = fmt.Sprint(v.Value)
			case *String:
				return args[0]
			default:
				return newError("Argument to `d2s` must be FLOAT or INTEGER, got %s", args[0].Type())
			}

			return &String{Value: stringValue}
		}, "type"),
	},
	{
		"append",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 2 {
				return newError("Wrong number of arguments. Expected 2, got %d", len(args))
			}

			arr, ok := args[0].(*Array)
			if !ok {
				return newError("Argument 0 to `append` must be ARRAY, got %s", args[0].Type())
			}

			length := len(arr.Elements)
			newElements := make([]Object, length+1)
			copy(newElements, arr.Elements)
			newElements[length] = args[1]

			return &Array{Elements: newElements}
		}, "array"),
	},
	{
		"read",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 0 && len(args) != 1 {
				return newError("Wrong number of arguments. Expected 0 or 1, got %d", len(args))
			}

			if len(args) == 1 {
				prompt, ok := args[0].(*String)
				if !ok {
					return newError("Argument 0 to `read` must be STRING, got %s", args[0].Type())
				}
				fmt.Print(prompt.Value)
			}

			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return newError("Failed to read input: %s", err)
			}

			input = strings.TrimSpace(input)

			var value Object
			if intVal, err := strconv.ParseInt(input, 10, 64); err == nil {
				value = &Integer{Value: intVal}
			} else if floatVal, err := strconv.ParseFloat(input, 64); err == nil {
				value = &Float{Value: floatVal}
			} else {
				value = &String{Value: input}
			}

			return value
		}, "io"),
	},
	{
		"write",
		createBuiltin(func(args ...Object) Object {
			var elements []string
			for _, arg := range args {
				elements = append(elements, arg.Inspect())
			}

			return &String{Value: strings.Join(elements, " ")}
		}, "io"),
	},
	{
		"echo",
		createBuiltin(func(args ...Object) Object {
			var elements []string
			for _, arg := range args {
				elements = append(elements, arg.Inspect())
			}

			output := strings.Join(elements, " ")
			fmt.Fprint(OutWriter, output)
			return &Null{}
		}, "io"),
	},
	{
		"on",
		createBuiltin(func(args ...Object) Object {
			if len(args) < 2 {
				return newError("Wrong number of arguments. Expected at least 2, got %d", len(args))
			}

			// Extract keys to listen for
			var keys []string
			for i := 0; i < len(args)-1; i++ {
				key, ok := args[i].(*String)
				if !ok {
					return newError("Argument %d to `keyboard.on` must be STRING, got %s", i, args[i].Type())
				}
				keys = append(keys, key.Value)
			}

			// Last argument should be a function or string callback
			callback := args[len(args)-1]
			switch callback.(type) {
			case *CompiledFunction, *Closure, *String:
				// Valid callback types
			default:
				return newError("Last argument to `keyboard.on` must be FUNCTION or STRING, got %s", callback.Type())
			}

			// Register the listener
			keyboardMutex.Lock()
			listenerID := fmt.Sprintf("listener_%d", len(keyboardListeners))
			keyboardListeners[listenerID] = &KeyboardListener{
				keys:     keys,
				callback: callback,
				id:       listenerID,
			}
			keyboardMutex.Unlock()

			// Start keyboard listener if not already started
			if err := enableRawMode(); err != nil {
				return newError("Failed to enable keyboard listening: %v", err)
			}
			startKeyboardListener()

			return &String{Value: listenerID}
		}, "keyboard"),
	},
	{
		"read",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 0 {
				return newError("Wrong number of arguments. Expected 0, got %d", len(args))
			}

			// If a background listener is active, prefer reading events from it
			if keyboardActive {
				// Block until an event is available
				e := <-keyboardEvents
				return &String{Value: e.Key}
			}

			// Otherwise fall back to single-key raw read
			fd := int(os.Stdin.Fd())
			if !term.IsTerminal(fd) {
				reader := bufio.NewReader(os.Stdin)
				input, err := reader.ReadString('\n')
				if err != nil {
					return newError("Failed to read input: %v", err)
				}
				input = strings.TrimSpace(input)
				if len(input) > 0 {
					return &String{Value: "Key" + strings.ToUpper(string(input[0]))}
				}
				return &String{Value: "KeyEnter"}
			}

			if err := enableRawMode(); err != nil {
				return newError("Failed to enable keyboard reading: %v", err)
			}
			defer disableRawMode()

			keyBytes, err := readKey()
			if err != nil {
				return newError("Failed to read key: %v", err)
			}

			keyName := normalizeKeyName(keyBytes)
			return &String{Value: keyName}
		}, "keyboard"),
	},
	{
		"listen",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 0 {
				return newError("Wrong number of arguments. Expected 0, got %d", len(args))
			}

			// Try to enable raw mode and start the background listener. If raw
			// mode cannot be enabled (e.g., not a terminal), behave as a
			// non-blocking reader and return null when no event is available.
			_ = enableRawMode()
			startKeyboardListener()

			// Non-blocking read: return a key if available, otherwise null.
			select {
			case e := <-keyboardEvents:
				return &String{Value: e.Key}
			default:
				return &Null{}
			}
		}, "keyboard"),
	},
	{
		"stop",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 0 {
				return newError("Wrong number of arguments. Expected 0, got %d", len(args))
			}

			// Stop keyboard listener and disable raw mode
			stopKeyboardListener()

			return &Null{}
		}, "keyboard"),
	},
	{
		"off",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expected 1, got %d", len(args))
			}

			idStr, ok := args[0].(*String)
			if !ok {
				return newError("Argument 0 to `keyboard.off` must be STRING, got %s", args[0].Type())
			}

			keyboardMutex.Lock()
			defer keyboardMutex.Unlock()
			if _, exists := keyboardListeners[idStr.Value]; exists {
				delete(keyboardListeners, idStr.Value)
				return &Boolean{Value: true}
			}
			return &Boolean{Value: false}
		}, "keyboard"),
	},
	// OS Builtins
	{
		"env",
		createBuiltin(func(args ...Object) Object {
			if len(args) == 0 {
				env := make(map[HashKey]HashPair)
				for _, e := range os.Environ() {
					parts := strings.SplitN(e, "=", 2)
					if len(parts) == 2 {
						key := &String{Value: parts[0]}
						value := &String{Value: parts[1]}
						env[key.HashKey()] = HashPair{Key: key, Value: value}
					}
				}
				return &Hash{Pairs: env}
			} else if len(args) == 1 {
				key, ok := args[0].(*String)
				if !ok {
					return newError("Argument 0 to `env` must be STRING, got %s", args[0].Type())
				}
				value := os.Getenv(key.Value)
				if value == "" {
					return newError("Environment variable '%s' not found", key.Value)
				}
				return &String{Value: value}
			} else {
				return newError("Wrong number of arguments. Expected 0 or 1, got %d", len(args))
			}
		}, "os"),
	},
	{
		"exec",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expected 1, got %d", len(args))
			}
			command, ok := args[0].(*String)
			if !ok {
				return newError("Argument 0 to `exec` must be STRING, got %s", args[0].Type())
			}

			seprcommand := strings.Split(command.Value, " ")

			args_ := seprcommand[1:]

			output, err := exec.Command(seprcommand[0], args_...).Output()
			if err != nil {
				return newError("Failed to execute command: %s", err)
			}

			return &String{Value: string(output)}
		}, "os"),
	},
	{
		"exit",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expected 1, got %d", len(args))
			}

			status, ok := args[0].(*Integer)

			if !ok {
				return newError("Argument 0 to `exit` must be INTEGER, got %s", args[0].Type())
			}

			os.Exit(int(status.Value))

			return &Null{}
		}, "os"),
	},

	// Time builtins
	{
		"sleep",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expected 1, got %d", len(args))
			}

			var duration time.Duration
			switch arg := args[0].(type) {
			case *Integer:
				duration = time.Duration(arg.Value) * time.Millisecond
			case *Float:
				duration = time.Duration(arg.Value*1000) * time.Millisecond
			default:
				return newError("Argument 0 to `sleep` must be INTEGER or FLOAT, got %s", args[0].Type())
			}

			time.Sleep(duration)
			return &Null{}
		}, "time"),
	},
	{
		"now",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 0 {
				return newError("Wrong number of arguments. Expected 0, got %d", len(args))
			}
			return &Integer{Value: time.Now().UnixMilli()}
		}, "time"),
	},
	// System builtins (runtime/config)
	{
		"set_overflow_size",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expected 1, got %d", len(args))
			}

			sizeObj, ok := args[0].(*Integer)
			if !ok {
				return newError("Argument 0 to `set_overflow_size` must be INTEGER, got %s", args[0].Type())
			}

			if sizeObj.Value < 1024 {
				return newError("Overflow size must be at least 1024")
			}

			SysMaxStackSize = int(sizeObj.Value)
			return &Integer{Value: int64(SysMaxStackSize)}
		}, "sys"),
	},
	{
		"get_overflow_size",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 0 {
				return newError("Wrong number of arguments. Expected 0, got %d", len(args))
			}
			return &Integer{Value: int64(SysMaxStackSize)}
		}, "sys"),
	},
	{
		"gc",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 0 {
				return newError("Wrong number of arguments. Expected 0, got %d", len(args))
			}
			// Call Go's GC as a convenience for embedders/tests
			runtime.GC()
			return &Null{}
		}, "sys"),
	},
	// Math builtins
	{
		"rand",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 2 {
				return newError("Wrong number of arguments. Expected 2, got %d", len(args))
			}

			minimum, ok1 := args[0].(*Integer)
			maximum, ok2 := args[1].(*Integer)
			if !ok1 || !ok2 {
				return newError("Arguments to `rand` must be INTEGER and INTEGER, got %s and %s", args[0].Type(), args[1].Type())
			}

			min := minimum.Value
			max := maximum.Value

			randomNumber := rand.Int64N(max-min+1) + min

			return &Integer{Value: randomNumber}
		}, "math"),
	},
	{
		"abs",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expected 1, got %d", len(args))
			}

			switch arg := args[0].(type) {
			case *Integer:
				if arg.Value < 0 {
					return &Integer{Value: -arg.Value}
				}
				return arg
			case *Float:
				return &Float{Value: math.Abs(arg.Value)}
			default:
				return newError("Argument 0 to `abs` must be INTEGER or FLOAT, got %s", args[0].Type())
			}
		}, "math"),
	},
	{
		"sqrt",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expected 1, got %d", len(args))
			}

			var value float64
			switch arg := args[0].(type) {
			case *Integer:
				value = float64(arg.Value)
			case *Float:
				value = arg.Value
			default:
				return newError("Argument 0 to `sqrt` must be INTEGER or FLOAT, got %s", args[0].Type())
			}

			if value < 0 {
				return newError("Square root of negative number is not defined.")
			}

			return &Float{Value: math.Sqrt(value)}
		}, "math"),
	},
	{
		"pow",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 2 {
				return newError("Wrong number of arguments. Expected 2, got %d", len(args))
			}

			var base, exponent float64

			switch arg := args[0].(type) {
			case *Integer:
				base = float64(arg.Value)
			case *Float:
				base = arg.Value
			default:
				return newError("Argument 0 to `pow` must be INTEGER or FLOAT, got %s", args[0].Type())
			}

			switch arg := args[1].(type) {
			case *Integer:
				exponent = float64(arg.Value)
			case *Float:
				exponent = arg.Value
			default:
				return newError("Argument 1 to `pow` must be INTEGER or FLOAT, got %s", args[0].Type())
			}

			return &Float{Value: math.Pow(base, exponent)}
		}, "math"),
	},
	{
		"sin",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expected 1, got %d", len(args))
			}

			var value float64
			switch arg := args[0].(type) {
			case *Integer:
				value = float64(arg.Value)
			case *Float:
				value = arg.Value
			default:
				return newError("Argument 0 to `sin` must be INTEGER or FLOAT, got %s", args[0].Type())
			}

			return &Float{Value: math.Sin(value)}
		}, "math"),
	},
	{
		"cos",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expected 1, got %d", len(args))
			}

			var value float64
			switch arg := args[0].(type) {
			case *Integer:
				value = float64(arg.Value)
			case *Float:
				value = arg.Value
			default:
				return newError("Argument 0 to `cos` must be INTEGER or FLOAT, got %s", args[0].Type())
			}

			return &Float{Value: math.Cos(value)}
		}, "math"),
	},
	{
		"pi",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 0 {
				return newError("Wrong number of arguments. Expected 0, got %d", len(args))
			}
			return &Float{Value: math.Pi}
		}, "math"),
	},
	{
		"e",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 0 {
				return newError("Wrong number of arguments. Expected 0, got %d", len(args))
			}
			return &Float{Value: math.E}
		}, "math"),
	},
	{
		"include",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expected 1, got %d", len(args))
			}

			path, ok := args[0].(*String)
			if !ok {
				return newError("Argument 0 to `include` must be STRING, got %s", args[0].Type())
			}

			if _, err := os.Stat(path.Value); os.IsNotExist(err) {
				return newError("File '%s' not found", path.Value)
			}

			content, err := os.ReadFile(path.Value)
			if err != nil {
				return newError("Could not read file '%s': %v", path.Value, err)
			}

			return &String{Value: string(content)}
		}, "pkg"),
	},
	{
		"create",
		createBuiltin(func(args ...Object) Object {
			if len(args) < 1 || len(args) > 2 {
				return newError("Wrong number of arguments. Expected 1 or 2, got %d", len(args))
			}

			name, ok := args[0].(*String)
			if !ok {
				return newError("Argument 0 to `pkg_create` must be STRING, got %s", args[0].Type())
			}

			description := ""
			if len(args) == 2 {
				desc, ok := args[1].(*String)
				if !ok {
					return newError("Argument 1 to `pkg_create` must be STRING, got %s", args[1].Type())
				}
				description = desc.Value
			}

			err := pkg.GlobalManager.CreatePackage(name.Value, description)
			if err != nil {
				return newError("Failed to create package: %v", err)
			}

			return &String{Value: "Package '" + name.Value + "' created successfully"}
		}, "pkg"),
	},
	{
		"list",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 0 {
				return newError("Wrong number of arguments. Expected 0, got %d", len(args))
			}

			packages, err := pkg.GlobalManager.ListPackages()
			if err != nil {
				return newError("Failed to list packages: %v", err)
			}

			// Create an array of package information
			elements := make([]Object, len(packages))
			for i, pkg := range packages {
				// Create a hash for each package
				pairs := make(map[HashKey]HashPair)

				nameKey := &String{Value: "name"}
				nameValue := &String{Value: pkg.Name}
				pairs[nameKey.HashKey()] = HashPair{Key: nameKey, Value: nameValue}

				versionKey := &String{Value: "version"}
				versionValue := &String{Value: pkg.Version}
				pairs[versionKey.HashKey()] = HashPair{Key: versionKey, Value: versionValue}

				descKey := &String{Value: "description"}
				descValue := &String{Value: pkg.Description}
				pairs[descKey.HashKey()] = HashPair{Key: descKey, Value: descValue}

				elements[i] = &Hash{Pairs: pairs}
			}

			return &Array{Elements: elements}
		}, "pkg"),
	},
	{
		"remove",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expected 1, got %d", len(args))
			}

			name, ok := args[0].(*String)
			if !ok {
				return newError("Argument 0 to `pkg_remove` must be STRING, got %s", args[0].Type())
			}

			err := pkg.GlobalManager.RemovePackage(name.Value)
			if err != nil {
				return newError("Failed to remove package: %v", err)
			}

			return &String{Value: "Package '" + name.Value + "' removed successfully"}
		}, "pkg"),
	},
	// String builtins
	{
		"upper",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expected 1, got %d", len(args))
			}

			str, ok := args[0].(*String)
			if !ok {
				return newError("Argument 0 to `upper` must be STRING, got %s", args[0].Type())
			}

			return &String{Value: strings.ToUpper(str.Value)}
		}, "string"),
	},
	{
		"lower",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expected 1, got %d", len(args))
			}

			str, ok := args[0].(*String)
			if !ok {
				return newError("Argument 0 to `lower` must be STRING, got %s", args[0].Type())
			}

			return &String{Value: strings.ToLower(str.Value)}
		}, "string"),
	},
	{
		"trim",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expected 1, got %d", len(args))
			}

			str, ok := args[0].(*String)
			if !ok {
				return newError("Argument 0 to `trim` must be STRING, got %s", args[0].Type())
			}

			return &String{Value: strings.TrimSpace(str.Value)}
		}, "string"),
	},
	{
		"sepr",
		createBuiltin(func(args ...Object) Object {
			if len(args) < 1 || len(args) > 2 {
				return newError("Wrong number of arguments. Expected 1 or 2, got %d", len(args))
			}

			str, ok := args[0].(*String)
			if !ok {
				return newError("Argument 0 to `sepr` must be STRING, got %s", args[0].Type())
			}

			sep := ""
			if len(args) == 2 {
				s, ok := args[1].(*String)
				if !ok {
					return newError("Argument 1 to `sepr` must be STRING, got %s", args[1].Type())
				}
				sep = s.Value
			}

			var parts []string
			if sep == "" {
				// Split into individual characters
				for _, r := range str.Value {
					parts = append(parts, string(r))
				}
			} else {
				parts = strings.Split(str.Value, sep)
			}

			elements := make([]Object, len(parts))
			for i, p := range parts {
				elements[i] = &String{Value: p}
			}

			return &Array{Elements: elements}
		}, "string"),
	},
	// File builtins
	{
		"read",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expected 1, got %d", len(args))
			}

			fileName, ok := args[0].(*String)
			if !ok {
				return newError("Argument 0 to `read` must be STRING, got %s", args[0].Type())
			}

			content, err := os.ReadFile(fileName.Value)

			if err != nil {
				return newError("Failed to read file: %s", err)
			}

			return &String{Value: string(content)}
		}, "file"),
	},
	{
		"write",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 2 && len(args) != 3 {
				return newError("Wrong number of arguments. Expected 2 or 3, got %d", len(args))
			}

			filepath, ok1 := args[0].(*String)
			data, ok2 := args[1].(*String)

			if !ok1 || !ok2 {
				return newError("Arguments 0 and 1 to `write` must be STRING and STRING, got %s and %s", args[0].Type(), args[1].Type())
			}

			if len(args) == 3 {
				permissions, ok := args[2].(*Integer)
				if !ok {
					return newError("Argument 2 to `write` must be INTEGER, got %s", args[2].Type())
				}

				_, err2 := os.Stat(filepath.Value)
				if err2 != nil {
					if os.IsNotExist(err2) {
						newfile, err3 := os.Create(filepath.Value)
						if err3 != nil {
							return newError("Error writing out file: %s", err3)
						}
						defer newfile.Close()
					} else {
						return newError("Error opening file: %s", err2)
					}
				}

				err := os.WriteFile(filepath.Value, []byte(data.Value), os.FileMode(permissions.Value))
				if err != nil {
					return newError("Error writing file: %s", err)
				}
				return &Null{}
			}

			_, err2 := os.Stat(filepath.Value)
			if err2 != nil {
				if os.IsNotExist(err2) {
					newfile, err3 := os.Create(filepath.Value)
					if err3 != nil {
						return newError("Error writing out file: %s", err3)
					}
					defer newfile.Close()
				} else {
					return newError("Error opening file: %s", err2)
				}
			}

			err := os.WriteFile(filepath.Value, []byte(data.Value), 0755)
			if err != nil {
				return newError("Error writing file: %s", err)
			}
			return &Null{}
		}, "file"),
	},
	{
		"pop",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expected 1, got %d", len(args))
			}

			array, ok := args[0].(*Array)
			if !ok {
				return newError("Argument 0 to `pop` must be ARRAY, got %s", args[0].Type())
			}

			length := len(array.Elements)
			if length == 0 {
				return &Null{}
			}

			array.Elements = array.Elements[:length-1]
			return array
		}, "array"),
	},
	{
		"remove",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 2 {
				return newError("Wrong number of arguments. Expected 2, got %d", len(args))
			}

			array, ok := args[0].(*Array)
			if !ok {
				return newError("Argument 0 to `remove` must be ARRAY, got %s", args[0].Type())
			}

			indexObj, ok := args[1].(*Integer)
			if !ok {
				return newError("Argument 1 to `remove` must be INTEGER, got %s", args[1].Type())
			}

			index := int(indexObj.Value)
			if index < 0 || index >= len(array.Elements) {
				return newError("Index %d is out of range (array length is %d)", index, len(array.Elements))
			}

			array.Elements = append(array.Elements[:index], array.Elements[index+1:]...)
			return array
		}, "array"),
	},
	{
		"cat",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 1 {
				return newError("Wrong number of arguments. Expected 1, got %d", len(args))
			}

			switch arg := args[0].(type) {
			case *Array:
				return &Integer{Value: int64(len(arg.Elements))}
			case *String:
				return &Integer{Value: int64(len(arg.Value))}
			default:
				return newError("Argument 0 to `cat` is not supported, got %s", args[0].Type())
			}
		}, "array"),
	},
	{
		"join",
		createBuiltin(func(args ...Object) Object {
			if len(args) != 2 {
				return newError("Wrong number of arguments. Expected 2, got %d", len(args))
			}

			arr, ok := args[0].(*Array)
			if !ok {
				return newError("Argument 0 to `join` must be ARRAY, got %s", args[0].Type())
			}

			sep, ok := args[1].(*String)
			if !ok {
				return newError("Argument 1 to `join` must be STRING, got %s", args[1].Type())
			}

			strs := make([]string, len(arr.Elements))
			for i, el := range arr.Elements {
				strs[i] = el.Inspect()
			}

			return &String{Value: strings.Join(strs, sep.Value)}
		}, "array"),
	},
}

func newError(format string, a ...interface{}) *Error {
	return &Error{Message: fmt.Sprintf(format, a...)}
}

func GetBuiltinByName(name string) *Builtin {
	for _, def := range Builtins {
		if def.Name == name {
			return def.Builtin
		}
	}

	return nil
}

var GUIEventHandlers map[string][]Object

func CreateClassObjects() map[string]*Hash {
	classes := make(map[string]*Hash)

	ioClass := &Hash{Pairs: make(map[HashKey]HashPair)}
	typeClass := &Hash{Pairs: make(map[HashKey]HashPair)}
	timeClass := &Hash{Pairs: make(map[HashKey]HashPair)}
	osClass := &Hash{Pairs: make(map[HashKey]HashPair)}
	mathClass := &Hash{Pairs: make(map[HashKey]HashPair)}
	stringClass := &Hash{Pairs: make(map[HashKey]HashPair)}
	fileClass := &Hash{Pairs: make(map[HashKey]HashPair)}
	pkgClass := &Hash{Pairs: make(map[HashKey]HashPair)}
	arrayClass := &Hash{Pairs: make(map[HashKey]HashPair)}
	sysClass := &Hash{Pairs: make(map[HashKey]HashPair)}
	keyboardClass := &Hash{Pairs: make(map[HashKey]HashPair)}

	for _, def := range Builtins {
		if def.Builtin.Class != "" {
			funcName := &String{Value: def.Name}
			key := funcName.HashKey()

			switch def.Builtin.Class {
			case "io":
				ioClass.Pairs[key] = HashPair{Key: funcName, Value: def.Builtin}
			case "type":
				typeClass.Pairs[key] = HashPair{Key: funcName, Value: def.Builtin}
			case "time":
				timeClass.Pairs[key] = HashPair{Key: funcName, Value: def.Builtin}
			case "os":
				osClass.Pairs[key] = HashPair{Key: funcName, Value: def.Builtin}
			case "math":
				mathClass.Pairs[key] = HashPair{Key: funcName, Value: def.Builtin}
			case "string":
				stringClass.Pairs[key] = HashPair{Key: funcName, Value: def.Builtin}
			case "file":
				fileClass.Pairs[key] = HashPair{Key: funcName, Value: def.Builtin}
			case "pkg":
				pkgClass.Pairs[key] = HashPair{Key: funcName, Value: def.Builtin}
			case "array":
				arrayClass.Pairs[key] = HashPair{Key: funcName, Value: def.Builtin}
			case "sys":
				sysClass.Pairs[key] = HashPair{Key: funcName, Value: def.Builtin}
			case "keyboard":
				keyboardClass.Pairs[key] = HashPair{Key: funcName, Value: def.Builtin}
			}
		}
	}

	classes["io"] = ioClass
	classes["type"] = typeClass
	classes["time"] = timeClass
	classes["os"] = osClass
	classes["math"] = mathClass
	classes["string"] = stringClass
	classes["file"] = fileClass
	classes["pkg"] = pkgClass
	classes["array"] = arrayClass
	classes["sys"] = sysClass
	classes["keyboard"] = keyboardClass

	return classes
}

func ListDefinedClasses() string {
	classes := CreateClassObjects()
	var classNames []string
	for className := range classes {
		classNames = append(classNames, className)
	}
	sort.Strings(classNames)
	return strings.Join(classNames, ", ")
}
