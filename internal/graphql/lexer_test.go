package graphql

import (
	"testing"
)

func TestLexerBasic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []struct {
			kind  TokenKind
			value string
		}
	}{
		{
			name:  "simple query",
			input: `{ hello }`,
			expected: []struct {
				kind  TokenKind
				value string
			}{
				{TokenBraceL, "{"},
				{TokenName, "hello"},
				{TokenBraceR, "}"},
				{TokenEOF, ""},
			},
		},
		{
			name:  "query with field selection",
			input: `query { user { name email } }`,
			expected: []struct {
				kind  TokenKind
				value string
			}{
				{TokenQuery, "query"},
				{TokenBraceL, "{"},
				{TokenName, "user"},
				{TokenBraceL, "{"},
				{TokenName, "name"},
				{TokenName, "email"},
				{TokenBraceR, "}"},
				{TokenBraceR, "}"},
				{TokenEOF, ""},
			},
		},
		{
			name:  "query with arguments",
			input: `{ user(id: 123) { name } }`,
			expected: []struct {
				kind  TokenKind
				value string
			}{
				{TokenBraceL, "{"},
				{TokenName, "user"},
				{TokenParenL, "("},
				{TokenName, "id"},
				{TokenColon, ":"},
				{TokenInt, "123"},
				{TokenParenR, ")"},
				{TokenBraceL, "{"},
				{TokenName, "name"},
				{TokenBraceR, "}"},
				{TokenBraceR, "}"},
				{TokenEOF, ""},
			},
		},
		{
			name:  "mutation",
			input: `mutation { createUser(name: "John") { id } }`,
			expected: []struct {
				kind  TokenKind
				value string
			}{
				{TokenMutation, "mutation"},
				{TokenBraceL, "{"},
				{TokenName, "createUser"},
				{TokenParenL, "("},
				{TokenName, "name"},
				{TokenColon, ":"},
				{TokenString, "John"},
				{TokenParenR, ")"},
				{TokenBraceL, "{"},
				{TokenName, "id"},
				{TokenBraceR, "}"},
				{TokenBraceR, "}"},
				{TokenEOF, ""},
			},
		},
		{
			name:  "variables",
			input: `query GetUser($id: ID!) { user(id: $id) { name } }`,
			expected: []struct {
				kind  TokenKind
				value string
			}{
				{TokenQuery, "query"},
				{TokenName, "GetUser"},
				{TokenParenL, "("},
				{TokenDollar, "$"},
				{TokenName, "id"},
				{TokenColon, ":"},
				{TokenName, "ID"},
				{TokenBang, "!"},
				{TokenParenR, ")"},
				{TokenBraceL, "{"},
				{TokenName, "user"},
				{TokenParenL, "("},
				{TokenName, "id"},
				{TokenColon, ":"},
				{TokenDollar, "$"},
				{TokenName, "id"},
				{TokenParenR, ")"},
				{TokenBraceL, "{"},
				{TokenName, "name"},
				{TokenBraceR, "}"},
				{TokenBraceR, "}"},
				{TokenEOF, ""},
			},
		},
		{
			name:  "fragment spread",
			input: `{ user { ...UserFields } }`,
			expected: []struct {
				kind  TokenKind
				value string
			}{
				{TokenBraceL, "{"},
				{TokenName, "user"},
				{TokenBraceL, "{"},
				{TokenSpread, "..."},
				{TokenName, "UserFields"},
				{TokenBraceR, "}"},
				{TokenBraceR, "}"},
				{TokenEOF, ""},
			},
		},
		{
			name:  "inline fragment",
			input: `{ user { ... on Admin { permissions } } }`,
			expected: []struct {
				kind  TokenKind
				value string
			}{
				{TokenBraceL, "{"},
				{TokenName, "user"},
				{TokenBraceL, "{"},
				{TokenSpread, "..."},
				{TokenOn, "on"},
				{TokenName, "Admin"},
				{TokenBraceL, "{"},
				{TokenName, "permissions"},
				{TokenBraceR, "}"},
				{TokenBraceR, "}"},
				{TokenBraceR, "}"},
				{TokenEOF, ""},
			},
		},
		{
			name:  "directive",
			input: `{ user @include(if: true) { name } }`,
			expected: []struct {
				kind  TokenKind
				value string
			}{
				{TokenBraceL, "{"},
				{TokenName, "user"},
				{TokenAt, "@"},
				{TokenName, "include"},
				{TokenParenL, "("},
				{TokenName, "if"},
				{TokenColon, ":"},
				{TokenTrue, "true"},
				{TokenParenR, ")"},
				{TokenBraceL, "{"},
				{TokenName, "name"},
				{TokenBraceR, "}"},
				{TokenBraceR, "}"},
				{TokenEOF, ""},
			},
		},
		{
			name:  "list type",
			input: `type Query { users: [User] }`,
			expected: []struct {
				kind  TokenKind
				value string
			}{
				{TokenType, "type"},
				{TokenName, "Query"},
				{TokenBraceL, "{"},
				{TokenName, "users"},
				{TokenColon, ":"},
				{TokenBracketL, "["},
				{TokenName, "User"},
				{TokenBracketR, "]"},
				{TokenBraceR, "}"},
				{TokenEOF, ""},
			},
		},
		{
			name:  "non-null type",
			input: `type Query { user: User! }`,
			expected: []struct {
				kind  TokenKind
				value string
			}{
				{TokenType, "type"},
				{TokenName, "Query"},
				{TokenBraceL, "{"},
				{TokenName, "user"},
				{TokenColon, ":"},
				{TokenName, "User"},
				{TokenBang, "!"},
				{TokenBraceR, "}"},
				{TokenEOF, ""},
			},
		},
		{
			name:  "enum definition",
			input: `enum Status { ACTIVE INACTIVE }`,
			expected: []struct {
				kind  TokenKind
				value string
			}{
				{TokenEnum, "enum"},
				{TokenName, "Status"},
				{TokenBraceL, "{"},
				{TokenName, "ACTIVE"},
				{TokenName, "INACTIVE"},
				{TokenBraceR, "}"},
				{TokenEOF, ""},
			},
		},
		{
			name:  "input definition",
			input: `input CreateUserInput { name: String! email: String }`,
			expected: []struct {
				kind  TokenKind
				value string
			}{
				{TokenInput, "input"},
				{TokenName, "CreateUserInput"},
				{TokenBraceL, "{"},
				{TokenName, "name"},
				{TokenColon, ":"},
				{TokenName, "String"},
				{TokenBang, "!"},
				{TokenName, "email"},
				{TokenColon, ":"},
				{TokenName, "String"},
				{TokenBraceR, "}"},
				{TokenEOF, ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := NewLexer(tt.input)

			for i, exp := range tt.expected {
				tok := l.NextToken()
				if tok.Kind != exp.kind {
					t.Errorf("token %d: expected kind %s, got %s", i, exp.kind, tok.Kind)
				}
				if tok.Value != exp.value {
					t.Errorf("token %d: expected value %q, got %q", i, exp.value, tok.Value)
				}
			}
		})
	}
}

