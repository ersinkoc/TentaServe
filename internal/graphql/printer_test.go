package graphql

import (
	"testing"
)

// Test round-trip: Parse(Print(Parse(query))) produces equivalent AST
func TestPrintRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{
			name:  "simple query",
			query: `{ user }`,
		},
		{
			name:  "query with multiple fields",
			query: `{ user { id name email } }`,
		},
		{
			name:  "query with alias",
			query: `{ user: getUser(id: 123) { id name } }`,
		},
		{
			name:  "query with arguments",
			query: `{ user(id: 123, active: true) { name } }`,
		},
		{
			name:  "query with variables",
			query: `query GetUser($id: ID!) { user(id: $id) { name } }`,
		},
		{
			name:  "query with default variable",
			query: `query GetUser($id: ID! = "123") { user(id: $id) { name } }`,
		},
		{
			name:  "mutation",
			query: `mutation CreateUser { createUser(name: "test") { id name } }`,
		},
		{
			name:  "subscription",
			query: `subscription OnUserCreated { userCreated { id name } }`,
		},
		{
			name:  "fragment spread",
			query: `{ user { ...UserFields } } fragment UserFields on User { id name }`,
		},
		{
			name:  "inline fragment",
			query: `{ user { ... on User { id name } } }`,
		},
		{
			name:  "directives",
			query: `{ user @include(if: true) { name @skip(if: false) } }`,
		},
		{
			name:  "directive with arguments",
			query: `{ user @cacheControl(maxAge: 3600) { name } }`,
		},
		{
			name:  "nested selection",
			query: `{ user { posts { comments { text } } } }`,
		},
		{
			name:  "list value",
			query: `{ users(ids: [1, 2, 3]) { name } }`,
		},
		{
			name:  "object value",
			query: `{ createUser(input: {name: "John", email: "john@example.com"}) { id } }`,
		},
		{
			name:  "enum value",
			query: `{ users(status: ACTIVE) { name } }`,
		},
		{
			name:  "null value",
			query: `{ updateUser(id: 1, name: null) { id } }`,
		},
		{
			name:  "multiple operations",
			query: `query GetUser { user { id } } query GetUsers { users { id } }`,
		},
		{
			name:  "float value",
			query: `{ product(price: 19.99) { name } }`,
		},
		{
			name:  "string with escapes",
			query: `{ user(name: "John \"The Man\" Doe") { id } }`,
		},
		{
			name:  "non-null type",
			query: `query GetUser($id: ID!, $name: String) { user(id: $id) { name } }`,
		},
		{
			name:  "list type",
			query: `query GetUsers($ids: [ID!]!) { users(ids: $ids) { id } }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse original
			doc1, err := Parse(tt.query)
			if err != nil {
				t.Fatalf("Failed to parse original: %v", err)
			}

			// Print
			printed := Print(doc1)

			// Parse printed
			doc2, err := Parse(printed)
			if err != nil {
				t.Fatalf("Failed to parse printed: %v\nPrinted: %s", err, printed)
			}

			// Compare
			if !documentsEqual(doc1, doc2) {
				t.Errorf("Documents not equal\nOriginal: %s\nPrinted: %s", tt.query, printed)
			}
		})
	}
}

// documentsEqual compares two documents for semantic equality
func documentsEqual(d1, d2 *Document) bool {
	if len(d1.Definitions) != len(d2.Definitions) {
		return false
	}
	for i := range d1.Definitions {
		if !definitionsEqual(d1.Definitions[i], d2.Definitions[i]) {
			return false
		}
	}
	return true
}

func definitionsEqual(def1, def2 Definition) bool {
	switch d1 := def1.(type) {
	case *OperationDefinition:
		d2, ok := def2.(*OperationDefinition)
		if !ok {
			return false
		}
		return operationDefinitionsEqual(d1, d2)
	case *FragmentDefinition:
		d2, ok := def2.(*FragmentDefinition)
		if !ok {
			return false
		}
		return fragmentDefinitionsEqual(d1, d2)
	default:
		return false
	}
}

func operationDefinitionsEqual(o1, o2 *OperationDefinition) bool {
	if o1.Operation != o2.Operation {
		return false
	}
	if !namesEqual(o1.Name, o2.Name) {
		return false
	}
	if len(o1.VariableDefinitions) != len(o2.VariableDefinitions) {
		return false
	}
	for i := range o1.VariableDefinitions {
		if !variableDefinitionsEqual(o1.VariableDefinitions[i], o2.VariableDefinitions[i]) {
			return false
		}
	}
	if len(o1.Directives) != len(o2.Directives) {
		return false
	}
	for i := range o1.Directives {
		if !directivesEqual(o1.Directives[i], o2.Directives[i]) {
			return false
		}
	}
	if !selectionSetsEqual(o1.SelectionSet, o2.SelectionSet) {
		return false
	}
	return true
}

func fragmentDefinitionsEqual(f1, f2 *FragmentDefinition) bool {
	if !namesEqual(f1.Name, f2.Name) {
		return false
	}
	if !namedTypesEqual(f1.TypeCondition, f2.TypeCondition) {
		return false
	}
	if len(f1.Directives) != len(f2.Directives) {
		return false
	}
	for i := range f1.Directives {
		if !directivesEqual(f1.Directives[i], f2.Directives[i]) {
			return false
		}
	}
	if !selectionSetsEqual(f1.SelectionSet, f2.SelectionSet) {
		return false
	}
	return true
}

func variableDefinitionsEqual(v1, v2 *VariableDefinition) bool {
	if !variablesEqual(v1.Variable, v2.Variable) {
		return false
	}
	if !typesEqual(v1.Type, v2.Type) {
		return false
	}
	if !valuesEqual(v1.DefaultValue, v2.DefaultValue) {
		return false
	}
	return true
}

func variablesEqual(v1, v2 *Variable) bool {
	return namesEqual(v1.Name, v2.Name)
}

func selectionSetsEqual(s1, s2 *SelectionSet) bool {
	if s1 == nil && s2 == nil {
		return true
	}
	if s1 == nil || s2 == nil {
		return false
	}
	if len(s1.Selections) != len(s2.Selections) {
		return false
	}
	for i := range s1.Selections {
		if !selectionsEqual(s1.Selections[i], s2.Selections[i]) {
			return false
		}
	}
	return true
}

func selectionsEqual(sel1, sel2 Selection) bool {
	switch s1 := sel1.(type) {
	case *Field:
		s2, ok := sel2.(*Field)
		if !ok {
			return false
		}
		return fieldsEqual(s1, s2)
	case *FragmentSpread:
		s2, ok := sel2.(*FragmentSpread)
		if !ok {
			return false
		}
		return fragmentSpreadsEqual(s1, s2)
	case *InlineFragment:
		s2, ok := sel2.(*InlineFragment)
		if !ok {
			return false
		}
		return inlineFragmentsEqual(s1, s2)
	default:
		return false
	}
}

func fieldsEqual(f1, f2 *Field) bool {
	if !namesEqual(f1.Alias, f2.Alias) {
		return false
	}
	if !namesEqual(f1.Name, f2.Name) {
		return false
	}
	if len(f1.Arguments) != len(f2.Arguments) {
		return false
	}
	for i := range f1.Arguments {
		if !argumentsEqual(f1.Arguments[i], f2.Arguments[i]) {
			return false
		}
	}
	if len(f1.Directives) != len(f2.Directives) {
		return false
	}
	for i := range f1.Directives {
		if !directivesEqual(f1.Directives[i], f2.Directives[i]) {
			return false
		}
	}
	if !selectionSetsEqual(f1.SelectionSet, f2.SelectionSet) {
		return false
	}
	return true
}

func fragmentSpreadsEqual(f1, f2 *FragmentSpread) bool {
	if !namesEqual(f1.Name, f2.Name) {
		return false
	}
	if len(f1.Directives) != len(f2.Directives) {
		return false
	}
	for i := range f1.Directives {
		if !directivesEqual(f1.Directives[i], f2.Directives[i]) {
			return false
		}
	}
	return true
}

func inlineFragmentsEqual(i1, i2 *InlineFragment) bool {
	if !namedTypesEqual(i1.TypeCondition, i2.TypeCondition) {
		return false
	}
	if len(i1.Directives) != len(i2.Directives) {
		return false
	}
	for i := range i1.Directives {
		if !directivesEqual(i1.Directives[i], i2.Directives[i]) {
			return false
		}
	}
	if !selectionSetsEqual(i1.SelectionSet, i2.SelectionSet) {
		return false
	}
	return true
}

func argumentsEqual(a1, a2 *Argument) bool {
	if !namesEqual(a1.Name, a2.Name) {
		return false
	}
	if !valuesEqual(a1.Value, a2.Value) {
		return false
	}
	return true
}

func directivesEqual(d1, d2 *Directive) bool {
	if !namesEqual(d1.Name, d2.Name) {
		return false
	}
	if len(d1.Arguments) != len(d2.Arguments) {
		return false
	}
	for i := range d1.Arguments {
		if !argumentsEqual(d1.Arguments[i], d2.Arguments[i]) {
			return false
		}
	}
	return true
}

func valuesEqual(v1, v2 Value) bool {
	if v1 == nil && v2 == nil {
		return true
	}
	if v1 == nil || v2 == nil {
		return false
	}

	switch val1 := v1.(type) {
	case *IntValue:
		val2, ok := v2.(*IntValue)
		return ok && val1.Value == val2.Value
	case *FloatValue:
		val2, ok := v2.(*FloatValue)
		return ok && val1.Value == val2.Value
	case *StringValue:
		val2, ok := v2.(*StringValue)
		return ok && val1.Value == val2.Value && val1.Block == val2.Block
	case *BooleanValue:
		val2, ok := v2.(*BooleanValue)
		return ok && val1.Value == val2.Value
	case *NullValue:
		_, ok := v2.(*NullValue)
		return ok
	case *EnumValue:
		val2, ok := v2.(*EnumValue)
		return ok && val1.Value == val2.Value
	case *ListValue:
		val2, ok := v2.(*ListValue)
		if !ok || len(val1.Values) != len(val2.Values) {
			return false
		}
		for i := range val1.Values {
			if !valuesEqual(val1.Values[i], val2.Values[i]) {
				return false
			}
		}
		return true
	case *ObjectValue:
		val2, ok := v2.(*ObjectValue)
		if !ok || len(val1.Fields) != len(val2.Fields) {
			return false
		}
		for i := range val1.Fields {
			if !objectFieldsEqual(val1.Fields[i], val2.Fields[i]) {
				return false
			}
		}
		return true
	case *VariableValue:
		val2, ok := v2.(*VariableValue)
		return ok && namesEqual(val1.Name, val2.Name)
	default:
		return false
	}
}

func objectFieldsEqual(o1, o2 *ObjectField) bool {
	if !namesEqual(o1.Name, o2.Name) {
		return false
	}
	if !valuesEqual(o1.Value, o2.Value) {
		return false
	}
	return true
}

func typesEqual(t1, t2 Type) bool {
	switch typ1 := t1.(type) {
	case *NamedType:
		typ2, ok := t2.(*NamedType)
		return ok && namesEqual(typ1.Name, typ2.Name)
	case *ListType:
		typ2, ok := t2.(*ListType)
		return ok && typesEqual(typ1.Type, typ2.Type)
	case *NonNullType:
		typ2, ok := t2.(*NonNullType)
		return ok && typesEqual(typ1.Type, typ2.Type)
	default:
		return false
	}
}

func namesEqual(n1, n2 *Name) bool {
	if n1 == nil && n2 == nil {
		return true
	}
	if n1 == nil || n2 == nil {
		return false
	}
	return n1.Value == n2.Value
}

func namedTypesEqual(n1, n2 *NamedType) bool {
	if n1 == nil && n2 == nil {
		return true
	}
	if n1 == nil || n2 == nil {
		return false
	}
	return namesEqual(n1.Name, n2.Name)
}

// Test specific output formatting
func TestPrintOutput(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "simple query",
			query:    `{ user }`,
			expected: `{user}`,
		},
		{
			name:     "query with fields",
			query:    `{ user { id name } }`,
			expected: `{user {id name}}`,
		},
		{
			name:     "named query",
			query:    `query GetUser { user { id } }`,
			expected: `query GetUser {user {id}}`,
		},
		{
			name:     "mutation",
			query:    `mutation CreateUser { createUser { id } }`,
			expected: `mutation CreateUser {createUser {id}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := Parse(tt.query)
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			result := Print(doc)
			if result != tt.expected {
				t.Errorf("Expected: %q, Got: %q", tt.expected, result)
			}
		})
	}
}

