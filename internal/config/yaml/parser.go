package yaml

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Parser converts YAML tokens into a tree structure.
type Parser struct {
	tokens []Token
	pos    int
}

// NewParser creates a new parser from tokens.
func NewParser(tokens []Token) *Parser {
	return &Parser{tokens: tokens}
}

// Parse reads YAML and returns a map[string]any.
func Parse(r io.Reader) (map[string]any, error) {
	tokens, err := Tokenize(r)
	if err != nil {
		return nil, fmt.Errorf("tokenization failed: %w", err)
	}

	// Filter out comments and newlines at the start
	var filtered []Token
	for _, tok := range tokens {
		if tok.Kind != TokenComment {
			filtered = append(filtered, tok)
		}
	}

	p := NewParser(filtered)
	result, err := p.parseDocument()
	if err != nil {
		return nil, err
	}

	return result, nil
}

// ParseString parses YAML from a string.
func ParseString(s string) (map[string]any, error) {
	return Parse(strings.NewReader(s))
}

// current returns the current token without consuming it.
func (p *Parser) current() Token {
	if p.pos >= len(p.tokens) {
		return Token{Kind: TokenEOF}
	}
	return p.tokens[p.pos]
}

// peek returns the next token without consuming it.
func (p *Parser) peek() Token {
	if p.pos+1 >= len(p.tokens) {
		return Token{Kind: TokenEOF}
	}
	return p.tokens[p.pos+1]
}

// consume returns the current token and advances.
func (p *Parser) consume() Token {
	if p.pos >= len(p.tokens) {
		return Token{Kind: TokenEOF}
	}
	tok := p.tokens[p.pos]
	p.pos++
	return tok
}

// expect consumes a token of the expected kind, or returns an error.
func (p *Parser) expect(kind TokenKind) (Token, error) {
	tok := p.current()
	if tok.Kind != kind {
		return Token{}, fmt.Errorf("expected %v, got %v at line %d, column %d", kind, tok.Kind, tok.Line, tok.Column)
	}
	return p.consume(), nil
}

// skipNewlines skips any newline tokens.
func (p *Parser) skipNewlines() {
	for p.current().Kind == TokenNewline {
		p.consume()
	}
}

// skipIndentsAndNewlines skips indentation and newline tokens.
func (p *Parser) skipIndentsAndNewlines() {
	for {
		switch p.current().Kind {
		case TokenIndent, TokenNewline:
			p.consume()
		default:
			return
		}
	}
}

// parseDocument parses the root document.
func (p *Parser) parseDocument() (map[string]any, error) {
	result := make(map[string]any)

	p.skipNewlines()

	for p.current().Kind != TokenEOF {
		// Skip leading indent at document level
		if p.current().Kind == TokenIndent {
			p.consume()
		}

		if p.current().Kind == TokenEOF {
			break
		}

		// Parse top-level entry
		if p.current().Kind != TokenKey {
			return nil, fmt.Errorf("expected key at line %d, got %v", p.current().Line, p.current().Kind)
		}

		keyTok := p.consume()
		key := keyTok.Value

		// Expect colon
		if _, err := p.expect(TokenColon); err != nil {
			return nil, err
		}

		// Parse value
		value, err := p.parseValue(0)
		if err != nil {
			return nil, fmt.Errorf("parsing value for key %q: %w", key, err)
		}

		result[key] = value

		p.skipIndentsAndNewlines()
	}

	return result, nil
}

// parseValue parses a value, tracking the base indentation.
func (p *Parser) parseValue(baseIndent int) (any, error) {
	p.skipIndentsAndNewlines()

	tok := p.current()

	switch tok.Kind {
	case TokenEOF:
		return nil, nil

	case TokenNewline:
		p.consume()
		// Multi-line value follows
		return p.parseMultiLine(baseIndent)

	case TokenLiteralPipe:
		p.consume()
		return p.parseLiteralBlock(baseIndent)

	case TokenFoldedGreater:
		p.consume()
		return p.parseFoldedBlock(baseIndent)

	case TokenListMarker:
		return p.parseList(baseIndent)

	case TokenKey:
		// This is a nested map
		return p.parseMap(baseIndent)

	case TokenValue:
		p.consume()
		return parseScalar(tok.Value)

	case TokenColon:
		// Empty value
		return nil, nil

	default:
		return nil, fmt.Errorf("unexpected token %v at line %d", tok.Kind, tok.Line)
	}
}

