package rest2gql

import (
	"fmt"
	"strings"

	"github.com/ersinkoc/tentaserve/internal/openapi"
	"github.com/ersinkoc/tentaserve/internal/schema"
)

// SchemaBuilder builds a GraphQL schema from an OpenAPI spec.
type SchemaBuilder struct {
	mapper    *TypeMapper
	registry  *schema.TypeRegistry
	prefix    string
	processed map[string]bool
}

// SchemaBuilderOptions configures the schema builder.
type SchemaBuilderOptions struct {
	// Prefix for generated types (e.g., "UsersApi")
	TypePrefix string
}

// NewSchemaBuilder creates a new schema builder.
func NewSchemaBuilder(opts SchemaBuilderOptions) *SchemaBuilder {
	return &SchemaBuilder{
		mapper:    NewTypeMapper(),
		registry:  schema.NewTypeRegistryWithBuiltins(),
		prefix:    opts.TypePrefix,
		processed: make(map[string]bool),
	}
}

// BuildSchema builds a schema from an OpenAPI spec.
func (b *SchemaBuilder) Build(spec *openapi.OpenAPISpec) (*schema.SchemaDefinition, error) {
	if spec == nil {
		return nil, fmt.Errorf("cannot build schema from nil spec")
	}

	// Process all schemas in components
	if spec.Components != nil && spec.Components.Schemas != nil {
		for name, s := range spec.Components.Schemas {
			if err := b.processSchema(name, s); err != nil {
				return nil, fmt.Errorf("processing schema %s: %w", name, err)
			}
		}
	}

	// Build Query type from GET operations
	queryFields := b.buildQueryFields(spec)

	// Build Mutation type from POST/PUT/DELETE operations
	mutationFields := b.buildMutationFields(spec)

	// Create schema
	schemaDef := schema.NewSchemaDefinition()

	// Add all registered types
	for _, name := range b.registry.Names() {
		if typ, ok := b.registry.Lookup(name); ok {
			schemaDef.AddType(typ)
		}
	}

	// Add Query type if it has fields
	if len(queryFields) > 0 {
		queryType := &schema.TypeDef{
			Name:   "Query",
			Kind:   schema.TypeKindObject,
			Fields: queryFields,
		}
		schemaDef.AddType(queryType)
		schemaDef.Query = &schema.OperationDef{
			Name:   "Query",
			Type:   "query",
			Fields: queryFields,
		}
	}

	// Add Mutation type if it has fields
	if len(mutationFields) > 0 {
		mutationType := &schema.TypeDef{
			Name:   "Mutation",
			Kind:   schema.TypeKindObject,
			Fields: mutationFields,
		}
		schemaDef.AddType(mutationType)
		schemaDef.Mutation = &schema.OperationDef{
			Name:   "Mutation",
			Type:   "mutation",
			Fields: mutationFields,
		}
	}

	return schemaDef, nil
}

// processSchema processes a schema definition and registers it.
func (b *SchemaBuilder) processSchema(name string, s *openapi.SchemaObject) error {
	if s == nil {
		return nil
	}

	// Skip if already processed
	if b.processed[name] {
		return nil
	}
	b.processed[name] = true

	// Handle references
	if s.Ref != "" {
		// Will be resolved when referenced
		return nil
	}

	// Convert name to PascalCase
	typeName := schema.ToPascalCase(name)
	if b.prefix != "" {
		typeName = b.prefix + typeName
	}

	switch s.Type {
	case "object":
		return b.buildObjectType(typeName, s)
	case "string":
		if len(s.Enum) > 0 {
			return b.buildEnumType(typeName, s)
		}
	}

	return nil
}

// buildObjectType builds an object type from a schema.
func (b *SchemaBuilder) buildObjectType(name string, s *openapi.SchemaObject) error {
	fields := make([]*schema.FieldDef, 0, len(s.Properties))

	for propName, prop := range s.Properties {
		field, err := b.buildField(propName, prop, s.Required)
		if err != nil {
			return fmt.Errorf("building field %s: %w", propName, err)
		}
		fields = append(fields, field)
	}

	typ := &schema.TypeDef{
		Name:        name,
		Description: s.Description,
		Kind:        schema.TypeKindObject,
		Fields:      fields,
	}

	return b.registry.Register(typ)
}

