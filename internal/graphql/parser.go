package graphql

import (
	"fmt"
)

// Parser parses GraphQL tokens into an AST.
type Parser struct {
	l        *Lexer
	curTok   Token
	savedTok Token // for peek without consuming
	hasSaved bool
	errors   []error
}

// NewParser creates a new parser from a lexer.
func NewParser(l *Lexer) *Parser {
	p := &Parser{l: l}
	p.nextToken() // read first token
	return p
}

// NewParserString creates a new parser from a string.
func NewParserString(input string) *Parser {
	return NewParser(NewLexer(input))
}

// nextToken advances to the next token.
func (p *Parser) nextToken() {
	if p.hasSaved {
		p.curTok = p.savedTok
		p.hasSaved = false
	} else {
		p.curTok = p.l.NextToken()
	}
}

// peek returns the next token without consuming it.
func (p *Parser) peek() Token {
	if !p.hasSaved {
		p.savedTok = p.l.NextToken()
		p.hasSaved = true
	}
	return p.savedTok
}

// expect advances if the current token matches the expected kind.
func (p *Parser) expect(kind TokenKind) error {
	if p.curTok.Kind != kind {
		return fmt.Errorf("expected %s, got %s at line %d, column %d",
			kind, p.curTok.Kind, p.curTok.Line, p.curTok.Column)
	}
	p.nextToken()
	return nil
}

// addError adds a parse error.
func (p *Parser) addError(format string, args ...interface{}) {
	p.errors = append(p.errors, fmt.Errorf(format+" at line %d, column %d", append(args, p.curTok.Line, p.curTok.Column)...))
}

// Errors returns all parse errors.
func (p *Parser) Errors() []error {
	return p.errors
}

// HasErrors returns true if there were parse errors.
func (p *Parser) HasErrors() bool {
	return len(p.errors) > 0
}

// Parse parses a GraphQL document.
func (p *Parser) Parse() (*Document, error) {
	doc := &Document{
		Definitions: []Definition{},
	}

	for p.curTok.Kind != TokenEOF {
		def, err := p.parseDefinition()
		if err != nil {
			p.addError("%v", err)
			// Skip to next potential definition
			p.skipToDefinition()
			continue
		}
		if def != nil {
			doc.Definitions = append(doc.Definitions, def)
		}
	}

	if p.HasErrors() {
		return doc, p.errors[0]
	}

	return doc, nil
}

// Parse parses a GraphQL query string into an AST.
func Parse(query string) (*Document, error) {
	p := NewParserString(query)
	return p.Parse()
}

// skipToDefinition skips tokens until we reach a potential definition start.
func (p *Parser) skipToDefinition() {
	for p.curTok.Kind != TokenEOF {
		switch p.curTok.Kind {
		case TokenQuery, TokenMutation, TokenSubscription, TokenFragment, TokenBraceL:
			return
		}
		p.nextToken()
	}
}

// parseDefinition parses a top-level definition.
func (p *Parser) parseDefinition() (Definition, error) {
	switch p.curTok.Kind {
	case TokenQuery, TokenMutation, TokenSubscription:
		return p.parseOperationDefinition()
	case TokenFragment:
		return p.parseFragmentDefinition()
	case TokenBraceL:
		// Anonymous operation
		return p.parseAnonymousOperation()
	default:
		return nil, fmt.Errorf("unexpected token %s", p.curTok.Kind)
	}
}

// parseOperationDefinition parses a query, mutation, or subscription.
func (p *Parser) parseOperationDefinition() (*OperationDefinition, error) {
	op := &OperationDefinition{
		Operation: p.curTok.Kind,
	}

	p.nextToken() // consume operation type

	// Optional name
	if p.curTok.Kind == TokenName {
		op.Name = &Name{Value: p.curTok.Value}
		p.nextToken()
	}

	// Optional variable definitions
	if p.curTok.Kind == TokenParenL {
		vars, err := p.parseVariableDefinitions()
		if err != nil {
			return nil, err
		}
		op.VariableDefinitions = vars
	}

	// Optional directives
	directives, err := p.parseDirectives()
	if err != nil {
		return nil, err
	}
	op.Directives = directives

	// Selection set (required)
	if p.curTok.Kind != TokenBraceL {
		return nil, fmt.Errorf("expected {, got %s", p.curTok.Kind)
	}
	ss, err := p.parseSelectionSet()
	if err != nil {
		return nil, err
	}
	op.SelectionSet = ss

	return op, nil
}

// parseAnonymousOperation parses an anonymous operation (just { ... }).
func (p *Parser) parseAnonymousOperation() (*OperationDefinition, error) {
	op := &OperationDefinition{
		Operation: TokenQuery, // Anonymous operations default to query
	}

	ss, err := p.parseSelectionSet()
	if err != nil {
		return nil, err
	}
	op.SelectionSet = ss

	return op, nil
}