// parseScalar parses a scalar value.
func parseScalar(s string) (any, error) {
	s = strings.TrimSpace(s)

	// Empty/null
	if s == "" || s == "~" || s == "null" || s == "Null" || s == "NULL" {
		return nil, nil
	}

	// Boolean
	switch s {
	case "true", "True", "TRUE", "yes", "Yes", "YES", "on", "On", "ON":
		return true, nil
	case "false", "False", "FALSE", "no", "No", "NO", "off", "Off", "OFF":
		return false, nil
	}

	// Integer
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i, nil
	}

	// Float
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f, nil
	}

	// String (unquote if quoted)
	if len(s) >= 2 && (s[0] == '"' && s[len(s)-1] == '"' || s[0] == '\'' && s[len(s)-1] == '\'') {
		return s[1 : len(s)-1], nil
	}

	return s, nil
}

// parseMap parses a nested map.
func (p *Parser) parseMap(baseIndent int) (map[string]any, error) {
	result := make(map[string]any)

	// Track the expected indentation level for this map
	expectedIndent := -1

	for {
		// Skip any indents/newlines before checking for key
		p.skipIndentsAndNewlines()

		// Check for end of map
		if p.current().Kind != TokenKey {
			break
		}

		// Get the key token to check its position
		keyTok := p.current()

		// Determine the indentation of this key
		// We need to look back to see what indent level this key is at
		// For now, we use a simple heuristic: if we're at the start, accept any indent
		// Otherwise, check if this key's column indicates it's at the right level
		keyIndent := keyTok.Column

		// Set expected indent on first key
		if expectedIndent == -1 {
			expectedIndent = keyIndent
		}

		// If this key is at a lower indent than expected, it's a parent-level key
		if keyIndent < expectedIndent {
			break
		}

		// If this key is at a higher indent, it's nested further - shouldn't happen
		// but we handle it gracefully
		if keyIndent > expectedIndent {
			// This is a nested map - but we're in a loop that shouldn't handle this
			// Actually, this shouldn't happen with proper YAML
		}

		// Consume key
		key := keyTok.Value
		p.consume()

		// Expect colon
		if _, err := p.expect(TokenColon); err != nil {
			return nil, err
		}

		// Parse value - pass the expected indent so nested structures work
		value, err := p.parseValue(expectedIndent)
		if err != nil {
			return nil, fmt.Errorf("parsing value for key %q: %w", key, err)
		}

		result[key] = value
	}

	return result, nil
}

// parseList parses a YAML list.
func (p *Parser) parseList(baseIndent int) ([]any, error) {
	var result []any

	for {
		// Skip to list marker
		p.skipIndentsAndNewlines()

		if p.current().Kind != TokenListMarker {
			break
		}

		p.consume() // consume '-'

		// Skip any whitespace after marker
		p.skipIndentsAndNewlines()

		// Parse the list item
		item, err := p.parseListItem(baseIndent)
		if err != nil {
			return nil, err
		}

		result = append(result, item)

		// Position for next item
		p.skipIndentsAndNewlines()
	}

	return result, nil
}

// parseListItem parses a single list item.
func (p *Parser) parseListItem(baseIndent int) (any, error) {
	tok := p.current()

	switch tok.Kind {
	case TokenKey:
		// Map item in list
		return p.parseMap(baseIndent)

	case TokenListMarker:
		// Nested list
		return p.parseList(baseIndent)

	case TokenValue:
		p.consume()
		return parseScalar(tok.Value)

	case TokenEOF:
		return nil, nil

	default:
		// Could be a nested structure
		return p.parseValue(baseIndent)
	}
}