// buildEnumType builds an enum type from a schema.
func (b *SchemaBuilder) buildEnumType(name string, s *openapi.SchemaObject) error {
	enumValues := make([]schema.EnumValueDef, len(s.Enum))
	for i, val := range s.Enum {
		// Convert interface{} to string
		var strVal string
		if s, ok := val.(string); ok {
			strVal = s
		} else {
			strVal = fmt.Sprintf("%v", val)
		}
		enumValues[i] = schema.EnumValueDef{
			Name:        schema.ToPascalCase(strVal),
			Description: fmt.Sprintf("%s value", strVal),
		}
	}

	typ := &schema.TypeDef{
		Name:        name,
		Description: s.Description,
		Kind:        schema.TypeKindEnum,
		EnumValues:  enumValues,
	}

	return b.registry.Register(typ)
}

// buildField builds a field definition from a property.
func (b *SchemaBuilder) buildField(name string, prop *openapi.SchemaObject, required []string) (*schema.FieldDef, error) {
	// Process nested schemas first
	if prop.Ref != "" {
		refName := extractRefName(prop.Ref)
		if !b.processed[refName] {
			// Need to resolve and process
			// For now, just use the referenced name
		}
	}

	// Map the type
	fieldType, err := b.mapper.MapOpenAPIType(prop)
	if err != nil {
		return nil, err
	}

	// Check if field is required
	isRequired := false
	for _, r := range required {
		if r == name {
			isRequired = true
			break
		}
	}

	// Wrap in NonNull if required
	if isRequired && !fieldType.IsNonNull() {
		fieldType = schema.NonNullType(fieldType)
	}

	return &schema.FieldDef{
		Name:        schema.ToCamelCase(name),
		Description: prop.Description,
		Type:        fieldType,
	}, nil
}

// buildQueryFields builds Query fields from GET operations.
func (b *SchemaBuilder) buildQueryFields(spec *openapi.OpenAPISpec) []*schema.FieldDef {
	fields := make([]*schema.FieldDef, 0)

	for path, pathItem := range spec.Paths {
		if pathItem.Get != nil {
			field := b.buildOperationField(path, pathItem.Get, "query")
			if field != nil {
				fields = append(fields, field)
			}
		}
	}

	return fields
}

// buildMutationFields builds Mutation fields from POST/PUT/DELETE operations.
func (b *SchemaBuilder) buildMutationFields(spec *openapi.OpenAPISpec) []*schema.FieldDef {
	fields := make([]*schema.FieldDef, 0)

	for path, pathItem := range spec.Paths {
		// POST -> mutation
		if pathItem.Post != nil {
			field := b.buildOperationField(path, pathItem.Post, "mutation")
			if field != nil {
				fields = append(fields, field)
			}
		}
		// PUT -> mutation
		if pathItem.Put != nil {
			field := b.buildOperationField(path, pathItem.Put, "mutation")
			if field != nil {
				fields = append(fields, field)
			}
		}
		// DELETE -> mutation
		if pathItem.Delete != nil {
			field := b.buildOperationField(path, pathItem.Delete, "mutation")
			if field != nil {
				fields = append(fields, field)
			}
		}
		// PATCH -> mutation
		if pathItem.Patch != nil {
			field := b.buildOperationField(path, pathItem.Patch, "mutation")
			if field != nil {
				fields = append(fields, field)
			}
		}
	}

	return fields
}

// buildOperationField builds a field from an operation.
func (b *SchemaBuilder) buildOperationField(path string, op *openapi.Operation, operationType string) *schema.FieldDef {
	if op == nil {
		return nil
	}

	// Generate field name from path
	fieldName := b.pathToFieldName(path, op, operationType)

	// Build arguments from parameters
	args := make([]*schema.ArgumentDef, 0)
	for _, param := range op.Parameters {
		arg := b.buildArgument(param)
		if arg != nil {
			args = append(args, arg)
		}
	}

	// Add request body as argument for mutations
	if operationType == "mutation" && op.RequestBody != nil {
		arg := b.buildRequestBodyArgument(op.RequestBody)
		if arg != nil {
			args = append(args, arg)
		}
	}

	// Determine return type
	returnType := b.determineReturnType(op)

	return &schema.FieldDef{
		Name:        fieldName,
		Description: op.Summary,
		Type:        returnType,
		Arguments:   args,
	}
}

