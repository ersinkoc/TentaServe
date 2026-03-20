package gql2rest

import (
	"strings"
)

// FieldParamBuilder builds GraphQL selection sets from the ?fields= query parameter.
//
// The fields parameter format follows common REST API patterns:
//   - Simple fields: ?fields=name,email
//   - Nested fields: ?fields=name,posts.title,posts.content
//   - Wildcards: ?fields=* (all fields)
//   - Exclusions: ?fields=-password (all except password)
//
// Examples:
//   ?fields=id,name,email
//     → { id name email }
//
//   ?fields=name,posts.title,posts.content
//     → { name posts { title content } }
//
//   ?fields=name,posts{title,content}
//     → { name posts { title content } }

// FieldParamParser handles parsing of the fields parameter.
type FieldParamParser struct {
	includeAll bool
	exclusions map[string]bool
	selectors  []FieldSelector
}

// NewFieldParamParser creates a new fields parameter parser.
func NewFieldParamParser() *FieldParamParser {
	return &FieldParamParser{
		exclusions: make(map[string]bool),
	}
}

// Parse parses a fields parameter value.
func (p *FieldParamParser) Parse(fields string) *FieldParamResult {
	if fields == "" {
		return &FieldParamResult{
			IncludeAll: true,
		}
	}

	// Check for wildcard
	if fields == "*" {
		return &FieldParamResult{
			IncludeAll: true,
		}
	}

	// Handle exclusions (fields starting with -)
	if strings.HasPrefix(fields, "-") {
		excludedFields := parseFieldList(fields[1:])
		exclusions := make(map[string]bool)
		for _, f := range excludedFields {
			exclusions[f] = true
		}
		return &FieldParamResult{
			IncludeAll: true,
			Exclusions: exclusions,
		}
	}

	// Parse normal field selectors
	selectors := ParseFieldsQuery(fields)

	return &FieldParamResult{
		Selectors:  selectors,
		IncludeAll: false,
	}
}

// FieldParamResult is the result of parsing a fields parameter.
type FieldParamResult struct {
	Selectors  []FieldSelector
	Exclusions map[string]bool
	IncludeAll bool
}

// HasField checks if a specific field should be included.
func (r *FieldParamResult) HasField(name string) bool {
	if r.IncludeAll {
		return !r.Exclusions[name]
	}
	for _, sel := range r.Selectors {
		if sel.Name == name {
			return true
		}
	}
	return false
}

// ToSelectionSet converts the result to a GraphQL selection set.
func (r *FieldParamResult) ToSelectionSet() string {
	if r.IncludeAll {
		return ""
	}
	return BuildSelectionSet(r.Selectors)
}

// parseFieldList parses a comma-separated list of field names.
func parseFieldList(fields string) []string {
	var result []string
	for _, f := range strings.Split(fields, ",") {
		f = strings.TrimSpace(f)
		if f != "" {
			result = append(result, f)
		}
	}
	return result
}

// SelectionSetBuilder builds a GraphQL selection set with filtering.
type SelectionSetBuilder struct {
	typeInfo TypeInfo
	maxDepth int
	exclude  map[string]bool
	only     map[string]bool
}

// TypeInfo provides information about a GraphQL type.
type TypeInfo struct {
	Name   string
	Fields []FieldInfo
}

// FieldInfo provides information about a GraphQL field.
type FieldInfo struct {
	Name      string
	Type      string
	HasArgs   bool
	SubFields []FieldInfo
}

// NewSelectionSetBuilder creates a new selection set builder.
func NewSelectionSetBuilder(typeInfo TypeInfo) *SelectionSetBuilder {
	return &SelectionSetBuilder{
		typeInfo: typeInfo,
		maxDepth: 10,
		exclude:  make(map[string]bool),
	}
}

// WithMaxDepth sets the maximum depth for nested selections.
func (b *SelectionSetBuilder) WithMaxDepth(depth int) *SelectionSetBuilder {
	b.maxDepth = depth
	return b
}

// ExcludeFields excludes specific fields from the selection.
func (b *SelectionSetBuilder) ExcludeFields(fields ...string) *SelectionSetBuilder {
	for _, f := range fields {
		b.exclude[f] = true
	}
	return b
}

// OnlyFields restricts the selection to only the specified fields.
func (b *SelectionSetBuilder) OnlyFields(fields ...string) *SelectionSetBuilder {
	b.only = make(map[string]bool)
	for _, f := range fields {
		b.only[f] = true
	}
	return b
}

// Build builds the selection set for the type.
func (b *SelectionSetBuilder) Build() string {
	return b.buildTypeSelection(b.typeInfo, 0)
}

// buildTypeSelection builds selection for a type at a given depth.
func (b *SelectionSetBuilder) buildTypeSelection(typeInfo TypeInfo, depth int) string {
	if depth >= b.maxDepth {
		return ""
	}

	var fields []string
	for _, field := range typeInfo.Fields {
		// Check exclusions
		if b.exclude[field.Name] {
			continue
		}

		// Check inclusions
		if b.only != nil && !b.only[field.Name] {
			continue
		}

		// Skip fields with required arguments
		if field.HasArgs {
			continue
		}

		// Build field selection
		fieldStr := b.buildFieldSelection(field, depth)
		if fieldStr != "" {
			fields = append(fields, fieldStr)
		}
	}

	if len(fields) == 0 {
		return "{}"
	}

	return "{ " + strings.Join(fields, " ") + " }"
}

// buildFieldSelection builds selection for a single field.
func (b *SelectionSetBuilder) buildFieldSelection(field FieldInfo, depth int) string {
	if len(field.SubFields) == 0 {
		return field.Name
	}

	subType := TypeInfo{
		Name:   field.Type,
		Fields: field.SubFields,
	}
	subSelection := b.buildTypeSelection(subType, depth+1)

	if subSelection == "{}" {
		return field.Name
	}

	return field.Name + " " + subSelection
}

// ApplyFieldParam applies a parsed fields parameter to a selection set builder.
func ApplyFieldParam(builder *SelectionSetBuilder, result *FieldParamResult) string {
	if result.IncludeAll {
		// Apply exclusions only
		for field := range result.Exclusions {
			builder.ExcludeFields(field)
		}
	} else if len(result.Selectors) > 0 {
		// Apply field restrictions
		var fields []string
		for _, sel := range result.Selectors {
			fields = append(fields, sel.Name)
		}
		builder.OnlyFields(fields...)
	}

	return builder.Build()
}
