package graphql

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// Executor executes GraphQL queries against registered resolvers.
type Executor struct {
	// resolvers maps field paths to resolver functions
	// Format: "TypeName.fieldName" -> resolver
	resolvers map[string]ResolverFunc

	// mu protects resolvers map
	mu sync.RWMutex
}

// ResolverFunc is a function that resolves a field value.
type ResolverFunc func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error)

// ExecutionContext holds the context for query execution.
type ExecutionContext struct {
	Context    context.Context
	Variables  map[string]interface{}
	Errors     []ExecutionError
	mu         sync.Mutex
}

// ExecutionError represents an error during execution.
type ExecutionError struct {
	Message    string
	Path       []string
	Extensions map[string]interface{}
}

// ExecutionResult is the result of executing a GraphQL query.
type ExecutionResult struct {
	Data   interface{}      `json:"data,omitempty"`
	Errors []ExecutionError `json:"errors,omitempty"`
}

// NewExecutor creates a new GraphQL executor.
func NewExecutor() *Executor {
	return &Executor{
		resolvers: make(map[string]ResolverFunc),
	}
}

// RegisterResolver registers a resolver for a field.
func (e *Executor) RegisterResolver(typeName, fieldName string, resolver ResolverFunc) {
	path := typeName + "." + fieldName
	e.mu.Lock()
	defer e.mu.Unlock()
	e.resolvers[path] = resolver
}

// LookupResolver finds a resolver for a field.
func (e *Executor) LookupResolver(typeName, fieldName string) (ResolverFunc, bool) {
	path := typeName + "." + fieldName
	e.mu.RLock()
	defer e.mu.RUnlock()
	resolver, ok := e.resolvers[path]
	return resolver, ok
}

// Execute executes a parsed GraphQL document.
func (e *Executor) Execute(ctx context.Context, doc *Document, variables map[string]interface{}) *ExecutionResult {
	execCtx := &ExecutionContext{
		Context:   ctx,
		Variables: variables,
		Errors:    make([]ExecutionError, 0),
	}

	// Find the operation (query, mutation, subscription)
	operation := e.findOperation(doc)
	if operation == nil {
		return &ExecutionResult{
			Errors: []ExecutionError{{
				Message: "No operation found in document",
			}},
		}
	}

	// Determine operation type
	operationType := e.getOperationType(operation)

	// Execute the selection set
	data := e.executeSelectionSet(execCtx, operation.SelectionSet, nil, operationType)

	return &ExecutionResult{
		Data:   data,
		Errors: execCtx.Errors,
	}
}

// findOperation finds the operation definition in the document.
func (e *Executor) findOperation(doc *Document) *OperationDefinition {
	if doc == nil {
		return nil
	}
	for _, def := range doc.Definitions {
		if op, ok := def.(*OperationDefinition); ok {
			return op
		}
	}
	return nil
}

// getOperationType returns the root type name for the operation.
func (e *Executor) getOperationType(op *OperationDefinition) string {
	switch op.Operation {
	case TokenMutation:
		return "Mutation"
	case TokenSubscription:
		return "Subscription"
	default:
		return "Query"
	}
}

// executeSelectionSet executes a selection set against a parent value.
func (e *Executor) executeSelectionSet(execCtx *ExecutionContext, selectionSet *SelectionSet, parent interface{}, parentType string) map[string]interface{} {
	if selectionSet == nil {
		return nil
	}

	result := make(map[string]interface{})

	for _, selection := range selectionSet.Selections {
		switch sel := selection.(type) {
		case *Field:
			fieldResult := e.executeField(execCtx, sel, parent, parentType)
			key := e.getFieldKey(sel)
			result[key] = fieldResult
		case *FragmentSpread:
			// TODO: Implement fragment spread resolution
		case *InlineFragment:
			// TODO: Implement inline fragment resolution
		}
	}

	return result
}

