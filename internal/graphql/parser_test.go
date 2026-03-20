package graphql

import (
	"testing"
)

func TestParseSimpleQuery(t *testing.T) {
	input := `{ hello }`
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(doc.Definitions) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(doc.Definitions))
	}

	// Should be an anonymous operation
	op, ok := doc.Definitions[0].(*OperationDefinition)
	if !ok {
		t.Fatalf("expected OperationDefinition, got %T", doc.Definitions[0])
	}

	if op.Operation != TokenQuery {
		t.Errorf("expected query, got %v", op.Operation)
	}

	if op.SelectionSet == nil || len(op.SelectionSet.Selections) != 1 {
		t.Fatalf("expected 1 selection")
	}

	field, ok := op.SelectionSet.Selections[0].(*Field)
	if !ok {
		t.Fatalf("expected Field, got %T", op.SelectionSet.Selections[0])
	}

	if field.Name.Value != "hello" {
		t.Errorf("expected field name 'hello', got %q", field.Name.Value)
	}
}

func TestParseNamedQuery(t *testing.T) {
	input := `query GetUser { user { name } }`
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	op := doc.Definitions[0].(*OperationDefinition)
	if op.Name == nil || op.Name.Value != "GetUser" {
		t.Errorf("expected query name 'GetUser', got %v", op.Name)
	}
}

func TestParseMutation(t *testing.T) {
	input := `mutation CreateUser { createUser(name: "John") { id } }`
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	op := doc.Definitions[0].(*OperationDefinition)
	if op.Operation != TokenMutation {
		t.Errorf("expected mutation, got %v", op.Operation)
	}
}

func TestParseSubscription(t *testing.T) {
	input := `subscription UserUpdates { userUpdated { id name } }`
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	op := doc.Definitions[0].(*OperationDefinition)
	if op.Operation != TokenSubscription {
		t.Errorf("expected subscription, got %v", op.Operation)
	}
}

func TestParseVariables(t *testing.T) {
	input := `query GetUser($id: ID!, $includeEmail: Boolean = false) { user(id: $id) }`
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	op := doc.Definitions[0].(*OperationDefinition)
	if len(op.VariableDefinitions) != 2 {
		t.Fatalf("expected 2 variables, got %d", len(op.VariableDefinitions))
	}

	// First variable: $id: ID!
	var1 := op.VariableDefinitions[0]
	if var1.Variable.Name.Value != "id" {
		t.Errorf("expected var name 'id', got %q", var1.Variable.Name.Value)
	}
	_, ok := var1.Type.(*NonNullType)
	if !ok {
		t.Errorf("expected non-null type for id")
	}

	// Second variable: $includeEmail: Boolean = false
	var2 := op.VariableDefinitions[1]
	if var2.Variable.Name.Value != "includeEmail" {
		t.Errorf("expected var name 'includeEmail', got %q", var2.Variable.Name.Value)
	}
	if var2.DefaultValue == nil {
		t.Errorf("expected default value")
	}
	boolVal, ok := var2.DefaultValue.(*BooleanValue)
	if !ok || boolVal.Value != false {
		t.Errorf("expected default value false")
	}
}

func TestParseFieldArguments(t *testing.T) {
	input := `{ user(id: 123, name: "John", active: true, count: 3.14) { id } }`
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	op := doc.Definitions[0].(*OperationDefinition)
	field := op.SelectionSet.Selections[0].(*Field)

	if len(field.Arguments) != 4 {
		t.Fatalf("expected 4 arguments, got %d", len(field.Arguments))
	}

	// id: 123
	if field.Arguments[0].Name.Value != "id" {
		t.Errorf("expected arg name 'id', got %q", field.Arguments[0].Name.Value)
	}
	intVal, ok := field.Arguments[0].Value.(*IntValue)
	if !ok || intVal.Value != "123" {
		t.Errorf("expected int value 123")
	}

	// name: "John"
	strVal, ok := field.Arguments[1].Value.(*StringValue)
	if !ok || strVal.Value != "John" {
		t.Errorf("expected string value 'John', got %v", field.Arguments[1].Value)
	}

	// active: true
	boolVal, ok := field.Arguments[2].Value.(*BooleanValue)
	if !ok || boolVal.Value != true {
		t.Errorf("expected boolean value true")
	}

	// count: 3.14
	floatVal, ok := field.Arguments[3].Value.(*FloatValue)
	if !ok || floatVal.Value != "3.14" {
		t.Errorf("expected float value 3.14")
	}
}

