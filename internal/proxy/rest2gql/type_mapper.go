package rest2gql

import (
	"fmt"
	"strings"

	"github.com/ersinkoc/tentaserve/internal/openapi"
	"github.com/ersinkoc/tentaserve/internal/schema"
)

// TypeMapper maps OpenAPI types to GraphQL types.
type TypeMapper struct {
	// Custom mappings for specific types
	mappings map[string]string
}

// NewTypeMapper creates a new type mapper.
func NewTypeMapper() *TypeMapper {
	return &TypeMapper{
		mappings: make(map[string]string),
	}
}

// MapOpenAPIType maps an OpenAPI schema to a GraphQL type.
func (m *TypeMapper) MapOpenAPIType(s *openapi.SchemaObject) (*schema.TypeRef, error) {
	if s == nil {
		return nil, fmt.Errorf("cannot map nil schema")
	}

	// Handle references
	if s.Ref != "" {
		name := extractRefName(s.Ref)
		return schema.NamedType(name), nil
	}

	// Map based on type
	var result *schema.TypeRef

	switch s.Type {
	case "string":
		result = m.mapStringType(s)
	case "integer":
		result = schema.IntType()
	case "number":
		result = schema.FloatType()
	case "boolean":
		result = schema.BoolType()
	case "array":
		itemType, err := m.MapOpenAPIType(s.Items)
		if err != nil {
			return nil, err
		}
		result = schema.ListType(itemType)
	case "object":
		result = m.mapObjectType(s)
	default:
		// Unknown type - treat as string
		result = schema.StringType()
	}

	// Wrap in NonNull if required
	if len(s.Required) > 0 {
		result = schema.NonNullType(result)
	}

	return result, nil
}

// mapStringType maps string schemas including enums.
func (m *TypeMapper) mapStringType(s *openapi.SchemaObject) *schema.TypeRef {
	// Handle enums
	if len(s.Enum) > 0 {
		// Generate enum name from field context or use generic
		name := generateEnumName(s)
		// Convert []interface{} to []string
		enumValues := make([]string, len(s.Enum))
		for i, v := range s.Enum {
			if str, ok := v.(string); ok {
				enumValues[i] = str
			} else {
				enumValues[i] = fmt.Sprintf("%v", v)
			}
		}
		return schema.EnumType(name, enumValues)
	}

	// Handle formats
	switch s.Format {
	case "date-time", "date":
		// Could map to custom DateTime scalar
		if scalar, ok := m.mappings["date-time"]; ok {
			return schema.ScalarType(scalar)
		}
		return schema.StringType()
	case "uuid":
		return schema.IDType()
	default:
		return schema.StringType()
	}
}

// mapObjectType maps object schemas.
func (m *TypeMapper) mapObjectType(s *openapi.SchemaObject) *schema.TypeRef {
	// If it has additional properties, it's a map type
	if s.AdditionalProperties != nil {
		// Map type - represented as List of key-value pairs in GraphQL
		return schema.NamedType("Map")
	}

	// Regular object - should be defined as a type
	// Return generic object for now
	return schema.NamedType("Object")
}

// generateEnumName generates a name for an enum type.
func generateEnumName(s *openapi.SchemaObject) string {
	// Try to extract meaningful name from context
	if s.Title != "" {
		return schema.ToPascalCase(s.Title)
	}
	return "Enum"
}

// extractRefName extracts the type name from a $ref.
func extractRefName(ref string) string {
	// refs are like "#/components/schemas/UserName"
	parts := strings.Split(ref, "/")
	if len(parts) > 0 {
		name := parts[len(parts)-1]
		// Convert to PascalCase for GraphQL
		return schema.ToPascalCase(name)
	}
	return ref
}

// OpenAPIToGraphQLType maps OpenAPI primitive types to GraphQL types.
func OpenAPIToGraphQLType(openAPIType, format string) string {
	switch openAPIType {
	case "string":
		if format == "uuid" {
			return "ID"
		}
		return "String"
	case "integer":
		return "Int"
	case "number":
		return "Float"
	case "boolean":
		return "Boolean"
	case "array":
		return "List"
	case "object":
		return "Object"
	default:
		return "String"
	}
}

// GraphQLToOpenAPIType maps GraphQL types back to OpenAPI types.
func GraphQLToOpenAPIType(graphQLType string) (string, string) {
	switch graphQLType {
	case "String":
		return "string", ""
	case "Int":
		return "integer", "int32"
	case "Float":
		return "number", "float64"
	case "Boolean":
		return "boolean", ""
	case "ID":
		return "string", "uuid"
	default:
		return "string", ""
	}
}

// AddCustomMapping adds a custom type mapping.
func (m *TypeMapper) AddCustomMapping(openAPIType, graphQLType string) {
	m.mappings[openAPIType] = graphQLType
}

// IsComplexType checks if an OpenAPI type is complex (object or array of objects).
func IsComplexType(s *openapi.SchemaObject) bool {
	if s == nil {
		return false
	}
	if s.Type == "object" {
		return true
	}
	if s.Type == "array" && s.Items != nil {
		return IsComplexType(s.Items)
	}
	return false
}

// GetNestedType returns the nested item type for arrays.
func GetNestedType(s *openapi.SchemaObject) *openapi.SchemaObject {
	if s == nil {
		return nil
	}
	if s.Type == "array" && s.Items != nil {
		return s.Items
	}
	return s
}

// CoerceValue coerces a string value to the appropriate Go type based on OpenAPI type.
func CoerceValue(value string, openAPIType, format string) (interface{}, error) {
	switch openAPIType {
	case "string":
		return value, nil
	case "integer":
		var i int64
		_, err := fmt.Sscanf(value, "%d", &i)
		if err != nil {
			return nil, fmt.Errorf("cannot parse integer: %w", err)
		}
		return i, nil
	case "number":
		var f float64
		_, err := fmt.Sscanf(value, "%f", &f)
		if err != nil {
			return nil, fmt.Errorf("cannot parse float: %w", err)
		}
		return f, nil
	case "boolean":
		return value == "true" || value == "1", nil
	default:
		return value, nil
	}
}
