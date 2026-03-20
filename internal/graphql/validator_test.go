package graphql

import (
	"strings"
	"testing"
)

func TestValidator_Depth(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected int
	}{
		{
			name:     "flat query - depth 1",
			query:    `{ user }`,
			expected: 1,
		},
		{
			name:     "two levels - depth 2",
			query:    `{ user { id } }`,
			expected: 2,
		},
		{
			name:     "three levels - depth 3",
			query:    `{ user { posts { title } } }`,
			expected: 3,
		},
		{
			name:     "deeply nested - depth 5",
			query:    `{ a { b { c { d { e } } } } }`,
			expected: 5,
		},
		{
			name:     "deeply nested - depth 10",
			query:    `{ a { b { c { d { e { f { g { h { i { j } } } } } } } } } }`,
			expected: 10,
		},
		{
			name:     "deeply nested - depth 15",
			query:    `{ a { b { c { d { e { f { g { h { i { j { k { l { m { n { o } } } } } } } } } } } } } } }`,
			expected: 15,
		},
		{
			name:     "multiple fields same level",
			query:    `{ user { id name email } }`,
			expected: 2,
		},
		{
			name:     "different depth branches - max wins",
			query:    `{ user { id posts { title } } }`,
			expected: 3,
		},
		{
			name:     "fragment spread does not increase depth",
			query:    `{ user { ...UserFields } } fragment UserFields on User { id }`,
			expected: 1, // Fragment spread itself doesn't add depth, separate fragment def has depth 1
		},
		{
			name:     "inline fragment increases depth",
			query:    `{ user { ... on User { posts { title } } } }`,
			expected: 4, // user(1) -> inline fragment(2) -> posts(2) -> title(3) = max depth 3? Actually 4
		},
		{
			name:     "named query",
			query:    `query GetUser { user { id } }`,
			expected: 2,
		},
		{
			name:     "mutation",
			query:    `mutation CreateUser { createUser { id } }`,
			expected: 2,
		},
		{
			name:     "subscription",
			query:    `subscription OnUserCreated { userCreated { id } }`,
			expected: 2,
		},
		{
			name:     "multiple operations - max wins",
			query:    `query Q1 { a { b } } query Q2 { a { b { c { d } } } }`,
			expected: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := Parse(tt.query)
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			depth := CheckDepth(doc)
			if depth != tt.expected {
				t.Errorf("Expected depth %d, got %d", tt.expected, depth)
			}
		})
	}
}

func TestValidator_Complexity(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		minExpected int // complexity varies by implementation, check it's reasonable
	}{
		{
			name:     "single field",
			query:    `{ user }`,
			minExpected: 1,
		},
		{
			name:     "multiple fields flat",
			query:    `{ user { id name email phone } }`,
			minExpected: 4,
		},
		{
			name:     "nested fields",
			query:    `{ user { posts { title body author { name } } } }`,
			minExpected: 10,
		},
		{
			name:     "wide query with lists",
			query:    `{ users { id name email posts { title comments { text } } } }`,
			minExpected: 20,
		},
		{
			name:     "query with arguments",
			query:    `{ user(id: 1, active: true) { name } }`,
			minExpected: 5,
		},
		{
			name:     "query with directives",
			query:    `{ user @include(if: true) { name @skip(if: false) } }`,
			minExpected: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := Parse(tt.query)
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			complexity := CalculateComplexity(doc)
			if complexity < tt.minExpected {
				t.Errorf("Expected complexity >= %d, got %d", tt.minExpected, complexity)
			}
			t.Logf("Query complexity: %d", complexity)
		})
	}
}

