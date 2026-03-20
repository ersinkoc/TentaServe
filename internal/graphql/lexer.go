package graphql

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Lexer tokenizes GraphQL query strings into a stream of tokens.
type Lexer struct {
	input   string // input string being tokenized
	pos     int    // current position in input (points to current char)
	readPos int    // current reading position in input (after current char)
	ch      rune   // current char under examination

	line   int // current line number (1-indexed)
	column int // current column number (1-indexed)
}

// NewLexer creates a new GraphQL lexer from a string.
func NewLexer(input string) *Lexer {
	l := &Lexer{
		input:  input,
		line:   1,
		column: 0,
	}
	l.readChar() // initialize l.ch
	return l
}

// readChar reads the next character from input and advances positions.
func (l *Lexer) readChar() {
	if l.readPos >= len(l.input) {
		l.ch = 0 // EOF represented as NUL character
	} else {
		var size int
		l.ch, size = utf8.DecodeRuneInString(l.input[l.readPos:])
		if l.ch == utf8.RuneError && size == 1 {
			// Invalid UTF-8 sequence
			l.ch = 0
		}
	}

	if l.ch == '\n' {
		l.line++
		l.column = 0
	} else {
		l.column++
	}

	l.pos = l.readPos
	l.readPos += utf8.RuneLen(l.ch)
}

// peek returns the next character without consuming it.
func (l *Lexer) peek() rune {
	if l.readPos >= len(l.input) {
		return 0
	}
	ch, _ := utf8.DecodeRuneInString(l.input[l.readPos:])
	return ch
}

// peekN returns the nth character ahead without consuming anything.
func (l *Lexer) peekN(n int) rune {
	pos := l.readPos
	for i := 0; i < n && pos < len(l.input); i++ {
		ch, size := utf8.DecodeRuneInString(l.input[pos:])
		if i == n-1 {
			return ch
		}
		pos += size
	}
	return 0
}

// skipWhitespace skips spaces, tabs, newlines, and commas.
// Note: GraphQL treats commas as insignificant whitespace.
func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' || l.ch == ',' || l.ch == '\ufeff' {
		l.readChar()
	}
}

// skipComment skips from # to end of line.
func (l *Lexer) skipComment() {
	if l.ch == '#' {
		for l.ch != '\n' && l.ch != 0 {
			l.readChar()
		}
	}
}

// NextToken returns the next token from the input.
func (l *Lexer) NextToken() Token {
	var tok Token

	// Skip whitespace and comments
	for {
		l.skipWhitespace()
		if l.ch == '#' {
			l.skipComment()
		} else {
			break
		}
	}

	tok.Line = l.line
	tok.Column = l.column

	switch l.ch {
	case 0:
		tok.Kind = TokenEOF
		tok.Value = ""

	case '{':
		tok.Kind = TokenBraceL
		tok.Value = "{"
		l.readChar()
	case '}':
		tok.Kind = TokenBraceR
		tok.Value = "}"
		l.readChar()
	case '(':
		tok.Kind = TokenParenL
		tok.Value = "("
		l.readChar()
	case ')':
		tok.Kind = TokenParenR
		tok.Value = ")"
		l.readChar()
	case '[':
		tok.Kind = TokenBracketL
		tok.Value = "["
		l.readChar()
	case ']':
		tok.Kind = TokenBracketR
		tok.Value = "]"
		l.readChar()
	case ':':
		tok.Kind = TokenColon
		tok.Value = ":"
		l.readChar()
	case ',':
		tok.Kind = TokenComma
		tok.Value = ","
		l.readChar()
	case '@':
		tok.Kind = TokenAt
		tok.Value = "@"
		l.readChar()
	case '$':
		tok.Kind = TokenDollar
		tok.Value = "$"
		l.readChar()
	case '=':
		tok.Kind = TokenEquals
		tok.Value = "="
		l.readChar()
	case '!':
		tok.Kind = TokenBang
		tok.Value = "!"
		l.readChar()
	case '|':
		tok.Kind = TokenPipe
		tok.Value = "|"
		l.readChar()
	case '&':
		tok.Kind = TokenAmpersand
		tok.Value = "&"
		l.readChar()

	case '.':
		// Check for spread operator (...)
		if l.peek() == '.' && l.peekN(2) == '.' {
			tok.Kind = TokenSpread
			tok.Value = "..."
			l.readChar()
			l.readChar()
			l.readChar()
		} else {
			tok.Kind = TokenIllegal
			tok.Value = string(l.ch)
			l.readChar()
		}

	case '"':
		// Check for block string (""") or regular string (")
		if l.peek() == '"' && l.peekN(2) == '"' {
			return l.readBlockString()
		}
		return l.readString()

	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return l.readNumber()

	default:
		if isNameStart(l.ch) {
			return l.readName()
		}
		tok.Kind = TokenIllegal
		tok.Value = string(l.ch)
		l.readChar()
	}

	return tok
}