// pathToFieldName converts a path to a field name.
func (b *SchemaBuilder) pathToFieldName(path string, op *openapi.Operation, operationType string) string {
	// Remove leading slash and replace special chars
	cleaned := strings.TrimPrefix(path, "/")
	cleaned = strings.ReplaceAll(cleaned, "/", "_")
	cleaned = strings.ReplaceAll(cleaned, "{", "")
	cleaned = strings.ReplaceAll(cleaned, "}", "")
	cleaned = strings.ReplaceAll(cleaned, "-", "_")

	// Convert to camelCase
	name := schema.ToCamelCase(cleaned)

	// For mutations, prefix with operation type based on HTTP method convention
	if operationType == "mutation" && op != nil {
		// Detect operation type from path pattern and operation method
		hasPathParam := strings.Contains(path, "{")
		pathEndsWithParam := strings.HasSuffix(path, "}")

		// Check operation tags/summary for hints
		summaryLower := strings.ToLower(op.Summary)

		if strings.Contains(summaryLower, "create") || strings.Contains(summaryLower, "add") {
			name = "create" + schema.ToPascalCase(name)
		} else if strings.Contains(summaryLower, "update") || strings.Contains(summaryLower, "edit") {
			name = "update" + schema.ToPascalCase(name)
		} else if strings.Contains(summaryLower, "delete") || strings.Contains(summaryLower, "remove") {
			name = "delete" + schema.ToPascalCase(name)
		} else if strings.HasSuffix(path, "create") {
			name = "create" + schema.ToPascalCase(strings.TrimSuffix(name, "create"))
		} else if strings.HasSuffix(path, "update") {
			name = "update" + schema.ToPascalCase(strings.TrimSuffix(name, "update"))
		} else if strings.HasSuffix(path, "delete") {
			name = "delete" + schema.ToPascalCase(strings.TrimSuffix(name, "delete"))
		} else if hasPathParam && pathEndsWithParam {
			// Path like /users/{id} - likely update or delete
			// Default to update for mutations without explicit hint
			name = "update" + schema.ToPascalCase(name)
		} else {
			// No path param at end - likely create
			name = "create" + schema.ToPascalCase(name)
		}
	}

	return name
}

// buildArgument builds an argument from a parameter.
func (b *SchemaBuilder) buildArgument(param *openapi.Parameter) *schema.ArgumentDef {
	if param == nil {
		return nil
	}

	argType, err := b.mapper.MapOpenAPIType(param.Schema)
	if err != nil {
		// Default to String on error
		argType = schema.StringType()
	}

	// Required for path parameters
	if param.In == "path" {
		argType = schema.NonNullType(argType)
	}

	return &schema.ArgumentDef{
		Name:        schema.ToCamelCase(param.Name),
		Description: param.Description,
		Type:        argType,
		Required:    param.Required,
	}
}

// buildRequestBodyArgument builds an argument from a request body.
func (b *SchemaBuilder) buildRequestBodyArgument(body *openapi.RequestBody) *schema.ArgumentDef {
	if body == nil || body.Content == nil {
		return nil
	}

	// Find JSON content
	jsonContent, ok := body.Content["application/json"]
	if !ok {
		return nil
	}

	argType, err := b.mapper.MapOpenAPIType(jsonContent.Schema)
	if err != nil {
		argType = schema.StringType()
	}

	return &schema.ArgumentDef{
		Name:     "input",
		Type:     schema.NonNullType(argType),
		Required: body.Required,
	}
}

// determineReturnType determines the return type from operation responses.
func (b *SchemaBuilder) determineReturnType(op *openapi.Operation) *schema.TypeRef {
	// Look for 200/201 response
	for code, resp := range op.Responses {
		if code == "200" || code == "201" {
			if resp.Content != nil {
				jsonContent, ok := resp.Content["application/json"]
				if ok && jsonContent.Schema != nil {
					returnType, _ := b.mapper.MapOpenAPIType(jsonContent.Schema)
					if returnType != nil {
						return returnType
					}
				}
			}
		}
	}

	// Default to Boolean for mutations, Object for queries
	return schema.NamedType("Object")
}

// GetRegistry returns the type registry.
func (b *SchemaBuilder) GetRegistry() *schema.TypeRegistry {
	return b.registry
}

// GetMapper returns the type mapper.
func (b *SchemaBuilder) GetMapper() *TypeMapper {
	return b.mapper
}