func TestParseFieldAlias(t *testing.T) {
	input := `{ myUser: user { name } }`
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	op := doc.Definitions[0].(*OperationDefinition)
	field := op.SelectionSet.Selections[0].(*Field)

	if field.Alias == nil || field.Alias.Value != "myUser" {
		t.Errorf("expected alias 'myUser', got %v", field.Alias)
	}

	if field.Name.Value != "user" {
		t.Errorf("expected field name 'user', got %q", field.Name.Value)
	}
}

func TestParseNestedFields(t *testing.T) {
	input := `{
		user {
			id
			name
			posts {
				title
			}
		}
	}`
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	op := doc.Definitions[0].(*OperationDefinition)
	userField := op.SelectionSet.Selections[0].(*Field)

	if userField.SelectionSet == nil || len(userField.SelectionSet.Selections) != 3 {
		t.Fatalf("expected 3 fields in user")
	}

	postsField := userField.SelectionSet.Selections[2].(*Field)
	if postsField.Name.Value != "posts" {
		t.Errorf("expected 'posts', got %q", postsField.Name.Value)
	}

	if postsField.SelectionSet == nil || len(postsField.SelectionSet.Selections) != 1 {
		t.Fatalf("expected 1 field in posts")
	}
}

func TestParseFragmentSpread(t *testing.T) {
	input := `{
		user {
			...UserFields
		}
	}`
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	op := doc.Definitions[0].(*OperationDefinition)
	userField := op.SelectionSet.Selections[0].(*Field)

	if len(userField.SelectionSet.Selections) != 1 {
		t.Fatalf("expected 1 selection")
	}

	spread, ok := userField.SelectionSet.Selections[0].(*FragmentSpread)
	if !ok {
		t.Fatalf("expected FragmentSpread, got %T", userField.SelectionSet.Selections[0])
	}

	if spread.Name.Value != "UserFields" {
		t.Errorf("expected fragment name 'UserFields', got %q", spread.Name.Value)
	}
}

func TestParseInlineFragment(t *testing.T) {
	input := `{
		user {
			... on Admin {
				permissions
			}
		}
	}`
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	op := doc.Definitions[0].(*OperationDefinition)
	userField := op.SelectionSet.Selections[0].(*Field)

	if len(userField.SelectionSet.Selections) != 1 {
		t.Fatalf("expected 1 selection")
	}

	frag, ok := userField.SelectionSet.Selections[0].(*InlineFragment)
	if !ok {
		t.Fatalf("expected InlineFragment, got %T", userField.SelectionSet.Selections[0])
	}

	if frag.TypeCondition.Name.Value != "Admin" {
		t.Errorf("expected type condition 'Admin', got %q", frag.TypeCondition.Name.Value)
	}

	if len(frag.SelectionSet.Selections) != 1 {
		t.Fatalf("expected 1 field in inline fragment")
	}
}

func TestParseFragmentDefinition(t *testing.T) {
	input := `
		fragment UserFields on User {
			id
			name
		}
	`
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(doc.Definitions) != 1 {
		t.Fatalf("expected 1 definition")
	}

	frag, ok := doc.Definitions[0].(*FragmentDefinition)
	if !ok {
		t.Fatalf("expected FragmentDefinition, got %T", doc.Definitions[0])
	}

	if frag.Name.Value != "UserFields" {
		t.Errorf("expected fragment name 'UserFields', got %q", frag.Name.Value)
	}

	if frag.TypeCondition.Name.Value != "User" {
		t.Errorf("expected type condition 'User', got %q", frag.TypeCondition.Name.Value)
	}
}

