package schema

import (
	"testing"
)

// TestNewFieldMapper tests mapper creation.
func TestNewFieldMapper(t *testing.T) {
	mapper := NewFieldMapper(FieldMapperOptions{
		Mappings: map[string]string{
			"usr_nm": "userName",
			"eml":    "email",
		},
		DefaultConvention: "snake",
	})

	if mapper == nil {
		t.Fatal("Expected FieldMapper to be created")
	}

	if mapper.defaultConvention != "snake" {
		t.Errorf("Expected defaultConvention 'snake', got %s", mapper.defaultConvention)
	}

	if !mapper.HasMapping("usr_nm") {
		t.Error("Expected usr_nm mapping to exist")
	}

	if !mapper.HasMapping("eml") {
		t.Error("Expected eml mapping to exist")
	}
}

// TestNewFieldMapper_DefaultConvention tests default convention fallback.
func TestNewFieldMapper_DefaultConvention(t *testing.T) {
	mapper := NewFieldMapper(FieldMapperOptions{})

	if mapper.defaultConvention != "camel" {
		t.Errorf("Expected default 'camel', got %s", mapper.defaultConvention)
	}
}

// TestFieldMapper_AddMapping tests adding mappings.
func TestFieldMapper_AddMapping(t *testing.T) {
	mapper := NewFieldMapper(FieldMapperOptions{})

	mapper.AddMapping("oldField", "newField")

	if !mapper.HasMapping("oldField") {
		t.Error("Expected oldField mapping to exist")
	}

	if !mapper.HasReverseMapping("newField") {
		t.Error("Expected newField reverse mapping to exist")
	}
}