// parseFragmentDefinition parses a fragment definition.
func (p *Parser) parseFragmentDefinition() (*FragmentDefinition, error) {
	frag := &FragmentDefinition{}

	p.nextToken() // consume 'fragment'

	// Fragment name
	if p.curTok.Kind != TokenName {
		return nil, fmt.Errorf("expected fragment name, got %s", p.curTok.Kind)
	}
	frag.Name = &Name{Value: p.curTok.Value}
	p.nextToken()

	// 'on'
	if p.curTok.Kind != TokenOn {
		return nil, fmt.Errorf("expected 'on', got %s", p.curTok.Kind)
	}
	p.nextToken()

	// Type condition
	if p.curTok.Kind != TokenName {
		return nil, fmt.Errorf("expected type name, got %s", p.curTok.Kind)
	}
	frag.TypeCondition = &NamedType{Name: &Name{Value: p.curTok.Value}}
	p.nextToken()

	// Optional directives
	directives, err := p.parseDirectives()
	if err != nil {
		return nil, err
	}
	frag.Directives = directives

	// Selection set (required)
	if p.curTok.Kind != TokenBraceL {
		return nil, fmt.Errorf("expected {, got %s", p.curTok.Kind)
	}
	ss, err := p.parseSelectionSet()
	if err != nil {
		return nil, err
	}
	frag.SelectionSet = ss

	return frag, nil
}

// parseVariableDefinitions parses variable definitions ($var: Type = default).
func (p *Parser) parseVariableDefinitions() ([]*VariableDefinition, error) {
	var vars []*VariableDefinition

	if err := p.expect(TokenParenL); err != nil {
		return nil, err
	}

	for p.curTok.Kind != TokenParenR {
		if p.curTok.Kind == TokenEOF {
			return nil, fmt.Errorf("unexpected end of input in variable definitions")
		}

		v, err := p.parseVariableDefinition()
		if err != nil {
			return nil, err
		}
		vars = append(vars, v)
	}

	if err := p.expect(TokenParenR); err != nil {
		return nil, err
	}

	return vars, nil
}

// parseVariableDefinition parses a single variable definition.
func (p *Parser) parseVariableDefinition() (*VariableDefinition, error) {
	v := &VariableDefinition{}

	// $var
	if p.curTok.Kind != TokenDollar {
		return nil, fmt.Errorf("expected $, got %s", p.curTok.Kind)
	}
	p.nextToken()

	if p.curTok.Kind != TokenName {
		return nil, fmt.Errorf("expected variable name, got %s", p.curTok.Kind)
	}
	v.Variable = &Variable{Name: &Name{Value: p.curTok.Value}}
	p.nextToken()

	// : Type
	if err := p.expect(TokenColon); err != nil {
		return nil, err
	}

	typ, err := p.parseType()
	if err != nil {
		return nil, err
	}
	v.Type = typ

	// Optional default value
	if p.curTok.Kind == TokenEquals {
		p.nextToken()
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		v.DefaultValue = val
	}

	return v, nil
}

// parseSelectionSet parses a selection set { ... }.
func (p *Parser) parseSelectionSet() (*SelectionSet, error) {
	ss := &SelectionSet{
		Selections: []Selection{},
	}

	if err := p.expect(TokenBraceL); err != nil {
		return nil, err
	}

	for p.curTok.Kind != TokenBraceR {
		if p.curTok.Kind == TokenEOF {
			return nil, fmt.Errorf("unexpected end of input in selection set")
		}

		sel, err := p.parseSelection()
		if err != nil {
			return nil, err
		}
		ss.Selections = append(ss.Selections, sel)
	}

	if err := p.expect(TokenBraceR); err != nil {
		return nil, err
	}

	return ss, nil
}

// parseSelection parses a single selection (field, fragment spread, or inline fragment).
func (p *Parser) parseSelection() (Selection, error) {
	switch p.curTok.Kind {
	case TokenSpread:
		return p.parseFragment()
	case TokenName:
		return p.parseField()
	default:
		return nil, fmt.Errorf("expected field or fragment, got %s", p.curTok.Kind)
	}
}

