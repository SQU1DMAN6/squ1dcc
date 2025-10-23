package token

type TokenType string
type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Column  int
}

const (
	ILLEGAL   = "ILLEGAL"
	EOF       = "EOF"
	IDENT     = "IDENT"
	INT       = "INT"
	FLOAT     = "FLOAT"
	ASSIGN    = "="
	PLUS      = "+"
	COMMA     = ","
	SEMICOLON = ";"
	LPAREN    = "("
	RPAREN    = ")"
	LBRACE    = "{"
	RBRACE    = "}"
	FUNCTION  = "FUNCTION"
	LET       = "VAR"
	MINUS     = "-"
	BANG      = "!"
	ASTERISK  = "*"
	SLASH     = "/"
	MODULO    = "%"
	LT        = "<"
	GT        = ">"
	LE        = "<="
	GE        = ">="
	RETURN    = "RETURN"
	TRUE      = "TRUE"
	FALSE     = "FALSE"
	NULL      = "NULL"
	IF        = "IF"
	ELSE      = "ELSE"
	ELIF      = "ELIF"
	WHILE     = "WHILE"
	EQ        = "=="
	NOT_EQ    = "!="
	STRING    = "STRING"
	LBRACKET  = "["
	RBRACKET  = "]"
	COLON     = ":"
	DOT       = "."
	COMMENT   = "COMMENT"
	AND       = "AND"
	OR        = "OR"
	BACKTICK  = "BACKTICK"
	SUPPRESS  = "SUPPRESS"
)

var keywords = map[string]TokenType{
	"def":      FUNCTION,
	"var":      LET,
	"true":     TRUE,
	"false":    FALSE,
	"nullus":   NULL,
	"if":       IF,
	"el":       ELSE,
	"elif":     ELIF,
	"while":    WHILE,
	"return":   RETURN,
	"ac":       AND,
	"aut":      OR,
	"suppress": SUPPRESS,
}

func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}
