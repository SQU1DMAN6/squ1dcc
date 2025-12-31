package lexer

import (
	"squ1d++/token"
)

type Lexer struct {
	input        string
	position     int
	readPosition int
	ch           byte
	line         int
	column       int
}

func New(input string) *Lexer {
	l := &Lexer{
		input:  input,
		line:   1,
		column: 1,
	}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPosition]
		if l.ch == '\n' {
			l.line++
			l.column = 1
		} else {
			l.column++
		}
	}
	l.position = l.readPosition
	l.readPosition += 1
}

func (l *Lexer) NextToken() token.Token {
	var tok token.Token

	l.skipWhitespace()
	startLine := l.line
	startCol := l.column
	if startCol > 1 {
		startCol = startCol - 1
	}
	tok.Line = startLine
	tok.Column = startCol

	switch l.ch {

	default:
		if isLetter(l.ch) {
			tok.Literal = l.readIdentifier()
			tok.Type = token.LookupIdent(tok.Literal)
			tok.Line = startLine
			tok.Column = startCol
			return tok
		} else if isDigit(l.ch) {
			position := l.position
			for isDigit(l.ch) {
				l.readChar()
			}

			if l.ch == '.' {
				l.readChar()
				for isDigit(l.ch) {
					l.readChar()
				}
				tok.Type = token.FLOAT
				tok.Literal = l.input[position:l.position]
				tok.Line = startLine
				tok.Column = startCol
				return tok
			}

			// This is just an integer
			tok.Type = token.INT
			tok.Literal = l.input[position:l.position]
			tok.Line = startLine
			tok.Column = startCol
			return tok
		} else {
			tok = newToken(token.ILLEGAL, l.ch)
			tok.Line = startLine
			tok.Column = startCol
		}
	case '=':
		tok = newToken(token.ASSIGN, l.ch)
		tok.Line = startLine
		tok.Column = startCol
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.EQ, Literal: string(ch) + string(l.ch), Line: startLine, Column: startCol}
		} else {
			tok = newToken(token.ASSIGN, l.ch)
			tok.Line = startLine
			tok.Column = startCol
		}
	case '-':
		tok = newToken(token.MINUS, l.ch)
		tok.Line = startLine
		tok.Column = startCol
	case '/':
		tok = newToken(token.SLASH, l.ch)
		tok.Line = startLine
		tok.Column = startCol
	case '*':
		tok = newToken(token.ASTERISK, l.ch)
		tok.Line = startLine
		tok.Column = startCol
	case '%':
		tok = newToken(token.MODULO, l.ch)
		tok.Line = startLine
		tok.Column = startCol
	case '<':
		if l.peekChar() == '<' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.ERROR_PIPE, Literal: string(ch) + string(l.ch), Line: startLine, Column: startCol}
		} else if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.LE, Literal: string(ch) + string(l.ch), Line: startLine, Column: startCol}
		} else {
			tok = newToken(token.LT, l.ch)
			tok.Line = startLine
			tok.Column = startCol
		}
	case '>':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.GE, Literal: string(ch) + string(l.ch), Line: startLine, Column: startCol}
		} else {
			tok = newToken(token.GT, l.ch)
			tok.Line = startLine
			tok.Column = startCol
		}
	case ';':
		tok = newToken(token.SEMICOLON, l.ch)
		tok.Line = startLine
		tok.Column = startCol
	case '(':
		tok = newToken(token.LPAREN, l.ch)
		tok.Line = startLine
		tok.Column = startCol
	case ')':
		tok = newToken(token.RPAREN, l.ch)
		tok.Line = startLine
		tok.Column = startCol
	case ',':
		tok = newToken(token.COMMA, l.ch)
		tok.Line = startLine
		tok.Column = startCol
	case '+':
		tok = newToken(token.PLUS, l.ch)
		tok.Line = startLine
		tok.Column = startCol
	case '{':
		tok = newToken(token.LBRACE, l.ch)
		tok.Line = startLine
		tok.Column = startCol
	case '}':
		tok = newToken(token.RBRACE, l.ch)
		tok.Line = startLine
		tok.Column = startCol
	case '!':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.NOT_EQ, Literal: string(ch) + string(l.ch), Line: startLine, Column: startCol}
		} else {
			tok = newToken(token.BANG, l.ch)
			tok.Line = startLine
			tok.Column = startCol
		}
	case '"':
		tok.Type = token.STRING
		tok.Literal = l.readString()
		tok.Line = startLine
		tok.Column = startCol
	case '\'':
		if l.peekChar() >= '0' && l.peekChar() <= '9' {
			l.readChar()
			position := l.position
			for isDigit(l.ch) {
				l.readChar()
			}
			if l.ch == '.' {
				l.readChar()
				for isDigit(l.ch) {
					l.readChar()
				}
			}
			tok.Type = token.FLOAT
			tok.Literal = l.input[position:l.position]
			tok.Line = startLine
			tok.Column = startCol
		} else {
			// This is a string literal
			tok.Type = token.STRING
			tok.Literal = l.readSingleQuoteString()
			tok.Line = startLine
			tok.Column = startCol
		}
	case '`':
		tok.Type = token.BACKTICK
		mlStr := l.readMLString()
		tok.Literal = mlStr
		tok.Line = startLine
		tok.Column = startCol
	case '[':
		tok = newToken(token.LBRACKET, l.ch)
		tok.Line = startLine
		tok.Column = startCol
	case ']':
		tok = newToken(token.RBRACKET, l.ch)
		tok.Line = startLine
		tok.Column = startCol
	case ':':
		tok = newToken(token.COLON, l.ch)
		tok.Line = startLine
		tok.Column = startCol
	case '.':
		tok = newToken(token.DOT, l.ch)
		tok.Line = startLine
		tok.Column = startCol
	case '#':
		l.skipComment()
		return l.NextToken()
	case 0:
		tok.Literal = ""
		tok.Type = token.EOF
	}

	l.readChar()
	return tok
}

