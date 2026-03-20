// Package yaml provides a minimal YAML parser using only Go standard library.
//
// This package implements a subset of YAML 1.2 sufficient for configuration files:
//   - Key-value pairs (key: value)
//   - Nested maps (indentation-based)
//   - Lists (- item)
//   - Strings (quoted and unquoted)
//   - Numbers, booleans, null
//   - Comments (# comment)
//   - Multi-line strings (| and >)
//   - Environment variable interpolation (${VAR}, ${VAR:default})
//
// NOT supported: anchors (&, *), tags (!!str), flow collections ({a: 1}), multiple documents.
package yaml

import (
	"fmt"
)

// TokenKind represents the type of a YAML token.
type TokenKind int

const (
	TokenEOF TokenKind = iota
	TokenNewline
	TokenIndent
	TokenKey
	TokenValue
	TokenListMarker
	TokenColon
	TokenComment
	TokenLiteralPipe   // |
	TokenFoldedGreater // >
)

// Token represents a lexical token in YAML.
type Token struct {
	Kind   TokenKind
	Value  string
	Indent int
	Line   int
	Column int
}

// String returns a human-readable representation of the token.
func (t Token) String() string {
	names := map[TokenKind]string{
		TokenEOF:           "EOF",
		TokenNewline:       "NEWLINE",
		TokenIndent:        "INDENT",
		TokenKey:           "KEY",
		TokenValue:         "VALUE",
		TokenListMarker:    "LIST",
		TokenColon:         "COLON",
		TokenComment:       "COMMENT",
		TokenLiteralPipe:   "PIPE",
		TokenFoldedGreater: "FOLDED",
	}
	name := names[t.Kind]
	if name == "" {
		name = fmt.Sprintf("Token(%d)", t.Kind)
	}
	return fmt.Sprintf("%s[%q]@%d:%d", name, t.Value, t.Line, t.Column)
}
