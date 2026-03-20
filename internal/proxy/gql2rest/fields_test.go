package gql2rest

import (
	"strings"
	"testing"
)

func TestNewFieldParamParser(t *testing.T) {
	parser := NewFieldParamParser()
	if parser == nil {
		t.Fatal("NewFieldParamParser returned nil")
	}
	if parser.exclusions == nil {
		t.Error("expected exclusions map to be initialized")
	}
}

func TestFieldParamParser_Parse(t *testing.T) {
	parser := NewFieldParamParser()

	t.Run("empty string returns include all", func(t *testing.T) {
		result := parser.Parse("")
		if !result.IncludeAll {
			t.Error("expected IncludeAll for empty string")
		}
	})

	t.Run("wildcard returns include all", func(t *testing.T) {
		result := parser.Parse("*")
		if !result.IncludeAll {
			t.Error("expected IncludeAll for wildcard")
		}
	})

	t.Run("exclusions", func(t *testing.T) {
		result := parser.Parse("-password,secret")
		if !result.IncludeAll {
			t.Error("expected IncludeAll for exclusions")
		}
		if result.Exclusions == nil {
			t.Fatal("expected exclusions map")
		}
		if !result.Exclusions["password"] {
			t.Error("expected password to be excluded")
		}
		if !result.Exclusions["secret"] {
			t.Error("expected secret to be excluded")
		}
	})

	t.Run("normal fields", func(t *testing.T) {
		result := parser.Parse("name,email")
		if result.IncludeAll {
			t.Error("expected IncludeAll to be false for normal fields")
		}
		if len(result.Selectors) != 2 {
			t.Errorf("expected 2 selectors, got %d", len(result.Selectors))
		}
	})

	t.Run("nested fields", func(t *testing.T) {
		result := parser.Parse("name,posts.title,posts.content")
		if result.IncludeAll {
			t.Error("expected IncludeAll to be false")
		}
		if len(result.Selectors) == 0 {
			t.Fatal("expected selectors")
		}
	})
}

func TestFieldParamResult_HasField(t *testing.T) {
	t.Run("include all no exclusions", func(t *testing.T) {
		result := &FieldParamResult{IncludeAll: true}
		if !result.HasField("anything") {
			t.Error("expected HasField to be true for IncludeAll")
		}
	})

	t.Run("include all with exclusions", func(t *testing.T) {
		result := &FieldParamResult{
			IncludeAll: true,
			Exclusions: map[string]bool{"password": true},
		}
		if result.HasField("password") {
			t.Error("expected password to be excluded")
		}
		if !result.HasField("name") {
			t.Error("expected name to be included")
		}
	})

	t.Run("specific fields", func(t *testing.T) {
		result := &FieldParamResult{
			IncludeAll: false,
			Selectors:  []FieldSelector{{Name: "name"}, {Name: "email"}},
		}
		if !result.HasField("name") {
			t.Error("expected name to be found")
		}
		if !result.HasField("email") {
			t.Error("expected email to be found")
		}
		if result.HasField("password") {
			t.Error("expected password to not be found")
		}
	})
}

func TestFieldParamResult_ToSelectionSet(t *testing.T) {
	t.Run("include all returns empty", func(t *testing.T) {
		result := &FieldParamResult{IncludeAll: true}
		sel := result.ToSelectionSet()
		if sel != "" {
			t.Errorf("expected empty selection set for IncludeAll, got %q", sel)
		}
	})

	t.Run("specific fields", func(t *testing.T) {
		result := &FieldParamResult{
			IncludeAll: false,
			Selectors:  []FieldSelector{{Name: "id"}, {Name: "name"}},
		}
		sel := result.ToSelectionSet()
		if sel == "" {
			t.Fatal("expected non-empty selection set")
		}
		if !strings.Contains(sel, "id") || !strings.Contains(sel, "name") {
			t.Errorf("selection set should contain id and name, got %q", sel)
		}
	})
}

func TestNewSelectionSetBuilder(t *testing.T) {
	typeInfo := TypeInfo{
		Name: "User",
		Fields: []FieldInfo{
			{Name: "id", Type: "ID"},
			{Name: "name", Type: "String"},
			{Name: "email", Type: "String"},
		},
	}
	builder := NewSelectionSetBuilder(typeInfo)
	if builder == nil {
		t.Fatal("NewSelectionSetBuilder returned nil")
	}
	if builder.maxDepth != 10 {
		t.Errorf("expected default maxDepth 10, got %d", builder.maxDepth)
	}
}