func newToken(tokenType token.TokenType, ch byte) token.Token {
	return token.Token{
		Type:    tokenType,
		Literal: string(ch),
		Line:    0,
		Column:  0,
	}
}

func (l *Lexer) readIdentifier() string {
	position := l.position
	for isLetter(l.ch) || isDigit(l.ch) {
		l.readChar()
	}
	return l.input[position:l.position]
}

func isLetter(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_'
}

func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		l.readChar()
	}
}

func (l *Lexer) readNumber() string {
	position := l.position
	for isDigit(l.ch) {
		l.readChar()
	}
	return l.input[position:l.position]
}

func (l *Lexer) readFloat() string {
	position := l.position

	// Read the integer part
	for isDigit(l.ch) {
		l.readChar()
	}

	// Read the decimal point
	if l.ch == '.' {
		l.readChar()
	}

	// Read the fractional part
	for isDigit(l.ch) {
		l.readChar()
	}
	return l.input[position:l.position]
}

func (l *Lexer) readString() string {
	var result []byte

	for {
		l.readChar()

		if l.ch == '\\' {
			l.readChar() // Move to the character after backslash
			switch l.ch {
			case 'n':
				result = append(result, '\n')
			case 't':
				result = append(result, '\t')
			case 'r':
				result = append(result, '\r')
			case '\\':
				result = append(result, '\\')
			case '"':
				result = append(result, '"')
			default:
				result = append(result, l.ch)
			}
			continue
		}

		if l.ch == '"' || l.ch == 0 {
			break
		}

		result = append(result, l.ch)
	}
	return string(result)
}

func (l *Lexer) readSingleQuoteString() string {
	var result []byte

	for {
		l.readChar()

		if l.ch == '\\' {
			l.readChar() // Move to the character after backslash
			switch l.ch {
			case 'n':
				result = append(result, '\n')
			case 't':
				result = append(result, '\t')
			case 'r':
				result = append(result, '\r')
			case '\\':
				result = append(result, '\\')
			case '\'':
				result = append(result, '\'')
			default:
				result = append(result, l.ch)
			}
			continue
		}

		if l.ch == '\'' || l.ch == 0 {
			break
		}

		result = append(result, l.ch)
	}
	return string(result)
}

func (l *Lexer) readMLString() string {
	var result []byte

	for {
		l.readChar()
		if l.ch == 0 {
			break
		}

		if l.ch == '\\' {
			l.readChar() // Move to the character after backslash
			switch l.ch {
			case 'n':
				result = append(result, '\n')
			case 't':
				result = append(result, '\t')
			case 'r':
				result = append(result, '\r')
			case '\\':
				result = append(result, '\\')
			case '`':
				result = append(result, '`')
			default:
				result = append(result, l.ch)
			}
			continue
		}

		if l.ch == '`' {
			break
		}

		result = append(result, l.ch)
	}
	return string(result)
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

func isFloatChar(ch byte) bool {
	return ('0' <= ch && ch <= '9') || ch == '.' || ch == '-' || ch == '\''
}

func (l *Lexer) peekChar() byte {
	if l.readPosition >= len(l.input) {
		return 0
	} else {
		return l.input[l.readPosition]
	}
}

func (l *Lexer) skipComment() {
	// Skip the opening #
	l.readChar()

	// Read until we find the closing # or end of input
	for l.ch != 0 && l.ch != '#' {
		l.readChar()
	}

	// If we found a closing #, skip it too
	if l.ch == '#' {
		l.readChar()
	}
}

// GetInput returns the original input string
func (l *Lexer) GetInput() string {
	return l.input
}
