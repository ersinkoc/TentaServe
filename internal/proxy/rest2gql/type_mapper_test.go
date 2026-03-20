package rest2gql

import (
	"testing"

	"github.com/ersinkoc/tentaserve/internal/openapi"
)

func TestNewTypeMapper(t *testing.T) {
	mapper := NewTypeMapper()
	if mapper == nil {
		t.Fatal("NewTypeMapper returned nil")
	}
	if mapper.mappings == nil {
		t.Error("expected mappings map to be initialized")
	}
}

func TestTypeMapper_MapOpenAPIType(t *testing.T) {
	mapper := NewTypeMapper()

	t.Run("nil schema", func(t *testing.T) {
		_, err := mapper.MapOpenAPIType(nil)
		if err == nil {
			t.Error("expected error for nil schema")
		}
	})

	t.Run("ref type", func(t *testing.T) {
		s := &openapi.SchemaObject{Ref: "#/components/schemas/User"}
		ref, err := mapper.MapOpenAPIType(s)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ref.Name != "User" {
			t.Errorf("expected name User, got %s", ref.Name)
		}
	})

	t.Run("string type", func(t *testing.T) {
		s := &openapi.SchemaObject{Type: "string"}
		ref, err := mapper.MapOpenAPIType(s)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ref == nil {
			t.Fatal("expected non-nil TypeRef")
		}
	})

	t.Run("integer type", func(t *testing.T) {
		s := &openapi.SchemaObject{Type: "integer"}
		ref, err := mapper.MapOpenAPIType(s)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ref == nil {
			t.Fatal("expected non-nil TypeRef")
		}
	})

	t.Run("number type", func(t *testing.T) {
		s := &openapi.SchemaObject{Type: "number"}
		ref, err := mapper.MapOpenAPIType(s)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ref == nil {
			t.Fatal("expected non-nil TypeRef")
		}
	})

	t.Run("boolean type", func(t *testing.T) {
		s := &openapi.SchemaObject{Type: "boolean"}
		ref, err := mapper.MapOpenAPIType(s)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ref == nil {
			t.Fatal("expected non-nil TypeRef")
		}
	})

	t.Run("array type", func(t *testing.T) {
		s := &openapi.SchemaObject{
			Type:  "array",
			Items: &openapi.SchemaObject{Type: "string"},
		}
		ref, err := mapper.MapOpenAPIType(s)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ref == nil {
			t.Fatal("expected non-nil TypeRef")
		}
	})

	t.Run("object type", func(t *testing.T) {
		s := &openapi.SchemaObject{Type: "object"}
		ref, err := mapper.MapOpenAPIType(s)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ref == nil {
			t.Fatal("expected non-nil TypeRef")
		}
	})

	t.Run("unknown type defaults to string", func(t *testing.T) {
		s := &openapi.SchemaObject{Type: "unknown"}
		ref, err := mapper.MapOpenAPIType(s)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ref == nil {
			t.Fatal("expected non-nil TypeRef")
		}
	})

	t.Run("string with uuid format", func(t *testing.T) {
		s := &openapi.SchemaObject{Type: "string", Format: "uuid"}
		ref, err := mapper.MapOpenAPIType(s)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ref == nil {
			t.Fatal("expected non-nil TypeRef")
		}
	})

	t.Run("string with date-time format", func(t *testing.T) {
		s := &openapi.SchemaObject{Type: "string", Format: "date-time"}
		ref, err := mapper.MapOpenAPIType(s)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ref == nil {
			t.Fatal("expected non-nil TypeRef")
		}
	})

	t.Run("string with enum", func(t *testing.T) {
		s := &openapi.SchemaObject{
			Type: "string",
			Enum: []interface{}{"active", "inactive"},
		}
		ref, err := mapper.MapOpenAPIType(s)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ref == nil {
			t.Fatal("expected non-nil TypeRef")
		}
	})

	t.Run("object with additionalProperties", func(t *testing.T) {
		s := &openapi.SchemaObject{
			Type:                 "object",
			AdditionalProperties: &openapi.SchemaObject{Type: "string"},
		}
		ref, err := mapper.MapOpenAPIType(s)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ref == nil {
			t.Fatal("expected non-nil TypeRef")
		}
	})
}

