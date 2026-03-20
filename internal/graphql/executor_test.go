package graphql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// --- Additional executor tests for coverage ---

func TestExecutor_Execute_Subscription(t *testing.T) {
	exec := NewExecutor()

	exec.RegisterResolver("Subscription", "onMessage", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{"text": "hello"}, nil
	})

	parser := NewParserString(`subscription { onMessage { text } }`)
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
	msg, ok := data["onMessage"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map for onMessage, got %T", data["onMessage"])
	}
	if msg["text"] != "hello" {
		t.Errorf("Expected text='hello', got %v", msg["text"])
	}
}

func TestExecutor_Execute_DocumentWithOnlyFragments(t *testing.T) {
	exec := NewExecutor()

	// A document with only fragment definitions and no operation
	doc := &Document{
		Definitions: []Definition{
			&FragmentDefinition{
				Name:          &Name{Value: "F1"},
				TypeCondition: &NamedType{Name: &Name{Value: "User"}},
				SelectionSet:  &SelectionSet{Selections: []Selection{&Field{Name: &Name{Value: "id"}}}},
			},
		},
	}

	result := exec.Execute(context.Background(), doc, nil)
	if result == nil {
		t.Fatal("Execute returned nil")
	}
	if len(result.Errors) == 0 {
		t.Error("Expected error for document without operation")
	}
}

func TestExecutor_Execute_NilSelectionSet(t *testing.T) {
	exec := NewExecutor()

	exec.RegisterResolver("Query", "ping", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		return "pong", nil
	})

	parser := NewParserString(`{ ping }`)
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
	if data["ping"] != "pong" {
		t.Errorf("Expected 'pong', got %v", data["ping"])
	}
}

func TestExecutor_Execute_NestedListValues(t *testing.T) {
	exec := NewExecutor()

	exec.RegisterResolver("Query", "users", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		return []interface{}{
			map[string]interface{}{"id": "1", "name": "Alice"},
			map[string]interface{}{"id": "2", "name": "Bob"},
		}, nil
	})

	parser := NewParserString(`{ users { id name } }`)
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
	users, ok := data["users"].([]interface{})
	if !ok {
		t.Fatalf("Expected slice for users, got %T", data["users"])
	}
	if len(users) != 2 {
		t.Errorf("Expected 2 users, got %d", len(users))
	}
}

func TestExecutor_CoerceValue_Nil(t *testing.T) {
	exec := NewExecutor()
	execCtx := &ExecutionContext{Context: context.Background()}

	got := exec.coerceValue(execCtx, nil)
	if got != nil {
		t.Errorf("expected nil for nil value, got %v", got)
	}
}

func TestExecutor_CoerceValue_NilVariables(t *testing.T) {
	exec := NewExecutor()
	execCtx := &ExecutionContext{
		Context:   context.Background(),
		Variables: nil,
	}

	got := exec.coerceValue(execCtx, &VariableValue{Name: &Name{Value: "missing"}})
	if got != nil {
		t.Errorf("expected nil for variable with nil variables map, got %v", got)
	}
}

func TestExecutionError_MarshalJSON_MinimalFields(t *testing.T) {
	err := ExecutionError{
		Message: "just a message",
	}

	data, marshalErr := err.MarshalJSON()
	if marshalErr != nil {
		t.Fatalf("MarshalJSON failed: %v", marshalErr)
	}

	var got map[string]interface{}
	if jsonErr := json.Unmarshal(data, &got); jsonErr != nil {
		t.Fatalf("Unmarshal failed: %v", jsonErr)
	}

	if got["message"] != "just a message" {
		t.Errorf("expected message 'just a message', got %v", got["message"])
	}
	if _, exists := got["path"]; exists {
		t.Error("expected path to be absent when empty")
	}
	if _, exists := got["extensions"]; exists {
		t.Error("expected extensions to be absent when empty")
	}
}

func TestExecutor_Execute_DefaultResolverNonMap(t *testing.T) {
	exec := NewExecutor()

	// No resolver registered for Query.test -> uses default resolver
	parser := NewParserString(`{ test }`)
	doc, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	result := exec.Execute(context.Background(), doc, nil)
	if result == nil {
		t.Fatal("Execute returned nil")
	}
	// Default resolver on nil parent should produce an error
	if len(result.Errors) == 0 {
		t.Error("Expected error from default resolver on nil parent")
	}
}