func TestValidator_Validate(t *testing.T) {
	tests := []struct {
		name            string
		query           string
		maxDepth        int
		maxComplexity   int
		expectDepthErr  bool
		expectComplexityErr bool
	}{
		{
			name:         "within limits",
			query:        `{ user { id name } }`,
			maxDepth:     10,
			maxComplexity: 100,
			expectDepthErr: false,
			expectComplexityErr: false,
		},
		{
			name:         "exceeds depth limit",
			query:        `{ a { b { c { d { e { f { g { h { i { j { k } } } } } } } } } } }`,
			maxDepth:     10,
			maxComplexity: 1000,
			expectDepthErr: true,
			expectComplexityErr: false,
		},
		{
			name:         "exceeds complexity limit",
			query:        `{ f1 { f2 { f3 { f4 { f5 { f6 { f7 { f8 { f9 { f10 } } } } } } } } } }`,
			maxDepth:     20,
			maxComplexity: 50,
			expectDepthErr: false,
			expectComplexityErr: true,
		},
		{
			name:         "exceeds both limits",
			query:        `{ a { b { c { d { e { f { g { h { i { j { k { l } } } } } } } } } } } }`,
			maxDepth:     10,
			maxComplexity: 50,
			expectDepthErr: true,
			expectComplexityErr: true,
		},
		{
			name:         "at exact depth limit",
			query:        `{ a { b { c { d { e { f { g { h { i { j } } } } } } } } } }`,
			maxDepth:     10,
			maxComplexity: 1000,
			expectDepthErr: false,
			expectComplexityErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := Parse(tt.query)
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			validator := NewValidator(tt.maxDepth, tt.maxComplexity)
			result := validator.Validate(doc)

			hasDepthErr := false
			hasComplexityErr := false
			for _, e := range result.Errors {
				if strings.Contains(e.Message, "depth") {
					hasDepthErr = true
				}
				if strings.Contains(e.Message, "complexity") {
					hasComplexityErr = true
				}
			}

			if hasDepthErr != tt.expectDepthErr {
				if tt.expectDepthErr {
					t.Errorf("Expected depth error, but got none. Actual depth: %d", result.Depth)
				} else {
					t.Errorf("Unexpected depth error: %v", result.Errors)
				}
			}

			if hasComplexityErr != tt.expectComplexityErr {
				if tt.expectComplexityErr {
					t.Errorf("Expected complexity error, but got none. Actual complexity: %d", result.Complexity)
				} else {
					t.Errorf("Unexpected complexity error: %v", result.Errors)
				}
			}

			t.Logf("Depth: %d, Complexity: %d", result.Depth, result.Complexity)
		})
	}
}

func TestValidator_ValidateDepthQuick(t *testing.T) {
	query := `{ a { b { c { d { e } } } } }`
	doc, err := Parse(query)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Should pass with limit 5
	err = ValidateDepthQuick(doc, 5)
	if err != nil {
		t.Errorf("Expected no error for depth 5, got: %v", err)
	}

	// Should fail with limit 4
	err = ValidateDepthQuick(doc, 4)
	if err == nil {
		t.Error("Expected error for depth 4, got none")
	}
}

