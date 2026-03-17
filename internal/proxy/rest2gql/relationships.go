package rest2gql

import (
	"regexp"
	"strings"

	"github.com/ersinkoc/tentaserve/internal/openapi"
)

// RelationshipType represents the type of relationship between resources.
type RelationshipType int

const (
	// RelationshipNone means no relationship detected
	RelationshipNone RelationshipType = 0
	// RelationshipOneToMany means parent has many children
	RelationshipOneToMany RelationshipType = 1
	// RelationshipManyToOne means child belongs to parent
	RelationshipManyToOne RelationshipType = 2
	// RelationshipOneToOne means one-to-one relationship
	RelationshipOneToOne RelationshipType = 3
)

func (r RelationshipType) String() string {
	switch r {
	case RelationshipOneToMany:
		return "oneToMany"
	case RelationshipManyToOne:
		return "manyToOne"
	case RelationshipOneToOne:
		return "oneToOne"
	default:
		return "none"
	}
}

// Relationship represents a detected parent-child relationship between paths.
type Relationship struct {
	// ParentPath is the parent resource path (e.g., "/users")
	ParentPath string
	// ChildPath is the child resource path (e.g., "/users/{id}/posts")
	ChildPath string
	// ParentResource is the parent resource name (e.g., "User")
	ParentResource string
	// ChildResource is the child resource name (e.g., "Post")
	ChildResource string
	// ParentParam is the parameter name in parent path (e.g., "id")
	ParentParam string
	// Type is the relationship type
	Type RelationshipType
	// FieldName is the GraphQL field name for this relationship (e.g., "posts")
	FieldName string
}

// RelationshipDetector detects parent-child relationships from OpenAPI paths.
type RelationshipDetector struct {
	// PathParamPattern matches {param} style path parameters
	pathParamPattern *regexp.Regexp
	// ResourceExtractor extracts resource names from paths
	resourceExtractor *ResourceExtractor
}

// ResourceExtractor extracts resource names from REST paths.
type ResourceExtractor struct {
	singularizer func(string) string
	pluralizer   func(string) string
}

// NewRelationshipDetector creates a new relationship detector.
func NewRelationshipDetector() *RelationshipDetector {
	return &RelationshipDetector{
		pathParamPattern: regexp.MustCompile(`\{([^}]+)\}`),
		resourceExtractor: &ResourceExtractor{
			singularizer: simpleSingularize,
			pluralizer:   simplePluralize,
		},
	}
}

// DetectRelationships analyzes paths and detects parent-child relationships.
// Returns relationships like /users/{id}/posts being detected as User -> posts.
func (d *RelationshipDetector) DetectRelationships(spec *openapi.OpenAPISpec) []*Relationship {
	if spec == nil || len(spec.Paths) == 0 {
		return nil
	}

	relationships := make([]*Relationship, 0)
	paths := d.getSortedPaths(spec.Paths)

	for i, parentPath := range paths {
		parentSegments := d.parsePath(parentPath)
		if len(parentSegments) == 0 {
			continue
		}

		// Look for child paths that extend this parent path
		for _, childPath := range paths[i+1:] {
			if d.isChildPath(parentPath, childPath) {
				rel := d.buildRelationship(parentPath, childPath, spec.Paths)
				if rel != nil {
					relationships = append(relationships, rel)
				}
			}
		}
	}

	return relationships
}

// isChildPath checks if childPath is a child of parentPath.
// e.g., /users/{id}/posts is a child of /users/{id}
// e.g., /users/{id}/posts/{postId} is NOT a direct child of /users (but is of /users/{id}/posts)
func (d *RelationshipDetector) isChildPath(parentPath, childPath string) bool {
	// Must start with parent path
	if !strings.HasPrefix(childPath, parentPath) {
		return false
	}

	// Must have additional segments after parent
	childRemainder := strings.TrimPrefix(childPath, parentPath)
	if childRemainder == "" || childRemainder == "/" {
		return false
	}

	// Must start with / (direct child) or after a path param
	if !strings.HasPrefix(childRemainder, "/") {
		return false
	}

	// Remove the leading /
	childRemainder = strings.TrimPrefix(childRemainder, "/")

	// Check if it's a direct child (only one segment after parent, or two if first is param)
	segments := strings.Split(childRemainder, "/")

	// Single segment is direct child
	if len(segments) == 1 {
		return true
	}

	// Two segments: if first is a param, check second isn't a param
	if len(segments) == 2 {
		if strings.HasPrefix(segments[0], "{") {
			// /parent/{param}/resource - direct child
			return !strings.HasPrefix(segments[1], "{")
		}
	}

	return false
}

// buildRelationship creates a Relationship from detected parent-child paths.
func (d *RelationshipDetector) buildRelationship(parentPath, childPath string, paths map[string]*openapi.PathItem) *Relationship {
	parentResource := d.extractResourceName(parentPath)
	childResource := d.extractResourceName(childPath)

	if parentResource == "" || childResource == "" {
		return nil
	}

	// Determine relationship type
	relType := d.determineRelationshipType(parentPath, childPath)

	// Determine field name (usually plural child resource in parent context)
	fieldName := d.determineFieldName(childResource, relType)

	// Extract parent param (e.g., "id" from /users/{id})
	parentParam := d.extractLastParam(parentPath)

	return &Relationship{
		ParentPath:     parentPath,
		ChildPath:      childPath,
		ParentResource: parentResource,
		ChildResource:  childResource,
		ParentParam:    parentParam,
		Type:           relType,
		FieldName:      fieldName,
	}
}

