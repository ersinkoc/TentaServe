package gql2rest

import (
	"testing"
)

func TestInferMethod(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		expected string
	}{
		{"create prefix", "createUser", "POST"},
		{"add prefix", "addItem", "POST"},
		{"new prefix", "newOrder", "POST"},
		{"insert prefix", "insertRecord", "POST"},
		{"update prefix", "updateUser", "PUT"},
		{"edit prefix", "editProfile", "PUT"},
		{"modify prefix", "modifySettings", "PUT"},
		{"patch prefix", "patchProfile", "PATCH"},
		{"delete prefix", "deleteUser", "DELETE"},
		{"remove prefix", "removeItem", "DELETE"},
		{"destroy prefix", "destroySession", "DELETE"},
		{"drop prefix", "dropTable", "DELETE"},
		{"upsert prefix", "upsertUser", "PUT"},
		{"merge prefix", "mergeAccounts", "PUT"},
		{"get prefix", "getUser", "GET"},
		{"fetch prefix", "fetchData", "GET"},
		{"find prefix", "findUser", "GET"},
		{"list prefix", "listUsers", "GET"},
		{"search prefix", "searchItems", "GET"},
		{"unknown defaults to POST", "doSomething", "POST"},
		{"case insensitive", "CreateUser", "POST"},
		{"case insensitive update", "UpdateProfile", "PUT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := InferMethod(tt.field)
			if result != tt.expected {
				t.Errorf("InferMethod(%q) = %q, want %q", tt.field, result, tt.expected)
			}
		})
	}
}

func TestInferMethodWithCustomRules(t *testing.T) {
	customRules := []MethodRule{
		{Prefixes: []string{"archive"}, Method: "PATCH"},
		{Prefixes: []string{"bulk"}, Method: "POST"},
	}

	tests := []struct {
		name     string
		field    string
		expected string
	}{
		{"custom archive rule", "archiveUser", "PATCH"},
		{"custom bulk rule", "bulkCreate", "POST"},
		{"falls back to standard", "createUser", "POST"},
		{"falls back to standard delete", "deleteUser", "DELETE"},
		{"unknown falls through custom then standard", "doSomething", "POST"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := InferMethodWithCustomRules(tt.field, customRules)
			if result != tt.expected {
				t.Errorf("InferMethodWithCustomRules(%q) = %q, want %q", tt.field, result, tt.expected)
			}
		})
	}
}

func TestIsCreateOperation(t *testing.T) {
	tests := []struct {
		field    string
		expected bool
	}{
		{"createUser", true},
		{"addItem", true},
		{"newOrder", true},
		{"insertRecord", true},
		{"updateUser", false},
		{"deleteUser", false},
		{"getUser", false},
		{"CreateUser", true},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			result := IsCreateOperation(tt.field)
			if result != tt.expected {
				t.Errorf("IsCreateOperation(%q) = %v, want %v", tt.field, result, tt.expected)
			}
		})
	}
}

func TestIsUpdateOperation(t *testing.T) {
	tests := []struct {
		field    string
		expected bool
	}{
		{"updateUser", true},
		{"editProfile", true},
		{"modifySettings", true},
		{"patchUser", true},
		{"upsertUser", true},
		{"mergeAccounts", true},
		{"createUser", false},
		{"deleteUser", false},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			result := IsUpdateOperation(tt.field)
			if result != tt.expected {
				t.Errorf("IsUpdateOperation(%q) = %v, want %v", tt.field, result, tt.expected)
			}
		})
	}
}

func TestIsDeleteOperation(t *testing.T) {
	tests := []struct {
		field    string
		expected bool
	}{
		{"deleteUser", true},
		{"removeItem", true},
		{"destroySession", true},
		{"dropTable", true},
		{"createUser", false},
		{"updateUser", false},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			result := IsDeleteOperation(tt.field)
			if result != tt.expected {
				t.Errorf("IsDeleteOperation(%q) = %v, want %v", tt.field, result, tt.expected)
			}
		})
	}
}

func TestIsReadOperation(t *testing.T) {
	tests := []struct {
		field    string
		expected bool
	}{
		{"getUser", true},
		{"fetchData", true},
		{"findUser", true},
		{"listUsers", true},
		{"searchItems", true},
		{"createUser", false},
		{"deleteUser", false},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			result := IsReadOperation(tt.field)
			if result != tt.expected {
				t.Errorf("IsReadOperation(%q) = %v, want %v", tt.field, result, tt.expected)
			}
		})
	}
}

func TestExtractResourceName(t *testing.T) {
	tests := []struct {
		field    string
		expected string
	}{
		{"createUser", "User"},
		{"updateUserProfile", "UserProfile"},
		{"deleteItem", "Item"},
		{"getUser", "User"},
		{"listUsers", "Users"},
		{"unknownField", "unknownField"},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			result := ExtractResourceName(tt.field)
			if result != tt.expected {
				t.Errorf("ExtractResourceName(%q) = %q, want %q", tt.field, result, tt.expected)
			}
		})
	}
}

func TestSuggestPath(t *testing.T) {
	tests := []struct {
		field      string
		hasIDParam bool
		expected   string
	}{
		{"createUser", false, "/users"},
		{"getUser", true, "/users/{id}"},
		{"updateUser", true, "/user/{id}"},
		{"deleteUser", true, "/user/{id}"},
		{"listUsers", false, "/userses"},
		{"createUserProfile", false, "/user-profiles"},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			result := SuggestPath(tt.field, tt.hasIDParam)
			if result != tt.expected {
				t.Errorf("SuggestPath(%q, %v) = %q, want %q", tt.field, tt.hasIDParam, result, tt.expected)
			}
		})
	}
}

func TestPluralize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"user", "users"},
		{"category", "categories"},
		{"bus", "buses"},
		{"box", "boxes"},
		{"church", "churches"},
		{"brush", "brushes"},
		{"item", "items"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := pluralize(tt.input)
			if result != tt.expected {
				t.Errorf("pluralize(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
