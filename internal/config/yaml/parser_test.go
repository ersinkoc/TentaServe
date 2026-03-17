package yaml

import (
	"testing"
)

func TestParseSimpleKeyValue(t *testing.T) {
	yaml := `key: value`

	result, err := ParseString(yaml)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if result["key"] != "value" {
		t.Errorf("Expected 'value', got %v", result["key"])
	}
}

func TestParseMultipleKeys(t *testing.T) {
	yaml := `
name: Tentaserve
version: 1.0
port: 8080
`

	result, err := ParseString(yaml)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if result["name"] != "Tentaserve" {
		t.Errorf("Expected 'Tentaserve', got %v", result["name"])
	}
	if result["version"] != float64(1.0) {
		t.Errorf("Expected 1.0, got %v (type %T)", result["version"], result["version"])
	}
	if result["port"] != int64(8080) {
		t.Errorf("Expected 8080, got %v (type %T)", result["port"], result["port"])
	}
}

func TestParseNestedMap(t *testing.T) {
	yaml := `
server:
  host: localhost
  port: 8080
`

	result, err := ParseString(yaml)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	server, ok := result["server"].(map[string]any)
	if !ok {
		t.Fatalf("Expected map for server, got %T", result["server"])
	}

	if server["host"] != "localhost" {
		t.Errorf("Expected 'localhost', got %v", server["host"])
	}
	if server["port"] != int64(8080) {
		t.Errorf("Expected 8080, got %v", server["port"])
	}
}

func TestParseList(t *testing.T) {
	yaml := `
items:
  - first
  - second
  - third
`

	result, err := ParseString(yaml)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	items, ok := result["items"].([]any)
	if !ok {
		t.Fatalf("Expected slice for items, got %T", result["items"])
	}

	if len(items) != 3 {
		t.Fatalf("Expected 3 items, got %d", len(items))
	}

	if items[0] != "first" {
		t.Errorf("Expected 'first', got %v", items[0])
	}
	if items[1] != "second" {
		t.Errorf("Expected 'second', got %v", items[1])
	}
	if items[2] != "third" {
		t.Errorf("Expected 'third', got %v", items[2])
	}
}

func TestParseBoolean(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"key: true", true},
		{"key: false", false},
		{"key: yes", true},
		{"key: no", false},
	}

	for _, tc := range tests {
		result, err := ParseString(tc.input)
		if err != nil {
			t.Fatalf("Parse failed for %q: %v", tc.input, err)
		}

		if result["key"] != tc.expected {
			t.Errorf("For %q: expected %v, got %v", tc.input, tc.expected, result["key"])
		}
	}
}

func TestParseNull(t *testing.T) {
	tests := []string{
		"key: null",
		"key: ~",
		"key: Null",
		"key: NULL",
	}

	for _, input := range tests {
		result, err := ParseString(input)
		if err != nil {
			t.Fatalf("Parse failed for %q: %v", input, err)
		}

		if result["key"] != nil {
			t.Errorf("For %q: expected nil, got %v", input, result["key"])
		}
	}
}

func TestParseFloat(t *testing.T) {
	yaml := `
timeout: 30.5
rate: 0.75
`

	result, err := ParseString(yaml)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if result["timeout"] != float64(30.5) {
		t.Errorf("Expected 30.5, got %v (type %T)", result["timeout"], result["timeout"])
	}
	if result["rate"] != float64(0.75) {
		t.Errorf("Expected 0.75, got %v", result["rate"])
	}
}

func TestParseQuotedString(t *testing.T) {
	yaml := `key: "quoted value"`

	result, err := ParseString(yaml)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if result["key"] != "quoted value" {
		t.Errorf("Expected 'quoted value', got %v", result["key"])
	}
}

func TestParseComment(t *testing.T) {
	yaml := `
# This is a comment
key: value  # inline comment
`

	result, err := ParseString(yaml)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if result["key"] != "value" {
		t.Errorf("Expected 'value', got %v", result["key"])
	}
}

func TestInterpolateEnv(t *testing.T) {
	t.Setenv("TEST_VAR", "testvalue")
	t.Setenv("DEFAULT_TEST", "defaultvalue")

	tests := []struct {
		input    string
		expected string
	}{
		{"${TEST_VAR}", "testvalue"},
		{"${MISSING:default}", "default"},
		{"prefix_${TEST_VAR}_suffix", "prefix_testvalue_suffix"},
	}

	for _, tc := range tests {
		result := InterpolateEnv(tc.input)
		if result != tc.expected {
			t.Errorf("InterpolateEnv(%q): expected %q, got %q", tc.input, tc.expected, result)
		}
	}
}

