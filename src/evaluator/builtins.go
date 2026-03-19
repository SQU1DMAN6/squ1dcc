package evaluator

import (
	"squ1d++/object"
)

var builtins map[string]*object.Builtin

func init() {
	builtins = make(map[string]*object.Builtin)

	for _, def := range object.Builtins {
		builtins[def.Name] = def.Builtin

		if def.Builtin.Class != "" {
			classPrefix := def.Builtin.Class + "."
			builtins[classPrefix+def.Name] = def.Builtin
		}
	}
}

func GetBuiltin(name string) (*object.Builtin, bool) {
	builtin, exists := builtins[name]
	return builtin, exists
}

func GetBuiltinsByClass(class string) map[string]*object.Builtin {
	classBuiltins := make(map[string]*object.Builtin)
	classPrefix := class + "."

	for name, builtin := range builtins {
		if len(name) > len(classPrefix) && name[:len(classPrefix)] == classPrefix {
			funcName := name[len(classPrefix):]
			classBuiltins[funcName] = builtin
		}
	}

	return classBuiltins
}

func GetAllBuiltins() map[string]map[string]*object.Builtin {
	allBuiltins := make(map[string]map[string]*object.Builtin)

	allBuiltins["core"] = make(map[string]*object.Builtin)

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