// readName reads a GraphQL name (identifier or keyword).
func (l *Lexer) readName() Token {
	startLine := l.line
	startCol := l.column
	startPos := l.pos

	for isNameContinue(l.ch) {
		l.readChar()
	}

	value := l.input[startPos:l.pos]
	kind := LookupKeyword(value)

	return Token{
		Kind:   kind,
		Value:  value,
		Line:   startLine,
		Column: startCol,
	}
}

// readNumber reads an integer or float literal.
func (l *Lexer) readNumber() Token {
	startLine := l.line
	startCol := l.column
	startPos := l.pos

	isFloat := false

	// Optional leading minus
	if l.ch == '-' {
		l.readChar()
	}

	// Integer part
	if l.ch == '0' {
		l.readChar()
		// After 0, we can't have more digits unless it's a float
	} else if isDigit(l.ch) {
		for isDigit(l.ch) {
			l.readChar()
		}
	}

	// Fractional part
	if l.ch == '.' {
		isFloat = true
		l.readChar()
		// Must have at least one digit after decimal point
		if !isDigit(l.ch) {
			return Token{
				Kind:   TokenIllegal,
				Value:  l.input[startPos:l.pos],
				Line:   startLine,
				Column: startCol,
			}
		}
		for isDigit(l.ch) {
			l.readChar()
		}
	}

	// Exponent part
	if l.ch == 'e' || l.ch == 'E' {
		isFloat = true
		l.readChar()
		if l.ch == '+' || l.ch == '-' {
			l.readChar()
		}
		if !isDigit(l.ch) {
			return Token{
				Kind:   TokenIllegal,
				Value:  l.input[startPos:l.pos],
				Line:   startLine,
				Column: startCol,
			}
		}
		for isDigit(l.ch) {
			l.readChar()
		}
	}

	value := l.input[startPos:l.pos]
	if isFloat {
		return Token{
			Kind:   TokenFloat,
			Value:  value,
			Line:   startLine,
			Column: startCol,
		}
	}
	return Token{
		Kind:   TokenInt,
		Value:  value,
		Line:   startLine,
		Column: startCol,
	}
}

// readString reads a double-quoted string literal.
func (l *Lexer) readString() Token {
	startLine := l.line
	startCol := l.column

	// Consume opening quote
	l.readChar()

	var result strings.Builder

	for l.ch != '"' && l.ch != 0 && l.ch != '\n' {
		if l.ch == '\\' {
			// Escape sequence
			l.readChar()                      // move to escape character
			escaped := l.readEscapeSequence() // this will position after the escape
			if escaped == utf8.RuneError {
				// Invalid escape, write as-is
				result.WriteRune('\\')
				if l.ch != 0 {
					result.WriteRune(l.ch)
					l.readChar()
				}
			} else {
				result.WriteRune(escaped)
			}
		} else {
			result.WriteRune(l.ch)
			l.readChar()
		}
	}

	if l.ch != '"' {
		return Token{
			Kind:   TokenIllegal,
			Value:  result.String(),
			Line:   startLine,
			Column: startCol,
		}
	}

	// Consume closing quote
	l.readChar()

	return Token{
		Kind:   TokenString,
		Value:  result.String(),
		Line:   startLine,
		Column: startCol,
	}
}

// readBlockString reads a block string literal ("""...""")
func (l *Lexer) readBlockString() Token {
	startLine := l.line
	startCol := l.column

	// Consume opening """
	l.readChar()
	l.readChar()
	l.readChar()
	startPos := l.pos

	// Find closing """
	for {
		if l.ch == 0 {
			// Unterminated block string
			return Token{
				Kind:   TokenIllegal,
				Value:  l.input[startPos:l.pos],
				Line:   startLine,
				Column: startCol,
			}
		}

		if l.ch == '"' && l.peek() == '"' && l.peekN(2) == '"' {
			// Found closing """
			value := l.input[startPos:l.pos]
			l.readChar() // first "
			l.readChar() // second "
			l.readChar() // third "

			// Process block string value (handle common leading whitespace)
			processed := processBlockString(value)

			return Token{
				Kind:   TokenBlockString,
				Value:  processed,
				Line:   startLine,
				Column: startCol,
			}
		}

		l.readChar()
	}
}

