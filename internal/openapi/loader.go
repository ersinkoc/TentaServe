package openapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ersinkoc/tentaserve/internal/config/yaml"
)

// LoadOptions provides options for loading OpenAPI specs.
type LoadOptions struct {
	// ResolveRefs determines whether to resolve $refs after loading
	ResolveRefs bool
	// AllowFileLoading allows loading specs from file paths
	AllowFileLoading bool
	// AllowURLLoading allows loading specs from URLs
	AllowURLLoading bool
}

// DefaultLoadOptions returns default load options.
func DefaultLoadOptions() *LoadOptions {
	return &LoadOptions{
		ResolveRefs:      true,
		AllowFileLoading: true,
		AllowURLLoading:  true,
	}
}

// LoadResult contains the result of loading an OpenAPI spec.
type LoadResult struct {
	Spec     *OpenAPISpec
	Source   string
	Format   string // "json" or "yaml"
	Resolved bool   // whether refs were resolved
}

// LoadOpenAPISpec loads an OpenAPI spec from a file path, URL, or inline JSON/YAML.
// It auto-detects the format (JSON vs YAML) based on extension or content.
func LoadOpenAPISpec(source string) (*OpenAPISpec, error) {
	return LoadOpenAPISpecWithOptions(source, DefaultLoadOptions())
}

// LoadOpenAPISpecWithOptions loads an OpenAPI spec with custom options.
func LoadOpenAPISpecWithOptions(source string, opts *LoadOptions) (*OpenAPISpec, error) {
	result, err := loadSpec(source, opts)
	if err != nil {
		return nil, err
	}
	return result.Spec, nil
}

// loadSpec loads a spec and returns the full result.
func loadSpec(source string, opts *LoadOptions) (*LoadResult, error) {
	// Determine the source type and load raw content
	var content []byte
	var format string
	var err error

	switch detectSourceType(source) {
	case sourceURL:
		if !opts.AllowURLLoading {
			return nil, fmt.Errorf("URL loading not allowed: %s", source)
		}
		content, err = loadFromURL(source)
		if err != nil {
			return nil, fmt.Errorf("loading from URL: %w", err)
		}
		format = detectFormatFromURL(source)
	case sourceFile:
		if !opts.AllowFileLoading {
			return nil, fmt.Errorf("file loading not allowed: %s", source)
		}
		content, err = loadFromFile(source)
		if err != nil {
			return nil, fmt.Errorf("loading from file: %w", err)
		}
		format = detectFormatFromPath(source)
	case sourceInline:
		content = []byte(source)
		format = detectFormatFromContent(content)
	}

	// Parse the content
	spec, err := parseContent(content, format)
	if err != nil {
		return nil, fmt.Errorf("parsing spec: %w", err)
	}

	result := &LoadResult{
		Spec:   spec,
		Source: source,
		Format: format,
	}

	// Resolve refs if requested
	if opts.ResolveRefs {
		resolver := NewResolver(spec)
		if err := resolver.ResolveAll(); err != nil {
			return nil, fmt.Errorf("resolving refs: %w", err)
		}
		result.Resolved = true
	}

	return result, nil
}

// sourceType represents the type of source for loading.
type sourceType int

const (
	sourceFile sourceType = iota
	sourceURL
	sourceInline
)

// detectSourceType determines if the source is a file, URL, or inline content.
func detectSourceType(source string) sourceType {
	// Check if it's a URL
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return sourceURL
	}

	// Check if it's a file path
	if filepath.IsAbs(source) || strings.Contains(source, string(filepath.Separator)) {
		return sourceFile
	}

	// Check if it looks like a relative file path
	if strings.HasSuffix(source, ".json") || strings.HasSuffix(source, ".yaml") || strings.HasSuffix(source, ".yml") {
		return sourceFile
	}

	// Otherwise treat as inline
	return sourceInline
}

// loadFromFile loads content from a file.
func loadFromFile(path string) ([]byte, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}
	return content, nil
}