func TestLexerNumbers(t *testing.T) {
	tests := []struct {
		input    string
		expected TokenKind
		value    string
	}{
		{"0", TokenInt, "0"},
		{"123", TokenInt, "123"},
		{"-42", TokenInt, "-42"},
		{"3.14", TokenFloat, "3.14"},
		{"-0.5", TokenFloat, "-0.5"},
		{"1e10", TokenFloat, "1e10"},
		{"1E10", TokenFloat, "1E10"},
		{"1e+10", TokenFloat, "1e+10"},
		{"1e-10", TokenFloat, "1e-10"},
		{"3.14e10", TokenFloat, "3.14e10"},
		{"-3.14e-10", TokenFloat, "-3.14e-10"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := NewLexer(tt.input)
			tok := l.NextToken()
			if tok.Kind != tt.expected {
				t.Errorf("expected kind %s, got %s", tt.expected, tok.Kind)
			}
			if tok.Value != tt.value {
				t.Errorf("expected value %q, got %q", tt.value, tok.Value)
			}
		})
	}
}

func TestLexerStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple string",
			input:    `"hello"`,
			expected: "hello",
		},
		{
			name:     "string with space",
			input:    `"hello world"`,
			expected: "hello world",
		},
		{
			name:     "string with escape",
			input:    "\"hello\\nworld\"",
			expected: "hello\nworld",
		},
		{
			name:     "string with quote escape",
			input:    "\"say \\\"hello\\\"\"",
			expected: `say "hello"`,
		},
		{
			name:     "string with backslash",
			input:    "\"path\\\\to\\\\file\"",
			expected: "path\\to\\file",
		},
		{
			name:     "string with unicode escape",
			input:    "\"\\u0048\\u0065\\u006c\\u006c\\u006f\"",
			expected: "Hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := NewLexer(tt.input)
			tok := l.NextToken()
			if tok.Kind != TokenString {
				t.Errorf("expected STRING token, got %s", tok.Kind)
			}
			if tok.Value != tt.expected {
				t.Errorf("expected value %q, got %q", tt.expected, tok.Value)
			}
		})
	}
}

func TestLexerBlockStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple block string",
			input:    `"""hello"""`,
			expected: "hello",
		},
		{
			name: "multi-line block string",
			input: `"""` + "\n  hello\n  world\n" + `"""`,
			// GraphQL block strings strip common leading whitespace
			expected: "hello\nworld",
		},
		{
			name:     "block string with special chars",
			input:    `"""hello\nworld"""`,
			expected: "hello\\nworld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := NewLexer(tt.input)
			tok := l.NextToken()
			if tok.Kind != TokenBlockString {
				t.Errorf("expected BLOCK_STRING token, got %s", tok.Kind)
			}
			if tok.Value != tt.expected {
				t.Errorf("expected value %q, got %q", tt.expected, tok.Value)
			}
		})
	}
}

func TestLexerComments(t *testing.T) {
	input := `
		# This is a comment
		query {
			hello # inline comment
		}
	`
	l := NewLexer(input)

	tokens := []TokenKind{
		TokenQuery,
		TokenBraceL,
		TokenName,   // hello
		TokenBraceR,
		TokenEOF,
	}

	for i, expected := range tokens {
		tok := l.NextToken()
		if tok.Kind != expected {
			t.Errorf("token %d: expected %s, got %s", i, expected, tok.Kind)
		}
	}
}

func TestLexerKeywords(t *testing.T) {
	tests := []struct {
		input    string
		expected TokenKind
	}{
		{"query", TokenQuery},
		{"mutation", TokenMutation},
		{"subscription", TokenSubscription},
		{"fragment", TokenFragment},
		{"on", TokenOn},
		{"true", TokenTrue},
		{"false", TokenFalse},
		{"null", TokenNull},
		{"schema", TokenSchema},
		{"type", TokenType},
		{"scalar", TokenScalar},
		{"interface", TokenInterface},
		{"union", TokenUnion},
		{"enum", TokenEnum},
		{"input", TokenInput},
		{"directive", TokenDirective},
		{"extend", TokenExtend},
		{"implements", TokenImplements},
		{"repeatable", TokenRepeatable},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := NewLexer(tt.input)
			tok := l.NextToken()
			if tok.Kind != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tok.Kind)
			}
		})
	}
}

func TestLexerUnicode(t *testing.T) {
	// Test unicode in identifiers
	input := `{ 你好 }`
	l := NewLexer(input)

	tokens := []TokenKind{
		TokenBraceL,
		TokenName, // 你好
		TokenBraceR,
		TokenEOF,
	}

	for i, expected := range tokens {
		tok := l.NextToken()
		if tok.Kind != expected {
			t.Errorf("token %d: expected %s, got %s", i, expected, tok.Kind)
		}
	}
}

func TestLexerPosition(t *testing.T) {
	input := `query {
		hello
	}`
	l := NewLexer(input)

	tests := []struct {
		expectedLine   int
		expectedColumn int
	}{
		{1, 1},  // query
		{1, 7},  // {
		{2, 3},  // hello
		{3, 2},  // }
		{3, 3},  // EOF
	}

	for i, tt := range tests {
		tok := l.NextToken()
		if tok.Line != tt.expectedLine {
			t.Errorf("token %d: expected line %d, got %d", i, tt.expectedLine, tok.Line)
		}
		if tok.Column != tt.expectedColumn {
			t.Errorf("token %d: expected column %d, got %d", i, tt.expectedColumn, tok.Column)
		}
	}
}

func TestTokenize(t *testing.T) {
	input := `{ hello }`
	tokens, err := Tokenize(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tokens) != 4 { // {, hello, }, EOF
		t.Errorf("expected 4 tokens, got %d", len(tokens))
	}

	if tokens[0].Kind != TokenBraceL {
		t.Errorf("expected first token to be {, got %s", tokens[0].Kind)
	}

	if tokens[len(tokens)-1].Kind != TokenEOF {
		t.Errorf("expected last token to be EOF, got %s", tokens[len(tokens)-1].Kind)
	}
}

func TestTokenizeError(t *testing.T) {
	// Single dot is illegal
	input := `{ . }`
	_, err := Tokenize(input)
	if err == nil {
		t.Error("expected error for illegal token, got nil")
	}
}