// parseMultiLine parses a value that spans multiple lines.
func (p *Parser) parseMultiLine(baseIndent int) (any, error) {
	p.skipIndentsAndNewlines()

	tok := p.current()

	switch tok.Kind {
	case TokenListMarker:
		return p.parseList(baseIndent)

	case TokenKey:
		return p.parseMap(baseIndent)

	case TokenValue:
		p.consume()
		return parseScalar(tok.Value)

	default:
		return nil, nil
	}
}

// parseLiteralBlock parses a literal block (|).
func (p *Parser) parseLiteralBlock(baseIndent int) (string, error) {
	p.skipNewlines()

	var lines []string
	contentIndent := -1

	for {
		// Get current indentation
		tok := p.current()
		if tok.Kind == TokenEOF {
			break
		}

		if tok.Kind == TokenIndent {
			p.consume()
			tok = p.current()
		}

		// Check if we've returned to base level
		if tok.Kind != TokenIndent && tok.Indent <= baseIndent && tok.Kind != TokenEOF {
			// End of block
			break
		}

		if tok.Kind == TokenNewline {
			p.consume()
			lines = append(lines, "")
			continue
		}

		if tok.Kind == TokenEOF {
			break
		}

		// Read line content
		if tok.Kind == TokenValue {
			if contentIndent == -1 {
				contentIndent = tok.Indent
			}
			lines = append(lines, tok.Value)
			p.consume()
		}

		// Skip newline
		if p.current().Kind == TokenNewline {
			p.consume()
		}
	}

	// Join lines with literal newlines
	return strings.Join(lines, "\n"), nil
}

// parseFoldedBlock parses a folded block (>).
func (p *Parser) parseFoldedBlock(baseIndent int) (string, error) {
	p.skipNewlines()

	var lines []string
	var paragraph strings.Builder

	for {
		tok := p.current()
		if tok.Kind == TokenEOF {
			if paragraph.Len() > 0 {
				lines = append(lines, paragraph.String())
			}
			break
		}

		if tok.Kind == TokenIndent {
			p.consume()
			tok = p.current()
		}

		// Check if we've returned to base level
		if tok.Indent <= baseIndent && tok.Kind != TokenEOF && tok.Kind != TokenNewline {
			// End of block
			if paragraph.Len() > 0 {
				lines = append(lines, paragraph.String())
			}
			break
		}

		// Empty line = paragraph break
		if tok.Kind == TokenNewline {
			p.consume()
			if paragraph.Len() > 0 {
				lines = append(lines, paragraph.String())
				paragraph.Reset()
			}
			continue
		}

		// Read line content
		if tok.Kind == TokenValue {
			if paragraph.Len() > 0 {
				paragraph.WriteString(" ")
			}
			paragraph.WriteString(tok.Value)
			p.consume()
		}

		// Skip newline
		if p.current().Kind == TokenNewline {
			p.consume()
		}
	}

	return strings.Join(lines, "\n"), nil
}

// envVarRegex matches ${VAR} and ${VAR:default} patterns.
var envVarRegex = regexp.MustCompile(`\$\{([^}]+)\}`)

// InterpolateEnv replaces ${VAR} and ${VAR:default} patterns in strings.
func InterpolateEnv(s string) string {
	return envVarRegex.ReplaceAllStringFunc(s, func(match string) string {
		// Extract content from ${...}
		content := match[2 : len(match)-1]

		// Check for default value
		parts := strings.SplitN(content, ":", 2)
		varName := parts[0]

		value := os.Getenv(varName)
		if value != "" {
			return value
		}

		// Return default if provided
		if len(parts) > 1 {
			return parts[1]
		}

		// Return original if no env var and no default
		return match
	})
}

// InterpolateEnvInMap recursively interpolates environment variables in a map.
func InterpolateEnvInMap(m map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range m {
		result[k] = interpolateValue(v)
	}
	return result
}

// interpolateValue recursively interpolates env vars in any value.
func interpolateValue(v any) any {
	switch val := v.(type) {
	case string:
		return InterpolateEnv(val)
	case map[string]any:
		return InterpolateEnvInMap(val)
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = interpolateValue(item)
		}
		return result
	default:
		return v
	}
}