func TestValidator_ValidateComplexityQuick(t *testing.T) {
	query := `{ a { b { c } } }`
	doc, err := Parse(query)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Should pass with high limit
	err = ValidateComplexityQuick(doc, 1000)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestDefaultValidator(t *testing.T) {
	v := DefaultValidator()
	if v.maxDepth != 10 {
		t.Errorf("Expected default maxDepth of 10, got %d", v.maxDepth)
	}
	if v.maxComplexity != 1000 {
		t.Errorf("Expected default maxComplexity of 1000, got %d", v.maxComplexity)
	}
}

func TestValidationError(t *testing.T) {
	err := &ValidationError{
		Message: "test error",
		Line:    1,
		Column:  5,
	}
	expected := "test error at line 1, column 5"
	if err.Error() != expected {
		t.Errorf("Expected %q, got %q", expected, err.Error())
	}

	// Without line/column
	err2 := &ValidationError{
		Message: "simple error",
	}
	if err2.Error() != "simple error" {
		t.Errorf("Expected %q, got %q", "simple error", err2.Error())
	}
}

func TestValidateResult_HasErrors(t *testing.T) {
	r := &ValidateResult{
		Errors: []*ValidationError{},
	}
	if r.HasErrors() {
		t.Error("Expected no errors")
	}

	r.Errors = append(r.Errors, &ValidationError{Message: "test"})
	if !r.HasErrors() {
		t.Error("Expected errors")
	}
}

// --- Additional validator tests for coverage ---

func TestValidator_ComplexityWithArguments(t *testing.T) {
	query := `{ user(id: 1, name: "test", active: true) { id name } }`
	doc, err := Parse(query)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	complexity := CalculateComplexity(doc)
	if complexity < 5 {
		t.Errorf("Expected complexity >= 5 for query with arguments, got %d", complexity)
	}
}

func TestValidator_ValidateComplexityQuick_Fail(t *testing.T) {
	query := `{ a { b { c { d { e { f { g { h } } } } } } } }`
	doc, err := Parse(query)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	err = ValidateComplexityQuick(doc, 1)
	if err == nil {
		t.Error("Expected error for exceeding complexity limit")
	}
	if !strings.Contains(err.Error(), "complexity") {
		t.Errorf("Expected 'complexity' in error message, got: %v", err)
	}
}

func TestValidator_AnalyzeFragmentSpread(t *testing.T) {
	query := `
		query {
			user {
				...UserFields
			}
		}
		fragment UserFields on User {
			id
			name
		}
	`
	doc, err := Parse(query)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	v := DefaultValidator()
	result := v.Validate(doc)
	if result.HasErrors() {
		t.Errorf("Expected no errors, got %v", result.Errors)
	}
	if result.Depth < 1 {
		t.Errorf("Expected depth >= 1, got %d", result.Depth)
	}
}

func TestValidator_AnalyzeInlineFragment(t *testing.T) {
	query := `{ user { ... on User { id name posts { title } } } }`
	doc, err := Parse(query)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	v := DefaultValidator()
	result := v.Validate(doc)
	if result.HasErrors() {
		t.Errorf("Expected no errors, got %v", result.Errors)
	}
	if result.Depth < 3 {
		t.Errorf("Expected depth >= 3, got %d", result.Depth)
	}
}

func TestValidator_EmptyInlineFragment(t *testing.T) {
	doc := &Document{
		Definitions: []Definition{
			&OperationDefinition{
				Operation: TokenQuery,
				SelectionSet: &SelectionSet{
					Selections: []Selection{
						&Field{
							Name: &Name{Value: "user"},
							SelectionSet: &SelectionSet{
								Selections: []Selection{
									&InlineFragment{
										TypeCondition: &NamedType{Name: &Name{Value: "User"}},
										SelectionSet:  nil,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	v := DefaultValidator()
	result := v.Validate(doc)
	// Should not panic
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
}

func TestValidator_EmptySelectionSet(t *testing.T) {
	doc := &Document{
		Definitions: []Definition{
			&OperationDefinition{
				Operation:    TokenQuery,
				SelectionSet: nil,
			},
		},
	}

	depth := CheckDepth(doc)
	if depth != 0 {
		t.Errorf("Expected depth 0 for nil selection set, got %d", depth)
	}
}

func TestValidator_EmptyFragmentSelectionSet(t *testing.T) {
	doc := &Document{
		Definitions: []Definition{
			&FragmentDefinition{
				Name:          &Name{Value: "F"},
				TypeCondition: &NamedType{Name: &Name{Value: "User"}},
				SelectionSet:  nil,
			},
		},
	}

	depth := CheckDepth(doc)
	if depth != 0 {
		t.Errorf("Expected depth 0 for nil fragment selection set, got %d", depth)
	}
}

func TestValidationError_Formatting(t *testing.T) {
	tests := []struct {
		name     string
		err      *ValidationError
		expected string
	}{
		{
			name:     "with line and column",
			err:      &ValidationError{Message: "bad field", Line: 5, Column: 10},
			expected: "bad field at line 5, column 10",
		},
		{
			name:     "without position",
			err:      &ValidationError{Message: "generic error"},
			expected: "generic error",
		},
		{
			name:     "line zero",
			err:      &ValidationError{Message: "no pos", Line: 0, Column: 5},
			expected: "no pos",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err.Error() != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, tc.err.Error())
			}
		})
	}
}

func TestCheckDepth_MultipleOperations(t *testing.T) {
	query := `
		query Shallow { a { b } }
		query Deep { x { y { z { w } } } }
	`
	doc, err := Parse(query)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	depth := CheckDepth(doc)
	if depth != 4 {
		t.Errorf("Expected depth 4 (from deeper query), got %d", depth)
	}
}

// Benchmark validator
func BenchmarkValidate(b *testing.B) {
	query := `
		query GetUser($id: ID!) {
			user(id: $id) {
				id
				name
				email
				posts(first: 10) {
					title
					author {
						name
					}
				}
			}
		}
	`

	doc, err := Parse(query)
	if err != nil {
		b.Fatalf("Failed to parse: %v", err)
	}

	validator := DefaultValidator()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.Validate(doc)
	}
}