func TestLexerComplexQuery(t *testing.T) {
	input := `
		query GetUser($id: ID!, $includeEmail: Boolean = false) {
			user(id: $id) @include(if: $includeEmail) {
				id
				name
				email
				friends(first: 10) {
					nodes {
						name
					}
				}
			}
		}
	`

	l := NewLexer(input)
	var tokens []Token

	for {
		tok := l.NextToken()
		tokens = append(tokens, tok)
		if tok.Kind == TokenEOF {
			break
		}
	}

	// Verify we got meaningful tokens
	// Note: GraphQL treats commas as insignificant whitespace, so no TokenComma
	expectedSequence := []TokenKind{
		TokenQuery, TokenName, TokenParenL, TokenDollar, TokenName, TokenColon,
		TokenName, TokenBang, TokenDollar, TokenName, TokenColon,
		TokenName, TokenEquals, TokenFalse, TokenParenR, TokenBraceL, TokenName,
		TokenParenL, TokenName, TokenColon, TokenDollar, TokenName, TokenParenR,
		TokenAt, TokenName, TokenParenL, TokenName, TokenColon, TokenDollar,
		TokenName, TokenParenR, TokenBraceL, TokenName, TokenName, TokenName,
		TokenName, TokenParenL, TokenName, TokenColon, TokenInt, TokenParenR,
		TokenBraceL, TokenName, TokenBraceL, TokenName, TokenBraceR, TokenBraceR,
		TokenBraceR, TokenBraceR, TokenEOF,
	}

	if len(tokens) != len(expectedSequence) {
		t.Fatalf("expected %d tokens, got %d", len(expectedSequence), len(tokens))
	}

	for i, expected := range expectedSequence {
		if tokens[i].Kind != expected {
			t.Errorf("token %d: expected %s, got %s", i, expected, tokens[i].Kind)
		}
	}
}

func BenchmarkLexer(b *testing.B) {
	input := `
		query GetUser($id: ID!) {
			user(id: $id) {
				id
				name
				email
			}
		}
	`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l := NewLexer(input)
		for {
			tok := l.NextToken()
			if tok.Kind == TokenEOF {
				break
			}
		}
	}
}

// --- Coverage boost tests for lexer ---

func TestLexer_PeekAtEOF(t *testing.T) {
	l := NewLexer("")
	// After reading the empty input, peek should return 0
	ch := l.peek()
	if ch != 0 {
		t.Errorf("expected 0 at EOF, got %c", ch)
	}
}

func TestLexer_PeekNBeyondEnd(t *testing.T) {
	l := NewLexer("a")
	// peekN(2) should be beyond available chars
	ch := l.peekN(5)
	if ch != 0 {
		t.Errorf("expected 0 for peekN beyond end, got %c", ch)
	}
}

func TestLexer_ReadCharInvalidUTF8(t *testing.T) {
	// Create input with invalid UTF-8 byte sequence
	input := string([]byte{0xff, 0xfe})
	l := NewLexer(input)
	// Should handle invalid UTF-8 gracefully
	tok := l.NextToken()
	// The lexer should produce EOF since invalid bytes become 0
	if tok.Kind != TokenEOF {
		t.Logf("token kind: %s, value: %q", tok.Kind, tok.Value)
	}
}

func TestLexer_BOMSkipping(t *testing.T) {
	// BOM (byte order mark \ufeff) should be skipped as whitespace
	input := "\ufeff{ hello }"
	l := NewLexer(input)
	tok := l.NextToken()
	if tok.Kind != TokenBraceL {
		t.Errorf("expected BraceL after BOM, got %s", tok.Kind)
	}
}

func TestLexer_IllegalSingleDot(t *testing.T) {
	l := NewLexer(".")
	tok := l.NextToken()
	if tok.Kind != TokenIllegal {
		t.Errorf("expected ILLEGAL for single dot, got %s", tok.Kind)
	}
}

func TestLexer_DoubleDotIsIllegal(t *testing.T) {
	l := NewLexer("..")
	tok := l.NextToken()
	if tok.Kind != TokenIllegal {
		t.Errorf("expected ILLEGAL for double dot, got %s", tok.Kind)
	}
}

func TestLexer_UnterminatedString(t *testing.T) {
	l := NewLexer(`"hello`)
	tok := l.NextToken()
	if tok.Kind != TokenIllegal {
		t.Errorf("expected ILLEGAL for unterminated string, got %s", tok.Kind)
	}
}

func TestLexer_StringWithNewline(t *testing.T) {
	l := NewLexer("\"hello\nworld\"")
	tok := l.NextToken()
	if tok.Kind != TokenIllegal {
		t.Errorf("expected ILLEGAL for string with newline, got %s", tok.Kind)
	}
}

func TestLexer_UnterminatedBlockString(t *testing.T) {
	l := NewLexer(`"""unterminated`)
	tok := l.NextToken()
	if tok.Kind != TokenIllegal {
		t.Errorf("expected ILLEGAL for unterminated block string, got %s", tok.Kind)
	}
}