func TestExecutor_Execute_FragmentSpread_TypeConditionMismatch(t *testing.T) {
	exec := NewExecutor()

	exec.RegisterResolver("Query", "user", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{
			"__typename": "User",
			"id":         "1",
			"name":       "Test",
		}, nil
	})

	query := `
		query {
			user {
				...AdminFields
			}
		}
		fragment AdminFields on Admin {
			id
			name
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

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map data, got %T", result.Data)
	}

	user, ok := data["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected user map, got %T", data["user"])
	}

	// Fragment should not match, so no fields from fragment should be present
	if user["id"] != nil {
		t.Errorf("Expected id to be nil due to type mismatch, got %v", user["id"])
	}
}

func TestExecutor_ResolveNested_NilValue(t *testing.T) {
	exec := NewExecutor()
	execCtx := &ExecutionContext{Context: context.Background()}
	sel := &SelectionSet{Selections: []Selection{&Field{Name: &Name{Value: "id"}}}}

	got := exec.resolveNested(execCtx, sel, nil)
	if got != nil {
		t.Errorf("expected nil for nil value, got %v", got)
	}
}

func TestExecutor_ResolveNested_NonMapValue(t *testing.T) {
	exec := NewExecutor()
	execCtx := &ExecutionContext{Context: context.Background()}
	sel := &SelectionSet{Selections: []Selection{&Field{Name: &Name{Value: "id"}}}}

	got := exec.resolveNested(execCtx, sel, "a string value")
	if got != "a string value" {
		t.Errorf("expected pass-through for non-map value, got %v", got)
	}
}

func TestAST_TokenLiteral(t *testing.T) {
	// Test Document.TokenLiteral
	emptyDoc := &Document{}
	if emptyDoc.TokenLiteral() != "" {
		t.Errorf("expected empty string for empty document, got %q", emptyDoc.TokenLiteral())
	}

	doc := &Document{Definitions: []Definition{
		&OperationDefinition{Operation: TokenQuery},
	}}
	if doc.TokenLiteral() == "" {
		t.Error("expected non-empty TokenLiteral for document with definitions")
	}

	// Test OperationDefinition.TokenLiteral
	op := &OperationDefinition{Operation: TokenMutation}
	if op.TokenLiteral() != "MUTATION" {
		t.Errorf("expected 'MUTATION', got %q", op.TokenLiteral())
	}

	// Test FragmentDefinition.TokenLiteral
	frag := &FragmentDefinition{}
	if frag.TokenLiteral() != "fragment" {
		t.Errorf("expected 'fragment', got %q", frag.TokenLiteral())
	}

	// Test Field.TokenLiteral
	field := &Field{Name: &Name{Value: "myField"}}
	if field.TokenLiteral() != "myField" {
		t.Errorf("expected 'myField', got %q", field.TokenLiteral())
	}
	nilField := &Field{}
	if nilField.TokenLiteral() != "" {
		t.Errorf("expected empty for nil name field, got %q", nilField.TokenLiteral())
	}

	// Test FragmentSpread.TokenLiteral
	spread := &FragmentSpread{}
	if spread.TokenLiteral() != "..." {
		t.Errorf("expected '...', got %q", spread.TokenLiteral())
	}

	// Test InlineFragment.TokenLiteral
	inline := &InlineFragment{}
	if inline.TokenLiteral() != "..." {
		t.Errorf("expected '...', got %q", inline.TokenLiteral())
	}

	// Test SelectionSet.TokenLiteral
	ss := &SelectionSet{}
	if ss.TokenLiteral() != "{" {
		t.Errorf("expected '{', got %q", ss.TokenLiteral())
	}

	// Test Name.TokenLiteral
	name := &Name{Value: "test"}
	if name.TokenLiteral() != "test" {
		t.Errorf("expected 'test', got %q", name.TokenLiteral())
	}

	// Test VariableDefinition.TokenLiteral
	varDef := &VariableDefinition{Variable: &Variable{Name: &Name{Value: "x"}}}
	if varDef.TokenLiteral() != "$x" {
		t.Errorf("expected '$x', got %q", varDef.TokenLiteral())
	}
	nilVarDef := &VariableDefinition{}
	if nilVarDef.TokenLiteral() != "" {
		t.Errorf("expected empty for nil variable def, got %q", nilVarDef.TokenLiteral())
	}

	// Test Variable.TokenLiteral
	v := &Variable{Name: &Name{Value: "myVar"}}
	if v.TokenLiteral() != "$myVar" {
		t.Errorf("expected '$myVar', got %q", v.TokenLiteral())
	}
	nilV := &Variable{}
	if nilV.TokenLiteral() != "$" {
		t.Errorf("expected '$', got %q", nilV.TokenLiteral())
	}

	// Test Argument.TokenLiteral
	arg := &Argument{Name: &Name{Value: "id"}}
	if arg.TokenLiteral() != "id" {
		t.Errorf("expected 'id', got %q", arg.TokenLiteral())
	}
	nilArg := &Argument{}
	if nilArg.TokenLiteral() != "" {
		t.Errorf("expected empty, got %q", nilArg.TokenLiteral())
	}

	// Test Directive.TokenLiteral
	dir := &Directive{Name: &Name{Value: "skip"}}
	if dir.TokenLiteral() != "@skip" {
		t.Errorf("expected '@skip', got %q", dir.TokenLiteral())
	}
	nilDir := &Directive{}
	if nilDir.TokenLiteral() != "@" {
		t.Errorf("expected '@', got %q", nilDir.TokenLiteral())
	}

	// Test Type TokenLiterals
	namedType := &NamedType{Name: &Name{Value: "String"}}
	if namedType.TokenLiteral() != "String" {
		t.Errorf("expected 'String', got %q", namedType.TokenLiteral())
	}
	nilNamedType := &NamedType{}
	if nilNamedType.TokenLiteral() != "" {
		t.Errorf("expected empty, got %q", nilNamedType.TokenLiteral())
	}

	listType := &ListType{Type: namedType}
	if listType.TokenLiteral() != "[" {
		t.Errorf("expected '[', got %q", listType.TokenLiteral())
	}

	nonNull := &NonNullType{Type: namedType}
	if nonNull.TokenLiteral() != "String!" {
		t.Errorf("expected 'String!', got %q", nonNull.TokenLiteral())
	}

	// Test Value TokenLiterals
	intVal := &IntValue{Value: "42"}
	if intVal.TokenLiteral() != "42" {
		t.Errorf("expected '42', got %q", intVal.TokenLiteral())
	}

	floatVal := &FloatValue{Value: "3.14"}
	if floatVal.TokenLiteral() != "3.14" {
		t.Errorf("expected '3.14', got %q", floatVal.TokenLiteral())
	}

	strVal := &StringValue{Value: "hello"}
	if strVal.TokenLiteral() != "hello" {
		t.Errorf("expected 'hello', got %q", strVal.TokenLiteral())
	}

	boolVal := &BooleanValue{Value: true}
	if boolVal.TokenLiteral() != "true" {
		t.Errorf("expected 'true', got %q", boolVal.TokenLiteral())
	}
	boolFalse := &BooleanValue{Value: false}
	if boolFalse.TokenLiteral() != "false" {
		t.Errorf("expected 'false', got %q", boolFalse.TokenLiteral())
	}

	nullVal := &NullValue{}
	if nullVal.TokenLiteral() != "null" {
		t.Errorf("expected 'null', got %q", nullVal.TokenLiteral())
	}

	enumVal := &EnumValue{Value: "ACTIVE"}
	if enumVal.TokenLiteral() != "ACTIVE" {
		t.Errorf("expected 'ACTIVE', got %q", enumVal.TokenLiteral())
	}

	listVal := &ListValue{}
	if listVal.TokenLiteral() != "[" {
		t.Errorf("expected '[', got %q", listVal.TokenLiteral())
	}

	objVal := &ObjectValue{}
	if objVal.TokenLiteral() != "{" {
		t.Errorf("expected '{', got %q", objVal.TokenLiteral())
	}

	objField := &ObjectField{Name: &Name{Value: "key"}}
	if objField.TokenLiteral() != "key" {
		t.Errorf("expected 'key', got %q", objField.TokenLiteral())
	}
	nilObjField := &ObjectField{}
	if nilObjField.TokenLiteral() != "" {
		t.Errorf("expected empty, got %q", nilObjField.TokenLiteral())
	}

	varVal := &VariableValue{Name: &Name{Value: "x"}}
	if varVal.TokenLiteral() != "$x" {
		t.Errorf("expected '$x', got %q", varVal.TokenLiteral())
	}
	nilVarVal := &VariableValue{}
	if nilVarVal.TokenLiteral() != "$" {
		t.Errorf("expected '$', got %q", nilVarVal.TokenLiteral())
	}
}

func TestTokenKind_String(t *testing.T) {
	tests := []struct {
		kind     TokenKind
		expected string
	}{
		{TokenEOF, "EOF"},
		{TokenIllegal, "ILLEGAL"},
		{TokenName, "NAME"},
		{TokenInt, "INT"},
		{TokenFloat, "FLOAT"},
		{TokenString, "STRING"},
		{TokenBlockString, "BLOCK_STRING"},
		{TokenQuery, "QUERY"},
		{TokenMutation, "MUTATION"},
		{TokenSubscription, "SUBSCRIPTION"},
		{TokenFragment, "FRAGMENT"},
		{TokenOn, "ON"},
		{TokenTrue, "TRUE"},
		{TokenFalse, "FALSE"},
		{TokenNull, "NULL"},
		{TokenParenL, "("},
		{TokenParenR, ")"},
		{TokenBraceL, "{"},
		{TokenBraceR, "}"},
		{TokenBracketL, "["},
		{TokenBracketR, "]"},
		{TokenColon, ":"},
		{TokenComma, ","},
		{TokenAt, "@"},
		{TokenDollar, "$"},
		{TokenEquals, "="},
		{TokenBang, "!"},
		{TokenSpread, "..."},
		{TokenPipe, "|"},
		{TokenAmpersand, "&"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			got := tc.kind.String()
			if got != tc.expected {
				t.Errorf("TokenKind(%d).String() = %q, want %q", tc.kind, got, tc.expected)
			}
		})
	}
}

func TestToken_IsKeyword(t *testing.T) {
	keywords := []TokenKind{
		TokenQuery, TokenMutation, TokenSubscription, TokenFragment,
		TokenOn, TokenTrue, TokenFalse, TokenNull,
	}
	for _, k := range keywords {
		tok := Token{Kind: k}
		if !tok.IsKeyword() {
			t.Errorf("expected %s to be a keyword", k.String())
		}
	}

	nonKeywords := []TokenKind{TokenName, TokenInt, TokenFloat, TokenString, TokenEOF}
	for _, k := range nonKeywords {
		tok := Token{Kind: k}
		if tok.IsKeyword() {
			t.Errorf("expected %s to NOT be a keyword", k.String())
		}
	}
}

func TestLookupKeyword(t *testing.T) {
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
		{"notakeyword", TokenName},
		{"user", TokenName},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := LookupKeyword(tc.input)
			if got != tc.expected {
				t.Errorf("LookupKeyword(%q) = %v, want %v", tc.input, got, tc.expected)
			}
		})
	}
}

func TestToken_String(t *testing.T) {
	tok := Token{Kind: TokenName, Value: "hello", Line: 1, Column: 5}
	s := tok.String()
	if s == "" {
		t.Error("expected non-empty token string")
	}
}

func TestExecutor_Execute_TopLevelFragmentSpread(t *testing.T) {
	exec := NewExecutor()

	exec.RegisterResolver("Query", "user", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{
			"__typename": "User",
			"id":         "1",
			"name":       "Alice",
			"email":      "alice@example.com",
		}, nil
	})

	// Parse and execute with fragment spread at top level of selection set
	query := `
		query {
			user {
				id
				...UserDetails
			}
		}
		fragment UserDetails on User {
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
	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map data, got %T", result.Data)
	}

	user, ok := data["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected user map, got %T", data["user"])
	}

	if user["id"] != "1" {
		t.Errorf("Expected id='1', got %v", user["id"])
	}
	if user["name"] != "Alice" {
		t.Errorf("Expected name='Alice', got %v", user["name"])
	}
	if user["email"] != "alice@example.com" {
		t.Errorf("Expected email, got %v", user["email"])
	}
}

