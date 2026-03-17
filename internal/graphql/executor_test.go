package graphql

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestNewExecutor(t *testing.T) {
	exec := NewExecutor()
	if exec == nil {
		t.Fatal("NewExecutor returned nil")
	}
	if exec.resolvers == nil {
		t.Error("Expected resolvers map to be initialized")
	}
}

func TestExecutor_RegisterAndLookupResolver(t *testing.T) {
	exec := NewExecutor()

	// Register a resolver
	exec.RegisterResolver("Query", "user", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{"id": args["id"]}, nil
	})

	// Lookup the resolver
	resolver, ok := exec.LookupResolver("Query", "user")
	if !ok {
		t.Fatal("Expected to find resolver")
	}

	// Execute the resolver
	result, err := resolver(context.Background(), nil, map[string]interface{}{"id": "123"})
	if err != nil {
		t.Fatalf("Resolver failed: %v", err)
	}

	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map result, got %T", result)
	}
	if m["id"] != "123" {
		t.Errorf("Expected id='123', got %v", m["id"])
	}

	// Lookup non-existent
	_, ok = exec.LookupResolver("Query", "nonexistent")
	if ok {
		t.Error("Expected not to find non-existent resolver")
	}
}

func TestExecutor_Execute_SimpleQuery(t *testing.T) {
	exec := NewExecutor()

	// Register a resolver
	exec.RegisterResolver("Query", "hello", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		return "world", nil
	})

	// Parse a query
	parser := NewParserString(`query { hello }`)
	doc, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Execute
	result := exec.Execute(context.Background(), doc, nil)
	if result == nil {
		t.Fatal("Execute returned nil")
	}

	// Check result
	if len(result.Errors) > 0 {
		t.Errorf("Unexpected errors: %v", result.Errors)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map data, got %T", result.Data)
	}
	if data["hello"] != "world" {
		t.Errorf("Expected hello='world', got %v", data["hello"])
	}
}

func TestExecutor_Execute_MultipleFields(t *testing.T) {
	exec := NewExecutor()

	exec.RegisterResolver("Query", "user", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{
			"id":   "123",
			"name": "John",
		}, nil
	})

	exec.RegisterResolver("Query", "posts", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		return []interface{}{}, nil
	})

	parser := NewParserString(`query { user posts }`)
	doc, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	result := exec.Execute(context.Background(), doc, nil)
	if result == nil {
		t.Fatal("Execute returned nil")
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map data, got %T", result.Data)
	}

	if data["user"] == nil {
		t.Error("Expected user field")
	}
	if data["posts"] == nil {
		t.Error("Expected posts field")
	}
}

func TestExecutor_Execute_WithAlias(t *testing.T) {
	exec := NewExecutor()

	exec.RegisterResolver("Query", "user", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{"id": "123"}, nil
	})

	parser := NewParserString(`query { me: user }`)
	doc, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	result := exec.Execute(context.Background(), doc, nil)
	if result == nil {
		t.Fatal("Execute returned nil")
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map data, got %T", result.Data)
	}

	// Field should be under alias "me"
	if data["me"] == nil {
		t.Error("Expected 'me' field (aliased from 'user')")
	}
	if data["user"] != nil {
		t.Error("Should not have 'user' field when aliased to 'me'")
	}
}

func TestExecutor_Execute_WithArguments(t *testing.T) {
	exec := NewExecutor()

	exec.RegisterResolver("Query", "user", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		id, ok := args["id"]
		if !ok {
			return nil, errors.New("missing id argument")
		}
		return map[string]interface{}{"id": id}, nil
	})

	parser := NewParserString(`query { user(id: "123") }`)
	doc, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	result := exec.Execute(context.Background(), doc, nil)
	if result == nil {
		t.Fatal("Execute returned nil")
	}

	if len(result.Errors) > 0 {
		t.Errorf("Unexpected errors: %v", result.Errors)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map data, got %T", result.Data)
	}

	user, ok := data["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected user map, got %T", data["user"])
	}
	if user["id"] != "123" {
		t.Errorf("Expected id='123', got %v", user["id"])
	}
}

func TestExecutor_Execute_WithVariables(t *testing.T) {
	exec := NewExecutor()

	exec.RegisterResolver("Query", "user", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		id := args["id"]
		return map[string]interface{}{"id": id}, nil
	})

	parser := NewParserString(`query($userId: ID!) { user(id: $userId) }`)
	doc, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	variables := map[string]interface{}{"userId": "456"}
	result := exec.Execute(context.Background(), doc, variables)
	if result == nil {
		t.Fatal("Execute returned nil")
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map data, got %T", result.Data)
	}

	user, ok := data["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected user map, got %T", data["user"])
	}
	if user["id"] != "456" {
		t.Errorf("Expected id='456', got %v", user["id"])
	}
}