func TestSelectionSetBuilder_Build(t *testing.T) {
	t.Run("simple fields", func(t *testing.T) {
		typeInfo := TypeInfo{
			Name: "User",
			Fields: []FieldInfo{
				{Name: "id", Type: "ID"},
				{Name: "name", Type: "String"},
				{Name: "email", Type: "String"},
			},
		}
		builder := NewSelectionSetBuilder(typeInfo)
		result := builder.Build()
		if !strings.Contains(result, "id") {
			t.Errorf("expected 'id' in result, got %q", result)
		}
		if !strings.Contains(result, "name") {
			t.Errorf("expected 'name' in result, got %q", result)
		}
	})

	t.Run("with exclusions", func(t *testing.T) {
		typeInfo := TypeInfo{
			Name: "User",
			Fields: []FieldInfo{
				{Name: "id", Type: "ID"},
				{Name: "name", Type: "String"},
				{Name: "password", Type: "String"},
			},
		}
		builder := NewSelectionSetBuilder(typeInfo)
		builder.ExcludeFields("password")
		result := builder.Build()
		if strings.Contains(result, "password") {
			t.Errorf("expected 'password' to be excluded, got %q", result)
		}
		if !strings.Contains(result, "id") {
			t.Errorf("expected 'id' in result, got %q", result)
		}
	})

	t.Run("with only fields", func(t *testing.T) {
		typeInfo := TypeInfo{
			Name: "User",
			Fields: []FieldInfo{
				{Name: "id", Type: "ID"},
				{Name: "name", Type: "String"},
				{Name: "email", Type: "String"},
			},
		}
		builder := NewSelectionSetBuilder(typeInfo)
		builder.OnlyFields("id", "name")
		result := builder.Build()
		if strings.Contains(result, "email") {
			t.Errorf("expected 'email' to not be included, got %q", result)
		}
		if !strings.Contains(result, "id") {
			t.Errorf("expected 'id' in result, got %q", result)
		}
	})

	t.Run("skips fields with args", func(t *testing.T) {
		typeInfo := TypeInfo{
			Name: "User",
			Fields: []FieldInfo{
				{Name: "id", Type: "ID"},
				{Name: "friendsOf", Type: "User", HasArgs: true},
			},
		}
		builder := NewSelectionSetBuilder(typeInfo)
		result := builder.Build()
		if strings.Contains(result, "friendsOf") {
			t.Errorf("expected 'friendsOf' (has args) to be excluded, got %q", result)
		}
	})

	t.Run("max depth", func(t *testing.T) {
		typeInfo := TypeInfo{
			Name: "Node",
			Fields: []FieldInfo{
				{Name: "value", Type: "String"},
				{Name: "child", Type: "Node", SubFields: []FieldInfo{
					{Name: "value", Type: "String"},
				}},
			},
		}
		builder := NewSelectionSetBuilder(typeInfo)
		builder.WithMaxDepth(1)
		result := builder.Build()
		// At depth 1, nested subfields should not be expanded beyond one level
		if result == "" {
			t.Error("expected non-empty result")
		}
	})

	t.Run("empty fields", func(t *testing.T) {
		typeInfo := TypeInfo{Name: "Empty", Fields: []FieldInfo{}}
		builder := NewSelectionSetBuilder(typeInfo)
		result := builder.Build()
		if result != "{}" {
			t.Errorf("expected '{}' for empty fields, got %q", result)
		}
	})

	t.Run("nested fields", func(t *testing.T) {
		typeInfo := TypeInfo{
			Name: "User",
			Fields: []FieldInfo{
				{Name: "id", Type: "ID"},
				{Name: "address", Type: "Address", SubFields: []FieldInfo{
					{Name: "street", Type: "String"},
					{Name: "city", Type: "String"},
				}},
			},
		}
		builder := NewSelectionSetBuilder(typeInfo)
		result := builder.Build()
		if !strings.Contains(result, "address") {
			t.Errorf("expected 'address' in result, got %q", result)
		}
		if !strings.Contains(result, "street") {
			t.Errorf("expected 'street' in nested result, got %q", result)
		}
	})
}

func TestApplyFieldParam(t *testing.T) {
	t.Run("include all with exclusions", func(t *testing.T) {
		typeInfo := TypeInfo{
			Name: "User",
			Fields: []FieldInfo{
				{Name: "id", Type: "ID"},
				{Name: "name", Type: "String"},
				{Name: "password", Type: "String"},
			},
		}
		builder := NewSelectionSetBuilder(typeInfo)
		result := &FieldParamResult{
			IncludeAll: true,
			Exclusions: map[string]bool{"password": true},
		}
		sel := ApplyFieldParam(builder, result)
		if strings.Contains(sel, "password") {
			t.Errorf("expected 'password' excluded, got %q", sel)
		}
	})

	t.Run("specific selectors", func(t *testing.T) {
		typeInfo := TypeInfo{
			Name: "User",
			Fields: []FieldInfo{
				{Name: "id", Type: "ID"},
				{Name: "name", Type: "String"},
				{Name: "email", Type: "String"},
			},
		}
		builder := NewSelectionSetBuilder(typeInfo)
		result := &FieldParamResult{
			IncludeAll: false,
			Selectors:  []FieldSelector{{Name: "id"}, {Name: "name"}},
		}
		sel := ApplyFieldParam(builder, result)
		if strings.Contains(sel, "email") {
			t.Errorf("expected 'email' not included, got %q", sel)
		}
	})

	t.Run("include all no exclusions", func(t *testing.T) {
		typeInfo := TypeInfo{
			Name: "User",
			Fields: []FieldInfo{
				{Name: "id", Type: "ID"},
				{Name: "name", Type: "String"},
			},
		}
		builder := NewSelectionSetBuilder(typeInfo)
		result := &FieldParamResult{IncludeAll: true}
		sel := ApplyFieldParam(builder, result)
		if !strings.Contains(sel, "id") || !strings.Contains(sel, "name") {
			t.Errorf("expected all fields, got %q", sel)
		}
	})
}