func TestTypeMapper_AddCustomMapping(t *testing.T) {
	mapper := NewTypeMapper()
	mapper.AddCustomMapping("date-time", "DateTime")

	// Now map a date-time format
	s := &openapi.SchemaObject{Type: "string", Format: "date-time"}
	ref, err := mapper.MapOpenAPIType(s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref == nil {
		t.Fatal("expected non-nil TypeRef")
	}
}

func TestOpenAPIToGraphQLType(t *testing.T) {
	tests := []struct {
		oaType   string
		format   string
		expected string
	}{
		{"string", "", "String"},
		{"string", "uuid", "ID"},
		{"integer", "", "Int"},
		{"number", "", "Float"},
		{"boolean", "", "Boolean"},
		{"array", "", "List"},
		{"object", "", "Object"},
		{"unknown", "", "String"},
	}

	for _, tt := range tests {
		t.Run(tt.oaType+"_"+tt.format, func(t *testing.T) {
			result := OpenAPIToGraphQLType(tt.oaType, tt.format)
			if result != tt.expected {
				t.Errorf("OpenAPIToGraphQLType(%q, %q) = %q, want %q", tt.oaType, tt.format, result, tt.expected)
			}
		})
	}
}

func TestGraphQLToOpenAPIType(t *testing.T) {
	tests := []struct {
		gqlType        string
		expectedType   string
		expectedFormat string
	}{
		{"String", "string", ""},
		{"Int", "integer", "int32"},
		{"Float", "number", "float64"},
		{"Boolean", "boolean", ""},
		{"ID", "string", "uuid"},
		{"Unknown", "string", ""},
	}

	for _, tt := range tests {
		t.Run(tt.gqlType, func(t *testing.T) {
			typ, format := GraphQLToOpenAPIType(tt.gqlType)
			if typ != tt.expectedType {
				t.Errorf("type: expected %q, got %q", tt.expectedType, typ)
			}
			if format != tt.expectedFormat {
				t.Errorf("format: expected %q, got %q", tt.expectedFormat, format)
			}
		})
	}
}

func TestIsComplexType(t *testing.T) {
	tests := []struct {
		name     string
		schema   *openapi.SchemaObject
		expected bool
	}{
		{"nil", nil, false},
		{"object", &openapi.SchemaObject{Type: "object"}, true},
		{"string", &openapi.SchemaObject{Type: "string"}, false},
		{"array of objects", &openapi.SchemaObject{Type: "array", Items: &openapi.SchemaObject{Type: "object"}}, true},
		{"array of strings", &openapi.SchemaObject{Type: "array", Items: &openapi.SchemaObject{Type: "string"}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsComplexType(tt.schema)
			if result != tt.expected {
				t.Errorf("IsComplexType() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetNestedType(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		result := GetNestedType(nil)
		if result != nil {
			t.Error("expected nil for nil input")
		}
	})

	t.Run("array returns items", func(t *testing.T) {
		items := &openapi.SchemaObject{Type: "string"}
		s := &openapi.SchemaObject{Type: "array", Items: items}
		result := GetNestedType(s)
		if result != items {
			t.Error("expected items schema")
		}
	})

	t.Run("non-array returns self", func(t *testing.T) {
		s := &openapi.SchemaObject{Type: "object"}
		result := GetNestedType(s)
		if result != s {
			t.Error("expected self for non-array")
		}
	})
}

func TestCoerceValue(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		oaType  string
		format  string
		wantErr bool
	}{
		{"string", "hello", "string", "", false},
		{"integer valid", "42", "integer", "", false},
		{"integer invalid", "abc", "integer", "", true},
		{"number valid", "3.14", "number", "", false},
		{"number invalid", "xyz", "number", "", true},
		{"boolean true", "true", "boolean", "", false},
		{"boolean false", "false", "boolean", "", false},
		{"boolean 1", "1", "boolean", "", false},
		{"unknown type", "data", "unknown", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CoerceValue(tt.value, tt.oaType, tt.format)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == nil {
				t.Error("expected non-nil result")
			}
		})
	}
}

func TestExtractRefName(t *testing.T) {
	tests := []struct {
		ref      string
		expected string
	}{
		{"#/components/schemas/User", "User"},
		{"#/components/schemas/UserProfile", "UserProfile"},
		{"#/components/schemas/my-type", "MyType"},
		{"SimpleRef", "SimpleRef"},
	}

	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			result := extractRefName(tt.ref)
			if result != tt.expected {
				t.Errorf("extractRefName(%q) = %q, want %q", tt.ref, result, tt.expected)
			}
		})
	}
}

func TestGenerateEnumName(t *testing.T) {
	t.Run("with title", func(t *testing.T) {
		s := &openapi.SchemaObject{Title: "user_status"}
		result := generateEnumName(s)
		if result != "UserStatus" {
			t.Errorf("expected UserStatus, got %s", result)
		}
	})

	t.Run("without title", func(t *testing.T) {
		s := &openapi.SchemaObject{}
		result := generateEnumName(s)
		if result != "Enum" {
			t.Errorf("expected Enum, got %s", result)
		}
	})
}