// processBlockString processes a block string value according to GraphQL spec.
// It handles the common leading whitespace stripping.
func processBlockString(value string) string {
	lines := strings.Split(value, "\n")
	if len(lines) == 0 {
		return ""
	}

	// If the first line is empty or whitespace-only, remove it
	// This handles the common case where """ is followed by a newline
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}

	// If the last line is empty or whitespace-only, remove it
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}

	if len(lines) == 0 {
		return ""
	}

	// Find the minimum common indentation (excluding empty lines)
	minIndent := -1
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if len(trimmed) > 0 {
			indent := len(line) - len(trimmed)
			if minIndent == -1 || indent < minIndent {
				minIndent = indent
			}
		}
	}

	if minIndent == -1 {
		minIndent = 0
	}

	// Strip common indentation from all lines
	var result strings.Builder
	for i, line := range lines {
		if i > 0 {
			result.WriteByte('\n')
		}
		trimmed := strings.TrimRight(line, " \t")
		if len(trimmed) >= minIndent {
			result.WriteString(trimmed[minIndent:])
		} else {
			result.WriteString(trimmed)
		}
	}

	return result.String()
}

// readEscapeSequence reads an escape sequence and returns the escaped character.
// The caller is responsible for ensuring l.ch is positioned on the escape character.
// After this function returns, l.ch will be positioned on the character AFTER the escape sequence.
func (l *Lexer) readEscapeSequence() rune {
	ch := l.ch

	switch ch {
	case '"':
		l.readChar()
		return '"'
	case '\\':
		l.readChar()
		return '\\'
	case '/':
		l.readChar()
		return '/'
	case 'b':
		l.readChar()
		return '\b'
	case 'f':
		l.readChar()
		return '\f'
	case 'n':
		l.readChar()
		return '\n'
	case 'r':
		l.readChar()
		return '\r'
	case 't':
		l.readChar()
		return '\t'
	case 'u':
		// Unicode escape: \uXXXX
		l.readChar() // move to first hex digit
		code := 0
		for i := 0; i < 4; i++ {
			if !isHexDigit(l.ch) {
				return utf8.RuneError
			}
			code = code*16 + hexValue(l.ch)
			l.readChar() // this will read 4 hex digits and position AFTER the sequence
		}
		return rune(code)
	default:
		// Invalid escape sequence
		l.readChar()
		return utf8.RuneError
	}
}

// isNameStart returns true if the rune can start a GraphQL name.
func isNameStart(ch rune) bool {
	return ch == '_' || unicode.IsLetter(ch)
}

// isNameContinue returns true if the rune can continue a GraphQL name.
func isNameContinue(ch rune) bool {
	return ch == '_' || unicode.IsLetter(ch) || unicode.IsDigit(ch)
}

// isDigit returns true if the rune is a decimal digit.
func isDigit(ch rune) bool {
	return ch >= '0' && ch <= '9'
}

// isHexDigit returns true if the rune is a hexadecimal digit.
func isHexDigit(ch rune) bool {
	return isDigit(ch) || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}

// hexValue returns the numeric value of a hex digit.
func hexValue(ch rune) int {
	switch {
	case ch >= '0' && ch <= '9':
		return int(ch - '0')
	case ch >= 'a' && ch <= 'f':
		return int(ch - 'a' + 10)
	case ch >= 'A' && ch <= 'F':
		return int(ch - 'A' + 10)
	}
	return 0
}

// Tokenize lexes the entire input and returns all tokens.
func Tokenize(input string) ([]Token, error) {
	l := NewLexer(input)
	var tokens []Token

	for {
		tok := l.NextToken()
		if tok.Kind == TokenIllegal {
			return nil, fmt.Errorf("illegal token at line %d, column %d: %q", tok.Line, tok.Column, tok.Value)
		}
		tokens = append(tokens, tok)
		if tok.Kind == TokenEOF {
			break
		}
	}

	return tokens, nil
}