func TestExecutor_Execute_ErrorHandling(t *testing.T) {
	exec := NewExecutor()

	exec.RegisterResolver("Query", "user", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		return nil, errors.New("user not found")
	})

	exec.RegisterResolver("Query", "posts", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		return []string{"post1", "post2"}, nil
	})

	parser := NewParserString(`query { user posts }`)
	doc, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	result := exec.Execute(context.Background(), doc, nil)
	if result == nil {
		t.Fatal("Execute returned nil")
	}

	// Should have error for user
	if len(result.Errors) == 0 {
		t.Error("Expected errors")
	}

	foundUserError := false
	for _, e := range result.Errors {
		if e.Message == "user not found" {
			foundUserError = true
			break
		}
	}
	if !foundUserError {
		t.Errorf("Expected 'user not found' error, got: %v", result.Errors)
	}

	// posts should still be returned (partial response)
	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map data, got %T", result.Data)
	}
	if data["user"] != nil {
		t.Error("Expected user to be null due to error")
	}
	if data["posts"] == nil {
		t.Error("Expected posts to be returned despite user error")
	}
}

func TestExecutor_Execute_Mutation(t *testing.T) {
	exec := NewExecutor()

	exec.RegisterResolver("Mutation", "createUser", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		input, ok := args["input"].(map[string]interface{})
		if !ok {
			return nil, errors.New("invalid input")
		}
		return map[string]interface{}{
			"id":   "new-id",
			"name": input["name"],
		}, nil
	})

	parser := NewParserString(`mutation { createUser(input: { name: "Alice" }) { id name } }`)
	doc, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	result := exec.Execute(context.Background(), doc, nil)
	if result == nil {
		t.Fatal("Execute returned nil")
	}

	if len(result.Errors) > 0 {
		t.Errorf("Unexpected errors: %v", result.Errors)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map data, got %T", result.Data)
	}

	createUser, ok := data["createUser"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected createUser map, got %T", data["createUser"])
	}
	if createUser["name"] != "Alice" {
		t.Errorf("Expected name='Alice', got %v", createUser["name"])
	}
}

func TestExecutor_Execute_NestedFields(t *testing.T) {
	exec := NewExecutor()

	exec.RegisterResolver("Query", "user", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{
			"__typename": "User",
			"id":         "123",
			"name":       "John",
			"posts": []map[string]interface{}{
				{"id": "p1", "title": "First Post"},
				{"id": "p2", "title": "Second Post"},
			},
		}, nil
	})

	// Register nested resolver for User.posts
	exec.RegisterResolver("User", "posts", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		if m, ok := parent.(map[string]interface{}); ok {
			return m["posts"], nil
		}
		return nil, nil
	})

	parser := NewParserString(`query { user { id name } }`)
	doc, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	result := exec.Execute(context.Background(), doc, nil)
	if result == nil {
		t.Fatal("Execute returned nil")
	}

	if len(result.Errors) > 0 {
		t.Errorf("Unexpected errors: %v", result.Errors)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map data, got %T", result.Data)
	}

	user, ok := data["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected user map, got %T", data["user"])
	}
	if user["id"] != "123" {
		t.Errorf("Expected id='123', got %v", user["id"])
	}
	if user["name"] != "John" {
		t.Errorf("Expected name='John', got %v", user["name"])
	}
}

func TestExecutor_Execute_EmptyDocument(t *testing.T) {
	exec := NewExecutor()

	result := exec.Execute(context.Background(), nil, nil)
	if result == nil {
		t.Fatal("Execute returned nil")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected error for nil document")
	}
}

