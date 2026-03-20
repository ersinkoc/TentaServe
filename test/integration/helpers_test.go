package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- Mock REST Server ---

// mockUser represents a user in the mock REST API.
type mockUser struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// mockRESTData holds the seed data for the mock REST server.
var mockRESTData = []mockUser{
	{ID: "1", Name: "Alice", Email: "alice@example.com"},
	{ID: "2", Name: "Bob", Email: "bob@example.com"},
	{ID: "3", Name: "Charlie", Email: "charlie@example.com"},
}

// startMockRESTServer starts an httptest server that serves REST endpoints:
//   - GET  /users          -> list all users
//   - GET  /users/{id}     -> get a user by ID
//   - POST /users          -> create a user (returns 201)
//   - GET  /error/404      -> always returns 404
//   - GET  /error/500      -> always returns 500
func startMockRESTServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(mockRESTData)

		case http.MethodPost:
			var u mockUser
			if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
				http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
				return
			}
			u.ID = fmt.Sprintf("%d", len(mockRESTData)+1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(u)

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/users/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		id := strings.TrimPrefix(r.URL.Path, "/users/")
		if id == "" {
			http.Error(w, `{"error":"missing id"}`, http.StatusBadRequest)
			return
		}

		for _, u := range mockRESTData {
			if u.ID == id {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(u)
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	})

	mux.HandleFunc("/error/404", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "resource not found"})
	})

	mux.HandleFunc("/error/500", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
	})

	return httptest.NewServer(mux)
}

// --- Mock GraphQL Server ---

// startMockGraphQLServer starts an httptest server that responds to GraphQL
// queries on POST /graphql. It understands:
//   - query { users { id name email } }
//   - query { user(id: "...") { id name email } }
//   - mutation { createUser(input: {...}) { id name email } }
//
// For error scenarios it also returns GraphQL errors with extension codes.
func startMockGraphQLServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeGQLError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Query     string                 `json:"query"`
			Variables map[string]interface{} `json:"variables,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeGQLError(w, "invalid JSON body", http.StatusBadRequest)
			return
		}

		q := req.Query

		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(q, "users") && !strings.Contains(q, "createUser") && !strings.Contains(q, "user("):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"users": mockRESTData,
				},
			})

		case strings.Contains(q, "user("):
			id := ""
			if req.Variables != nil {
				if v, ok := req.Variables["id"]; ok {
					id = fmt.Sprintf("%v", v)
				}
			}
			if id == "" {
				if idx := strings.Index(q, `id: "`); idx >= 0 {
					rest := q[idx+5:]
					if end := strings.Index(rest, `"`); end >= 0 {
						id = rest[:end]
					}
				}
			}

			for _, u := range mockRESTData {
				if u.ID == id {
					json.NewEncoder(w).Encode(map[string]interface{}{
						"data": map[string]interface{}{
							"user": u,
						},
					})
					return
				}
			}

			json.NewEncoder(w).Encode(map[string]interface{}{
				"data":   nil,
				"errors": []map[string]interface{}{{"message": "User not found", "extensions": map[string]interface{}{"code": "NOT_FOUND"}}},
			})

		case strings.Contains(q, "createUser"):
			name := ""
			email := ""
			if req.Variables != nil {
				if input, ok := req.Variables["input"].(map[string]interface{}); ok {
					if n, ok := input["name"].(string); ok {
						name = n
					}
					if e, ok := input["email"].(string); ok {
						email = e
					}
				}
			}
			newUser := mockUser{
				ID:    "99",
				Name:  name,
				Email: email,
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"createUser": newUser,
				},
			})

		default:
			writeGQLError(w, "unknown query", http.StatusOK)
		}
	})

	return httptest.NewServer(mux)
}

// writeGQLError writes a GraphQL-style error response.
func writeGQLError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":   nil,
		"errors": []map[string]string{{"message": msg}},
	})
}

// --- Assertion helpers ---

// assertStatusCode fails the test if the response status code does not match.
func assertStatusCode(t *testing.T, resp *http.Response, expected int) {
	t.Helper()
	if resp.StatusCode != expected {
		t.Errorf("expected status %d, got %d", expected, resp.StatusCode)
	}
}

// assertContains fails the test if body does not contain substr.
func assertContains(t *testing.T, body, substr string) {
	t.Helper()
	if !strings.Contains(body, substr) {
		t.Errorf("expected body to contain %q, got: %s", substr, truncate(body, 300))
	}
}

// assertNotContains fails the test if body contains substr.
func assertNotContains(t *testing.T, body, substr string) {
	t.Helper()
	if strings.Contains(body, substr) {
		t.Errorf("expected body NOT to contain %q, got: %s", substr, truncate(body, 300))
	}
}

// assertJSONField checks that a JSON object has a given top-level key.
func assertJSONField(t *testing.T, body string, key string) {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v\nbody: %s", err, truncate(body, 300))
	}
	if _, ok := m[key]; !ok {
		t.Errorf("expected JSON key %q, not found in: %s", key, truncate(body, 300))
	}
}

// assertJSONNoErrors verifies the JSON response has no "errors" key or the
// errors array is empty/null.
func assertJSONNoErrors(t *testing.T, body string) {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v\nbody: %s", err, truncate(body, 300))
	}
	if errs, ok := m["errors"]; ok && errs != nil {
		if arr, ok := errs.([]interface{}); ok && len(arr) > 0 {
			t.Errorf("expected no errors, got: %v", errs)
		}
	}
}

// assertHeaderPresent fails if the named header is missing from the response.
func assertHeaderPresent(t *testing.T, resp *http.Response, header string) {
	t.Helper()
	if resp.Header.Get(header) == "" {
		t.Errorf("expected header %q to be present", header)
	}
}

// assertHeaderValue fails if the named header does not equal the expected value.
func assertHeaderValue(t *testing.T, resp *http.Response, header, expected string) {
	t.Helper()
	got := resp.Header.Get(header)
	if got != expected {
		t.Errorf("expected header %q = %q, got %q", header, expected, got)
	}
}

// truncate shortens a string for readable error messages.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// readBody reads the entire response body as a string and closes it.
func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	return sb.String()
}
