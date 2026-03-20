package graphql

// Node is the base interface for all AST nodes.
type Node interface {
	// TokenLiteral returns the literal value of the token associated with this node.
	TokenLiteral() string
}

// Document is the root node of a GraphQL query.
type Document struct {
	Definitions []Definition
}

// TokenLiteral returns a string representation for debugging.
func (d *Document) TokenLiteral() string {
	if len(d.Definitions) > 0 {
		return d.Definitions[0].TokenLiteral()
	}
	return ""
}

// Definition is the interface for top-level definitions in a document.
type Definition interface {
	Node
	definitionNode()
}

// OperationDefinition represents a query, mutation, or subscription.
type OperationDefinition struct {
	Operation           TokenKind // TokenQuery, TokenMutation, or TokenSubscription
	Name                *Name
	VariableDefinitions []*VariableDefinition
	Directives          []*Directive
	SelectionSet        *SelectionSet
}

func (o *OperationDefinition) definitionNode() {}

// TokenLiteral returns the operation type as a string.
func (o *OperationDefinition) TokenLiteral() string {
	return o.Operation.String()
}

// FragmentDefinition represents a fragment definition.
type FragmentDefinition struct {
	Name          *Name
	TypeCondition *NamedType
	Directives    []*Directive
	SelectionSet  *SelectionSet
}

func (f *FragmentDefinition) definitionNode() {}

// TokenLiteral returns the fragment name.
func (f *FragmentDefinition) TokenLiteral() string {
	return "fragment"
}

// Selection is the interface for field selections.
type Selection interface {
	Node
	selectionNode()
}

// Field represents a field selection.
type Field struct {
	Alias        *Name
	Name         *Name
	Arguments    []*Argument
	Directives   []*Directive
	SelectionSet *SelectionSet
}

func (f *Field) selectionNode() {}

// TokenLiteral returns the field name.
func (f *Field) TokenLiteral() string {
	if f.Name != nil {
		return f.Name.Value
	}
	return ""
}

// FragmentSpread represents a fragment spread (...FragmentName).
type FragmentSpread struct {
	Name       *Name
	Directives []*Directive
}

func (f *FragmentSpread) selectionNode() {}

// TokenLiteral returns the spread operator.
func (f *FragmentSpread) TokenLiteral() string {
	return "..."
}

// InlineFragment represents an inline fragment (... on Type { ... }).
type InlineFragment struct {
	TypeCondition *NamedType
	Directives    []*Directive
	SelectionSet  *SelectionSet
}

func (i *InlineFragment) selectionNode() {}

// TokenLiteral returns the spread operator.
func (i *InlineFragment) TokenLiteral() string {
	return "..."
}

// SelectionSet represents a set of selections wrapped in braces.
type SelectionSet struct {
	Selections []Selection
}

// TokenLiteral returns the opening brace.
func (s *SelectionSet) TokenLiteral() string {
	return "{"
}

// Name represents an identifier.
type Name struct {
	Value string
}

// TokenLiteral returns the name value.
func (n *Name) TokenLiteral() string {
	return n.Value
}

// VariableDefinition represents a variable definition ($var: Type = default).
type VariableDefinition struct {
	Variable     *Variable
	Type         Type
	DefaultValue Value
}

// TokenLiteral returns the variable name.
func (v *VariableDefinition) TokenLiteral() string {
	if v.Variable != nil {
		return v.Variable.TokenLiteral()
	}
	return ""
}

// Variable represents a variable reference ($var).
type Variable struct {
	Name *Name
}

// TokenLiteral returns the variable name with $.
func (v *Variable) TokenLiteral() string {
	if v.Name != nil {
		return "$" + v.Name.Value
	}
	return "$"
}

// Argument represents a field argument (name: value).
type Argument struct {
	Name  *Name
	Value Value
}

// TokenLiteral returns the argument name.
func (a *Argument) TokenLiteral() string {
	if a.Name != nil {
		return a.Name.Value
	}
	return ""
}