func TestExecutor_CoerceValue(t *testing.T) {
	exec := NewExecutor()
	execCtx := &ExecutionContext{
		Context:   context.Background(),
		Variables: map[string]interface{}{"var1": "value1"},
	}

	tests := []struct {
		name     string
		value    Value
		expected interface{}
	}{
		{"string", &StringValue{Value: "hello"}, "hello"},
		{"int", &IntValue{Value: "42"}, "42"},
		{"float", &FloatValue{Value: "3.14"}, "3.14"},
		{"bool", &BooleanValue{Value: true}, true},
		{"null", &NullValue{}, nil},
		{"enum", &EnumValue{Value: "ACTIVE"}, "ACTIVE"},
		{"list", &ListValue{Values: []Value{&StringValue{Value: "a"}, &StringValue{Value: "b"}}}, []interface{}{"a", "b"}},
		{"object", &ObjectValue{Fields: []*ObjectField{{Name: &Name{Value: "key"}, Value: &StringValue{Value: "val"}}}}, map[string]interface{}{"key": "val"}},
		{"variable", &VariableValue{Name: &Name{Value: "var1"}}, "value1"},
		{"undefined variable", &VariableValue{Name: &Name{Value: "undefined"}}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := exec.coerceValue(execCtx, tt.value)

			// Handle nil comparison first
			if got == nil && tt.expected == nil {
				return
			}
			if got == nil || tt.expected == nil {
				t.Errorf("coerceValue() = %v, want %v", got, tt.expected)
				return
			}

			// Handle slice comparison
			if gotSlice, ok1 := got.([]interface{}); ok1 {
				if expSlice, ok2 := tt.expected.([]interface{}); ok2 {
					if len(gotSlice) != len(expSlice) {
						t.Errorf("coerceValue() slice length = %d, want %d", len(gotSlice), len(expSlice))
					}
					return
				}
			}

			// Handle map comparison
			if gotMap, ok1 := got.(map[string]interface{}); ok1 {
				if expMap, ok2 := tt.expected.(map[string]interface{}); ok2 {
					if len(gotMap) != len(expMap) {
						t.Errorf("coerceValue() map length = %d, want %d", len(gotMap), len(expMap))
					}
					return
				}
			}

			// For other types, use standard comparison
			if got != tt.expected {
				t.Errorf("coerceValue() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestExecutionError_MarshalJSON(t *testing.T) {
	err := ExecutionError{
		Message: "test error",
		Path:    []string{"user", "name"},
		Extensions: map[string]interface{}{
			"code": "NOT_FOUND",
		},
	}

	data, marshalErr := err.MarshalJSON()
	if marshalErr != nil {
		t.Fatalf("MarshalJSON failed: %v", marshalErr)
	}

	// Parse and compare to avoid key ordering issues
	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	expected := map[string]interface{}{
		"message": "test error",
		"path":    []interface{}{"user", "name"},
		"extensions": map[string]interface{}{
			"code": "NOT_FOUND",
		},
	}

	if !jsonEqual(got, expected) {
		t.Errorf("JSON mismatch:\ngot:  %s\nwant: %s", string(data), mustJSON(expected))
	}
}

// jsonEqual compares two JSON-compatible values
func jsonEqual(a, b interface{}) bool {
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	// Unmarshal to normalized form and compare
	var aMap, bMap interface{}
	json.Unmarshal(aJSON, &aMap)
	json.Unmarshal(bJSON, &bMap)
	aJSON2, _ := json.Marshal(aMap)
	bJSON2, _ := json.Marshal(bMap)
	return string(aJSON2) == string(bJSON2)
}

// mustJSON returns JSON string or panics
func mustJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func TestIsNull(t *testing.T) {
	if !IsNull(nil) {
		t.Error("IsNull(nil) should be true")
	}
	if IsNull("test") {
		t.Error("IsNull(\"test\") should be false")
	}
	if IsNull(0) {
		t.Error("IsNull(0) should be false")
	}
}

func TestExecutor_Execute_FragmentSpread(t *testing.T) {
	exec := NewExecutor()

	exec.RegisterResolver("Query", "user", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{
			"__typename": "User",
			"id":         "123",
			"name":       "John",
			"email":      "john@example.com",
		}, nil
	})

	// Query using fragment spread
	query := `
		query {
			user {
				...UserFields
			}
		}
		fragment UserFields on User {
			id
			name
			email
		}
	`
	parser := NewParserString(query)
	doc, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	result := exec.Execute(context.Background(), doc, nil)
	if result == nil {
		t.Fatal("Execute returned nil")
	}

	if len(result.Errors) > 0 {
		t.Errorf("Unexpected errors: %v", result.Errors)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map data, got %T", result.Data)
	}

	user, ok := data["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected user map, got %T", data["user"])
	}

	if user["id"] != "123" {
		t.Errorf("Expected id='123', got %v", user["id"])
	}
	if user["name"] != "John" {
		t.Errorf("Expected name='John', got %v", user["name"])
	}
	if user["email"] != "john@example.com" {
		t.Errorf("Expected email='john@example.com', got %v", user["email"])
	}
}

func TestExecutor_Execute_InlineFragment(t *testing.T) {
	exec := NewExecutor()

	exec.RegisterResolver("Query", "user", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{
			"__typename": "User",
			"id":         "123",
			"name":       "John",
			"email":      "john@example.com",
		}, nil
	})

	// Query using inline fragment
	query := `
		query {
			user {
				id
				... on User {
					name
					email
				}
			}
		}
	`
	parser := NewParserString(query)
	doc, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	result := exec.Execute(context.Background(), doc, nil)
	if result == nil {
		t.Fatal("Execute returned nil")
	}

	if len(result.Errors) > 0 {
		t.Errorf("Unexpected errors: %v", result.Errors)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map data, got %T", result.Data)
	}

	user, ok := data["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected user map, got %T", data["user"])
	}

	if user["id"] != "123" {
		t.Errorf("Expected id='123', got %v", user["id"])
	}
	if user["name"] != "John" {
		t.Errorf("Expected name='John', got %v", user["name"])
	}
	if user["email"] != "john@example.com" {
		t.Errorf("Expected email='john@example.com', got %v", user["email"])
	}
}

