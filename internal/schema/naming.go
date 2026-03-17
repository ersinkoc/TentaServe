package schema

import (
	"strings"
	"unicode"
)

// ToCamelCase converts a string to camelCase.
// Examples: "user_name" -> "userName", "UserName" -> "userName", "user-name" -> "userName"
func ToCamelCase(s string) string {
	if s == "" {
		return ""
	}

	// Split by common separators
	words := splitWords(s)
	if len(words) == 0 {
		return ""
	}

	// First word lowercase
	result := strings.ToLower(words[0])

	// Rest: capitalize first letter, lowercase rest
	for _, word := range words[1:] {
		if word == "" {
			continue
		}
		result += capitalize(word)
	}

	return result
}

// ToPascalCase converts a string to PascalCase (UpperCamelCase).
// Examples: "user_name" -> "UserName", "userName" -> "UserName"
func ToPascalCase(s string) string {
	if s == "" {
		return ""
	}

	words := splitWords(s)
	if len(words) == 0 {
		return ""
	}

	var result strings.Builder
	for _, word := range words {
		if word == "" {
			continue
		}
		result.WriteString(capitalize(word))
	}

	return result.String()
}

// ToSnakeCase converts a string to snake_case.
// Examples: "UserName" -> "user_name", "userName" -> "user_name", "user-name" -> "user_name"
func ToSnakeCase(s string) string {
	if s == "" {
		return ""
	}

	words := splitWords(s)
	if len(words) == 0 {
		return ""
	}

	var result []string
	for _, word := range words {
		if word != "" {
			result = append(result, strings.ToLower(word))
		}
	}

	return strings.Join(result, "_")
}

// ToKebabCase converts a string to kebab-case.
// Examples: "UserName" -> "user-name", "userName" -> "user-name", "user_name" -> "user-name"
func ToKebabCase(s string) string {
	if s == "" {
		return ""
	}

	words := splitWords(s)
	if len(words) == 0 {
		return ""
	}

	var result []string
	for _, word := range words {
		if word != "" {
			result = append(result, strings.ToLower(word))
		}
	}

	return strings.Join(result, "-")
}

// ToGraphQLField converts a string to GraphQL field naming convention (camelCase).
func ToGraphQLField(s string) string {
	return ToCamelCase(s)
}

// ToGraphQLType converts a string to GraphQL type naming convention (PascalCase).
func ToGraphQLType(s string) string {
	return ToPascalCase(s)
}

// ToRESTPath converts a string to REST path naming convention (kebab-case).
func ToRESTPath(s string) string {
	return ToKebabCase(s)
}

// ToRESTParam converts a string to REST parameter naming convention (snake_case).
func ToRESTParam(s string) string {
	return ToSnakeCase(s)
}

// splitWords splits a string into words based on various conventions.
func splitWords(s string) []string {
	var words []string
	var current strings.Builder

	for i, r := range s {
		// Handle separators
		if r == '_' || r == '-' || r == ' ' || r == '.' {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
			continue
		}

		// Handle uppercase transitions
		if i > 0 && unicode.IsUpper(r) {
			// Check if previous char was lowercase (camelCase) or if next is lowercase (Acronym)
			prev := s[i-1]
			if unicode.IsLower(rune(prev)) {
				// camelCase: userName -> [user, Name]
				words = append(words, current.String())
				current.Reset()
			} else if i+1 < len(s) && unicode.IsLower(rune(s[i+1])) {
				// Acronym end: HTMLParser -> [HTML, Parser]
				if current.Len() > 1 {
					// Keep the last uppercase char for next word
					str := current.String()
					words = append(words, str[:len(str)-1])
					current.Reset()
					current.WriteByte(str[len(str)-1])
				}
			}
		}

		current.WriteRune(r)
	}

	if current.Len() > 0 {
		words = append(words, current.String())
	}

	return words
}

