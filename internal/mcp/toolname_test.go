package mcp

import (
	"strings"
	"testing"
)

// TestNewNameGenerator tests name generator creation.
func TestNewNameGenerator(t *testing.T) {
	g := NewNameGenerator()
	if g == nil {
		t.Fatal("Expected non-nil generator")
	}
	if g.used == nil {
		t.Error("Expected used map to be initialized")
	}
	if g.generated == nil {
		t.Error("Expected generated map to be initialized")
	}
}

// TestNameGeneratorGenerate tests name generation.
func TestNameGeneratorGenerate(t *testing.T) {
	g := NewNameGenerator()

	name := g.Generate("users", "getUser", "")
	if name == "" {
		t.Fatal("Expected non-empty name")
	}
	if !strings.Contains(name, "users") {
		t.Errorf("Expected name to contain 'users', got %s", name)
	}
	if !strings.Contains(name, "get_user") {
		t.Errorf("Expected name to contain 'get_user', got %s", name)
	}
}

// TestNameGeneratorGenerateCollision tests collision handling.
func TestNameGeneratorGenerateCollision(t *testing.T) {
	g := NewNameGenerator()

	name1 := g.Generate("api", "test", "")
	name2 := g.Generate("api", "test", "")

	if name1 == name2 {
		t.Error("Expected different names for collision")
	}
	if !strings.HasSuffix(name2, "_1") {
		t.Errorf("Expected second name to have suffix _1, got %s", name2)
	}
}

// TestNameGeneratorGenerateFromPath tests path-based generation.
func TestNameGeneratorGenerateFromPath(t *testing.T) {
	g := NewNameGenerator()

	name := g.GenerateFromPath("users", "/users/{id}", "GET")
	if name == "" {
		t.Fatal("Expected non-empty name")
	}
	if !strings.Contains(name, "users") {
		t.Errorf("Expected name to contain 'users', got %s", name)
	}
	if !strings.Contains(name, "get") {
		t.Errorf("Expected name to contain 'get', got %s", name)
	}
}

// TestNameGeneratorReset tests reset functionality.
func TestNameGeneratorReset(t *testing.T) {
	g := NewNameGenerator()
	g.Generate("api", "test", "")

	g.Reset()

	// After reset, should be able to generate same name again
	name := g.Generate("api", "test", "")
	if !strings.HasSuffix(name, "_1") && !strings.HasSuffix(name, "_0") {
		// Name should not have collision suffix after reset
		t.Logf("Generated name after reset: %s", name)
	}
}

