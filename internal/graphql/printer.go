package graphql

import "strings"

// Printer converts GraphQL AST back to a query string.
type Printer struct {
	builder strings.Builder
}

// Print converts a Document to a GraphQL query string.
func Print(doc *Document) string {
	p := &Printer{}
	p.printDocument(doc)
	return p.builder.String()
}

// printDocument prints the root document.
func (p *Printer) printDocument(doc *Document) {
	for i, def := range doc.Definitions {
		if i > 0 {
			p.builder.WriteString("\n")
		}
		p.printDefinition(def)
	}
}

// printDefinition prints a top-level definition.
func (p *Printer) printDefinition(def Definition) {
	switch d := def.(type) {
	case *OperationDefinition:
		p.printOperationDefinition(d)
	case *FragmentDefinition:
		p.printFragmentDefinition(d)
	}
}

// printOperationDefinition prints a query, mutation, or subscription.
func (p *Printer) printOperationDefinition(op *OperationDefinition) {
	// Operation type - only print if not implicit query
	if op.Operation != TokenQuery || op.Name != nil && op.Name.Value != "" || len(op.VariableDefinitions) > 0 || len(op.Directives) > 0 {
		p.builder.WriteString(operationTypeString(op.Operation))

		// Name (optional for query)
		if op.Name != nil && op.Name.Value != "" {
			p.builder.WriteString(" ")
			p.builder.WriteString(op.Name.Value)
		}

		// Variable definitions
		if len(op.VariableDefinitions) > 0 {
			p.builder.WriteString("(")
			for i, v := range op.VariableDefinitions {
				if i > 0 {
					p.builder.WriteString(", ")
				}
				p.printVariableDefinition(v)
			}
			p.builder.WriteString(")")
		}

		// Directives
		p.printDirectives(op.Directives)

		p.builder.WriteString(" ")
	}

	// Selection set
	if op.SelectionSet != nil {
		p.printSelectionSet(op.SelectionSet)
	}
}

// operationTypeString returns the lowercase operation type.
func operationTypeString(kind TokenKind) string {
	switch kind {
	case TokenQuery:
		return "query"
	case TokenMutation:
		return "mutation"
	case TokenSubscription:
		return "subscription"
	default:
		return ""
	}
}

// printFragmentDefinition prints a fragment definition.
func (p *Printer) printFragmentDefinition(frag *FragmentDefinition) {
	p.builder.WriteString("fragment ")
	p.builder.WriteString(frag.Name.Value)
	p.builder.WriteString(" on ")
	p.builder.WriteString(frag.TypeCondition.Name.Value)

	// Directives
	p.printDirectives(frag.Directives)

	p.builder.WriteString(" ")
	p.printSelectionSet(frag.SelectionSet)
}

// printVariableDefinition prints a variable definition.
func (p *Printer) printVariableDefinition(v *VariableDefinition) {
	p.builder.WriteString("$")
	p.builder.WriteString(v.Variable.Name.Value)
	p.builder.WriteString(": ")
	p.printType(v.Type)

	if v.DefaultValue != nil {
		p.builder.WriteString(" = ")
		p.printValue(v.DefaultValue)
	}
}

// printSelectionSet prints a set of selections.
func (p *Printer) printSelectionSet(set *SelectionSet) {
	p.builder.WriteString("{")
	for i, sel := range set.Selections {
		if i > 0 {
			p.builder.WriteString(" ")
		}
		p.printSelection(sel)
	}
	p.builder.WriteString("}")
}

// printSelection prints a single selection.
func (p *Printer) printSelection(sel Selection) {
	switch s := sel.(type) {
	case *Field:
		p.printField(s)
	case *FragmentSpread:
		p.printFragmentSpread(s)
	case *InlineFragment:
		p.printInlineFragment(s)
	}
}

// printField prints a field selection.
func (p *Printer) printField(f *Field) {
	// Alias
	if f.Alias != nil && f.Alias.Value != "" && f.Alias.Value != f.Name.Value {
		p.builder.WriteString(f.Alias.Value)
		p.builder.WriteString(": ")
	}

	// Name
	p.builder.WriteString(f.Name.Value)

	// Arguments
	if len(f.Arguments) > 0 {
		p.builder.WriteString("(")
		for i, arg := range f.Arguments {
			if i > 0 {
				p.builder.WriteString(", ")
			}
			p.printArgument(arg)
		}
		p.builder.WriteString(")")
	}

	// Directives
	p.printDirectives(f.Directives)

	// Selection set
	if f.SelectionSet != nil {
		p.builder.WriteString(" ")
		p.printSelectionSet(f.SelectionSet)
	}
}

