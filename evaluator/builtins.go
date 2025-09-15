package evaluator

import (
	"squ1d++/object"
)

// builtins is automatically generated from object.Builtins
var builtins map[string]*object.Builtin

// init automatically populates the builtins map from object.Builtins
func init() {
	builtins = make(map[string]*object.Builtin)
	
	// Add all builtins from object.Builtins
	for _, def := range object.Builtins {
		// Add the basic builtin name
		builtins[def.Name] = def.Builtin
		
		// If the builtin has a class, also add it with the class prefix
		if def.Builtin.Class != "" {
			classPrefix := def.Builtin.Class + "."
			builtins[classPrefix+def.Name] = def.Builtin
		}
	}
}

// GetBuiltin retrieves a builtin by name, supporting both direct and namespaced access
func GetBuiltin(name string) (*object.Builtin, bool) {
	builtin, exists := builtins[name]
	return builtin, exists
}

// GetBuiltinsByClass returns all builtins for a specific class
func GetBuiltinsByClass(class string) map[string]*object.Builtin {
	classBuiltins := make(map[string]*object.Builtin)
	classPrefix := class + "."
	
	for name, builtin := range builtins {
		if len(name) > len(classPrefix) && name[:len(classPrefix)] == classPrefix {
			// Extract the function name without the class prefix
			funcName := name[len(classPrefix):]
			classBuiltins[funcName] = builtin
		}
	}
	
	return classBuiltins
}

// GetAllBuiltins returns all builtins, organized by class
func GetAllBuiltins() map[string]map[string]*object.Builtin {
	allBuiltins := make(map[string]map[string]*object.Builtin)
	
	// Initialize with core builtins (no class)
	allBuiltins["core"] = make(map[string]*object.Builtin)
	
	// Group builtins by class
	for _, def := range object.Builtins {
		if def.Builtin.Class == "" {
			allBuiltins["core"][def.Name] = def.Builtin
		} else {
			if allBuiltins[def.Builtin.Class] == nil {
				allBuiltins[def.Builtin.Class] = make(map[string]*object.Builtin)
			}
			allBuiltins[def.Builtin.Class][def.Name] = def.Builtin
		}
	}
	
	return allBuiltins
}
