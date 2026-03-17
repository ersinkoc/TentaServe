package mcp

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"unicode"
)

// NameGenerator generates unique tool names.
type NameGenerator struct {
	mu        sync.Mutex
	used      map[string]bool
	generated map[string]int // Track collisions per base name
}

// NewNameGenerator creates a new name generator.
func NewNameGenerator() *NameGenerator {
	return &NameGenerator{
		used:      make(map[string]bool),
		generated: make(map[string]int),
	}
}

// Reset clears all used names.
func (g *NameGenerator) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.used = make(map[string]bool)
	g.generated = make(map[string]int)
}

// Generate creates a unique tool name from upstream, operation, and suffix.
func (g *NameGenerator) Generate(upstream, operation, suffix string) string {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Build base name
	base := sanitizeName(upstream)
	if operation != "" {
		if base != "" {
			base += "_"
		}
		base += sanitizeName(operation)
	}
	if suffix != "" {
		if base != "" {
			base += "_"
		}
		base += sanitizeName(suffix)
	}

	// If base is empty, use a default
	if base == "" {
		base = "tool"
	}

	// Truncate to max 64 chars
	base = truncateName(base, 64)

	// Check for collisions
	name := base
	count := g.generated[base]
	for g.used[name] {
		count++
		suffix := fmt.Sprintf("_%d", count)
		name = truncateName(base, 64-len(suffix)) + suffix
	}

	g.generated[base] = count
	g.used[name] = true
	return name
}

// GenerateFromPath creates a tool name from an HTTP path.
func (g *NameGenerator) GenerateFromPath(upstream, path, method string) string {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Extract meaningful parts from path
	parts := extractPathParts(path)

	// Build name: {upstream}_{method}_{parts}
	var components []string
	if upstream != "" {
		components = append(components, sanitizeName(upstream))
	}
	if method != "" {
		components = append(components, strings.ToLower(method))
	}
	if len(parts) > 0 {
		components = append(components, strings.Join(parts, "_"))
	}

	name := strings.Join(components, "_")
	if name == "" {
		name = "tool"
	}

	// Truncate and handle collisions
	name = truncateName(name, 64)
	base := name
	count := g.generated[base]
	for g.used[name] {
		count++
		suffix := fmt.Sprintf("_%d", count)
		name = truncateName(base, 64-len(suffix)) + suffix
	}

	g.generated[base] = count
	g.used[name] = true
	return name
}

// IsValid checks if a name is a valid tool name.
func IsValid(name string) bool {
	if name == "" {
		return false
	}
	if len(name) > 64 {
		return false
	}

	// Must start with letter or underscore
	first := name[0]
	if !unicode.IsLetter(rune(first)) && first != '_' {
		return false
	}

	// Must contain only letters, numbers, underscores
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}

	return true
}

// sanitizeName converts a string to a valid tool name (snake_case).
func sanitizeName(s string) string {
	if s == "" {
		return "tool"
	}

	// Check if original string starts with a digit
	startsWithDigit := false
	for _, r := range s {
		if unicode.IsDigit(r) {
			startsWithDigit = true
		}
		break // Only check first rune
	}

	var result strings.Builder
	prevUnderscore := false

	for i, r := range s {
		// Handle first character specially
		if i == 0 {
			if unicode.IsLetter(r) {
				result.WriteRune(unicode.ToLower(r))
			} else if unicode.IsDigit(r) {
				// Start with underscore if first char is digit
				result.WriteRune('_')
				result.WriteRune(r)
			} else {
				result.WriteRune('_')
				if unicode.IsLetter(r) {
					result.WriteRune(unicode.ToLower(r))
				}
			}
			continue
		}

		// Handle subsequent characters
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			// Insert underscore on case change (camelCase -> snake_case)
			if unicode.IsUpper(r) && !prevUnderscore {
				result.WriteRune('_')
			}
			result.WriteRune(unicode.ToLower(r))
			prevUnderscore = false
		} else if r == '.' || r == '-' || r == '/' || r == ' ' {
			// Convert separators to underscore
			if !prevUnderscore {
				result.WriteRune('_')
				prevUnderscore = true
			}
		} else {
			// Other characters -> underscore
			if !prevUnderscore {
				result.WriteRune('_')
				prevUnderscore = true
			}
		}
	}

	// Clean up multiple underscores
	name := result.String()
	name = regexp.MustCompile(`_+`).ReplaceAllString(name, "_")

	if startsWithDigit {
		// For digit-starting strings, only trim trailing underscores
		name = strings.TrimRight(name, "_")
	} else {
		// For other strings, trim both leading and trailing underscores
		name = strings.Trim(name, "_")
	}

	if name == "" {
		return "tool"
	}

	return name
}

// truncateName truncates a name to max length while preserving word boundaries.
func truncateName(name string, maxLen int) string {
	if len(name) <= maxLen {
		return name
	}

	// Try to break at underscore
	if idx := strings.LastIndex(name[:maxLen], "_"); idx > 0 {
		// Only truncate if we keep at least half
		if idx > maxLen/2 {
			return name[:idx]
		}
	}

	return name[:maxLen]
}

// extractPathParts extracts meaningful parts from an HTTP path.
func extractPathParts(path string) []string {
	if path == "" || path == "/" {
		return []string{"root"}
	}

	parts := strings.Split(path, "/")
	var result []string

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Skip path parameters {id}, :id, etc.
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			continue
		}
		if strings.HasPrefix(part, ":") {
			continue
		}

		// Clean the part
		part = sanitizeName(part)
		if part != "" && part != "_" {
			result = append(result, part)
		}
	}

	return result
}

// normalizeName normalizes a name for collision checking.
func normalizeName(s string) string {
	return strings.ToLower(sanitizeName(s))
}

// WouldCollide checks if two names would collide.
func WouldCollide(a, b string) bool {
	return normalizeName(a) == normalizeName(b)
}

// ReservedWords returns words that cannot be used as tool names.
func ReservedWords() []string {
	return []string{
		"initialize",
		"tools_list",
		"tools_call",
		"resources_list",
		"resources_read",
		"ping",
		"notification",
		"cancelled",
		"progress",
		"logging",
	}
}

// IsReserved checks if a name is reserved.
func IsReserved(name string) bool {
	normalized := normalizeName(name)
	for _, word := range ReservedWords() {
		if normalized == word {
			return true
		}
	}
	return false
}