// parseField parses a field selection.
func (p *Parser) parseField() (*Field, error) {
	f := &Field{}

	// Field name (could be an alias)
	if p.curTok.Kind != TokenName {
		return nil, fmt.Errorf("expected field name, got %s", p.curTok.Kind)
	}
	name := &Name{Value: p.curTok.Value}
	p.nextToken()

	// Check for alias (name: fieldName)
	if p.curTok.Kind == TokenColon {
		f.Alias = name
		p.nextToken() // consume :
		if p.curTok.Kind != TokenName {
			return nil, fmt.Errorf("expected field name after alias, got %s", p.curTok.Kind)
		}
		f.Name = &Name{Value: p.curTok.Value}
		p.nextToken()
	} else {
		f.Name = name
	}

	// Optional arguments
	if p.curTok.Kind == TokenParenL {
		args, err := p.parseArguments()
		if err != nil {
			return nil, err
		}
		f.Arguments = args
	}

	// Optional directives
	directives, err := p.parseDirectives()
	if err != nil {
		return nil, err
	}
	f.Directives = directives

	// Optional selection set
	if p.curTok.Kind == TokenBraceL {
		ss, err := p.parseSelectionSet()
		if err != nil {
			return nil, err
		}
		f.SelectionSet = ss
	}

	return f, nil
}

// parseFragment parses a fragment spread or inline fragment.
func (p *Parser) parseFragment() (Selection, error) {
	if p.curTok.Kind != TokenSpread {
		return nil, fmt.Errorf("expected ..., got %s", p.curTok.Kind)
	}
	p.nextToken() // consume ...

	// Check for inline fragment (... on TypeCondition)
	if p.curTok.Kind == TokenOn {
		return p.parseInlineFragment()
	}

	// Fragment spread (...FragmentName)
	if p.curTok.Kind != TokenName {
		return nil, fmt.Errorf("expected fragment name or 'on', got %s", p.curTok.Kind)
	}

	fs := &FragmentSpread{
		Name: &Name{Value: p.curTok.Value},
	}
	p.nextToken()

	// Optional directives
	directives, err := p.parseDirectives()
	if err != nil {
		return nil, err
	}
	fs.Directives = directives

	return fs, nil
}

// parseInlineFragment parses an inline fragment.
func (p *Parser) parseInlineFragment() (*InlineFragment, error) {
	frag := &InlineFragment{}

	p.nextToken() // consume 'on'

	// Type condition
	if p.curTok.Kind != TokenName {
		return nil, fmt.Errorf("expected type name, got %s", p.curTok.Kind)
	}
	frag.TypeCondition = &NamedType{Name: &Name{Value: p.curTok.Value}}
	p.nextToken()

	// Optional directives
	directives, err := p.parseDirectives()
	if err != nil {
		return nil, err
	}
	frag.Directives = directives

	// Selection set (required)
	if p.curTok.Kind != TokenBraceL {
		return nil, fmt.Errorf("expected {, got %s", p.curTok.Kind)
	}
	ss, err := p.parseSelectionSet()
	if err != nil {
		return nil, err
	}
	frag.SelectionSet = ss

	return frag, nil
}

// parseArguments parses field arguments (arg1: value1, arg2: value2).
func (p *Parser) parseArguments() ([]*Argument, error) {
	var args []*Argument

	if err := p.expect(TokenParenL); err != nil {
		return nil, err
	}

	for p.curTok.Kind != TokenParenR {
		if p.curTok.Kind == TokenEOF {
			return nil, fmt.Errorf("unexpected end of input in arguments")
		}

		arg, err := p.parseArgument()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
	}

	if err := p.expect(TokenParenR); err != nil {
		return nil, err
	}

	return args, nil
}

// parseArgument parses a single argument (name: value).
func (p *Parser) parseArgument() (*Argument, error) {
	arg := &Argument{}

	// Argument name can be TokenName or any keyword
	if p.curTok.Kind != TokenName && !p.curTok.IsKeyword() {
		return nil, fmt.Errorf("expected argument name, got %s", p.curTok.Kind)
	}
	arg.Name = &Name{Value: p.curTok.Value}
	p.nextToken()

	if err := p.expect(TokenColon); err != nil {
		return nil, err
	}

	val, err := p.parseValue()
	if err != nil {
		return nil, err
	}
	arg.Value = val

	return arg, nil
}

// parseDirectives parses directives (@dir1 @dir2(arg: value)).
func (p *Parser) parseDirectives() ([]*Directive, error) {
	var directives []*Directive

	for p.curTok.Kind == TokenAt {
		d, err := p.parseDirective()
		if err != nil {
			return nil, err
		}
		directives = append(directives, d)
	}

	return directives, nil
}

// parseDirective parses a single directive.
func (p *Parser) parseDirective() (*Directive, error) {
	d := &Directive{}

	p.nextToken() // consume @

	if p.curTok.Kind != TokenName {
		return nil, fmt.Errorf("expected directive name, got %s", p.curTok.Kind)
	}
	d.Name = &Name{Value: p.curTok.Value}
	p.nextToken()

	// Optional arguments
	if p.curTok.Kind == TokenParenL {
		args, err := p.parseArguments()
		if err != nil {
			return nil, err
		}
		d.Arguments = args
	}

	return d, nil
}