func TestParseDirective(t *testing.T) {
	input := `{ user @include(if: true) { name } }`
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	op := doc.Definitions[0].(*OperationDefinition)
	field := op.SelectionSet.Selections[0].(*Field)

	if len(field.Directives) != 1 {
		t.Fatalf("expected 1 directive, got %d", len(field.Directives))
	}

	dir := field.Directives[0]
	if dir.Name.Value != "include" {
		t.Errorf("expected directive name 'include', got %q", dir.Name.Value)
	}

	if len(dir.Arguments) != 1 {
		t.Fatalf("expected 1 argument in directive")
	}
}

func TestParseMultipleDirectives(t *testing.T) {
	input := `{ user @include(if: true) @cacheControl(maxAge: 3600) { name } }`
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	op := doc.Definitions[0].(*OperationDefinition)
	field := op.SelectionSet.Selections[0].(*Field)

	if len(field.Directives) != 2 {
		t.Fatalf("expected 2 directives, got %d", len(field.Directives))
	}
}

func TestParseListValue(t *testing.T) {
	input := `{ users(ids: [1, 2, 3]) { name } }`
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	op := doc.Definitions[0].(*OperationDefinition)
	field := op.SelectionSet.Selections[0].(*Field)
	arg := field.Arguments[0]

	listVal, ok := arg.Value.(*ListValue)
	if !ok {
		t.Fatalf("expected ListValue, got %T", arg.Value)
	}

	if len(listVal.Values) != 3 {
		t.Fatalf("expected 3 values in list, got %d", len(listVal.Values))
	}
}

func TestParseObjectValue(t *testing.T) {
	input := `{ createUser(input: { name: "John", email: "john@example.com" }) { id } }`
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	op := doc.Definitions[0].(*OperationDefinition)
	field := op.SelectionSet.Selections[0].(*Field)
	arg := field.Arguments[0]

	objVal, ok := arg.Value.(*ObjectValue)
	if !ok {
		t.Fatalf("expected ObjectValue, got %T", arg.Value)
	}

	if len(objVal.Fields) != 2 {
		t.Fatalf("expected 2 fields in object, got %d", len(objVal.Fields))
	}
}

func TestParseVariableAsValue(t *testing.T) {
	input := `{ user(id: $userId) { name } }`
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	op := doc.Definitions[0].(*OperationDefinition)
	field := op.SelectionSet.Selections[0].(*Field)
	arg := field.Arguments[0]

	varVal, ok := arg.Value.(*VariableValue)
	if !ok {
		t.Fatalf("expected VariableValue, got %T", arg.Value)
	}

	if varVal.Name.Value != "userId" {
		t.Errorf("expected variable name 'userId', got %q", varVal.Name.Value)
	}
}

func TestParseType(t *testing.T) {
	tests := []struct {
		input     string
		expected  string
		isNonNull bool
		isList    bool
	}{
		{"String", "String", false, false},
		{"String!", "String", true, false},
		{"[String]", "String", false, true},
		{"[String!]", "String", true, true},
		{"[String]!", "String", true, true},
		{"[String!]!", "String", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			typ, err := ParseType(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Check type structure (unwrap conditionally based on actual type)
			if tt.isNonNull {
				// NonNull could be outer wrapper (e.g., [String]!) or inner (e.g., [String!])
				// Check outer first
				if nonNull, ok := typ.(*NonNullType); ok {
					typ = nonNull.Type
				} else if tt.isList {
					// NonNull is probably inside the list, continue to unwrap list first
				} else {
					t.Fatalf("expected NonNullType")
				}
			}

			if tt.isList {
				listType, ok := typ.(*ListType)
				if !ok {
					t.Fatalf("expected ListType")
				}
				typ = listType.Type
			}

			// Check for inner NonNull (e.g., [String!])
			if tt.isNonNull {
				if nonNull, ok := typ.(*NonNullType); ok {
					typ = nonNull.Type
				}
			}

			named, ok := typ.(*NamedType)
			if !ok {
				t.Fatalf("expected NamedType")
			}

			if named.Name.Value != tt.expected {
				t.Errorf("expected type %q, got %q", tt.expected, named.Name.Value)
			}
		})
	}
}

