package evaluator

import (
	"squ1d++/object"
)

var builtins = map[string]*object.Builtin{
	"cat":    object.GetBuiltinByName("cat"),
	"write":  object.GetBuiltinByName("write"),
	"append": object.GetBuiltinByName("append"),
	"read":   object.GetBuiltinByName("read"),
	"tp":     object.GetBuiltinByName("tp"),
}
