package graphql

import "fmt"

// TokenKind represents the type of a GraphQL token.
type TokenKind int

const (
	// Special tokens
	TokenEOF TokenKind = iota
	TokenIllegal

	// Identifiers and literals
	TokenName      // fieldName, Type, etc.
	TokenInt       // 123
	TokenFloat     // 3.14
	TokenString    // "hello"
	TokenBlockString // """multi-line"""

	// Keywords
	TokenQuery       // query
	TokenMutation    // mutation
	TokenSubscription // subscription
	TokenFragment    // fragment
	TokenOn          // on
	TokenTrue        // true
	TokenFalse       // false
	TokenNull        // null
	TokenSchema      // schema
	TokenType        // type
	TokenScalar      // scalar
	TokenInterface   // interface
	TokenUnion       // union
	TokenEnum        // enum
	TokenInput       // input
	TokenDirective   // directive
	TokenExtend      // extend
	TokenImplements  // implements
	TokenRepeatable  // repeatable

	// Punctuation
	TokenParenL      // (
	TokenParenR      // )
	TokenBraceL      // {
	TokenBraceR      // }
	TokenBracketL    // [
	TokenBracketR    // ]
	TokenColon       // :
	TokenComma       // ,
	TokenAt          // @
	TokenDollar      // $
	TokenEquals      // =
	TokenBang        // !
	TokenSpread      // ...
	TokenPipe        // |
	TokenAmpersand   // &
)

// Token represents a lexical token in GraphQL.
type Token struct {
	Kind   TokenKind
	Value  string
	Line   int
	Column int
}

// String returns a human-readable representation of the token.
func (t Token) String() string {
	return fmt.Sprintf("%s[%q]@%d:%d", t.Kind.String(), t.Value, t.Line, t.Column)
}

// IsKeyword returns true if the token is a keyword.
func (t Token) IsKeyword() bool {
	switch t.Kind {
	case TokenQuery, TokenMutation, TokenSubscription, TokenFragment,
		TokenOn, TokenTrue, TokenFalse, TokenNull, TokenSchema,
		TokenType, TokenScalar, TokenInterface, TokenUnion, TokenEnum,
		TokenInput, TokenDirective, TokenExtend, TokenImplements, TokenRepeatable:
		return true
	}
	return false
}

// String returns the string representation of a token kind.
func (k TokenKind) String() string {
	switch k {
	case TokenEOF:
		return "EOF"
	case TokenIllegal:
		return "ILLEGAL"
	case TokenName:
		return "NAME"
	case TokenInt:
		return "INT"
	case TokenFloat:
		return "FLOAT"
	case TokenString:
		return "STRING"
	case TokenBlockString:
		return "BLOCK_STRING"
	case TokenQuery:
		return "QUERY"
	case TokenMutation:
		return "MUTATION"
	case TokenSubscription:
		return "SUBSCRIPTION"
	case TokenFragment:
		return "FRAGMENT"
	case TokenOn:
		return "ON"
	case TokenTrue:
		return "TRUE"
	case TokenFalse:
		return "FALSE"
	case TokenNull:
		return "NULL"
	case TokenSchema:
		return "SCHEMA"
	case TokenType:
		return "TYPE"
	case TokenScalar:
		return "SCALAR"
	case TokenInterface:
		return "INTERFACE"
	case TokenUnion:
		return "UNION"
	case TokenEnum:
		return "ENUM"
	case TokenInput:
		return "INPUT"
	case TokenDirective:
		return "DIRECTIVE"
	case TokenExtend:
		return "EXTEND"
	case TokenImplements:
		return "IMPLEMENTS"
	case TokenRepeatable:
		return "REPEATABLE"
	case TokenParenL:
		return "("
	case TokenParenR:
		return ")"
	case TokenBraceL:
		return "{"
	case TokenBraceR:
		return "}"
	case TokenBracketL:
		return "["
	case TokenBracketR:
		return "]"
	case TokenColon:
		return ":"
	case TokenComma:
		return ","
	case TokenAt:
		return "@"
	case TokenDollar:
		return "$"
	case TokenEquals:
		return "="
	case TokenBang:
		return "!"
	case TokenSpread:
		return "..."
	case TokenPipe:
		return "|"
	case TokenAmpersand:
		return "&"
	default:
		return fmt.Sprintf("Token(%d)", k)
	}
}

// keywords maps keyword strings to their token kinds.
var keywords = map[string]TokenKind{
	"query":        TokenQuery,
	"mutation":     TokenMutation,
	"subscription": TokenSubscription,
	"fragment":     TokenFragment,
	"on":           TokenOn,
	"true":         TokenTrue,
	"false":        TokenFalse,
	"null":         TokenNull,
	"schema":       TokenSchema,
	"type":         TokenType,
	"scalar":       TokenScalar,
	"interface":    TokenInterface,
	"union":        TokenUnion,
	"enum":         TokenEnum,
	"input":        TokenInput,
	"directive":    TokenDirective,
	"extend":       TokenExtend,
	"implements":   TokenImplements,
	"repeatable":   TokenRepeatable,
}

// LookupKeyword checks if an identifier is a keyword.
// If it is, it returns the keyword's token kind.
// Otherwise, it returns TokenName.
func LookupKeyword(ident string) TokenKind {
	if kind, ok := keywords[ident]; ok {
		return kind
	}
	return TokenName
}
