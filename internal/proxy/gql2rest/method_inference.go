package gql2rest

import (
	"strings"
)

// MethodInference provides heuristics for inferring HTTP methods from GraphQL mutation names.
//
// The inference follows SPECIFICATION.md §7.2:
//
// | GraphQL Mutation Prefix | HTTP Method | Example GraphQL → REST |
// |------------------------|-------------|------------------------|
// | create, add, new, insert | POST | createUser → POST /users |
// | update, edit, modify | PUT | updateUser → PUT /users/{id} |
// | patch | PATCH | patchUser → PATCH /users/{id} |
// | delete, remove, destroy | DELETE | deleteUser → DELETE /users/{id} |
// | upsert, merge | PUT | upsertUser → PUT /users/{id} |
// | get, fetch, find, list, search | GET | getUser → GET /users/{id} |
//
// For mutations without recognized prefixes, POST is used as the default.

type MethodRule struct {
	Prefixes []string
	Method   string
}

// MethodRules defines the standard method inference rules.
var MethodRules = []MethodRule{
	{
		Prefixes: []string{"create", "add", "new", "insert"},
		Method:   "POST",
	},
	{
		Prefixes: []string{"update", "edit", "modify"},
		Method:   "PUT",
	},
	{
		Prefixes: []string{"patch"},
		Method:   "PATCH",
	},
	{
		Prefixes: []string{"delete", "remove", "destroy", "drop"},
		Method:   "DELETE",
	},
	{
		Prefixes: []string{"upsert", "merge"},
		Method:   "PUT",
	},
	{
		Prefixes: []string{"get", "fetch", "find", "list", "search"},
		Method:   "GET",
	},
}

// InferMethod infers the HTTP method from a field name.
// It checks the field name against known prefixes and returns the corresponding HTTP method.
// If no prefix matches, it returns "POST" as the default for mutations.
func InferMethod(fieldName string) string {
	lower := strings.ToLower(fieldName)

	for _, rule := range MethodRules {
		for _, prefix := range rule.Prefixes {
			if strings.HasPrefix(lower, prefix) {
				return rule.Method
			}
		}
	}

	// Default to POST for mutations
	return "POST"
}

// InferMethodWithCustomRules infers the HTTP method using custom rules.
// Custom rules are checked first, then standard rules.
func InferMethodWithCustomRules(fieldName string, customRules []MethodRule) string {
	lower := strings.ToLower(fieldName)

	// Check custom rules first
	for _, rule := range customRules {
		for _, prefix := range rule.Prefixes {
			if strings.HasPrefix(lower, prefix) {
				return rule.Method
			}
		}
	}

	// Fall back to standard rules
	return InferMethod(fieldName)
}

// IsCreateOperation returns true if the mutation is a create operation.
func IsCreateOperation(fieldName string) bool {
	lower := strings.ToLower(fieldName)
	return hasPrefix(lower, "create", "add", "new", "insert")
}

// IsUpdateOperation returns true if the mutation is an update operation.
func IsUpdateOperation(fieldName string) bool {
	lower := strings.ToLower(fieldName)
	return hasPrefix(lower, "update", "edit", "modify", "patch", "upsert", "merge")
}

// IsDeleteOperation returns true if the mutation is a delete operation.
func IsDeleteOperation(fieldName string) bool {
	lower := strings.ToLower(fieldName)
	return hasPrefix(lower, "delete", "remove", "destroy", "drop")
}

// IsReadOperation returns true if the mutation is a read operation.
func IsReadOperation(fieldName string) bool {
	lower := strings.ToLower(fieldName)
	return hasPrefix(lower, "get", "fetch", "find", "list", "search")
}

// ExtractResourceName extracts the resource name from a mutation field name.
// It removes method prefixes and returns the remaining part.
//
// Examples:
//   - createUser → "User"
//   - updateUserProfile → "UserProfile"
//   - deleteUserById → "UserById"
func ExtractResourceName(fieldName string) string {
	// Try to find and remove known prefixes
	lower := strings.ToLower(fieldName)

	for _, rule := range MethodRules {
		for _, prefix := range rule.Prefixes {
			if strings.HasPrefix(lower, prefix) {
				// Return the remaining part, preserving original case
				return fieldName[len(prefix):]
			}
		}
	}

	// No prefix found, return the original name
	return fieldName
}

// SuggestPath suggests a REST path for a mutation.
// It combines the resource name (in kebab-case) with optional ID parameter.
//
// Examples:
//   - createUser → "/users"
//   - updateUser(id) → "/users/{id}"
//   - deleteUserAccount(id) → "/user-accounts/{id}"
func SuggestPath(fieldName string, hasIDParam bool) string {
	resource := ExtractResourceName(fieldName)
	kebab := toKebabCase(resource)

	// Pluralize for collection endpoints
	if IsCreateOperation(fieldName) || IsReadOperation(fieldName) {
		kebab = pluralize(kebab)
	}

	if hasIDParam {
		return "/" + kebab + "/{id}"
	}
	return "/" + kebab
}

// toKebabCase converts camelCase/PascalCase to kebab-case.
func toKebabCase(s string) string {
	if s == "" {
		return s
	}

	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteByte('-')
		}
		result.WriteRune(r)
	}

	return strings.ToLower(result.String())
}

// pluralize attempts to pluralize a kebab-case word.
// This is a simple implementation that just adds "s" for most cases.
func pluralize(s string) string {
	if s == "" {
		return s
	}
	// Handle some common cases
	if strings.HasSuffix(s, "y") {
		return s[:len(s)-1] + "ies"
	}
	if strings.HasSuffix(s, "s") || strings.HasSuffix(s, "x") ||
		strings.HasSuffix(s, "ch") || strings.HasSuffix(s, "sh") {
		return s + "es"
	}
	return s + "s"
}