// Test empty document
func TestPrintEmptyDocument(t *testing.T) {
	doc := &Document{Definitions: []Definition{}}
	result := Print(doc)
	if result != "" {
		t.Errorf("Expected empty string, got: %q", result)
	}
}

// Test escapeString function
func TestEscapeString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`hello`, `hello`},
		{`hello"world`, `hello\"world`},
		{`hello\world`, `hello\\world`},
		{`hello\nworld`, `hello\\nworld`},
		{`hello\tworld`, `hello\\tworld`},
		{`hello\rworld`, `hello\\rworld`},
		{`hello\bworld`, `hello\\bworld`},
		{`hello\fworld`, `hello\\fworld`},
		{"hello\nworld", `hello\nworld`},
		{"hello\tworld", `hello\tworld`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeString(tt.input)
			if result != tt.expected {
				t.Errorf("escapeString(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Benchmark printer
func BenchmarkPrint(b *testing.B) {
	query := `
		query GetUser($id: ID!, $includeEmail: Boolean = true) {
			user(id: $id) @cacheControl(maxAge: 3600) {
				id
				name
				email @include(if: $includeEmail)
				posts(first: 10) {
					edges {
						node {
							title
							author {
								name
							}
						}
					}
				}
			}
		}
		fragment UserFields on User {
			id
			name
		}
	`

	doc, err := Parse(query)
	if err != nil {
		b.Fatalf("Failed to parse: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Print(doc)
	}
}
