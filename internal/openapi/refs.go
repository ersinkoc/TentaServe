package openapi

import (
	"fmt"
	"strings"
)

// Resolver handles $ref resolution in OpenAPI specs.
type Resolver struct {
	// spec is the OpenAPI specification being resolved
	spec *OpenAPISpec
	// visited tracks refs currently being resolved (for circular detection)
	visited map[string]bool
	// resolved caches already-resolved refs
	resolved map[string]interface{}
}

// NewResolver creates a new resolver for the given spec.
func NewResolver(spec *OpenAPISpec) *Resolver {
	return &Resolver{
		spec:     spec,
		visited:  make(map[string]bool),
		resolved: make(map[string]interface{}),
	}
}

// ResolveAll resolves all $refs in the spec.
// This should be called after parsing but before using the spec.
func (r *Resolver) ResolveAll() error {
	// Resolve refs in paths
	for pathKey, pathItem := range r.spec.Paths {
		if err := r.resolvePathItem(pathItem, fmt.Sprintf("paths.%s", pathKey)); err != nil {
			return err
		}
	}

	// Resolve refs in component schemas (for nested refs)
	if r.spec.Components != nil {
		for name, schema := range r.spec.Components.Schemas {
			if err := r.resolveSchema(schema, fmt.Sprintf("components.schemas.%s", name)); err != nil {
				return err
			}
		}
		// Also resolve refs in component responses
		for name, resp := range r.spec.Components.Responses {
			if err := r.resolveResponse(resp, fmt.Sprintf("components.responses.%s", name)); err != nil {
				return err
			}
		}
		// And component parameters
		for name, param := range r.spec.Components.Parameters {
			if err := r.resolveParameter(param, fmt.Sprintf("components.parameters.%s", name)); err != nil {
				return err
			}
		}
		// And component request bodies
		for name, body := range r.spec.Components.RequestBodies {
			if err := r.resolveRequestBody(body, fmt.Sprintf("components.requestBodies.%s", name)); err != nil {
				return err
			}
		}
	}

	return nil
}

// resolvePathItem resolves refs in a PathItem.
func (r *Resolver) resolvePathItem(item *PathItem, path string) error {
	// Resolve ref on the path item itself
	if item.Ref != "" {
		resolved, err := r.resolveRef(item.Ref, path)
		if err != nil {
			return err
		}
		if resolvedPath, ok := resolved.(*PathItem); ok {
			// Merge the resolved path item
			*r.mergePathItems(item, resolvedPath) = *resolvedPath
		}
	}

	// Resolve refs in operations
	ops := item.GetOperations()
	for method, op := range ops {
		if op != nil {
			if err := r.resolveOperation(op, fmt.Sprintf("%s.%s", path, method)); err != nil {
				return err
			}
		}
	}

	// Resolve refs in parameters
	for i, param := range item.Parameters {
		if err := r.resolveParameter(param, fmt.Sprintf("%s.parameters[%d]", path, i)); err != nil {
			return err
		}
	}

	return nil
}

// resolveOperation resolves refs in an Operation.
func (r *Resolver) resolveOperation(op *Operation, path string) error {
	// Resolve refs in parameters
	for i, param := range op.Parameters {
		if err := r.resolveParameter(param, fmt.Sprintf("%s.parameters[%d]", path, i)); err != nil {
			return err
		}
	}

	// Resolve request body
	if op.RequestBody != nil {
		if err := r.resolveRequestBody(op.RequestBody, path+".requestBody"); err != nil {
			return err
		}
	}

	// Resolve refs in responses
	for code, resp := range op.Responses {
		if err := r.resolveResponse(resp, fmt.Sprintf("%s.responses.%s", path, code)); err != nil {
			return err
		}
	}

	return nil
}

// resolveParameter resolves refs in a Parameter.
func (r *Resolver) resolveParameter(param *Parameter, path string) error {
	if param == nil {
		return nil
	}

	// Resolve parameter $ref
	if param.Ref != "" {
		resolved, err := r.resolveRef(param.Ref, path)
		if err != nil {
			return err
		}
		if resolvedParam, ok := resolved.(*Parameter); ok {
			*param = *resolvedParam
		}
		return nil // Parameter refs replace the entire object
	}

	// Resolve schema ref
	if param.Schema != nil {
		if err := r.resolveSchema(param.Schema, path+".schema"); err != nil {
			return err
		}
	}

	return nil
}