func TestExecutor_Execute_InlineFragment_TypeConditionMismatch(t *testing.T) {
	exec := NewExecutor()

	// User has typename "User", but fragment is on "Admin"
	exec.RegisterResolver("Query", "user", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{
			"__typename": "User",
			"id":         "123",
			"name":       "John",
		}, nil
	})

	// Query using inline fragment with mismatched type
	query := `
		query {
			user {
				id
				... on Admin {
					name
				}
			}
		}
	`
	parser := NewParserString(query)
	doc, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	result := exec.Execute(context.Background(), doc, nil)
	if result == nil {
		t.Fatal("Execute returned nil")
	}

	if len(result.Errors) > 0 {
		t.Errorf("Unexpected errors: %v", result.Errors)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map data, got %T", result.Data)
	}

	user, ok := data["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected user map, got %T", data["user"])
	}

	// id should be present
	if user["id"] != "123" {
		t.Errorf("Expected id='123', got %v", user["id"])
	}

	// name should NOT be present because fragment type condition doesn't match
	if user["name"] != nil {
		t.Errorf("Expected name to be nil due to type mismatch, got %v", user["name"])
	}
}

func TestExecutor_Execute_UndefinedFragment(t *testing.T) {
	exec := NewExecutor()

	exec.RegisterResolver("Query", "user", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{"id": "123"}, nil
	})

	// Query using undefined fragment
	query := `
		query {
			user {
				...UndefinedFragment
			}
		}
	`
	parser := NewParserString(query)
	doc, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	result := exec.Execute(context.Background(), doc, nil)
	if result == nil {
		t.Fatal("Execute returned nil")
	}

	// Should have error for undefined fragment
	if len(result.Errors) == 0 {
		t.Error("Expected error for undefined fragment")
	}

	foundFragmentError := false
	for _, e := range result.Errors {
		if e.Message == "Fragment 'UndefinedFragment' is not defined" {
			foundFragmentError = true
			break
		}
	}
	if !foundFragmentError {
		t.Errorf("Expected 'Fragment UndefinedFragment is not defined' error, got: %v", result.Errors)
	}
}

func TestExecutor_Execute_MultipleFragmentSpreads(t *testing.T) {
	exec := NewExecutor()

	exec.RegisterResolver("Query", "user", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{
			"__typename": "User",
			"id":         "123",
			"name":       "John",
			"email":      "john@example.com",
			"phone":      "555-1234",
		}, nil
	})

	// Query using multiple fragment spreads
	query := `
		query {
			user {
				...BasicInfo
				...ContactInfo
			}
		}
		fragment BasicInfo on User {
			id
			name
		}
		fragment ContactInfo on User {
			email
			phone
		}
	`
	parser := NewParserString(query)
	doc, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	result := exec.Execute(context.Background(), doc, nil)
	if result == nil {
		t.Fatal("Execute returned nil")
	}

	if len(result.Errors) > 0 {
		t.Errorf("Unexpected errors: %v", result.Errors)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map data, got %T", result.Data)
	}

	user, ok := data["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected user map, got %T", data["user"])
	}

	// All fields from both fragments should be present
	if user["id"] != "123" {
		t.Errorf("Expected id='123', got %v", user["id"])
	}
	if user["name"] != "John" {
		t.Errorf("Expected name='John', got %v", user["name"])
	}
	if user["email"] != "john@example.com" {
		t.Errorf("Expected email='john@example.com', got %v", user["email"])
	}
	if user["phone"] != "555-1234" {
		t.Errorf("Expected phone='555-1234', got %v", user["phone"])
	}
}