// printFragmentSpread prints a fragment spread.
func (p *Printer) printFragmentSpread(f *FragmentSpread) {
	p.builder.WriteString("...")
	p.builder.WriteString(f.Name.Value)
	p.printDirectives(f.Directives)
}

// printInlineFragment prints an inline fragment.
func (p *Printer) printInlineFragment(i *InlineFragment) {
	p.builder.WriteString("...")

	if i.TypeCondition != nil {
		p.builder.WriteString(" on ")
		p.builder.WriteString(i.TypeCondition.Name.Value)
	}

	p.printDirectives(i.Directives)
	p.builder.WriteString(" ")
	p.printSelectionSet(i.SelectionSet)
}

// printDirectives prints a list of directives.
func (p *Printer) printDirectives(directives []*Directive) {
	for _, d := range directives {
		p.builder.WriteString(" ")
		p.printDirective(d)
	}
}

// printDirective prints a single directive.
func (p *Printer) printDirective(d *Directive) {
	p.builder.WriteString("@")
	p.builder.WriteString(d.Name.Value)

	if len(d.Arguments) > 0 {
		p.builder.WriteString("(")
		for i, arg := range d.Arguments {
			if i > 0 {
				p.builder.WriteString(", ")
			}
			p.printArgument(arg)
		}
		p.builder.WriteString(")")
	}
}

// printArgument prints an argument.
func (p *Printer) printArgument(arg *Argument) {
	p.builder.WriteString(arg.Name.Value)
	p.builder.WriteString(": ")
	p.printValue(arg.Value)
}

// printType prints a GraphQL type.
func (p *Printer) printType(t Type) {
	switch typ := t.(type) {
	case *NamedType:
		p.builder.WriteString(typ.Name.Value)
	case *ListType:
		p.builder.WriteString("[")
		p.printType(typ.Type)
		p.builder.WriteString("]")
	case *NonNullType:
		p.printType(typ.Type)
		p.builder.WriteString("!")
	}
}

// printValue prints a GraphQL value.
func (p *Printer) printValue(v Value) {
	switch val := v.(type) {
	case *IntValue:
		p.builder.WriteString(val.Value)
	case *FloatValue:
		p.builder.WriteString(val.Value)
	case *StringValue:
		p.printStringValue(val)
	case *BooleanValue:
		if val.Value {
			p.builder.WriteString("true")
		} else {
			p.builder.WriteString("false")
		}
	case *NullValue:
		p.builder.WriteString("null")
	case *EnumValue:
		p.builder.WriteString(val.Value)
	case *ListValue:
		p.builder.WriteString("[")
		for i, item := range val.Values {
			if i > 0 {
				p.builder.WriteString(", ")
			}
			p.printValue(item)
		}
		p.builder.WriteString("]")
	case *ObjectValue:
		p.builder.WriteString("{")
		for i, field := range val.Fields {
			if i > 0 {
				p.builder.WriteString(", ")
			}
			p.builder.WriteString(field.Name.Value)
			p.builder.WriteString(": ")
			p.printValue(field.Value)
		}
		p.builder.WriteString("}")
	case *VariableValue:
		p.builder.WriteString("$")
		p.builder.WriteString(val.Name.Value)
	}
}

// printStringValue prints a string value with proper escaping.
func (p *Printer) printStringValue(s *StringValue) {
	if s.Block {
		// Block string
		p.builder.WriteString("\"\"\"\n")
		p.builder.WriteString(s.Value)
		p.builder.WriteString("\"\"\"")
	} else {
		// Regular string with escaping
		p.builder.WriteString("\"")
		p.builder.WriteString(escapeString(s.Value))
		p.builder.WriteString("\"")
	}
}

// escapeString escapes special characters in a string value.
func escapeString(s string) string {
	var result strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			result.WriteString("\\\\")
		case '"':
			result.WriteString("\\\"")
		case '\n':
			result.WriteString("\\n")
		case '\r':
			result.WriteString("\\r")
		case '\t':
			result.WriteString("\\t")
		case '\b':
			result.WriteString("\\b")
		case '\f':
			result.WriteString("\\f")
		default:
			result.WriteRune(r)
		}
	}
	return result.String()
}