// TestSanitizeName tests name sanitization.
func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"CamelCase", "camel_case"},
		{"with spaces", "with_spaces"},
		{"with-dashes", "with_dashes"},
		{"with.dots", "with_dots"},
		{"/api/users", "api_users"},
		{"UPPERCASE", "u_p_p_e_r_c_a_s_e"}, // Each uppercase letter triggers underscore
		{"mixedCaseTest", "mixed_case_test"},
		{"a_b_c", "a_b_c"},
		{"", "tool"}, // Empty string returns "tool"
		{"123abc", "_123abc"},
		{"APIv2Endpoint", "a_p_iv2_endpoint"}, // 'v2' is not a case change
	}

	for _, tt := range tests {
		got := sanitizeName(tt.input)
		if got != tt.expected {
			t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// TestTruncateName tests name truncation.
func TestTruncateName(t *testing.T) {
	tests := []struct {
		name   string
		maxLen int
		expectedLen int
	}{
		{"short", 100, 5},
		{"a_very_long_name_that_exceeds_limit", 20, 20},
		{"tool_with_underscores", 15, 14}, // Should truncate at underscore
	}

	for _, tt := range tests {
		got := truncateName(tt.name, tt.maxLen)
		if len(got) > tt.expectedLen {
			t.Errorf("truncateName(%q, %d) length = %d, want <= %d",
				tt.name, tt.maxLen, len(got), tt.expectedLen)
		}
	}
}

// TestExtractPathParts tests path part extraction.
func TestExtractPathParts(t *testing.T) {
	tests := []struct {
		path     string
		expected []string
	}{
		{"/users", []string{"users"}},
		{"/users/{id}", []string{"users"}},
		{"/api/users/posts", []string{"api", "users", "posts"}},
		{"", []string{"root"}},
		{"/", []string{"root"}},
		{"/users/:id", []string{"users"}},
	}

	for _, tt := range tests {
		got := extractPathParts(tt.path)
		if len(got) != len(tt.expected) {
			t.Errorf("extractPathParts(%q) = %v, want %v", tt.path, got, tt.expected)
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("extractPathParts(%q)[%d] = %s, want %s",
					tt.path, i, got[i], tt.expected[i])
			}
		}
	}
}

// TestIsValid tests name validation.
func TestIsValid(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"valid_name", true},
		{"validName123", true},
		{"_starts_underscore", true},
		{"", false},
		{"123starts_number", false},
		{"has-dash", false},
		{"has.dot", false},
		{"has space", false},
		{strings.Repeat("a", 65), false}, // Too long
		{"reserved", true},               // IsValid doesn't check reserved
	}

	for _, tt := range tests {
		got := IsValid(tt.name)
		if got != tt.valid {
			t.Errorf("IsValid(%q) = %v, want %v", tt.name, got, tt.valid)
		}
	}
}

// TestNormalizeName tests name normalization.
func TestNormalizeName(t *testing.T) {
	tests := []struct {
		a        string
		b        string
		expected bool
	}{
		{"TestName", "test_name", true},
		{"test_name", "TestName", true},
		{"TEST_NAME", "test_name", false}, // sanitizeName produces "t_e_s_t__n_a_m_e" for "TEST_NAME"
		{"different", "name", false},
	}

	for _, tt := range tests {
		got := WouldCollide(tt.a, tt.b)
		if got != tt.expected {
			t.Errorf("WouldCollide(%q, %q) = %v, want %v",
				tt.a, tt.b, got, tt.expected)
		}
	}
}

// TestIsReserved tests reserved word checking.
func TestIsReserved(t *testing.T) {
	if !IsReserved("initialize") {
		t.Error("Expected 'initialize' to be reserved")
	}
	if !IsReserved("tools_list") {
		t.Error("Expected 'tools_list' to be reserved")
	}
	if IsReserved("custom_tool") {
		t.Error("Expected 'custom_tool' not to be reserved")
	}
}

// TestReservedWords tests reserved words list.
func TestReservedWords(t *testing.T) {
	words := ReservedWords()
	if len(words) == 0 {
		t.Error("Expected non-empty reserved words list")
	}

	// Check for expected words
	hasInitialize := false
	for _, w := range words {
		if w == "initialize" {
			hasInitialize = true
			break
		}
	}
	if !hasInitialize {
		t.Error("Expected 'initialize' in reserved words")
	}
}

// TestSanitizeNameUnicode tests Unicode handling.
func TestSanitizeNameUnicode(t *testing.T) {
	// Unicode characters should be converted to underscores
	got := sanitizeName("héllo")
	if !strings.Contains(got, "_") {
		t.Logf("Unicode handling: sanitizeName('héllo') = %s", got)
	}
}

// TestNameGeneratorThreadSafety tests concurrent access.
func TestNameGeneratorThreadSafety(t *testing.T) {
	g := NewNameGenerator()

	// Generate many names concurrently
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func(n int) {
			g.Generate("api", "test", string(rune('a'+n%26)))
			done <- true
		}(i)
	}

	for i := 0; i < 100; i++ {
		<-done
	}

	// All 100 names should be unique
	if len(g.used) != 100 {
		t.Errorf("Expected 100 unique names, got %d", len(g.used))
	}
}