func TestParseComplexQuery(t *testing.T) {
	input := `
		query GetUser($id: ID!, $includeEmail: Boolean = false) {
			user(id: $id) @include(if: $includeEmail) {
				id
				name
				email
				friends(first: 10) @cacheControl(maxAge: 3600) {
					nodes {
						name
					}
				}
			}
		}
	`

	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Just verify it parses without error
	if len(doc.Definitions) != 1 {
		t.Fatalf("expected 1 definition")
	}
}

func TestParseMultipleDefinitions(t *testing.T) {
	input := `
		query GetUser { user { id } }
		query GetPost { post { title } }
		fragment UserFields on User { id name }
	`

	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(doc.Definitions) != 3 {
		t.Fatalf("expected 3 definitions, got %d", len(doc.Definitions))
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "unclosed brace",
			input: `{ user { id `,
		},
		{
			name:  "missing colon",
			input: `{ user(id "test") }`,
		},
		{
			name:  "invalid token",
			input: `{ user { . } }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.input)
			if err == nil {
				t.Errorf("expected error for input %q", tt.input)
			}
		})
	}
}

func BenchmarkParse(b *testing.B) {
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
		_, _ = Parse(input)
	}
}

// --- Coverage boost tests for parser ---

func TestParseValue_AllTypes(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"int", "42"},
		{"float", "3.14"},
		{"string", `"hello"`},
		{"bool true", "true"},
		{"bool false", "false"},
		{"null", "null"},
		{"enum", "ACTIVE"},
		{"list", "[1, 2, 3]"},
		{"object", `{key: "value"}`},
		{"variable", "$myVar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := ParseValue(tt.input)
			if err != nil {
				t.Fatalf("ParseValue(%q) error: %v", tt.input, err)
			}
			if val == nil {
				t.Fatalf("ParseValue(%q) returned nil", tt.input)
			}
		})
	}
}

func TestParser_Errors_Method(t *testing.T) {
	p := NewParserString(`{ valid }`)
	_, _ = p.Parse()
	errs := p.Errors()
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}

	p2 := NewParserString(`{ . }`)
	_, _ = p2.Parse()
	errs2 := p2.Errors()
	if len(errs2) == 0 {
		t.Error("expected parse errors for invalid input")
	}
}

func TestParser_Peek(t *testing.T) {
	// Peek is used internally in parsing, verify it via behavior
	// Variable definition uses peek when checking default values
	input := `query Q($x: Int = 5) { field }`
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	op := doc.Definitions[0].(*OperationDefinition)
	if len(op.VariableDefinitions) != 1 {
		t.Fatalf("expected 1 var def, got %d", len(op.VariableDefinitions))
	}
	if op.VariableDefinitions[0].DefaultValue == nil {
		t.Error("expected default value for variable")
	}
}

func TestParseType_Standalone(t *testing.T) {
	tests := []struct {
		input string
	}{
		{"String"},
		{"Int"},
		{"[String]"},
		{"[Int!]!"},
		{"[[String]]"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			typ, err := ParseType(tt.input)
			if err != nil {
				t.Fatalf("ParseType(%q) error: %v", tt.input, err)
			}
			if typ == nil {
				t.Fatalf("ParseType(%q) returned nil", tt.input)
			}
		})
	}
}