// resolveRequestBody resolves refs in a RequestBody.
func (r *Resolver) resolveRequestBody(body *RequestBody, path string) error {
	if body == nil {
		return nil
	}

	// Resolve request body $ref
	if body.Ref != "" {
		resolved, err := r.resolveRef(body.Ref, path)
		if err != nil {
			return err
		}
		if resolvedBody, ok := resolved.(*RequestBody); ok {
			*body = *resolvedBody
		}
		return nil // Request body refs replace the entire object
	}

	// Resolve schema refs in content
	for mediaType, mt := range body.Content {
		if mt != nil && mt.Schema != nil {
			if err := r.resolveSchema(mt.Schema, fmt.Sprintf("%s.content.%s.schema", path, mediaType)); err != nil {
				return err
			}
		}
	}

	return nil
}

// resolveResponse resolves refs in a Response.
func (r *Resolver) resolveResponse(resp *Response, path string) error {
	if resp == nil {
		return nil
	}

	// Resolve response $ref
	if resp.Ref != "" {
		resolved, err := r.resolveRef(resp.Ref, path)
		if err != nil {
			return err
		}
		if resolvedResp, ok := resolved.(*Response); ok {
			*resp = *resolvedResp
		}
		return nil // Response refs replace the entire object
	}

	// Resolve schema refs in content
	for mediaType, mt := range resp.Content {
		if mt != nil && mt.Schema != nil {
			if err := r.resolveSchema(mt.Schema, fmt.Sprintf("%s.content.%s.schema", path, mediaType)); err != nil {
				return err
			}
		}
	}

	return nil
}

// resolveSchema resolves refs in a SchemaObject.
func (r *Resolver) resolveSchema(schema *SchemaObject, path string) error {
	if schema == nil {
		return nil
	}

	// Resolve schema $ref
	if schema.Ref != "" {
		ref := schema.Ref
		resolved, err := r.resolveRef(ref, path)
		if err != nil {
			// If it's a circular reference, just clear the ref and continue
			if _, ok := err.(*CircularError); ok {
				schema.Ref = ""
				return nil
			}
			return err
		}
		if resolvedSchema, ok := resolved.(*SchemaObject); ok {
			// Copy all fields from resolved schema
			*schema = *resolvedSchema
			// Clear the ref
			schema.Ref = ""
			// Cache the schema pointer
			r.resolved[ref] = schema
			// Note: We do NOT recursively resolve here.
			// The properties will be resolved when they are iterated over
			// in the ResolveAll loop, and proper circular ref detection
			// will be applied at that time.
		}
		return nil // Schema refs replace the entire object
	}

	// Resolve items (for arrays)
	if schema.Items != nil {
		if err := r.resolveSchema(schema.Items, path+".items"); err != nil {
			return err
		}
	}

	// Resolve properties (for objects)
	for propName, prop := range schema.Properties {
		if err := r.resolveSchema(prop, fmt.Sprintf("%s.properties.%s", path, propName)); err != nil {
			return err
		}
	}

	// Resolve allOf
	for i, s := range schema.AllOf {
		if err := r.resolveSchema(s, fmt.Sprintf("%s.allOf[%d]", path, i)); err != nil {
			return err
		}
	}

	// Resolve oneOf
	for i, s := range schema.OneOf {
		if err := r.resolveSchema(s, fmt.Sprintf("%s.oneOf[%d]", path, i)); err != nil {
			return err
		}
	}

	// Resolve anyOf
	for i, s := range schema.AnyOf {
		if err := r.resolveSchema(s, fmt.Sprintf("%s.anyOf[%d]", path, i)); err != nil {
			return err
		}
	}

	// Resolve not
	if schema.Not != nil {
		if err := r.resolveSchema(schema.Not, path+".not"); err != nil {
			return err
		}
	}

	// Resolve additionalProperties
	if schema.AdditionalProperties != nil {
		if err := r.resolveSchema(schema.AdditionalProperties, path+".additionalProperties"); err != nil {
			return err
		}
	}

	return nil
}

