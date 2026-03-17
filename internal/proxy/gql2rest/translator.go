package gql2rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/ersinkoc/tentaserve/internal/graphql"
)

// FieldSelector represents a field selection with optional sub-selections.
type FieldSelector struct {
	Name     string
	SubFields []FieldSelector
}

// ParseFieldsQuery parses a fields query parameter into field selectors.
// Format: "name,email,posts.title,posts.content"
// Returns the root field selectors.
func ParseFieldsQuery(fields string) []FieldSelector {
	if fields == "" {
		return nil
	}

	// Split by comma, but respect nested paths
	parts := strings.Split(fields, ",")
	var selectors []FieldSelector

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Parse dotted path like "posts.title"
		selector := parseFieldPath(part)
		selectors = append(selectors, selector)
	}

	return mergeSelectors(selectors)
}

// parseFieldPath parses a single field path like "posts.title" or just "name".
func parseFieldPath(path string) FieldSelector {
	parts := strings.Split(path, ".")
	if len(parts) == 1 {
		return FieldSelector{Name: parts[0]}
	}

	// Build nested structure
	root := FieldSelector{Name: parts[0]}
	current := &root

	for i := 1; i < len(parts); i++ {
		child := FieldSelector{Name: parts[i]}
		current.SubFields = []FieldSelector{child}
		current = &current.SubFields[0]
	}

	return root
}

// mergeSelectors merges selectors with the same root name.
func mergeSelectors(selectors []FieldSelector) []FieldSelector {
	merged := make(map[string]FieldSelector)

	for _, sel := range selectors {
		if existing, ok := merged[sel.Name]; ok {
			// Merge subfields
			merged[sel.Name] = mergeFieldSelector(existing, sel)
		} else {
			merged[sel.Name] = sel
		}
	}

	// Convert map back to slice
	result := make([]FieldSelector, 0, len(merged))
	for _, sel := range merged {
		result = append(result, sel)
	}
	return result
}

// mergeFieldSelector merges two field selectors with the same name.
func mergeFieldSelector(a, b FieldSelector) FieldSelector {
	if len(a.SubFields) == 0 {
		return b
	}
	if len(b.SubFields) == 0 {
		return a
	}

	// Merge subfields recursively
	allSubs := append(a.SubFields, b.SubFields...)
	a.SubFields = mergeSelectors(allSubs)
	return a
}

// BuildSelectionSet builds a GraphQL selection set from field selectors.
func BuildSelectionSet(selectors []FieldSelector) string {
	if len(selectors) == 0 {
		return ""
	}

	var parts []string
	for _, sel := range selectors {
		parts = append(parts, buildFieldSelection(sel))
	}
	return "{ " + strings.Join(parts, " ") + " }"
}

// buildFieldSelection builds a single field selection string.
func buildFieldSelection(sel FieldSelector) string {
	if len(sel.SubFields) == 0 {
		return sel.Name
	}

	sub := BuildSelectionSet(sel.SubFields)
	return sel.Name + " " + sub
}

// RESTRequest represents an incoming REST request.
type RESTRequest struct {
	Method      string
	Path        string
	QueryParams map[string]string
	Body        map[string]interface{}
	Fields      []FieldSelector // Parsed ?fields= parameter
}

// GraphQLRequest represents a translated GraphQL request.
type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
	Operation string                 `json:"operationName,omitempty"`
}

// Translator converts REST requests to GraphQL requests.
type Translator struct {
	schema *graphql.Schema
}

// NewTranslator creates a new REST to GraphQL translator.
func NewTranslator(schema *graphql.Schema) *Translator {
	return &Translator{schema: schema}
}

// Translate converts a REST request to a GraphQL request.
// It looks up the appropriate query/mutation in the schema and builds the GraphQL request.
func (t *Translator) Translate(req RESTRequest, endpoint Endpoint) (*GraphQLRequest, error) {
	// Determine if this is a query or mutation
	isQuery := endpoint.Method == "GET"

	// Build the GraphQL operation
	operationType := "query"
	if !isQuery {
		operationType = "mutation"
	}

	// Build arguments
	args, variables := t.buildArguments(req, endpoint)

	// Build selection set
	selection := t.buildSelection(endpoint, req.Fields)

	// Construct the GraphQL query
	query := fmt.Sprintf("%s { %s%s %s }",
		operationType,
		endpoint.Field,
		args,
		selection,
	)

	return &GraphQLRequest{
		Query:     query,
		Variables: variables,
		Operation: endpoint.Field,
	}, nil
}

