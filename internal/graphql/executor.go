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
	Context   context.Context
	Variables map[string]interface{}
	Errors    []ExecutionError
	Document  *Document // Reference to the parsed document for fragment resolution
	mu        sync.Mutex
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
		Document:  doc, // Store document for fragment resolution
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
			// Resolve fragment spread by looking up the fragment definition
			fragmentResult := e.executeFragmentSpread(execCtx, sel, parent, parentType)
			// Merge fragment results into current result
			for k, v := range fragmentResult {
				result[k] = v
			}
		case *InlineFragment:
			// Resolve inline fragment if type condition matches
			fragmentResult := e.executeInlineFragment(execCtx, sel, parent, parentType)
			// Merge fragment results into current result
			for k, v := range fragmentResult {
				result[k] = v
			}
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

// executeFragmentSpread resolves a fragment spread (...FragmentName).
func (e *Executor) executeFragmentSpread(execCtx *ExecutionContext, spread *FragmentSpread, parent interface{}, parentType string) map[string]interface{} {
	// Find the fragment definition in the document
	fragmentName := spread.Name.Value
	fragment := e.findFragmentDefinition(execCtx.Document, fragmentName)

	if fragment == nil {
		execCtx.addError(ExecutionError{
			Message: fmt.Sprintf("Fragment '%s' is not defined", fragmentName),
			Path:    []string{fragmentName},
		})
		return nil
	}

	// Determine the actual type from the parent object
	actualType := parentType
	if m, ok := parent.(map[string]interface{}); ok {
		if tn, ok := m["__typename"].(string); ok {
			actualType = tn
		}
	}

	// Check type condition if present
	if fragment.TypeCondition != nil && fragment.TypeCondition.Name != nil {
		if fragment.TypeCondition.Name.Value != actualType {
			// Type doesn't match - return empty result
			return nil
		}
	}

	// Execute the fragment's selection set with the fragment's type
	fragmentType := actualType
	if fragment.TypeCondition != nil && fragment.TypeCondition.Name != nil {
		fragmentType = fragment.TypeCondition.Name.Value
	}
	return e.executeSelectionSet(execCtx, fragment.SelectionSet, parent, fragmentType)
}

// executeInlineFragment resolves an inline fragment (... on Type { ... }).
func (e *Executor) executeInlineFragment(execCtx *ExecutionContext, fragment *InlineFragment, parent interface{}, parentType string) map[string]interface{} {
	// Determine the actual type from the parent object
	actualType := parentType
	if m, ok := parent.(map[string]interface{}); ok {
		if tn, ok := m["__typename"].(string); ok {
			actualType = tn
		}
	}

	// Check type condition
	if fragment.TypeCondition != nil && fragment.TypeCondition.Name != nil {
		if fragment.TypeCondition.Name.Value != actualType {
			// Type doesn't match - return empty result
			return nil
		}
	}

	// Execute the inline fragment's selection set with the fragment's type
	fragmentType := actualType
	if fragment.TypeCondition != nil && fragment.TypeCondition.Name != nil {
		fragmentType = fragment.TypeCondition.Name.Value
	}
	return e.executeSelectionSet(execCtx, fragment.SelectionSet, parent, fragmentType)
}

