package mcp

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/ersinkoc/tentaserve/internal/openapi"
	"github.com/ersinkoc/tentaserve/internal/schema"
)

// Tool represents an MCP tool.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`

	// Internal fields for execution
	Upstream    string                 `json:"-"`
	Operation   string                 `json:"-"` // REST: "GET /path", GraphQL: "query.field"
	Method      string                 `json:"-"` // HTTP method for REST
	Path        string                 `json:"-"` // REST path template
	ArgMapping  map[string]string      `json:"-"` // Maps tool arg -> upstream param
	ExtraConfig map[string]interface{} `json:"-"`
}

// ToolRegistry manages MCP tools.
type ToolRegistry struct {
	mu              sync.RWMutex
	tools           map[string]*Tool
	logger          *slog.Logger
	nameGen         *NameGenerator
	excludePatterns []string
}

// NewToolRegistry creates a new tool registry.
func NewToolRegistry(logger *slog.Logger) *ToolRegistry {
	if logger == nil {
		logger = slog.Default()
	}
	return &ToolRegistry{
		tools:           make(map[string]*Tool),
		logger:          logger,
		nameGen:         NewNameGenerator(),
		excludePatterns: []string{},
	}
}

// SetExcludePatterns sets patterns to exclude from tool generation.
func (r *ToolRegistry) SetExcludePatterns(patterns []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.excludePatterns = patterns
}

// shouldExclude checks if a tool name matches an exclude pattern.
func (r *ToolRegistry) shouldExclude(name string) bool {
	for _, pattern := range r.excludePatterns {
		if pattern == "" {
			continue
		}
		// Simple substring match for now
		if strings.Contains(name, pattern) {
			return true
		}
		// Exact match
		if name == pattern {
			return true
		}
	}
	return false
}

// Register adds a tool to the registry.
func (r *ToolRegistry) Register(tool *Tool) error {
	if tool == nil {
		return fmt.Errorf("cannot register nil tool")
	}
	if tool.Name == "" {
		return fmt.Errorf("cannot register tool with empty name")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.tools[tool.Name] = tool
	r.logger.Debug("registered tool", "name", tool.Name)
	return nil
}

// Unregister removes a tool from the registry.
func (r *ToolRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, name)
	r.logger.Debug("unregistered tool", "name", name)
}

// Get retrieves a tool by name.
func (r *ToolRegistry) Get(name string) *Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

// List returns all registered tools.
func (r *ToolRegistry) List() []*Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]*Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}

	// Sort alphabetically by name
	for i := 0; i < len(tools)-1; i++ {
		for j := i + 1; j < len(tools); j++ {
			if tools[i].Name > tools[j].Name {
				tools[i], tools[j] = tools[j], tools[i]
			}
		}
	}

	return tools
}