// executeField executes a single field.
func (e *Executor) executeField(execCtx *ExecutionContext, field *Field, parent interface{}, parentType string) interface{} {
	fieldName := field.Name.Value

	// Build the argument map
	args := e.buildArguments(execCtx, field.Arguments)

	// Look up the resolver
	resolver, ok := e.LookupResolver(parentType, fieldName)
	if !ok {
		// Try default resolver
		resolver = e.defaultResolver
	}

	// Execute the resolver
	value, err := resolver(execCtx.Context, parent, args)
	if err != nil {
		execCtx.addError(ExecutionError{
			Message: err.Error(),
			Path:    []string{fieldName},
		})
		return nil
	}

	// Handle nested selection set
	if field.SelectionSet != nil && len(field.SelectionSet.Selections) > 0 {
		return e.resolveNested(execCtx, field.SelectionSet, value)
	}

	return value
}

// resolveNested resolves nested fields for object or list values.
func (e *Executor) resolveNested(execCtx *ExecutionContext, selectionSet *SelectionSet, value interface{}) interface{} {
	if value == nil {
		return nil
	}

	// Handle list types
	if list, ok := value.([]interface{}); ok {
		results := make([]interface{}, len(list))
		for i, item := range list {
			results[i] = e.resolveNested(execCtx, selectionSet, item)
		}
		return results
	}

	// Handle maps - extract fields directly
	if m, ok := value.(map[string]interface{}); ok {
		result := make(map[string]interface{})
		for _, selection := range selectionSet.Selections {
			if field, ok := selection.(*Field); ok {
				fieldName := field.Name.Value
				key := e.getFieldKey(field)
				// Get the field value from the map
				fieldValue := m[fieldName]
				// Recursively resolve if there's a nested selection set
				if field.SelectionSet != nil && len(field.SelectionSet.Selections) > 0 {
					result[key] = e.resolveNested(execCtx, field.SelectionSet, fieldValue)
				} else {
					result[key] = fieldValue
				}
			}
		}
		return result
	}

	// For other types, return as-is
	return value
}

// buildArguments builds the argument map from field arguments.
func (e *Executor) buildArguments(execCtx *ExecutionContext, args []*Argument) map[string]interface{} {
	result := make(map[string]interface{})
	for _, arg := range args {
		name := arg.Name.Value
		value := e.coerceValue(execCtx, arg.Value)
		result[name] = value
	}
	return result
}

// coerceValue converts an AST value to a runtime value.
func (e *Executor) coerceValue(execCtx *ExecutionContext, value Value) interface{} {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case *StringValue:
		return v.Value
	case *IntValue:
		return v.Value
	case *FloatValue:
		return v.Value
	case *BooleanValue:
		return v.Value
	case *NullValue:
		return nil
	case *EnumValue:
		return v.Value
	case *ListValue:
		result := make([]interface{}, len(v.Values))
		for i, item := range v.Values {
			result[i] = e.coerceValue(execCtx, item)
		}
		return result
	case *ObjectValue:
		result := make(map[string]interface{})
		for _, field := range v.Fields {
			result[field.Name.Value] = e.coerceValue(execCtx, field.Value)
		}
		return result
	case *VariableValue:
		// Look up variable value
		varName := v.Name.Value
		if execCtx.Variables != nil {
			if val, ok := execCtx.Variables[varName]; ok {
				return val
			}
		}
		return nil
	default:
		return nil
	}
}

// getFieldKey returns the key to use in the result (alias or name).
func (e *Executor) getFieldKey(field *Field) string {
	if field.Alias != nil {
		return field.Alias.Value
	}
	return field.Name.Value
}

// defaultResolver is the default field resolver.
// It attempts to extract the field value from the parent map.
func (e *Executor) defaultResolver(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
	// Get the field name from context or parent
	// This is a simplified version - in practice you'd need to track the field name
	if m, ok := parent.(map[string]interface{}); ok {
		// The field name would need to be passed in args or context
		// For now, return the parent itself
		return m, nil
	}
	return nil, fmt.Errorf("cannot resolve field on non-object type")
}

// addError adds an error to the execution context (thread-safe).
func (ec *ExecutionContext) addError(err ExecutionError) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.Errors = append(ec.Errors, err)
}

// MarshalJSON implements json.Marshaler for ExecutionError.
func (ee ExecutionError) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"message": ee.Message,
	}
	if len(ee.Path) > 0 {
		m["path"] = ee.Path
	}
	if len(ee.Extensions) > 0 {
		m["extensions"] = ee.Extensions
	}
	return json.Marshal(m)
}

// IsNull checks if a value is effectively null.
func IsNull(v interface{}) bool {
	if v == nil {
		return true
	}
	return false
}