// loadFromURL loads content from a URL.
func loadFromURL(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	return content, nil
}

// detectFormatFromPath detects the format from a file path.
func detectFormatFromPath(path string) string {
	path = strings.ToLower(path)
	if strings.HasSuffix(path, ".json") {
		return "json"
	}
	if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
		return "yaml"
	}
	return "yaml" // default
}

// detectFormatFromURL detects the format from a URL.
func detectFormatFromURL(url string) string {
	return detectFormatFromPath(url)
}

// detectFormatFromContent detects the format from content.
func detectFormatFromContent(content []byte) string {
	// Check if it looks like JSON (starts with { or [)
	trimmed := strings.TrimSpace(string(content))
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		// Try to parse as JSON
		if json.Valid(content) {
			return "json"
		}
	}
	return "yaml"
}

// parseContent parses raw content into an OpenAPISpec.
func parseContent(content []byte, format string) (*OpenAPISpec, error) {
	// Convert to map[string]any first
	var raw map[string]any

	switch format {
	case "json":
		if err := json.Unmarshal(content, &raw); err != nil {
			return nil, fmt.Errorf("parsing JSON: %w", err)
		}
	case "yaml":
		// Use our custom YAML parser
		raw, err := yaml.Parse(bytes.NewReader(content))
		if err != nil {
			return nil, fmt.Errorf("parsing YAML: %w", err)
		}
		// yaml.Parse returns map[string]any directly
		spec, err := Parse(raw)
		if err != nil {
			return nil, err
		}
		return spec, nil
	default:
		return nil, fmt.Errorf("unknown format: %s", format)
	}

	return Parse(raw)
}

// LoadFromInlineJSON loads a spec from inline JSON content.
func LoadFromInlineJSON(content string) (*OpenAPISpec, error) {
	return LoadOpenAPISpecWithOptions(content, &LoadOptions{
		ResolveRefs:      true,
		AllowFileLoading: false,
		AllowURLLoading:  false,
	})
}

// LoadFromInlineYAML loads a spec from inline YAML content.
func LoadFromInlineYAML(content string) (*OpenAPISpec, error) {
	return LoadOpenAPISpecWithOptions(content, &LoadOptions{
		ResolveRefs:      true,
		AllowFileLoading: false,
		AllowURLLoading:  false,
	})
}

// ValidateSpec performs basic validation on a loaded spec.
func ValidateSpec(spec *OpenAPISpec) []error {
	var errs []error

	// Check version
	if spec.OpenAPI == "" {
		errs = append(errs, &ValidationError{Message: "openapi version is required"})
	} else if !isValidVersion(spec.OpenAPI) {
		errs = append(errs, &ValidationError{Message: fmt.Sprintf("unsupported openapi version: %s", spec.OpenAPI)})
	}

	// Check info
	if spec.Info.Title == "" {
		errs = append(errs, &ValidationError{Path: "info.title", Message: "title is required"})
	}
	if spec.Info.Version == "" {
		errs = append(errs, &ValidationError{Path: "info.version", Message: "version is required"})
	}

	// Check for duplicate operation IDs
	operationIDs := make(map[string]string) // operationID -> path
	for path, pathItem := range spec.Paths {
		ops := pathItem.GetOperations()
		for method, op := range ops {
			if op != nil && op.OperationID != "" {
				if existingPath, ok := operationIDs[op.OperationID]; ok {
					errs = append(errs, &ValidationError{
						Path:    fmt.Sprintf("paths.%s.%s.operationId", path, method),
						Message: fmt.Sprintf("duplicate operationId %q (also at %s)", op.OperationID, existingPath),
					})
				} else {
					operationIDs[op.OperationID] = fmt.Sprintf("paths.%s.%s", path, method)
				}
			}
		}
	}

	return errs
}

// isValidVersion checks if the OpenAPI version is supported.
func isValidVersion(version string) bool {
	return strings.HasPrefix(version, "3.0.") || strings.HasPrefix(version, "3.1.")
}