// determineRelationshipType determines the type of relationship.
func (d *RelationshipDetector) determineRelationshipType(parentPath, childPath string) RelationshipType {
	// If child path is like /parents/{id}/child - it's one-to-many from parent perspective
	if d.pathEndsWithParam(parentPath) {
		return RelationshipOneToMany
	}

	// If child path is like /parent/child/{id} - it's one-to-one or many-to-one
	if strings.Contains(childPath, "{") {
		return RelationshipManyToOne
	}

	return RelationshipOneToMany
}

// determineFieldName generates a GraphQL field name for the relationship.
func (d *RelationshipDetector) determineFieldName(childResource string, relType RelationshipType) string {
	switch relType {
	case RelationshipOneToMany:
		// Parent has many children - use plural form (camelCase)
		return toCamelCase(d.resourceExtractor.pluralizer(childResource))
	case RelationshipOneToOne:
		// Parent has one child - use singular form (camelCase)
		return toCamelCase(childResource)
	case RelationshipManyToOne:
		// Child belongs to parent - use singular form (camelCase)
		return toCamelCase(childResource)
	default:
		return toCamelCase(childResource)
	}
}

// toCamelCase converts PascalCase to camelCase.
func toCamelCase(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// extractResourceName extracts the resource name from a path.
// e.g., /users/{id} -> "User", /users/{id}/posts -> "Post"
func (d *RelationshipDetector) extractResourceName(path string) string {
	segments := d.parsePath(path)
	if len(segments) == 0 {
		return ""
	}

	// Get last non-param segment
	for i := len(segments) - 1; i >= 0; i-- {
		seg := segments[i]
		if !strings.HasPrefix(seg, "{") {
			singular := d.resourceExtractor.singularizer(seg)
			return toPascalCase(singular)
		}
	}

	return ""
}

// toPascalCase converts a string to PascalCase (simple implementation).
func toPascalCase(s string) string {
	if s == "" {
		return ""
	}
	// Split by hyphen or underscore and capitalize each part
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_'
	})
	result := ""
	for _, p := range parts {
		if len(p) > 0 {
			result += strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return result
}

// extractLastParam extracts the last path parameter name.
func (d *RelationshipDetector) extractLastParam(path string) string {
	matches := d.pathParamPattern.FindAllStringSubmatch(path, -1)
	if len(matches) > 0 {
		last := matches[len(matches)-1]
		if len(last) > 1 {
			return last[1]
		}
	}
	return ""
}

// pathEndsWithParam checks if path ends with a path parameter.
func (d *RelationshipDetector) pathEndsWithParam(path string) bool {
	return strings.HasSuffix(path, "}")
}

// parsePath splits a path into segments.
func (d *RelationshipDetector) parsePath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

// getSortedPaths returns sorted path keys for consistent processing.
func (d *RelationshipDetector) getSortedPaths(paths map[string]*openapi.PathItem) []string {
	result := make([]string, 0, len(paths))
	for path := range paths {
		result = append(result, path)
	}
	// Sort by length (shortest first) so parents come before children
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if len(result[i]) > len(result[j]) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}

// simpleSingularize converts plural to singular (basic implementation).
func simpleSingularize(s string) string {
	if strings.HasSuffix(s, "ies") && len(s) > 3 {
		return strings.TrimSuffix(s, "ies") + "y"
	}
	if strings.HasSuffix(s, "es") && len(s) > 2 {
		return strings.TrimSuffix(s, "es")
	}
	if strings.HasSuffix(s, "s") && !strings.HasSuffix(s, "ss") && len(s) > 1 {
		return strings.TrimSuffix(s, "s")
	}
	return s
}

// simplePluralize converts singular to plural (basic implementation).
func simplePluralize(s string) string {
	// Already ends with 's' - assume already plural
	if strings.HasSuffix(s, "s") {
		return s
	}
	if strings.HasSuffix(s, "y") && !strings.HasSuffix(s, "ay") && !strings.HasSuffix(s, "ey") && !strings.HasSuffix(s, "oy") && !strings.HasSuffix(s, "uy") {
		return strings.TrimSuffix(s, "y") + "ies"
	}
	if strings.HasSuffix(s, "x") || strings.HasSuffix(s, "ch") || strings.HasSuffix(s, "sh") {
		return s + "es"
	}
	return s + "s"
}

// HasParentRelationship checks if a path has a detected parent.
func (r *Relationship) HasParentRelationship() bool {
	return r.Type != RelationshipNone && r.ParentPath != ""
}

// IsNestedResource returns true if this is a nested resource relationship.
func (r *Relationship) IsNestedResource() bool {
	return strings.Contains(r.ChildPath, "{") && strings.Contains(r.ChildPath, r.ParentPath)
}