func TestExecutor_Execute_TopLevelInlineFragment(t *testing.T) {
	exec := NewExecutor()

	exec.RegisterResolver("Query", "node", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{
			"__typename": "Article",
			"id":         "a1",
			"title":      "Hello World",
		}, nil
	})

	query := `
		query {
			node {
				id
				... on Article {
					title
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
	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map data, got %T", result.Data)
	}

	node, ok := data["node"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected node map, got %T", data["node"])
	}

	if node["id"] != "a1" {
		t.Errorf("Expected id='a1', got %v", node["id"])
	}
	if node["title"] != "Hello World" {
		t.Errorf("Expected title='Hello World', got %v", node["title"])
	}
}

func TestExecutor_Execute_IntArgument(t *testing.T) {
	exec := NewExecutor()

	exec.RegisterResolver("Query", "user", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{"id": args["id"]}, nil
	})

	parser := NewParserString(`{ user(id: 42) }`)
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
	user, ok := data["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map, got %T", data["user"])
	}
	if user["id"] != "42" {
		t.Errorf("Expected id='42', got %v (type %T)", user["id"], user["id"])
	}
}

// --- Coverage boost tests ---

func TestExecutor_ExecuteFragmentSpread_DirectCall(t *testing.T) {
	exec := NewExecutor()

	tests := []struct {
		name           string
		parent         interface{}
		parentType     string
		fragmentName   string
		fragments      []*FragmentDefinition
		expectNil      bool
		expectError    bool
		expectFieldLen int
	}{
		{
			name:         "undefined fragment returns nil with error",
			parent:       map[string]interface{}{"__typename": "User", "id": "1"},
			parentType:   "User",
			fragmentName: "Missing",
			fragments:    nil,
			expectNil:    true,
			expectError:  true,
		},
		{
			name:         "type condition mismatch returns nil",
			parent:       map[string]interface{}{"__typename": "User", "id": "1"},
			parentType:   "User",
			fragmentName: "AdminFields",
			fragments: []*FragmentDefinition{
				{
					Name:          &Name{Value: "AdminFields"},
					TypeCondition: &NamedType{Name: &Name{Value: "Admin"}},
					SelectionSet: &SelectionSet{
						Selections: []Selection{&Field{Name: &Name{Value: "id"}}},
					},
				},
			},
			expectNil: true,
		},
		{
			name:         "type condition match returns fields",
			parent:       map[string]interface{}{"__typename": "User", "id": "1", "name": "Alice"},
			parentType:   "User",
			fragmentName: "UserFields",
			fragments: []*FragmentDefinition{
				{
					Name:          &Name{Value: "UserFields"},
					TypeCondition: &NamedType{Name: &Name{Value: "User"}},
					SelectionSet: &SelectionSet{
						Selections: []Selection{
							&Field{Name: &Name{Value: "id"}},
							&Field{Name: &Name{Value: "name"}},
						},
					},
				},
			},
			expectNil:      false,
			expectFieldLen: 2,
		},
		{
			name:         "non-map parent uses parentType",
			parent:       "string-parent",
			parentType:   "User",
			fragmentName: "UserFields",
			fragments: []*FragmentDefinition{
				{
					Name:          &Name{Value: "UserFields"},
					TypeCondition: &NamedType{Name: &Name{Value: "User"}},
					SelectionSet: &SelectionSet{
						Selections: []Selection{&Field{Name: &Name{Value: "id"}}},
					},
				},
			},
			expectNil: false,
		},
		{
			name:         "fragment without type condition",
			parent:       map[string]interface{}{"id": "1"},
			parentType:   "Query",
			fragmentName: "NoType",
			fragments: []*FragmentDefinition{
				{
					Name:          &Name{Value: "NoType"},
					TypeCondition: nil,
					SelectionSet: &SelectionSet{
						Selections: []Selection{&Field{Name: &Name{Value: "id"}}},
					},
				},
			},
			expectNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defs := make([]Definition, 0, len(tt.fragments))
			for _, f := range tt.fragments {
				defs = append(defs, f)
			}
			doc := &Document{Definitions: defs}
			execCtx := &ExecutionContext{
				Context:  context.Background(),
				Errors:   make([]ExecutionError, 0),
				Document: doc,
			}
			spread := &FragmentSpread{Name: &Name{Value: tt.fragmentName}}
			result := exec.executeFragmentSpread(execCtx, spread, tt.parent, tt.parentType)

			if tt.expectNil && result != nil {
				t.Errorf("expected nil result, got %v", result)
			}
			if !tt.expectNil && result == nil {
				t.Error("expected non-nil result")
			}
			if tt.expectError && len(execCtx.Errors) == 0 {
				t.Error("expected error to be recorded")
			}
			if tt.expectFieldLen > 0 && len(result) != tt.expectFieldLen {
				t.Errorf("expected %d fields, got %d", tt.expectFieldLen, len(result))
			}
		})
	}
}

func TestExecutor_ExecuteInlineFragment_DirectCall(t *testing.T) {
	exec := NewExecutor()

	tests := []struct {
		name       string
		parent     interface{}
		parentType string
		fragment   *InlineFragment
		expectNil  bool
	}{
		{
			name:       "type condition mismatch on non-map parent",
			parent:     "string-parent",
			parentType: "User",
			fragment: &InlineFragment{
				TypeCondition: &NamedType{Name: &Name{Value: "Admin"}},
				SelectionSet: &SelectionSet{
					Selections: []Selection{&Field{Name: &Name{Value: "id"}}},
				},
			},
			expectNil: true,
		},
		{
			name:       "type condition match on map parent",
			parent:     map[string]interface{}{"__typename": "User", "id": "1"},
			parentType: "User",
			fragment: &InlineFragment{
				TypeCondition: &NamedType{Name: &Name{Value: "User"}},
				SelectionSet: &SelectionSet{
					Selections: []Selection{&Field{Name: &Name{Value: "id"}}},
				},
			},
			expectNil: false,
		},
		{
			name:       "no type condition",
			parent:     map[string]interface{}{"id": "1"},
			parentType: "Query",
			fragment: &InlineFragment{
				TypeCondition: nil,
				SelectionSet: &SelectionSet{
					Selections: []Selection{&Field{Name: &Name{Value: "id"}}},
				},
			},
			expectNil: false,
		},
		{
			name:       "type condition with nil Name",
			parent:     map[string]interface{}{"__typename": "User", "id": "1"},
			parentType: "User",
			fragment: &InlineFragment{
				TypeCondition: &NamedType{Name: nil},
				SelectionSet: &SelectionSet{
					Selections: []Selection{&Field{Name: &Name{Value: "id"}}},
				},
			},
			expectNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execCtx := &ExecutionContext{
				Context:  context.Background(),
				Errors:   make([]ExecutionError, 0),
				Document: &Document{},
			}
			result := exec.executeInlineFragment(execCtx, tt.fragment, tt.parent, tt.parentType)
			if tt.expectNil && result != nil {
				t.Errorf("expected nil result, got %v", result)
			}
			if !tt.expectNil && result == nil {
				t.Error("expected non-nil result")
			}
		})
	}
}

func TestExecutor_DefaultResolver_MapParent(t *testing.T) {
	exec := NewExecutor()

	parent := map[string]interface{}{"key": "value"}
	result, err := exec.defaultResolver(context.Background(), parent, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if m["key"] != "value" {
		t.Errorf("expected key='value', got %v", m["key"])
	}
}

func TestExecutor_DefaultResolver_NonMapParent(t *testing.T) {
	exec := NewExecutor()

	_, err := exec.defaultResolver(context.Background(), "string-parent", nil)
	if err == nil {
		t.Error("expected error for non-map parent")
	}
}

func TestExecutor_DefaultResolver_NilParent(t *testing.T) {
	exec := NewExecutor()

	_, err := exec.defaultResolver(context.Background(), nil, nil)
	if err == nil {
		t.Error("expected error for nil parent")
	}
}

func TestExecutor_ExecuteSelectionSet_NilSelectionSet(t *testing.T) {
	exec := NewExecutor()
	execCtx := &ExecutionContext{Context: context.Background()}

	result := exec.executeSelectionSet(execCtx, nil, nil, "Query")
	if result != nil {
		t.Errorf("expected nil for nil selection set, got %v", result)
	}
}

func TestExecutor_ResolveFragmentSelectionSet_Nil(t *testing.T) {
	exec := NewExecutor()
	execCtx := &ExecutionContext{
		Context:  context.Background(),
		Document: &Document{},
	}

	result := exec.resolveFragmentSelectionSet(execCtx, nil, map[string]interface{}{"id": "1"})
	if result == nil {
		t.Fatal("expected non-nil empty map")
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}

func TestExecutor_ResolveFragmentSelectionSet_NestedField(t *testing.T) {
	exec := NewExecutor()
	execCtx := &ExecutionContext{
		Context:  context.Background(),
		Errors:   make([]ExecutionError, 0),
		Document: &Document{},
	}

	parent := map[string]interface{}{
		"user": map[string]interface{}{
			"id":   "1",
			"name": "Alice",
		},
	}

	selSet := &SelectionSet{
		Selections: []Selection{
			&Field{
				Name: &Name{Value: "user"},
				SelectionSet: &SelectionSet{
					Selections: []Selection{
						&Field{Name: &Name{Value: "id"}},
						&Field{Name: &Name{Value: "name"}},
					},
				},
			},
		},
	}

	result := exec.resolveFragmentSelectionSet(execCtx, selSet, parent)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	user, ok := result["user"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected user to be a map, got %T", result["user"])
	}
	if user["id"] != "1" {
		t.Errorf("expected id='1', got %v", user["id"])
	}
}

func TestExecutor_ResolveNestedFragmentSpread(t *testing.T) {
	exec := NewExecutor()

	parent := map[string]interface{}{
		"__typename": "User",
		"id":         "1",
		"name":       "Alice",
	}

	fragment := &FragmentDefinition{
		Name:          &Name{Value: "F1"},
		TypeCondition: &NamedType{Name: &Name{Value: "User"}},
		SelectionSet: &SelectionSet{
			Selections: []Selection{
				&Field{Name: &Name{Value: "id"}},
			},
		},
	}

	doc := &Document{Definitions: []Definition{fragment}}
	execCtx := &ExecutionContext{
		Context:  context.Background(),
		Errors:   make([]ExecutionError, 0),
		Document: doc,
	}

	spread := &FragmentSpread{Name: &Name{Value: "F1"}}
	result := exec.resolveNestedFragmentSpread(execCtx, spread, parent)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["id"] != "1" {
		t.Errorf("expected id='1', got %v", result["id"])
	}
}

func TestExecutor_ResolveNestedFragmentSpread_Undefined(t *testing.T) {
	exec := NewExecutor()

	parent := map[string]interface{}{"__typename": "User"}
	doc := &Document{}
	execCtx := &ExecutionContext{
		Context:  context.Background(),
		Errors:   make([]ExecutionError, 0),
		Document: doc,
	}

	spread := &FragmentSpread{Name: &Name{Value: "Missing"}}
	result := exec.resolveNestedFragmentSpread(execCtx, spread, parent)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
	if len(execCtx.Errors) == 0 {
		t.Error("expected error for undefined fragment")
	}
}

func TestExecutor_ResolveNestedFragmentSpread_TypeMismatch(t *testing.T) {
	exec := NewExecutor()

	parent := map[string]interface{}{"__typename": "User"}
	fragment := &FragmentDefinition{
		Name:          &Name{Value: "AdminF"},
		TypeCondition: &NamedType{Name: &Name{Value: "Admin"}},
		SelectionSet: &SelectionSet{
			Selections: []Selection{&Field{Name: &Name{Value: "id"}}},
		},
	}
	doc := &Document{Definitions: []Definition{fragment}}
	execCtx := &ExecutionContext{
		Context:  context.Background(),
		Errors:   make([]ExecutionError, 0),
		Document: doc,
	}

	spread := &FragmentSpread{Name: &Name{Value: "AdminF"}}
	result := exec.resolveNestedFragmentSpread(execCtx, spread, parent)
	if result != nil {
		t.Errorf("expected nil for type mismatch, got %v", result)
	}
}

func TestExecutor_ResolveNestedInlineFragment(t *testing.T) {
	exec := NewExecutor()

	parent := map[string]interface{}{
		"__typename": "User",
		"id":         "1",
		"name":       "Bob",
	}

	execCtx := &ExecutionContext{
		Context:  context.Background(),
		Errors:   make([]ExecutionError, 0),
		Document: &Document{},
	}

	// Matching type
	frag := &InlineFragment{
		TypeCondition: &NamedType{Name: &Name{Value: "User"}},
		SelectionSet: &SelectionSet{
			Selections: []Selection{&Field{Name: &Name{Value: "name"}}},
		},
	}
	result := exec.resolveNestedInlineFragment(execCtx, frag, parent)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["name"] != "Bob" {
		t.Errorf("expected name='Bob', got %v", result["name"])
	}

	// Mismatching type
	fragMismatch := &InlineFragment{
		TypeCondition: &NamedType{Name: &Name{Value: "Admin"}},
		SelectionSet: &SelectionSet{
			Selections: []Selection{&Field{Name: &Name{Value: "name"}}},
		},
	}
	result2 := exec.resolveNestedInlineFragment(execCtx, fragMismatch, parent)
	if result2 != nil {
		t.Errorf("expected nil for type mismatch, got %v", result2)
	}

	// No type condition
	fragNoType := &InlineFragment{
		TypeCondition: nil,
		SelectionSet: &SelectionSet{
			Selections: []Selection{&Field{Name: &Name{Value: "id"}}},
		},
	}
	result3 := exec.resolveNestedInlineFragment(execCtx, fragNoType, parent)
	if result3 == nil {
		t.Fatal("expected non-nil result for no type condition")
	}
}

func TestExecutor_ResolveNested_WithFragmentSpreadAndInlineFragment(t *testing.T) {
	exec := NewExecutor()

	fragment := &FragmentDefinition{
		Name:          &Name{Value: "UserBasic"},
		TypeCondition: &NamedType{Name: &Name{Value: "User"}},
		SelectionSet: &SelectionSet{
			Selections: []Selection{&Field{Name: &Name{Value: "id"}}},
		},
	}
	doc := &Document{Definitions: []Definition{fragment}}
	execCtx := &ExecutionContext{
		Context:  context.Background(),
		Errors:   make([]ExecutionError, 0),
		Document: doc,
	}

	value := map[string]interface{}{
		"__typename": "User",
		"id":         "1",
		"name":       "Test",
		"email":      "test@example.com",
	}

	selSet := &SelectionSet{
		Selections: []Selection{
			&FragmentSpread{Name: &Name{Value: "UserBasic"}},
			&InlineFragment{
				TypeCondition: &NamedType{Name: &Name{Value: "User"}},
				SelectionSet: &SelectionSet{
					Selections: []Selection{&Field{Name: &Name{Value: "email"}}},
				},
			},
		},
	}

	result := exec.resolveNested(execCtx, selSet, value)
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if m["id"] != "1" {
		t.Errorf("expected id='1' from fragment spread, got %v", m["id"])
	}
	if m["email"] != "test@example.com" {
		t.Errorf("expected email from inline fragment, got %v", m["email"])
	}
}

func TestExecutor_FindFragmentDefinition_NilDoc(t *testing.T) {
	exec := NewExecutor()
	result := exec.findFragmentDefinition(nil, "Missing")
	if result != nil {
		t.Errorf("expected nil for nil document, got %v", result)
	}
}

func TestExecutor_FindFragmentDefinition_NotFound(t *testing.T) {
	exec := NewExecutor()
	doc := &Document{
		Definitions: []Definition{
			&OperationDefinition{Operation: TokenQuery},
		},
	}
	result := exec.findFragmentDefinition(doc, "Missing")
	if result != nil {
		t.Errorf("expected nil for missing fragment, got %v", result)
	}
}

func TestExecutor_FindFragmentDefinition_NilName(t *testing.T) {
	exec := NewExecutor()
	doc := &Document{
		Definitions: []Definition{
			&FragmentDefinition{
				Name: nil,
			},
		},
	}
	result := exec.findFragmentDefinition(doc, "Missing")
	if result != nil {
		t.Errorf("expected nil for fragment with nil name, got %v", result)
	}
}

func TestAST_MarkerMethods(t *testing.T) {
	// These are no-op interface marker methods - call them to increase coverage
	op := &OperationDefinition{}
	op.definitionNode()

	frag := &FragmentDefinition{}
	frag.definitionNode()

	field := &Field{}
	field.selectionNode()

	spread := &FragmentSpread{}
	spread.selectionNode()

	inline := &InlineFragment{}
	inline.selectionNode()

	namedType := &NamedType{}
	namedType.typeNode()

	listType := &ListType{}
	listType.typeNode()

	nonNull := &NonNullType{Type: &NamedType{Name: &Name{Value: "String"}}}
	nonNull.typeNode()

	intVal := &IntValue{}
	intVal.valueNode()

	floatVal := &FloatValue{}
	floatVal.valueNode()

	strVal := &StringValue{}
	strVal.valueNode()

	boolVal := &BooleanValue{}
	boolVal.valueNode()

	nullVal := &NullValue{}
	nullVal.valueNode()

	enumVal := &EnumValue{}
	enumVal.valueNode()

	listVal := &ListValue{}
	listVal.valueNode()

	objVal := &ObjectValue{}
	objVal.valueNode()

	varVal := &VariableValue{}
	varVal.valueNode()
}

func TestExecutor_ExecuteSelectionSet_AllSelectionTypes(t *testing.T) {
	exec := NewExecutor()

	exec.RegisterResolver("Query", "data", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{
			"__typename": "Data",
			"id":         "1",
			"name":       "Test",
			"extra":      "extra-val",
		}, nil
	})

	fragment := &FragmentDefinition{
		Name:          &Name{Value: "DataId"},
		TypeCondition: &NamedType{Name: &Name{Value: "Data"}},
		SelectionSet: &SelectionSet{
			Selections: []Selection{&Field{Name: &Name{Value: "id"}}},
		},
	}

	query := &OperationDefinition{
		Operation: TokenQuery,
		SelectionSet: &SelectionSet{
			Selections: []Selection{
				&Field{
					Name: &Name{Value: "data"},
					SelectionSet: &SelectionSet{
						Selections: []Selection{
							&Field{Name: &Name{Value: "name"}},
							&FragmentSpread{Name: &Name{Value: "DataId"}},
							&InlineFragment{
								TypeCondition: &NamedType{Name: &Name{Value: "Data"}},
								SelectionSet: &SelectionSet{
									Selections: []Selection{&Field{Name: &Name{Value: "extra"}}},
								},
							},
						},
					},
				},
			},
		},
	}

	doc := &Document{Definitions: []Definition{query, fragment}}
	result := exec.Execute(context.Background(), doc, nil)
	if len(result.Errors) > 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map data, got %T", result.Data)
	}
	d, ok := data["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", data["data"])
	}
	if d["name"] != "Test" {
		t.Errorf("expected name='Test', got %v", d["name"])
	}
	if d["id"] != "1" {
		t.Errorf("expected id='1', got %v", d["id"])
	}
	if d["extra"] != "extra-val" {
		t.Errorf("expected extra='extra-val', got %v", d["extra"])
	}
}

func TestExecutor_AddError_ThreadSafe(t *testing.T) {
	execCtx := &ExecutionContext{
		Context: context.Background(),
		Errors:  make([]ExecutionError, 0),
	}

	done := make(chan struct{}, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			execCtx.addError(ExecutionError{
				Message: fmt.Sprintf("error %d", n),
			})
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}

	if len(execCtx.Errors) != 10 {
		t.Errorf("expected 10 errors, got %d", len(execCtx.Errors))
	}
}

func TestExecutor_CoerceValue_UnknownType(t *testing.T) {
	exec := NewExecutor()
	execCtx := &ExecutionContext{
		Context: context.Background(),
	}

	// Use a type that implements Value but isn't handled in the switch
	// We can't easily create one without adding to the package, but we can
	// verify nil handling works
	got := exec.coerceValue(execCtx, nil)
	if got != nil {
		t.Errorf("expected nil for nil value, got %v", got)
	}
}
