package yaml

import (
	"strings"
	"testing"
)

func TestLexerSimple(t *testing.T) {
	yaml := `key: value`

	tokens, err := TokenizeString(yaml)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	t.Logf("Tokens:")
	for i, tok := range tokens {
		t.Logf("  %d: %v", i, tok)
	}

	// Expected: Key, Colon, Value, EOF
	if len(tokens) < 4 {
		t.Fatalf("Expected at least 4 tokens, got %d", len(tokens))
	}

	if tokens[0].Kind != TokenKey || tokens[0].Value != "key" {
		t.Errorf("Expected TokenKey 'key', got %v %q", tokens[0].Kind, tokens[0].Value)
	}
	if tokens[1].Kind != TokenColon {
		t.Errorf("Expected TokenColon, got %v", tokens[1].Kind)
	}
}

func TestLexerNested(t *testing.T) {
	yaml := `
server:
  host: localhost
`

	tokens, err := TokenizeString(yaml)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	t.Logf("Tokens:")
	for i, tok := range tokens {
		t.Logf("  %d: %v", i, tok)
	}
}

// --- Additional lexer tests for coverage ---

func TestNewLexerString_Empty(t *testing.T) {
	l := NewLexerString("")
	if l == nil {
		t.Fatal("NewLexerString returned nil")
	}
	tok, err := l.NextToken()
	if err != nil {
		t.Fatalf("NextToken error: %v", err)
	}
	if tok.Kind != TokenEOF {
		t.Errorf("expected TokenEOF for empty input, got %v", tok.Kind)
	}
}

func TestNewLexerString_OnlyWhitespace(t *testing.T) {
	l := NewLexerString("   \t  ")
	tok, err := l.NextToken()
	if err != nil {
		t.Fatalf("NextToken error: %v", err)
	}
	if tok.Kind != TokenIndent {
		t.Errorf("expected TokenIndent for whitespace-only input, got %v", tok.Kind)
	}
}

func TestLexer_ListMarker(t *testing.T) {
	tokens, err := TokenizeString("- item1\n- item2")
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	listCount := 0
	for _, tok := range tokens {
		if tok.Kind == TokenListMarker {
			listCount++
		}
	}
	if listCount != 2 {
		t.Errorf("expected 2 list markers, got %d", listCount)
	}
}

func TestLexer_Comment(t *testing.T) {
	tokens, err := TokenizeString("# this is a comment\nkey: value")
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	foundComment := false
	for _, tok := range tokens {
		if tok.Kind == TokenComment {
			foundComment = true
		}
	}
	if !foundComment {
		t.Error("expected to find a comment token")
	}
}

func TestLexer_LiteralPipe(t *testing.T) {
	tokens, err := TokenizeString("desc: |\n  line1\n  line2")
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	foundPipe := false
	for _, tok := range tokens {
		if tok.Kind == TokenLiteralPipe {
			foundPipe = true
		}
	}
	if !foundPipe {
		t.Error("expected to find a literal pipe token")
	}
}

func TestLexer_FoldedGreater(t *testing.T) {
	tokens, err := TokenizeString("desc: >\n  line1\n  line2")
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	foundFolded := false
	for _, tok := range tokens {
		if tok.Kind == TokenFoldedGreater {
			foundFolded = true
		}
	}
	if !foundFolded {
		t.Error("expected to find a folded greater token")
	}
}

func TestLexer_QuotedValue(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantKey string
		wantVal string
	}{
		{"double quoted", `key: "hello world"`, "key", "hello world"},
		{"single quoted", `key: 'hello world'`, "key", "hello world"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := TokenizeString(tc.input)
			if err != nil {
				t.Fatalf("Tokenize failed: %v", err)
			}

			foundKey := false
			for _, tok := range tokens {
				if tok.Kind == TokenKey && tok.Value == tc.wantKey {
					foundKey = true
				}
			}
			if !foundKey {
				t.Errorf("expected key %q in tokens", tc.wantKey)
			}
		})
	}
}