// capitalize capitalizes the first letter of a string.
func capitalize(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// IsAcronym checks if a string is likely an acronym (all uppercase).
func IsAcronym(s string) bool {
	if len(s) <= 1 {
		return false
	}
	for _, r := range s {
		if !unicode.IsUpper(r) {
			return false
		}
	}
	return true
}

// PreserveAcronym preserves acronyms during case conversion.
// Example: "getURL" -> "getURL", "get_url" -> "getURL"
func PreserveAcronym(s string) string {
	words := splitWords(s)
	var result strings.Builder

	for i, word := range words {
		if i > 0 {
			result.WriteString("_")
		}
		if IsAcronym(word) {
			result.WriteString(word)
		} else {
			result.WriteString(strings.ToLower(word))
		}
	}

	return result.String()
}

// FieldNameMapper provides bidirectional field name mapping.
type FieldNameMapper struct {
	graphqlToREST map[string]string
	restToGraphQL map[string]string
}

// NewFieldNameMapper creates a new field name mapper.
func NewFieldNameMapper() *FieldNameMapper {
	return &FieldNameMapper{
		graphqlToREST: make(map[string]string),
		restToGraphQL: make(map[string]string),
	}
}

// AddMapping adds a bidirectional mapping.
func (m *FieldNameMapper) AddMapping(graphQLName, restName string) {
	m.graphqlToREST[graphQLName] = restName
	m.restToGraphQL[restName] = graphQLName
}

// ToREST converts a GraphQL field name to REST.
func (m *FieldNameMapper) ToREST(graphQLName string) string {
	if restName, ok := m.graphqlToREST[graphQLName]; ok {
		return restName
	}
	return ToSnakeCase(graphQLName)
}

// ToGraphQL converts a REST field name to GraphQL.
func (m *FieldNameMapper) ToGraphQL(restName string) string {
	if graphQLName, ok := m.restToGraphQL[restName]; ok {
		return graphQLName
	}
	return ToCamelCase(restName)
}

// HasMapping checks if a mapping exists.
func (m *FieldNameMapper) HasMapping(name string) bool {
	_, ok := m.graphqlToREST[name]
	if ok {
		return true
	}
	_, ok = m.restToGraphQL[name]
	return ok
}

// SanitizeName removes invalid characters from a name.
func SanitizeName(s string) string {
	var result strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// EnsureValidIdentifier ensures a string is a valid identifier.
func EnsureValidIdentifier(s string) string {
	s = SanitizeName(s)
	if s == "" {
		return "_"
	}

	// Ensure starts with letter or underscore
	runes := []rune(s)
	if !unicode.IsLetter(runes[0]) && runes[0] != '_' {
		s = "_" + s
	}

	return s
}

// Pluralize attempts to pluralize a word (basic implementation).
func Pluralize(s string) string {
	if s == "" {
		return ""
	}

	// Simple rules
	if strings.HasSuffix(s, "s") || strings.HasSuffix(s, "x") ||
		strings.HasSuffix(s, "ch") || strings.HasSuffix(s, "sh") {
		return s + "es"
	}
	if strings.HasSuffix(s, "y") && len(s) > 1 && !isVowel(s[len(s)-2]) {
		return s[:len(s)-1] + "ies"
	}

	return s + "s"
}

// Singularize attempts to singularize a word (basic implementation).
func Singularize(s string) string {
	if s == "" {
		return ""
	}

	if strings.HasSuffix(s, "ies") && len(s) > 3 {
		return s[:len(s)-3] + "y"
	}
	if strings.HasSuffix(s, "es") && len(s) > 2 {
		return s[:len(s)-2]
	}
	if strings.HasSuffix(s, "s") && len(s) > 1 {
		return s[:len(s)-1]
	}

	return s
}

func isVowel(b byte) bool {
	switch b {
	case 'a', 'e', 'i', 'o', 'u', 'A', 'E', 'I', 'O', 'U':
		return true
	}
	return false
}
