package graphql

import (
	"fmt"
)

// Validator provides GraphQL query validation.
type Validator struct {
	maxDepth     int
	maxComplexity int
}

// NewValidator creates a new validator with the given limits.
func NewValidator(maxDepth, maxComplexity int) *Validator {
	return &Validator{
		maxDepth:      maxDepth,
		maxComplexity: maxComplexity,
	}
}

// DefaultValidator creates a validator with default limits.
func DefaultValidator() *Validator {
	return NewValidator(10, 1000)
}

// ValidationError represents a validation error.
type ValidationError struct {
	Message string
	Line    int
	Column  int
}

func (e *ValidationError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s at line %d, column %d", e.Message, e.Line, e.Column)
	}
	return e.Message
}

// ValidateResult contains validation results.
type ValidateResult struct {
	Errors     []*ValidationError
	Depth      int
	Complexity int
}

// HasErrors returns true if there are validation errors.
func (r *ValidateResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// Validate validates a GraphQL document.
func (v *Validator) Validate(doc *Document) *ValidateResult {
	result := &ValidateResult{
		Errors: make([]*ValidationError, 0),
	}

	// Calculate depth and complexity
	depth, complexity := v.analyzeDocument(doc)
	result.Depth = depth
	result.Complexity = complexity

	// Check depth limit
	if depth > v.maxDepth {
		result.Errors = append(result.Errors, &ValidationError{
			Message: fmt.Sprintf("query exceeds maximum depth of %d (found %d)", v.maxDepth, depth),
		})
	}

	// Check complexity limit
	if complexity > v.maxComplexity {
		result.Errors = append(result.Errors, &ValidationError{
			Message: fmt.Sprintf("query exceeds maximum complexity of %d (found %d)", v.maxComplexity, complexity),
		})
	}

	return result
}

// CheckDepth calculates the maximum depth of a query without validating.
func CheckDepth(doc *Document) int {
	v := NewValidator(0, 0)
	depth, _ := v.analyzeDocument(doc)
	return depth
}

// CalculateComplexity calculates the complexity score of a query.
func CalculateComplexity(doc *Document) int {
	v := NewValidator(0, 0)
	_, complexity := v.analyzeDocument(doc)
	return complexity
}

// analyzeDocument analyzes the entire document for depth and complexity.
func (v *Validator) analyzeDocument(doc *Document) (depth, complexity int) {
	maxDepth := 0
	totalComplexity := 0

	for _, def := range doc.Definitions {
		d, c := v.analyzeDefinition(def)
		if d > maxDepth {
			maxDepth = d
		}
		totalComplexity += c
	}

	return maxDepth, totalComplexity
}

// analyzeDefinition analyzes a definition for depth and complexity.
func (v *Validator) analyzeDefinition(def Definition) (depth, complexity int) {
	switch d := def.(type) {
	case *OperationDefinition:
		return v.analyzeOperation(d)
	case *FragmentDefinition:
		return v.analyzeFragment(d)
	}
	return 0, 0
}

// analyzeOperation analyzes an operation for depth and complexity.
func (v *Validator) analyzeOperation(op *OperationDefinition) (depth, complexity int) {
	if op.SelectionSet == nil {
		return 0, 0
	}
	return v.analyzeSelectionSet(op.SelectionSet, 0)
}

// analyzeFragment analyzes a fragment for depth and complexity.
func (v *Validator) analyzeFragment(frag *FragmentDefinition) (depth, complexity int) {
	if frag.SelectionSet == nil {
		return 0, 0
	}
	return v.analyzeSelectionSet(frag.SelectionSet, 0)
}

// analyzeSelectionSet analyzes a selection set.
// depth is the current depth level (0 for root level fields).
func (v *Validator) analyzeSelectionSet(set *SelectionSet, depth int) (maxDepth, complexity int) {
	if set == nil || len(set.Selections) == 0 {
		return depth, 0
	}

	// Complexity for this level: number of fields
	// Multiplier increases complexity for list fields
	levelComplexity := len(set.Selections)
	if depth > 0 {
		// Apply depth multiplier: deeper fields cost more
		levelComplexity *= (depth + 1)
	}

	maxChildDepth := depth
	totalComplexity := levelComplexity

	for _, sel := range set.Selections {
		childDepth, childComplexity := v.analyzeSelection(sel, depth+1)
		if childDepth > maxChildDepth {
			maxChildDepth = childDepth
		}
		totalComplexity += childComplexity
	}

	return maxChildDepth, totalComplexity
}

// analyzeSelection analyzes a single selection.
func (v *Validator) analyzeSelection(sel Selection, depth int) (maxDepth, complexity int) {
	switch s := sel.(type) {
	case *Field:
		return v.analyzeField(s, depth)
	case *FragmentSpread:
		return v.analyzeFragmentSpread(s, depth)
	case *InlineFragment:
		return v.analyzeInlineFragment(s, depth)
	}
	return depth - 1, 0
}

// analyzeField analyzes a field selection.
func (v *Validator) analyzeField(f *Field, depth int) (maxDepth, complexity int) {
	// Base complexity for a field
	fieldComplexity := 1
	if depth > 0 {
		fieldComplexity = depth + 1
	}

	// If field has arguments, add some complexity
	if len(f.Arguments) > 0 {
		fieldComplexity += len(f.Arguments)
	}

	// Analyze nested selection set
	if f.SelectionSet != nil && len(f.SelectionSet.Selections) > 0 {
		nestedDepth, nestedComplexity := v.analyzeSelectionSet(f.SelectionSet, depth)
		if nestedDepth > depth {
			return nestedDepth, fieldComplexity + nestedComplexity
		}
		return depth, fieldComplexity + nestedComplexity
	}

	return depth, fieldComplexity
}

// analyzeFragmentSpread analyzes a fragment spread.
func (v *Validator) analyzeFragmentSpread(fs *FragmentSpread, depth int) (maxDepth, complexity int) {
	// Fragment spreads don't add depth, just complexity
	return depth - 1, 1
}

// analyzeInlineFragment analyzes an inline fragment.
func (v *Validator) analyzeInlineFragment(frag *InlineFragment, depth int) (maxDepth, complexity int) {
	// Inline fragments add depth
	if frag.SelectionSet == nil {
		return depth, 1
	}
	return v.analyzeSelectionSet(frag.SelectionSet, depth)
}

// ValidateDepthQuick quickly checks if a query exceeds depth limit.
func ValidateDepthQuick(doc *Document, maxDepth int) error {
	depth := CheckDepth(doc)
	if depth > maxDepth {
		return fmt.Errorf("query exceeds maximum depth of %d (found %d)", maxDepth, depth)
	}
	return nil
}

// ValidateComplexityQuick quickly checks if a query exceeds complexity limit.
func ValidateComplexityQuick(doc *Document, maxComplexity int) error {
	complexity := CalculateComplexity(doc)
	if complexity > maxComplexity {
		return fmt.Errorf("query exceeds maximum complexity of %d (found %d)", maxComplexity, complexity)
	}
	return nil
}