// findFragmentDefinition finds a fragment definition by name in the document.
func (e *Executor) findFragmentDefinition(doc *Document, name string) *FragmentDefinition {
	if doc == nil {
		return nil
	}

	for _, def := range doc.Definitions {
		if fragment, ok := def.(*FragmentDefinition); ok {
			if fragment.Name != nil && fragment.Name.Value == name {
				return fragment
			}
		}
	}
	return nil
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
			switch sel := selection.(type) {
			case *Field:
				fieldName := sel.Name.Value
				key := e.getFieldKey(sel)
				// Get the field value from the map
				fieldValue := m[fieldName]
				// Recursively resolve if there's a nested selection set
				if sel.SelectionSet != nil && len(sel.SelectionSet.Selections) > 0 {
					result[key] = e.resolveNested(execCtx, sel.SelectionSet, fieldValue)
				} else {
					result[key] = fieldValue
				}
			case *FragmentSpread:
				// Resolve fragment spread and merge results
				fragmentResult := e.resolveNestedFragmentSpread(execCtx, sel, m)
				for k, v := range fragmentResult {
					result[k] = v
				}
			case *InlineFragment:
				// Resolve inline fragment and merge results
				fragmentResult := e.resolveNestedInlineFragment(execCtx, sel, m)
				for k, v := range fragmentResult {
					result[k] = v
				}
			}
		}
		return result
	}

	// For other types, return as-is
	return value
}

// resolveNestedFragmentSpread resolves a fragment spread in nested context.
func (e *Executor) resolveNestedFragmentSpread(execCtx *ExecutionContext, spread *FragmentSpread, parent map[string]interface{}) map[string]interface{} {
	// Find the fragment definition
	fragmentName := spread.Name.Value
	fragment := e.findFragmentDefinition(execCtx.Document, fragmentName)

	if fragment == nil {
		execCtx.addError(ExecutionError{
			Message: fmt.Sprintf("Fragment '%s' is not defined", fragmentName),
			Path:    []string{fragmentName},
		})
		return nil
	}

	// Determine the actual type from the parent object
	actualType := ""
	if tn, ok := parent["__typename"].(string); ok {
		actualType = tn
	}

	// Check type condition if present
	if fragment.TypeCondition != nil && fragment.TypeCondition.Name != nil {
		if fragment.TypeCondition.Name.Value != actualType {
			// Type doesn't match - return empty result
			return nil
		}
	}

	// Resolve the fragment's selection set against the parent data
	return e.resolveFragmentSelectionSet(execCtx, fragment.SelectionSet, parent)
}

// resolveNestedInlineFragment resolves an inline fragment in nested context.
func (e *Executor) resolveNestedInlineFragment(execCtx *ExecutionContext, fragment *InlineFragment, parent map[string]interface{}) map[string]interface{} {
	// Determine the actual type from the parent object
	actualType := ""
	if tn, ok := parent["__typename"].(string); ok {
		actualType = tn
	}

	// Check type condition
	if fragment.TypeCondition != nil && fragment.TypeCondition.Name != nil {
		if fragment.TypeCondition.Name.Value != actualType {
			// Type doesn't match - return empty result
			return nil
		}
	}

	// Resolve the inline fragment's selection set against the parent data
	return e.resolveFragmentSelectionSet(execCtx, fragment.SelectionSet, parent)
}

// resolveFragmentSelectionSet resolves a selection set against parent map data.
func (e *Executor) resolveFragmentSelectionSet(execCtx *ExecutionContext, selectionSet *SelectionSet, parent map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	if selectionSet == nil {
		return result
	}

	for _, selection := range selectionSet.Selections {
		switch sel := selection.(type) {
		case *Field:
			fieldName := sel.Name.Value
			key := e.getFieldKey(sel)
			// Get the field value from the parent
			fieldValue := parent[fieldName]
			// Recursively resolve if there's a nested selection set
			if sel.SelectionSet != nil && len(sel.SelectionSet.Selections) > 0 {
				result[key] = e.resolveNested(execCtx, sel.SelectionSet, fieldValue)
			} else {
				result[key] = fieldValue
			}
		case *FragmentSpread:
			// Recursively resolve nested fragment spreads
			nestedResult := e.resolveNestedFragmentSpread(execCtx, sel, parent)
			for k, v := range nestedResult {
				result[k] = v
			}
		case *InlineFragment:
			// Recursively resolve nested inline fragments
			nestedResult := e.resolveNestedInlineFragment(execCtx, sel, parent)
			for k, v := range nestedResult {
				result[k] = v
			}
		}
	}
	return result
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