// Directive represents a directive (@name or @name(arg: value)).
type Directive struct {
	Name      *Name
	Arguments []*Argument
}

// TokenLiteral returns the directive name with @.
func (d *Directive) TokenLiteral() string {
	if d.Name != nil {
		return "@" + d.Name.Value
	}
	return "@"
}

// Type is the interface for GraphQL types.
type Type interface {
	Node
	typeNode()
}

// NamedType represents a named type (e.g., String, User).
type NamedType struct {
	Name *Name
}

func (n *NamedType) typeNode() {}

// TokenLiteral returns the type name.
func (n *NamedType) TokenLiteral() string {
	if n.Name != nil {
		return n.Name.Value
	}
	return ""
}

// ListType represents a list type (e.g., [String]).
type ListType struct {
	Type Type
}

func (l *ListType) typeNode() {}

// TokenLiteral returns the opening bracket.
func (l *ListType) TokenLiteral() string {
	return "["
}

// NonNullType represents a non-null type (e.g., String!).
type NonNullType struct {
	Type Type // Must be NamedType or ListType
}

func (n *NonNullType) typeNode() {}

// TokenLiteral returns the underlying type with !.
func (n *NonNullType) TokenLiteral() string {
	return n.Type.TokenLiteral() + "!"
}

// Value is the interface for GraphQL values.
type Value interface {
	Node
	valueNode()
}

// IntValue represents an integer value.
type IntValue struct {
	Value string // Keep as string to preserve original representation
}

func (i *IntValue) valueNode() {}

// TokenLiteral returns the integer value.
func (i *IntValue) TokenLiteral() string {
	return i.Value
}

// FloatValue represents a float value.
type FloatValue struct {
	Value string // Keep as string to preserve original representation
}

func (f *FloatValue) valueNode() {}

// TokenLiteral returns the float value.
func (f *FloatValue) TokenLiteral() string {
	return f.Value
}

// StringValue represents a string value.
type StringValue struct {
	Value string
	Block bool // true if this is a block string
}

func (s *StringValue) valueNode() {}

// TokenLiteral returns the string value.
func (s *StringValue) TokenLiteral() string {
	return s.Value
}

// BooleanValue represents a boolean value.
type BooleanValue struct {
	Value bool
}

func (b *BooleanValue) valueNode() {}

// TokenLiteral returns "true" or "false".
func (b *BooleanValue) TokenLiteral() string {
	if b.Value {
		return "true"
	}
	return "false"
}

// NullValue represents a null value.
type NullValue struct{}

func (n *NullValue) valueNode() {}

// TokenLiteral returns "null".
func (n *NullValue) TokenLiteral() string {
	return "null"
}

// EnumValue represents an enum value (identifier that's not true/false/null).
type EnumValue struct {
	Value string
}

func (e *EnumValue) valueNode() {}

// TokenLiteral returns the enum value.
func (e *EnumValue) TokenLiteral() string {
	return e.Value
}

// ListValue represents a list value ([1, 2, 3]).
type ListValue struct {
	Values []Value
}

func (l *ListValue) valueNode() {}

// TokenLiteral returns the opening bracket.
func (l *ListValue) TokenLiteral() string {
	return "["
}

// ObjectValue represents an object value ({key: value}).
type ObjectValue struct {
	Fields []*ObjectField
}

func (o *ObjectValue) valueNode() {}

// TokenLiteral returns the opening brace.
func (o *ObjectValue) TokenLiteral() string {
	return "{"
}

// ObjectField represents a field in an object value.
type ObjectField struct {
	Name  *Name
	Value Value
}

// TokenLiteral returns the field name.
func (o *ObjectField) TokenLiteral() string {
	if o.Name != nil {
		return o.Name.Value
	}
	return ""
}

// VariableValue represents a variable reference as a value.
type VariableValue struct {
	Name *Name
}

func (v *VariableValue) valueNode() {}

// TokenLiteral returns the variable name with $.
func (v *VariableValue) TokenLiteral() string {
	if v.Name != nil {
		return "$" + v.Name.Value
	}
	return "$"
}
