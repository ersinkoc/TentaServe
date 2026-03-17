package yaml

import (
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
