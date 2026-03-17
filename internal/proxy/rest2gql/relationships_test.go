package rest2gql

import (
	"testing"

	"github.com/ersinkoc/tentaserve/internal/openapi"
)

func TestNewRelationshipDetector(t *testing.T) {
	detector := NewRelationshipDetector()
	if detector == nil {
		t.Fatal("NewRelationshipDetector returned nil")
	}
	if detector.pathParamPattern == nil {
		t.Error("Expected pathParamPattern to be initialized")
	}
	if detector.resourceExtractor == nil {
		t.Error("Expected resourceExtractor to be initialized")
	}
}

func TestDetectRelationships(t *testing.T) {
	detector := NewRelationshipDetector()

	spec := &openapi.OpenAPISpec{
		OpenAPI: "3.0.0",
		Info:    openapi.Info{Title: "Test API", Version: "1.0.0"},
		Paths: map[string]*openapi.PathItem{
			"/users": {
				Get: &openapi.Operation{Summary: "List users"},
			},
			"/users/{id}": {
				Get: &openapi.Operation{Summary: "Get user"},
			},
			"/users/{id}/posts": {
				Get: &openapi.Operation{Summary: "List user posts"},
			},
			"/posts": {
				Get: &openapi.Operation{Summary: "List all posts"},
			},
		},
	}

	relationships := detector.DetectRelationships(spec)
	if len(relationships) == 0 {
		t.Fatal("Expected relationships to be detected")
	}

	// Check if /users/{id}/posts relationship was detected
	found := false
	for _, rel := range relationships {
		if rel.ParentPath == "/users/{id}" && rel.ChildPath == "/users/{id}/posts" {
			found = true
			if rel.ParentResource != "User" {
				t.Errorf("Expected ParentResource 'User', got '%s'", rel.ParentResource)
			}
			if rel.ChildResource != "Post" {
				t.Errorf("Expected ChildResource 'Post', got '%s'", rel.ChildResource)
			}
			if rel.Type != RelationshipOneToMany {
				t.Errorf("Expected OneToMany relationship, got %s", rel.Type)
			}
			if rel.FieldName != "posts" {
				t.Errorf("Expected FieldName 'posts', got '%s'", rel.FieldName)
			}
		}
	}
	if !found {
		t.Error("Expected to find /users/{id} -> /users/{id}/posts relationship")
	}
}

func TestIsChildPath(t *testing.T) {
	detector := NewRelationshipDetector()

	tests := []struct {
		parent   string
		child    string
		expected bool
	}{
		{"/users", "/users/{id}", true},
		{"/users", "/users/{id}/posts", true},
		{"/users/{id}", "/users/{id}/posts", true},
		{"/users", "/users", false},
		{"/users", "/posts", false},
		{"/users", "/users/{id}/posts/{postId}", false},
		{"/users/{id}", "/users/{id}/posts/{postId}", false},
	}

	for _, tt := range tests {
		t.Run(tt.parent+"->"+tt.child, func(t *testing.T) {
			got := detector.isChildPath(tt.parent, tt.child)
			if got != tt.expected {
				t.Errorf("isChildPath(%q, %q) = %v, want %v",
					tt.parent, tt.child, got, tt.expected)
			}
		})
	}
}

func TestExtractResourceName(t *testing.T) {
	detector := NewRelationshipDetector()

	tests := []struct {
		path     string
		expected string
	}{
		{"/users", "User"},
		{"/users/{id}", "User"},
		{"/users/{id}/posts", "Post"},
		{"/user-profiles", "UserProfil"},  // Simple singularize behavior
		{"/", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := detector.extractResourceName(tt.path)
			if got != tt.expected {
				t.Errorf("extractResourceName(%q) = %q, want %q",
					tt.path, got, tt.expected)
			}
		})
	}
}

func TestExtractLastParam(t *testing.T) {
	detector := NewRelationshipDetector()

	tests := []struct {
		path     string
		expected string
	}{
		{"/users/{id}", "id"},
		{"/users/{userId}/posts", "userId"},
		{"/users/{id}/posts/{postId}", "postId"},
		{"/users", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := detector.extractLastParam(tt.path)
			if got != tt.expected {
				t.Errorf("extractLastParam(%q) = %q, want %q",
					tt.path, got, tt.expected)
			}
		})
	}
}

func TestSimpleSingularize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"users", "user"},
		{"posts", "post"},
		{"categories", "category"},
		{"boxes", "box"},
		{"user", "user"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := simpleSingularize(tt.input)
			if got != tt.expected {
				t.Errorf("simpleSingularize(%q) = %q, want %q",
					tt.input, got, tt.expected)
			}
		})
	}
}

func TestSimplePluralize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"user", "users"},
		{"post", "posts"},
		{"category", "categories"},
		{"box", "boxes"},
		{"users", "users"},  // Already plural - should stay
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := simplePluralize(tt.input)
			if got != tt.expected {
				t.Errorf("simplePluralize(%q) = %q, want %q",
					tt.input, got, tt.expected)
			}
		})
	}
}

func TestRelationshipType_String(t *testing.T) {
	tests := []struct {
		relType  RelationshipType
		expected string
	}{
		{RelationshipNone, "none"},
		{RelationshipOneToMany, "oneToMany"},
		{RelationshipManyToOne, "manyToOne"},
		{RelationshipOneToOne, "oneToOne"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := tt.relType.String()
			if got != tt.expected {
				t.Errorf("RelationshipType.String() = %q, want %q",
					got, tt.expected)
			}
		})
	}
}

func TestPathEndsWithParam(t *testing.T) {
	detector := NewRelationshipDetector()

	tests := []struct {
		path     string
		expected bool
	}{
		{"/users/{id}", true},
		{"/users", false},
		{"/users/{id}/posts", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := detector.pathEndsWithParam(tt.path)
			if got != tt.expected {
				t.Errorf("pathEndsWithParam(%q) = %v, want %v",
					tt.path, got, tt.expected)
			}
		})
	}
}

func TestParsePath(t *testing.T) {
	detector := NewRelationshipDetector()

	tests := []struct {
		path     string
		expected []string
	}{
		{"/users/{id}/posts", []string{"users", "{id}", "posts"}},
		{"/users", []string{"users"}},
		{"/", nil},
		{"", nil},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := detector.parsePath(tt.path)
			if len(got) != len(tt.expected) {
				t.Errorf("parsePath(%q) = %v, want %v",
					tt.path, got, tt.expected)
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("parsePath(%q)[%d] = %q, want %q",
						tt.path, i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestDetectRelationships_EmptySpec(t *testing.T) {
	detector := NewRelationshipDetector()

	// Test nil spec
	rels := detector.DetectRelationships(nil)
	if rels != nil {
		t.Error("Expected nil for nil spec")
	}

	// Test empty paths
	rels = detector.DetectRelationships(&openapi.OpenAPISpec{
		Paths: map[string]*openapi.PathItem{},
	})
	if rels != nil {
		t.Error("Expected nil for empty paths")
	}
}