// buildArguments builds the GraphQL arguments string and variables map.
func (t *Translator) buildArguments(req RESTRequest, endpoint Endpoint) (string, map[string]interface{}) {
	var argParts []string
	variables := make(map[string]interface{})

	for _, arg := range endpoint.Arguments {
		var value interface{}
		var present bool

		switch arg.Location {
		case "path":
			// Path params are in the URL path
			if v, ok := req.QueryParams[arg.Name]; ok {
				value = v
				present = true
			}
		case "query":
			if v, ok := req.QueryParams[arg.Name]; ok {
				value = t.coerceValue(v, arg.Type)
				present = true
			}
		case "body":
			if req.Body != nil {
				if v, ok := req.Body[arg.Name]; ok {
					value = v
					present = true
				}
			}
		}

		if present {
			argParts = append(argParts, fmt.Sprintf("%s: $%s", arg.Name, arg.Name))
			variables[arg.Name] = value
		}
	}

	if len(argParts) == 0 {
		return "", variables
	}

	return "(" + strings.Join(argParts, ", ") + ")", variables
}

// coerceValue coerces a string value to the appropriate type.
func (t *Translator) coerceValue(v string, typ string) interface{} {
	switch typ {
	case "integer":
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
		return v
	case "number":
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
		return v
	case "boolean":
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
		return v
	default:
		return v
	}
}

// buildSelection builds the selection set for a field.
func (t *Translator) buildSelection(endpoint Endpoint, fields []FieldSelector) string {
	// If fields are specified, use them
	if len(fields) > 0 {
		return BuildSelectionSet(fields)
	}

	// Otherwise, build from the return type's fields in the schema
	returnType := endpoint.ReturnType
	if returnType == "" || t.schema == nil {
		return "{}"
	}

	// Look up the type in the schema
	schemaType := t.schema.GetType(returnType)
	if schemaType == nil {
		return "{}"
	}

	// Build selection from type fields
	return t.buildTypeSelection(schemaType)
}

// buildTypeSelection builds a selection set from a schema type.
func (t *Translator) buildTypeSelection(schemaType *graphql.IntrospectionType) string {
	if len(schemaType.Fields) == 0 {
		return "{}"
	}

	var parts []string
	for _, field := range schemaType.Fields {
		// Skip fields with required arguments
		if len(field.Args) > 0 {
			continue
		}
		parts = append(parts, field.Name)
	}

	if len(parts) == 0 {
		return "{}"
	}

	return "{ " + strings.Join(parts, " ") + " }"
}

// HTTPToRESTRequest converts an HTTP request to a RESTRequest.
func HTTPToRESTRequest(r *http.Request, body json.RawMessage) (RESTRequest, error) {
	// Parse query params
	queryParams := make(map[string]string)
	for key, values := range r.URL.Query() {
		if key == "fields" {
			continue // Handle fields separately
		}
		if len(values) > 0 {
			queryParams[key] = values[0]
		}
	}

	// Parse fields parameter
	fieldsStr := r.URL.Query().Get("fields")
	fields := ParseFieldsQuery(fieldsStr)

	// Parse body if present
	var bodyMap map[string]interface{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &bodyMap); err != nil {
			return RESTRequest{}, fmt.Errorf("invalid JSON body: %w", err)
		}
	}

	return RESTRequest{
		Method:      r.Method,
		Path:        r.URL.Path,
		QueryParams: queryParams,
		Body:        bodyMap,
		Fields:      fields,
	}, nil
}

// ParsePathParams extracts path parameters from a URL path based on a pattern.
// Pattern: "/api/users/{id}"
// Path: "/api/users/123"
// Returns: map[string]string{"id": "123"}
func ParsePathParams(pattern, path string) map[string]string {
	params := make(map[string]string)

	// Split pattern and path into segments
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")

	if len(patternParts) != len(pathParts) {
		return params
	}

	for i, part := range patternParts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			// This is a parameter
			paramName := part[1 : len(part)-1]
			params[paramName] = pathParts[i]
		}
	}

	return params
}