// resolveRef resolves a reference string to its target object.
func (r *Resolver) resolveRef(ref string, contextPath string) (interface{}, error) {
	// Check for circular reference
	if r.visited[ref] {
		return nil, &CircularError{Ref: ref}
	}

	// Check cache
	if resolved, ok := r.resolved[ref]; ok {
		return resolved, nil
	}

	// Mark as visited
	r.visited[ref] = true
	defer func() { r.visited[ref] = false }()

	// Parse the reference
	target, err := r.parseRef(ref)
	if err != nil {
		return nil, &ParseError{
			Path:    contextPath,
			Message: fmt.Sprintf("invalid reference %q: %v", ref, err),
		}
	}

	// Resolve the reference
	resolved, err := target.resolve(r.spec)
	if err != nil {
		return nil, &ParseError{
			Path:    contextPath,
			Message: fmt.Sprintf("cannot resolve reference %q: %v", ref, err),
		}
	}

	// Cache the resolved value
	r.resolved[ref] = resolved

	return resolved, nil
}

// refTarget represents a parsed reference.
type refTarget struct {
	// source is the source document (empty for local refs)
	source string
	// path is the JSON pointer path within the document
	path []string
}

// parseRef parses a reference string into its components.
// Supports:
//   - Local refs: #/components/schemas/User
//   - External refs (not fully supported): ./models.yaml#/User
func (r *Resolver) parseRef(ref string) (*refTarget, error) {
	// Currently only supporting local refs
	if !strings.HasPrefix(ref, "#") {
		return nil, fmt.Errorf("external references not supported: %s", ref)
	}

	// Remove the leading #
	ptr := ref[1:]

	// Parse JSON pointer
	if !strings.HasPrefix(ptr, "/") {
		return nil, fmt.Errorf("invalid JSON pointer: %s", ptr)
	}

	// Split path and decode
	parts := strings.Split(ptr[1:], "/")
	for i, part := range parts {
		parts[i] = decodeJSONPointer(part)
	}

	return &refTarget{
		source: "",
		path:   parts,
	}, nil
}

// resolve resolves the reference against the spec.
func (t *refTarget) resolve(spec *OpenAPISpec) (interface{}, error) {
	if len(t.path) == 0 {
		return spec, nil
	}

	// Navigate through the path
	current := interface{}(spec)

	for _, part := range t.path {
		switch v := current.(type) {
		case *OpenAPISpec:
			current = resolveInSpec(v, part)
		case *Components:
			current = resolveInComponents(v, part)
		case map[string]*SchemaObject:
			if s, ok := v[part]; ok {
				current = s
			} else {
				return nil, fmt.Errorf("schema %q not found", part)
			}
		case map[string]*Response:
			if r, ok := v[part]; ok {
				current = r
			} else {
				return nil, fmt.Errorf("response %q not found", part)
			}
		case map[string]*Parameter:
			if p, ok := v[part]; ok {
				current = p
			} else {
				return nil, fmt.Errorf("parameter %q not found", part)
			}
		case map[string]*RequestBody:
			if rb, ok := v[part]; ok {
				current = rb
			} else {
				return nil, fmt.Errorf("request body %q not found", part)
			}
		default:
			return nil, fmt.Errorf("cannot navigate in %T", current)
		}

		if current == nil {
			return nil, fmt.Errorf("path component %q not found", part)
		}
	}

	return current, nil
}

// resolveInSpec resolves a path component in the OpenAPISpec.
func resolveInSpec(spec *OpenAPISpec, part string) interface{} {
	switch part {
	case "components":
		return spec.Components
	case "paths":
		return spec.Paths
	default:
		return nil
	}
}

// resolveInComponents resolves a path component in Components.
func resolveInComponents(comp *Components, part string) interface{} {
	switch part {
	case "schemas":
		return comp.Schemas
	case "responses":
		return comp.Responses
	case "parameters":
		return comp.Parameters
	case "requestBodies":
		return comp.RequestBodies
	case "headers":
		return comp.Headers
	case "examples":
		return comp.Examples
	case "links":
		return comp.Links
	case "securitySchemes":
		return comp.SecuritySchemes
	default:
		return nil
	}
}

// decodeJSONPointer decodes a JSON pointer path component.
// JSON pointers use ~0 for ~ and ~1 for /.
func decodeJSONPointer(s string) string {
	s = strings.ReplaceAll(s, "~1", "/")
	s = strings.ReplaceAll(s, "~0", "~")
	return s
}

// mergePathItems merges a resolved path item into the target.
// The target retains its own values, only filling in from resolved where empty.
func (r *Resolver) mergePathItems(target, source *PathItem) *PathItem {
	// For now, we just replace the entire item
	// A more sophisticated merge could be implemented if needed
	return source
}

// CircularError represents a circular reference error.
type CircularError struct {
	Ref string
}

func (e *CircularError) Error() string {
	return fmt.Sprintf("circular reference detected: %s", e.Ref)
}