// Count returns the number of registered tools.
func (r *ToolRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

// Clear removes all tools from the registry.
func (r *ToolRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools = make(map[string]*Tool)
	r.nameGen.Reset()
}

// BuildFromSchema generates tools from a unified schema.
func (r *ToolRegistry) BuildFromSchema(schemaDef *schema.SchemaDefinition, upstreamName string) error {
	if schemaDef == nil {
		return fmt.Errorf("cannot build tools from nil schema")
	}

	r.logger.Info("building tools from schema",
		"upstream", upstreamName,
		"types", len(schemaDef.Types),
	)

	// Generate tools from Query operations
	if schemaDef.Query != nil {
		for _, field := range schemaDef.Query.Fields {
			if err := r.buildToolFromField(field, upstreamName, "query", schemaDef); err != nil {
				r.logger.Warn("failed to build tool from query field",
					"field", field.Name,
					"error", err,
				)
			}
		}
	}

	// Generate tools from Mutation operations
	if schemaDef.Mutation != nil {
		for _, field := range schemaDef.Mutation.Fields {
			if err := r.buildToolFromField(field, upstreamName, "mutation", schemaDef); err != nil {
				r.logger.Warn("failed to build tool from mutation field",
					"field", field.Name,
					"error", err,
				)
			}
		}
	}

	r.logger.Info("tools built from schema",
		"upstream", upstreamName,
		"count", r.Count(),
	)
	return nil
}

// BuildFromOpenAPI generates tools from an OpenAPI spec.
func (r *ToolRegistry) BuildFromOpenAPI(spec *openapi.OpenAPISpec, upstreamName string) error {
	if spec == nil {
		return fmt.Errorf("cannot build tools from nil spec")
	}

	r.logger.Info("building tools from OpenAPI",
		"upstream", upstreamName,
		"title", spec.Info.Title,
	)

	// Iterate over all paths
	for path, pathItem := range spec.Paths {
		if pathItem == nil {
			continue
		}

		// Generate tool for each HTTP method
		methods := []struct {
			op     *openapi.Operation
			method string
		}{
			{pathItem.Get, "GET"},
			{pathItem.Post, "POST"},
			{pathItem.Put, "PUT"},
			{pathItem.Delete, "DELETE"},
			{pathItem.Patch, "PATCH"},
		}

		for _, m := range methods {
			if m.op == nil {
				continue
			}
			if err := r.buildToolFromOpenAPIOperation(m.op, path, m.method, upstreamName); err != nil {
				r.logger.Warn("failed to build tool from OpenAPI operation",
					"path", path,
					"method", m.method,
					"error", err,
				)
			}
		}
	}

	r.logger.Info("tools built from OpenAPI",
		"upstream", upstreamName,
		"count", r.Count(),
	)
	return nil
}

// buildToolFromField creates a tool from a schema field.
func (r *ToolRegistry) buildToolFromField(field *schema.FieldDef, upstreamName, operationType string, schemaDef *schema.SchemaDefinition) error {
	// Generate tool name
	toolName := r.nameGen.Generate(upstreamName, field.Name, "")

	// Check exclude patterns
	if r.shouldExclude(toolName) {
		r.logger.Debug("skipping excluded tool", "name", toolName)
		return nil
	}

	// Generate description
	description := GenerateToolDescription(upstreamName, field.Description, operationType)

	// Generate input schema
	inputSchema, err := GenerateInputSchema(field.Arguments, schemaDef)
	if err != nil {
		return fmt.Errorf("generating input schema: %w", err)
	}

	// Build argument mapping
	argMapping := make(map[string]string)
	for _, arg := range field.Arguments {
		argMapping[arg.Name] = arg.Name
	}

	tool := &Tool{
		Name:        toolName,
		Description: description,
		InputSchema: inputSchema,
		Upstream:    upstreamName,
		Operation:   operationType + "." + field.Name,
		ArgMapping:  argMapping,
	}

	return r.Register(tool)
}

// buildToolFromOpenAPIOperation creates a tool from an OpenAPI operation.
func (r *ToolRegistry) buildToolFromOpenAPIOperation(op *openapi.Operation, path, method, upstreamName string) error {
	// Generate tool name from operationId or path
	var toolName string
	if op.OperationID != "" {
		toolName = r.nameGen.Generate(upstreamName, op.OperationID, "")
	} else {
		toolName = r.nameGen.GenerateFromPath(upstreamName, path, method)
	}

	// Check exclude patterns
	if r.shouldExclude(toolName) {
		r.logger.Debug("skipping excluded tool", "name", toolName)
		return nil
	}

	// Generate description
	description := GenerateToolDescription(upstreamName, op.Summary, method+" "+path)

	// Generate input schema from parameters and request body
	inputSchema, err := GenerateInputSchemaFromOpenAPI(op.Parameters, op.RequestBody)
	if err != nil {
		return fmt.Errorf("generating input schema: %w", err)
	}

	// Build argument mapping
	argMapping := make(map[string]string)
	for _, param := range op.Parameters {
		if param != nil {
			argMapping[param.Name] = param.Name
		}
	}

	tool := &Tool{
		Name:        toolName,
		Description: description,
		InputSchema: inputSchema,
		Upstream:    upstreamName,
		Operation:   method + " " + path,
		Method:      method,
		Path:        path,
		ArgMapping:  argMapping,
	}

	return r.Register(tool)
}

// JSONSchema represents a JSON Schema for tool inputs.
type JSONSchema struct {
	Type                 string                 `json:"type,omitempty"`
	Title                string                 `json:"title,omitempty"`
	Description          string                 `json:"description,omitempty"`
	Properties           map[string]*JSONSchema `json:"properties,omitempty"`
	Required             []string               `json:"required,omitempty"`
	Default              interface{}            `json:"default,omitempty"`
	Enum                 []interface{}          `json:"enum,omitempty"`
	Items                *JSONSchema            `json:"items,omitempty"`
	AdditionalProperties bool                   `json:"additionalProperties,omitempty"`
}

// GenerateInputSchema generates a JSON Schema from schema arguments.
func GenerateInputSchema(args []*schema.ArgumentDef, schemaDef *schema.SchemaDefinition) (json.RawMessage, error) {
	if len(args) == 0 {
		// No arguments - return empty object schema
		schema := &JSONSchema{
			Type:                 "object",
			AdditionalProperties: false,
		}
		return json.Marshal(schema)
	}

	schema := &JSONSchema{
		Type:                 "object",
		Properties:           make(map[string]*JSONSchema),
		AdditionalProperties: false,
	}

	for _, arg := range args {
		prop := typeRefToJSONSchema(arg.Type, schemaDef)
		prop.Description = arg.Description
		if arg.DefaultValue != nil {
			prop.Default = arg.DefaultValue
		}
		schema.Properties[arg.Name] = prop

		if arg.Required {
			schema.Required = append(schema.Required, arg.Name)
		}
	}

	return json.Marshal(schema)
}

// GenerateInputSchemaFromOpenAPI generates JSON Schema from OpenAPI parameters.
func GenerateInputSchemaFromOpenAPI(params []*openapi.Parameter, reqBody *openapi.RequestBody) (json.RawMessage, error) {
	schema := &JSONSchema{
		Type:                 "object",
		Properties:           make(map[string]*JSONSchema),
		AdditionalProperties: false,
	}

	// Add parameters as properties
	for _, param := range params {
		if param == nil {
			continue
		}
		prop := openAPISchemaToJSONSchema(param.Schema)
		prop.Description = param.Description
		schema.Properties[param.Name] = prop

		if param.Required {
			schema.Required = append(schema.Required, param.Name)
		}
	}

	// Handle request body if present
	if reqBody != nil && reqBody.Content != nil {
		// Try JSON content first
		if content, ok := reqBody.Content["application/json"]; ok && content.Schema != nil {
			bodySchema := openAPISchemaToJSONSchema(content.Schema)
			// Merge body properties into main schema
			if bodySchema.Type == "object" && bodySchema.Properties != nil {
				for name, prop := range bodySchema.Properties {
					schema.Properties[name] = prop
					// If body is required, add all body properties as required
					if reqBody.Required {
						schema.Required = append(schema.Required, name)
					}
				}
			}
		}
	}

	if len(schema.Properties) == 0 {
		// No properties - return empty object
		return json.Marshal(&JSONSchema{
			Type:                 "object",
			AdditionalProperties: false,
		})
	}

	return json.Marshal(schema)
}

// typeRefToJSONSchema converts a schema TypeRef to JSON Schema.
func typeRefToJSONSchema(t *schema.TypeRef, schemaDef *schema.SchemaDefinition) *JSONSchema {
	if t == nil {
		return &JSONSchema{Type: "null"}
	}

	switch t.Kind {
	case schema.TypeKindString:
		return &JSONSchema{Type: "string"}
	case schema.TypeKindInt:
		return &JSONSchema{Type: "integer"}
	case schema.TypeKindFloat:
		return &JSONSchema{Type: "number"}
	case schema.TypeKindBool:
		return &JSONSchema{Type: "boolean"}
	case schema.TypeKindID:
		return &JSONSchema{Type: "string"}
	case schema.TypeKindList:
		return &JSONSchema{
			Type:  "array",
			Items: typeRefToJSONSchema(t.OfType, schemaDef),
		}
	case schema.TypeKindNonNull:
		return typeRefToJSONSchema(t.OfType, schemaDef)
	case schema.TypeKindEnum:
		enum := make([]interface{}, len(t.EnumVals))
		for i, v := range t.EnumVals {
			enum[i] = v
		}
		return &JSONSchema{
			Type: "string",
			Enum: enum,
		}
	case schema.TypeKindObject, schema.TypeKindInput:
		// Look up the type
		if schemaDef != nil {
			if typeDef, ok := schemaDef.GetType(t.Name); ok {
				return typeDefToJSONSchema(typeDef, schemaDef)
			}
		}
		return &JSONSchema{Type: "object"}
	default:
		return &JSONSchema{Type: "object"}
	}
}

// typeDefToJSONSchema converts a schema TypeDef to JSON Schema.
func typeDefToJSONSchema(t *schema.TypeDef, schemaDef *schema.SchemaDefinition) *JSONSchema {
	if t.Kind == schema.TypeKindInput || t.Kind == schema.TypeKindObject {
		s := &JSONSchema{
			Type:       "object",
			Properties: make(map[string]*JSONSchema),
		}
		for _, field := range t.Fields {
			s.Properties[field.Name] = typeRefToJSONSchema(field.Type, schemaDef)
		}
		return s
	}
	return &JSONSchema{Type: "object"}
}

// openAPISchemaToJSONSchema converts OpenAPI schema to JSON Schema.
func openAPISchemaToJSONSchema(s *openapi.SchemaObject) *JSONSchema {
	if s == nil {
		return &JSONSchema{Type: "object"}
	}

	schema := &JSONSchema{
		Description: s.Description,
	}

	// Map OpenAPI types to JSON Schema types
	switch s.Type {
	case "string":
		schema.Type = "string"
		if len(s.Enum) > 0 {
			schema.Enum = s.Enum
		}
	case "integer":
		schema.Type = "integer"
	case "number":
		schema.Type = "number"
	case "boolean":
		schema.Type = "boolean"
	case "array":
		schema.Type = "array"
		if s.Items != nil {
			schema.Items = openAPISchemaToJSONSchema(s.Items)
		}
	case "object":
		schema.Type = "object"
		if len(s.Properties) > 0 {
			schema.Properties = make(map[string]*JSONSchema)
			for name, prop := range s.Properties {
				schema.Properties[name] = openAPISchemaToJSONSchema(prop)
			}
		}
	default:
		schema.Type = "object"
	}

	if s.Default != nil {
		schema.Default = s.Default
	}

	return schema
}

// GenerateToolDescription generates a tool description.
func GenerateToolDescription(upstream, summary, operation string) string {
	parts := []string{}
	if upstream != "" {
		parts = append(parts, "["+upstream+"]")
	}
	if summary != "" {
		parts = append(parts, summary)
	} else if operation != "" {
		parts = append(parts, operation)
	}
	return strings.Join(parts, " ")
}