func TestUnmarshal(t *testing.T) {
	type TestConfig struct {
		Name    string `yaml:"name"`
		Port    int    `yaml:"port"`
		Enabled bool   `yaml:"enabled"`
	}

	data := map[string]any{
		"name":    "test",
		"port":    int64(8080),
		"enabled": true,
	}

	var cfg TestConfig
	err := Unmarshal(data, &cfg)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if cfg.Name != "test" {
		t.Errorf("Expected Name='test', got %q", cfg.Name)
	}
	if cfg.Port != 8080 {
		t.Errorf("Expected Port=8080, got %d", cfg.Port)
	}
	if !cfg.Enabled {
		t.Errorf("Expected Enabled=true, got false")
	}
}

func TestUnmarshalNested(t *testing.T) {
	type ServerConfig struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	}

	type TestConfig struct {
		Name   string       `yaml:"name"`
		Server ServerConfig `yaml:"server"`
	}

	data := map[string]any{
		"name": "test",
		"server": map[string]any{
			"host": "localhost",
			"port": int64(8080),
		},
	}

	var cfg TestConfig
	err := Unmarshal(data, &cfg)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if cfg.Name != "test" {
		t.Errorf("Expected Name='test', got %q", cfg.Name)
	}
	if cfg.Server.Host != "localhost" {
		t.Errorf("Expected Server.Host='localhost', got %q", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Expected Server.Port=8080, got %d", cfg.Server.Port)
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"30s", "30s"},
		{"5m", "5m0s"},
		{"1h", "1h0m0s"},
	}

	for _, tc := range tests {
		d, err := ParseDuration(tc.input)
		if err != nil {
			t.Errorf("ParseDuration(%q) failed: %v", tc.input, err)
			continue
		}
		if d.String() != tc.expected {
			t.Errorf("ParseDuration(%q): expected %q, got %q", tc.input, tc.expected, d.String())
		}
	}
}

func TestParseMemorySize(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"256MB", 256 * 1024 * 1024},
		{"1GB", 1024 * 1024 * 1024},
		{"512KB", 512 * 1024},
		{"1024", 1024},
	}

	for _, tc := range tests {
		size, err := ParseMemorySize(tc.input)
		if err != nil {
			t.Errorf("ParseMemorySize(%q) failed: %v", tc.input, err)
			continue
		}
		if size != tc.expected {
			t.Errorf("ParseMemorySize(%q): expected %d, got %d", tc.input, tc.expected, size)
		}
	}
}

// TestParseExampleConfig tests parsing the example config from the spec
func TestParseExampleConfig(t *testing.T) {
	yaml := `
server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: 30s
  write_timeout: 30s

gateway:
  rest_prefix: "/api"
  graphql_path: "/graphql"
  mcp_path: "/mcp"

  rate_limit:
    enabled: true
    requests_per_second: 100

upstreams:
  - name: "users-api"
    type: "rest"
    base_url: "https://api.example.com"
`

	result, err := ParseString(yaml)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Check server config
	server, ok := result["server"].(map[string]any)
	if !ok {
		t.Fatal("Expected server to be a map")
	}
	if server["port"] != int64(8080) {
		t.Errorf("Expected port=8080, got %v", server["port"])
	}

	// Check gateway config
	gateway, ok := result["gateway"].(map[string]any)
	if !ok {
		t.Fatal("Expected gateway to be a map")
	}
	if gateway["rest_prefix"] != "/api" {
		t.Errorf("Expected rest_prefix='/api', got %v", gateway["rest_prefix"])
	}

	// Check nested rate_limit
	rateLimit, ok := gateway["rate_limit"].(map[string]any)
	if !ok {
		t.Fatal("Expected rate_limit to be a map")
	}
	if rateLimit["enabled"] != true {
		t.Errorf("Expected enabled=true, got %v", rateLimit["enabled"])
	}

	// Check upstreams list
	upstreams, ok := result["upstreams"].([]any)
	if !ok {
		t.Fatalf("Expected upstreams to be a slice, got %T", result["upstreams"])
	}
	if len(upstreams) != 1 {
		t.Fatalf("Expected 1 upstream, got %d", len(upstreams))
	}
}

func BenchmarkParseSimple(b *testing.B) {
	yaml := `
name: Tentaserve
version: 1.0
port: 8080
host: localhost
enabled: true
rate: 0.75
`

	for i := 0; i < b.N; i++ {
		_, err := ParseString(yaml)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseComplex(b *testing.B) {
	yaml := `
server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: 30s
  write_timeout: 30s

gateway:
  rest_prefix: "/api"
  graphql_path: "/graphql"
  mcp_path: "/mcp"

  rate_limit:
    enabled: true
    requests_per_second: 100
    burst_size: 150

upstreams:
  - name: "users-api"
    type: "rest"
    base_url: "https://api.example.com"
    timeout: 30s
  - name: "products-api"
    type: "graphql"
    endpoint: "https://graphql.example.com/query"
`

	for i := 0; i < b.N; i++ {
		_, err := ParseString(yaml)
		if err != nil {
			b.Fatal(err)
		}
	}
}