func TestLexer_NewlineHandling(t *testing.T) {
	tokens, err := TokenizeString("key1: val1\nkey2: val2\n")
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	newlineCount := 0
	for _, tok := range tokens {
		if tok.Kind == TokenNewline {
			newlineCount++
		}
	}
	if newlineCount < 1 {
		t.Errorf("expected at least 1 newline token, got %d", newlineCount)
	}
}

func TestLexer_CarriageReturn(t *testing.T) {
	// \r should be skipped
	tokens, err := TokenizeString("key: value\r\n")
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	foundKey := false
	for _, tok := range tokens {
		if tok.Kind == TokenKey && tok.Value == "key" {
			foundKey = true
		}
	}
	if !foundKey {
		t.Error("expected to find key token after carriage return")
	}
}

func TestTokenString(t *testing.T) {
	tok := Token{Kind: TokenKey, Value: "mykey", Line: 1, Column: 0}
	s := tok.String()
	if s == "" {
		t.Error("expected non-empty token string")
	}

	// Test unknown token kind
	tok2 := Token{Kind: TokenKind(999), Value: "", Line: 1, Column: 0}
	s2 := tok2.String()
	if s2 == "" {
		t.Error("expected non-empty string for unknown token kind")
	}
}

func TestNewLexer_Reader(t *testing.T) {
	r := strings.NewReader("key: value")
	l := NewLexer(r)
	if l == nil {
		t.Fatal("NewLexer returned nil")
	}
	tok, err := l.NextToken()
	if err != nil {
		t.Fatalf("NextToken error: %v", err)
	}
	if tok.Kind != TokenKey || tok.Value != "key" {
		t.Errorf("expected TokenKey 'key', got %v %q", tok.Kind, tok.Value)
	}
}

func TestLexer_EOF(t *testing.T) {
	l := NewLexerString("a")
	// The lexer should reach eof after reading the input
	if l.eof() {
		t.Error("expected eof to be false at start of non-empty input")
	}
	// Read through all tokens until EOF
	for {
		tok, err := l.NextToken()
		if err != nil {
			t.Fatalf("NextToken error: %v", err)
		}
		if tok.Kind == TokenEOF {
			break
		}
	}
	if !l.eof() {
		t.Error("expected eof to be true after consuming all tokens")
	}
}

func TestLexer_SkipWhitespace(t *testing.T) {
	// Input with leading spaces before a value after colon
	l := NewLexerString("   hello")
	l.skipWhitespace()
	// After skipping whitespace, the next read should be 'h'
	r := l.read()
	if r != 'h' {
		t.Errorf("expected 'h' after skipWhitespace, got %q", r)
	}
}

func TestLexer_SkipWhitespace_Tabs(t *testing.T) {
	l := NewLexerString("\t\t value")
	l.skipWhitespace()
	r := l.read()
	if r != 'v' {
		t.Errorf("expected 'v' after skipWhitespace with tabs, got %q", r)
	}
}

func TestLexer_SkipWhitespace_NoWhitespace(t *testing.T) {
	l := NewLexerString("hello")
	l.skipWhitespace()
	r := l.read()
	if r != 'h' {
		t.Errorf("expected 'h' when no whitespace to skip, got %q", r)
	}
}

func TestLexer_SkipWhitespace_Empty(t *testing.T) {
	l := NewLexerString("")
	// Should not panic on empty input
	l.skipWhitespace()
	if !l.eof() {
		t.Error("expected eof on empty input after skipWhitespace")
	}
}

func TestLexer_EOF_EmptyInput(t *testing.T) {
	l := NewLexerString("")
	if !l.eof() {
		t.Error("expected eof to be true for empty input")
	}
}

func TestLexer_UnreadNewline(t *testing.T) {
	l := NewLexerString("a\nb")
	l.read() // 'a'
	l.read() // '\n'
	// After reading newline, line should be 2
	if l.line != 2 {
		t.Errorf("expected line 2 after newline, got %d", l.line)
	}
	l.unread() // unread the '\n'
	if l.line != 1 {
		t.Errorf("expected line 1 after unread of newline, got %d", l.line)
	}
}