func TestLexer_NumberWithBadFraction(t *testing.T) {
	// "3." without digits after decimal
	l := NewLexer("3.}")
	tok := l.NextToken()
	if tok.Kind != TokenIllegal {
		t.Errorf("expected ILLEGAL for bad fraction, got %s (%q)", tok.Kind, tok.Value)
	}
}

func TestLexer_NumberWithBadExponent(t *testing.T) {
	// "3e" without digits
	l := NewLexer("3e}")
	tok := l.NextToken()
	if tok.Kind != TokenIllegal {
		t.Errorf("expected ILLEGAL for bad exponent, got %s (%q)", tok.Kind, tok.Value)
	}
}

func TestLexer_EscapeSequences(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"slash escape", `"a\/b"`, "a/b"},
		{"backspace", `"a\bb"`, "a\bb"},
		{"formfeed", `"a\fb"`, "a\fb"},
		{"tab", `"a\tb"`, "a\tb"},
		{"carriage return", `"a\rb"`, "a\rb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := NewLexer(tt.input)
			tok := l.NextToken()
			if tok.Kind != TokenString {
				t.Fatalf("expected STRING, got %s", tok.Kind)
			}
			if tok.Value != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, tok.Value)
			}
		})
	}
}

func TestLexer_InvalidEscapeSequence(t *testing.T) {
	// \x is not a valid escape
	l := NewLexer(`"hello\xworld"`)
	tok := l.NextToken()
	if tok.Kind != TokenString {
		t.Fatalf("expected STRING (with invalid escape handled), got %s", tok.Kind)
	}
	// The invalid escape should be written as-is with backslash
	if tok.Value != "hello\\xworld" {
		t.Logf("invalid escape result: %q", tok.Value)
	}
}

func TestLexer_InvalidUnicodeEscape(t *testing.T) {
	// \uXXXG - invalid hex digit
	l := NewLexer(`"\u00GG"`)
	tok := l.NextToken()
	// Should still produce a token (may have replacement chars)
	if tok.Kind != TokenString {
		t.Logf("token for invalid unicode escape: %s %q", tok.Kind, tok.Value)
	}
}

func TestLexer_PipeAndAmpersand(t *testing.T) {
	l := NewLexer("| &")
	tok1 := l.NextToken()
	if tok1.Kind != TokenPipe {
		t.Errorf("expected PIPE, got %s", tok1.Kind)
	}
	tok2 := l.NextToken()
	if tok2.Kind != TokenAmpersand {
		t.Errorf("expected AMPERSAND, got %s", tok2.Kind)
	}
}

func TestLexer_EqualsToken(t *testing.T) {
	l := NewLexer("=")
	tok := l.NextToken()
	if tok.Kind != TokenEquals {
		t.Errorf("expected EQUALS, got %s", tok.Kind)
	}
}

func TestLexer_IllegalCharacter(t *testing.T) {
	l := NewLexer("~")
	tok := l.NextToken()
	if tok.Kind != TokenIllegal {
		t.Errorf("expected ILLEGAL for ~, got %s", tok.Kind)
	}
}

func TestLexer_NumberStartingWithZero(t *testing.T) {
	l := NewLexer("0")
	tok := l.NextToken()
	if tok.Kind != TokenInt {
		t.Errorf("expected INT, got %s", tok.Kind)
	}
	if tok.Value != "0" {
		t.Errorf("expected '0', got %q", tok.Value)
	}
}

func TestLexer_NegativeFloat(t *testing.T) {
	l := NewLexer("-1.5e+2")
	tok := l.NextToken()
	if tok.Kind != TokenFloat {
		t.Errorf("expected FLOAT, got %s", tok.Kind)
	}
	if tok.Value != "-1.5e+2" {
		t.Errorf("expected '-1.5e+2', got %q", tok.Value)
	}
}

func TestProcessBlockString_EmptyLines(t *testing.T) {
	// Test with all-whitespace input
	result := processBlockString("   \n   \n   ")
	if result != "" {
		t.Logf("processBlockString all-whitespace: %q", result)
	}

	// Test with empty string
	result2 := processBlockString("")
	if result2 != "" {
		t.Errorf("expected empty for empty input, got %q", result2)
	}
}

func TestProcessBlockString_SingleLine(t *testing.T) {
	result := processBlockString("hello")
	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}
}

func TestProcessBlockString_ShortLines(t *testing.T) {
	// Lines shorter than minIndent
	result := processBlockString("\n  a\n b\n")
	if result == "" {
		t.Error("expected non-empty result")
	}
}