// parseType parses a GraphQL type.
func (p *Parser) parseType() (Type, error) {
	var typ Type

	switch p.curTok.Kind {
	case TokenName:
		typ = &NamedType{Name: &Name{Value: p.curTok.Value}}
		p.nextToken()
	case TokenBracketL:
		// List type [Type]
		p.nextToken() // consume [
		inner, err := p.parseType()
		if err != nil {
			return nil, err
		}
		if p.curTok.Kind != TokenBracketR {
			return nil, fmt.Errorf("expected ], got %s", p.curTok.Kind)
		}
		p.nextToken() // consume ]
		typ = &ListType{Type: inner}
	default:
		return nil, fmt.Errorf("expected type, got %s", p.curTok.Kind)
	}

	// Check for non-null
	if p.curTok.Kind == TokenBang {
		p.nextToken()
		typ = &NonNullType{Type: typ}
	}

	return typ, nil
}

// parseValue parses a GraphQL value.
func (p *Parser) parseValue() (Value, error) {
	switch p.curTok.Kind {
	case TokenDollar:
		// Variable
		p.nextToken()
		if p.curTok.Kind != TokenName {
			return nil, fmt.Errorf("expected variable name, got %s", p.curTok.Kind)
		}
		v := &VariableValue{Name: &Name{Value: p.curTok.Value}}
		p.nextToken()
		return v, nil

	case TokenInt:
		v := &IntValue{Value: p.curTok.Value}
		p.nextToken()
		return v, nil

	case TokenFloat:
		v := &FloatValue{Value: p.curTok.Value}
		p.nextToken()
		return v, nil

	case TokenString:
		v := &StringValue{Value: p.curTok.Value, Block: false}
		p.nextToken()
		return v, nil

	case TokenBlockString:
		v := &StringValue{Value: p.curTok.Value, Block: true}
		p.nextToken()
		return v, nil

	case TokenTrue:
		v := &BooleanValue{Value: true}
		p.nextToken()
		return v, nil

	case TokenFalse:
		v := &BooleanValue{Value: false}
		p.nextToken()
		return v, nil

	case TokenNull:
		v := &NullValue{}
		p.nextToken()
		return v, nil

	case TokenBracketL:
		return p.parseListValue()

	case TokenBraceL:
		return p.parseObjectValue()

	case TokenName:
		// Could be an enum value
		v := &EnumValue{Value: p.curTok.Value}
		p.nextToken()
		return v, nil

	default:
		return nil, fmt.Errorf("unexpected value token %s", p.curTok.Kind)
	}
}

// parseListValue parses a list value [value1, value2].
func (p *Parser) parseListValue() (*ListValue, error) {
	lv := &ListValue{
		Values: []Value{},
	}

	if err := p.expect(TokenBracketL); err != nil {
		return nil, err
	}

	for p.curTok.Kind != TokenBracketR {
		if p.curTok.Kind == TokenEOF {
			return nil, fmt.Errorf("unexpected end of input in list value")
		}

		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		lv.Values = append(lv.Values, val)
	}

	if err := p.expect(TokenBracketR); err != nil {
		return nil, err
	}

	return lv, nil
}

// parseObjectValue parses an object value {key: value}.
func (p *Parser) parseObjectValue() (*ObjectValue, error) {
	ov := &ObjectValue{
		Fields: []*ObjectField{},
	}

	if err := p.expect(TokenBraceL); err != nil {
		return nil, err
	}

	for p.curTok.Kind != TokenBraceR {
		if p.curTok.Kind == TokenEOF {
			return nil, fmt.Errorf("unexpected end of input in object value")
		}

		field, err := p.parseObjectField()
		if err != nil {
			return nil, err
		}
		ov.Fields = append(ov.Fields, field)
	}

	if err := p.expect(TokenBraceR); err != nil {
		return nil, err
	}

	return ov, nil
}

// parseObjectField parses a field in an object value.
func (p *Parser) parseObjectField() (*ObjectField, error) {
	field := &ObjectField{}

	// Field name can be TokenName or any keyword
	if p.curTok.Kind != TokenName && !p.curTok.IsKeyword() {
		return nil, fmt.Errorf("expected field name, got %s", p.curTok.Kind)
	}
	field.Name = &Name{Value: p.curTok.Value}
	p.nextToken()

	if err := p.expect(TokenColon); err != nil {
		return nil, err
	}

	val, err := p.parseValue()
	if err != nil {
		return nil, err
	}
	field.Value = val

	return field, nil
}

// ParseValue parses a GraphQL value string.
func ParseValue(input string) (Value, error) {
	p := NewParserString(input)
	return p.parseValue()
}

// ParseType parses a GraphQL type string.
func ParseType(input string) (Type, error) {
	p := NewParserString(input)
	return p.parseType()
}
