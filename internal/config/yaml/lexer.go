package yaml

import (
	"bytes"
	"io"
	"strings"
)

// Lexer tokenizes YAML input into a stream of tokens.
type Lexer struct {
	input   []rune
	pos     int
	line    int
	column  int
	lastRune rune
}

// NewLexer creates a new YAML lexer from an io.Reader.
func NewLexer(r io.Reader) *Lexer {
	buf := new(strings.Builder)
	io.Copy(buf, r)
	return &Lexer{
		input:  []rune(buf.String()),
		line:   1,
		column: 0,
	}
}

// NewLexerString creates a new YAML lexer from a string.
func NewLexerString(s string) *Lexer {
	return &Lexer{
		input:  []rune(s),
		line:   1,
		column: 0,
	}
}

// read returns the next rune and advances.
func (l *Lexer) read() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	r := l.input[l.pos]
	l.pos++
	l.lastRune = r
	if r == '\n' {
		l.line++
		l.column = 0
	} else {
		l.column++
	}
	return r
}

// peek returns the next rune without advancing.
func (l *Lexer) peek() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	return l.input[l.pos]
}

// unread goes back one position.
func (l *Lexer) unread() {
	if l.pos > 0 {
		l.pos--
		r := l.input[l.pos]
		if r == '\n' {
			l.line--
			// Need to recalculate column from previous line
			l.column = 0
			for i := l.pos - 1; i >= 0 && l.input[i] != '\n'; i-- {
				l.column++
			}
		} else {
			l.column--
		}
	}
}

// eof returns true if we're at the end of input.
func (l *Lexer) eof() bool {
	return l.pos >= len(l.input)
}

// skipWhitespace skips spaces and tabs (but not newlines).
func (l *Lexer) skipWhitespace() {
	for {
		r := l.peek()
		if r != ' ' && r != '\t' {
			return
		}
		l.read()
	}
}

// readLine reads until newline or EOF.
func (l *Lexer) readLine() string {
	var buf bytes.Buffer
	for {
		r := l.read()
		if r == 0 || r == '\n' {
			return buf.String()
		}
		buf.WriteRune(r)
	}
}

// NextToken returns the next token from the input.
func (l *Lexer) NextToken() (Token, error) {
	// Skip empty lines and count indentation
	for {
		r := l.peek()

		if r == 0 {
			return Token{Kind: TokenEOF, Line: l.line, Column: l.column}, nil
		}

		// Handle newline
		if r == '\n' {
			l.read() // consume newline
			return Token{Kind: TokenNewline, Line: l.line - 1, Column: l.column}, nil
		}

		// Count leading whitespace (indentation)
		if r == ' ' || r == '\t' {
			startCol := l.column
			indent := 0
			for {
				r2 := l.peek()
				if r2 != ' ' && r2 != '\t' {
					break
				}
				l.read()
				indent++
			}
			return Token{Kind: TokenIndent, Value: "", Indent: indent, Line: l.line, Column: startCol}, nil
		}

		// Skip empty lines
		if r == '\r' {
			l.read()
			continue
		}

		break
	}

	startLine := l.line
	startCol := l.column

	r := l.read()

	// Comment
	if r == '#' {
		line := l.readLine()
		return Token{Kind: TokenComment, Value: line, Line: startLine, Column: startCol}, nil
	}

	// List marker
	if r == '-' {
		next := l.peek()
		if next == ' ' || next == '\t' || next == '\n' || next == 0 {
			return Token{Kind: TokenListMarker, Value: "-", Line: startLine, Column: startCol}, nil
		}
		l.unread()
	}

	// Colon (key separator)
	if r == ':' {
		next := l.peek()
		if next == ' ' || next == '\t' || next == '\n' || next == 0 {
			return Token{Kind: TokenColon, Value: ":", Line: startLine, Column: startCol}, nil
		}
		l.unread()
	}

	// Literal multi-line
	if r == '|' {
		return Token{Kind: TokenLiteralPipe, Value: "|", Line: startLine, Column: startCol}, nil
	}
	if r == '>' {
		return Token{Kind: TokenFoldedGreater, Value: ">", Line: startLine, Column: startCol}, nil
	}

	// Read value/key
	l.unread()
	return l.readValue(startLine, startCol)
}

// readValue reads a value (key or scalar) from the input.
func (l *Lexer) readValue(line, col int) (Token, error) {
	var buf bytes.Buffer
	quoted := false
	quoteChar := rune(0)

	for {
		r := l.read()
		if r == 0 {
			break
		}

		// Handle quotes
		if (r == '"' || r == '\'') && !quoted {
			if buf.Len() == 0 {
				quoted = true
				quoteChar = r
				continue
			}
		}
		if quoted && r == quoteChar {
			quoted = false
			quoteChar = 0
			continue
		}

		// Check for key separator (unquoted colon followed by space/newline)
		if !quoted && r == ':' {
			next := l.peek()
			if next == ' ' || next == '\t' || next == '\n' || next == 0 {
				// This is a key - put back the colon so parser can consume it
				l.unread()
				return Token{Kind: TokenKey, Value: buf.String(), Line: line, Column: col}, nil
			}
		}

		// End of value on unquoted special chars
		if !quoted && (r == '\n' || r == '#') {
			l.unread()
			break
		}

		buf.WriteRune(r)
	}

	value := strings.TrimSpace(buf.String())
	if value == "" {
		// Check if this might be a key waiting for value
		next := l.peek()
		if next == ':' {
			return Token{Kind: TokenKey, Value: value, Line: line, Column: col}, nil
		}
	}

	return Token{Kind: TokenValue, Value: value, Line: line, Column: col}, nil
}

// Tokenize reads all tokens from the input.
func Tokenize(r io.Reader) ([]Token, error) {
	l := NewLexer(r)
	var tokens []Token

	for {
		tok, err := l.NextToken()
		if err != nil {
			return nil, err
		}
		if tok.Kind == TokenEOF {
			tokens = append(tokens, tok)
			break
		}
		tokens = append(tokens, tok)
	}

	return tokens, nil
}

// TokenizeString tokenizes a string.
func TokenizeString(s string) ([]Token, error) {
	return Tokenize(strings.NewReader(s))
}