// TestFieldMapper_Map tests field mapping with explicit mappings.
func TestFieldMapper_Map(t *testing.T) {
	mapper := NewFieldMapper(FieldMapperOptions{
		Mappings: map[string]string{
			"usr_nm": "userName",
			"usr_id": "userId",
		},
		DefaultConvention: "camel",
	})

	tests := []struct {
		input    string
		expected string
	}{
		{"usr_nm", "userName"},      // Explicit mapping
		{"usr_id", "userId"},        // Explicit mapping
		{"first_name", "firstName"}, // Convention: snake -> camel
		{"last_name", "lastName"},   // Convention: snake -> camel
		{"email", "email"},          // Convention: already camel
	}

	for _, tt := range tests {
		result := mapper.Map(tt.input)
		if result != tt.expected {
			t.Errorf("Map(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// TestFieldMapper_MapReverse tests reverse field mapping.
func TestFieldMapper_MapReverse(t *testing.T) {
	mapper := NewFieldMapper(FieldMapperOptions{
		Mappings: map[string]string{
			"usr_nm": "userName",
			"usr_id": "userId",
		},
		DefaultConvention: "camel",
	})

	tests := []struct {
		input    string
		expected string
	}{
		{"userName", "usr_nm"},      // Explicit reverse mapping
		{"userId", "usr_id"},        // Explicit reverse mapping
		{"firstName", "first_name"}, // Convention: camel -> snake
		{"lastName", "last_name"},   // Convention: camel -> snake
	}

	for _, tt := range tests {
		result := mapper.MapReverse(tt.input)
		if result != tt.expected {
			t.Errorf("MapReverse(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// TestFieldMapper_Map_SnakeConvention tests snake_case default convention.
func TestFieldMapper_Map_SnakeConvention(t *testing.T) {
	mapper := NewFieldMapper(FieldMapperOptions{
		DefaultConvention: "snake",
	})

	tests := []struct {
		input    string
		expected string
	}{
		{"userName", "user_name"},  // camel -> snake
		{"UserName", "user_name"},  // Pascal -> snake
		{"user-name", "user_name"}, // kebab -> snake
		{"user_name", "user_name"}, // already snake
	}

	for _, tt := range tests {
		result := mapper.Map(tt.input)
		if result != tt.expected {
			t.Errorf("Map(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// TestFieldMapper_Map_PascalConvention tests PascalCase default convention.
func TestFieldMapper_Map_PascalConvention(t *testing.T) {
	mapper := NewFieldMapper(FieldMapperOptions{
		DefaultConvention: "pascal",
	})

	tests := []struct {
		input    string
		expected string
	}{
		{"user_name", "UserName"}, // snake -> Pascal
		{"userName", "UserName"},  // camel -> Pascal
		{"user-name", "UserName"}, // kebab -> Pascal
	}

	for _, tt := range tests {
		result := mapper.Map(tt.input)
		if result != tt.expected {
			t.Errorf("Map(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// TestFieldMapper_Map_KebabConvention tests kebab-case default convention.
func TestFieldMapper_Map_KebabConvention(t *testing.T) {
	mapper := NewFieldMapper(FieldMapperOptions{
		DefaultConvention: "kebab",
	})

	tests := []struct {
		input    string
		expected string
	}{
		{"userName", "user-name"},  // camel -> kebab
		{"user_name", "user-name"}, // snake -> kebab
		{"UserName", "user-name"},  // Pascal -> kebab
	}

	for _, tt := range tests {
		result := mapper.Map(tt.input)
		if result != tt.expected {
			t.Errorf("Map(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// TestFieldMapper_MapFields tests mapping fields in a map.
func TestFieldMapper_MapFields(t *testing.T) {
	mapper := NewFieldMapper(FieldMapperOptions{
		Mappings: map[string]string{
			"usr_nm": "userName",
		},
		DefaultConvention: "camel",
	})

	input := map[string]interface{}{
		"usr_nm":     "John",
		"first_name": "Doe",
		"age":        30,
	}

	result := mapper.MapFields(input)

	// Check explicit mapping
	if _, ok := result["userName"]; !ok {
		t.Error("Expected userName key in result")
	}

	// Check convention mapping
	if _, ok := result["firstName"]; !ok {
		t.Error("Expected firstName key in result")
	}

	// Check unmapped field passes through convention
	if _, ok := result["age"]; !ok {
		t.Error("Expected age key in result")
	}

	// Check old keys don't exist
	if _, ok := result["usr_nm"]; ok {
		t.Error("usr_nm should not exist in result")
	}
}

// TestFieldMapper_MapFields_Nested tests mapping nested structures.
func TestFieldMapper_MapFields_Nested(t *testing.T) {
	mapper := NewFieldMapper(FieldMapperOptions{
		Mappings: map[string]string{
			"usr_nm": "userName",
		},
		DefaultConvention: "camel",
	})

	input := map[string]interface{}{
		"usr_nm": "John",
		"profile": map[string]interface{}{
			"first_name": "Doe",
			"age":        30,
		},
		"posts": []interface{}{
			map[string]interface{}{
				"post_title": "Hello",
			},
			map[string]interface{}{
				"post_title": "World",
			},
		},
	}

	result := mapper.MapFields(input)

	// Check top level
	if _, ok := result["userName"]; !ok {
		t.Error("Expected userName key in result")
	}

	// Check nested map
	profile, ok := result["profile"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected profile to be a map")
	}

	if _, ok := profile["firstName"]; !ok {
		t.Error("Expected firstName in profile")
	}

	// Check slice of maps
	posts, ok := result["posts"].([]interface{})
	if !ok {
		t.Fatal("Expected posts to be a slice")
	}

	if len(posts) != 2 {
		t.Errorf("Expected 2 posts, got %d", len(posts))
	}

	firstPost, ok := posts[0].(map[string]interface{})
	if !ok {
		t.Fatal("Expected post to be a map")
	}

	if _, ok := firstPost["postTitle"]; !ok {
		t.Error("Expected postTitle in first post")
	}
}

// TestFieldMapper_MapFields_Nil tests mapping nil input.
func TestFieldMapper_MapFields_Nil(t *testing.T) {
	mapper := NewFieldMapper(FieldMapperOptions{})

	result := mapper.MapFields(nil)
	if result != nil {
		t.Error("Expected nil for nil input")
	}
}

// TestFieldMapper_MapFieldsReverse tests reverse mapping.
func TestFieldMapper_MapFieldsReverse(t *testing.T) {
	mapper := NewFieldMapper(FieldMapperOptions{
		Mappings: map[string]string{
			"usr_nm": "userName",
		},
		DefaultConvention: "camel",
	})

	input := map[string]interface{}{
		"userName":  "John", // Explicit reverse mapping -> usr_nm
		"firstName": "Doe",  // Convention reverse -> first_name
	}

	result := mapper.MapFieldsReverse(input)

	if _, ok := result["usr_nm"]; !ok {
		t.Error("Expected usr_nm key in result (explicit reverse)")
	}

	if _, ok := result["first_name"]; !ok {
		t.Error("Expected first_name key in result (convention reverse)")
	}
}

// TestPerUpstreamFieldMapper tests per-upstream field mapper.
func TestPerUpstreamFieldMapper(t *testing.T) {
	pm := NewPerUpstreamFieldMapper()

	// Create custom mapper for users-api
	usersMapper := NewFieldMapper(FieldMapperOptions{
		Mappings: map[string]string{
			"usr_nm": "userName",
		},
		DefaultConvention: "camel",
	})

	pm.RegisterMapper("users-api", usersMapper)

	// Test users-api mapper
	result := pm.MapForUpstream("users-api", "usr_nm")
	if result != "userName" {
		t.Errorf("Expected userName, got %s", result)
	}

	// Test default mapper for unknown upstream
	result = pm.MapForUpstream("other-api", "first_name")
	if result != "firstName" {
		t.Errorf("Expected firstName, got %s", result)
	}
}

// TestPerUpstreamFieldMapper_Reverse tests per-upstream reverse mapping.
func TestPerUpstreamFieldMapper_Reverse(t *testing.T) {
	pm := NewPerUpstreamFieldMapper()

	mapper := NewFieldMapper(FieldMapperOptions{
		Mappings: map[string]string{
			"usr_nm": "userName",
		},
	})

	pm.RegisterMapper("users-api", mapper)

	result := pm.MapReverseForUpstream("users-api", "userName")
	if result != "usr_nm" {
		t.Errorf("Expected usr_nm, got %s", result)
	}
}

// TestFieldMapper_EmptyMapping tests empty mapping handling.
func TestFieldMapper_EmptyMapping(t *testing.T) {
	mapper := NewFieldMapper(FieldMapperOptions{})

	// Empty strings should be ignored
	mapper.AddMapping("", "target")
	mapper.AddMapping("source", "")

	if mapper.HasMapping("") {
		t.Error("Empty string mapping should not exist")
	}
}

// TestFieldMapper_ConventionFallback tests that unmapped fields use convention.
func TestFieldMapper_ConventionFallback(t *testing.T) {
	mapper := NewFieldMapper(FieldMapperOptions{
		DefaultConvention: "camel",
	})

	// Unmapped fields should use convention
	result := mapper.Map("some_field_name")
	if result != "someFieldName" {
		t.Errorf("Expected someFieldName, got %s", result)
	}
}

// TestFieldMapper_AcronymHandling tests acronym handling in conventions.
func TestFieldMapper_AcronymHandling(t *testing.T) {
	mapper := NewFieldMapper(FieldMapperOptions{
		DefaultConvention: "camel",
	})

	tests := []struct {
		input    string
		expected string
	}{
		{"getURL", "getURL"},          // Acronym preserved in camelCase
		{"html_parser", "htmlParser"}, // Normal word conversion
		{"parseHTML", "parseHTML"},    // Acronym at end preserved
		{"userID", "userID"},          // ID acronym preserved
	}

	for _, tt := range tests {
		result := mapper.Map(tt.input)
		if result != tt.expected {
			t.Errorf("Map(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// BenchmarkFieldMapper_Map benchmarks field mapping.
func BenchmarkFieldMapper_Map(b *testing.B) {
	mapper := NewFieldMapper(FieldMapperOptions{
		Mappings: map[string]string{
			"usr_nm": "userName",
			"usr_id": "userId",
		},
		DefaultConvention: "camel",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mapper.Map("first_name")
	}
}

// BenchmarkFieldMapper_MapFields benchmarks field mapping for maps.
func BenchmarkFieldMapper_MapFields(b *testing.B) {
	mapper := NewFieldMapper(FieldMapperOptions{
		Mappings: map[string]string{
			"usr_nm": "userName",
		},
		DefaultConvention: "camel",
	})

	input := map[string]interface{}{
		"usr_nm":     "John",
		"first_name": "Doe",
		"age":        30,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mapper.MapFields(input)
	}
}
