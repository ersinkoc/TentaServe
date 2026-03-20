package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ersinkoc/tentaserve/internal/graphql"
)

// TestREST2GQL_QueryListUsers verifies a GraphQL query that calls a REST
// upstream and returns a list of users.
func TestREST2GQL_QueryListUsers(t *testing.T) {
	restSrv := startMockRESTServer()
	defer restSrv.Close()

	handler := graphql.NewHandler(graphql.HandlerConfig{
		MaxDepth:      10,
		MaxComplexity: 1000,
	})

	handler.RegisterResolver("Query", "users", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		resp, err := http.Get(restSrv.URL + "/users")
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		var users []mockUser
		if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
			return nil, err
		}
		result := make([]interface{}, len(users))
		for i, u := range users {
			result[i] = map[string]interface{}{
				"id": u.ID, "name": u.Name, "email": u.Email,
			}
		}
		return result, nil
	})

	gqlServer := httptest.NewServer(handler)
	defer gqlServer.Close()

	body := `{"query":"query { users { id name email } }"}`
	resp, err := http.Post(gqlServer.URL, "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	respBody := readBody(t, resp)

	assertStatusCode(t, resp, http.StatusOK)
	assertContains(t, respBody, "Alice")
	assertContains(t, respBody, "Bob")
	assertJSONNoErrors(t, respBody)
}

// TestREST2GQL_MutationCreateUser tests a GraphQL mutation that POSTs to a
// REST upstream and returns the created resource.
func TestREST2GQL_MutationCreateUser(t *testing.T) {
	restSrv := startMockRESTServer()
	defer restSrv.Close()

	handler := graphql.NewHandler(graphql.HandlerConfig{
		MaxDepth:      10,
		MaxComplexity: 1000,
	})

	handler.RegisterResolver("Mutation", "createUser", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		input, _ := args["input"].(map[string]interface{})
		jsonBody, _ := json.Marshal(input)
		resp, err := http.Post(restSrv.URL+"/users", "application/json", bytes.NewReader(jsonBody))
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		var user mockUser
		if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"id": user.ID, "name": user.Name, "email": user.Email,
		}, nil
	})

	gqlServer := httptest.NewServer(handler)
	defer gqlServer.Close()

	body := `{"query":"mutation { createUser(input: {name: \"Dave\", email: \"dave@test.com\"}) { id name } }"}`
	resp, err := http.Post(gqlServer.URL, "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	respBody := readBody(t, resp)

	assertStatusCode(t, resp, http.StatusOK)
	assertJSONField(t, respBody, "data")
}

// TestREST2GQL_ErrorHandling_TableDriven verifies that HTTP error codes from
// the REST upstream propagate as GraphQL errors using table-driven cases.
func TestREST2GQL_ErrorHandling_TableDriven(t *testing.T) {
	restSrv := startMockRESTServer()
	defer restSrv.Close()

	tests := []struct {
		name        string
		fieldName   string
		upstreamURL string
		wantInBody  string
	}{
		{
			name:        "404 from upstream",
			fieldName:   "missing",
			upstreamURL: restSrv.URL + "/error/404",
			wantInBody:  "upstream error",
		},
		{
			name:        "500 from upstream",
			fieldName:   "serverError",
			upstreamURL: restSrv.URL + "/error/500",
			wantInBody:  "upstream error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := graphql.NewHandler(graphql.HandlerConfig{
				MaxDepth:      10,
				MaxComplexity: 1000,
			})

			targetURL := tt.upstreamURL
			handler.RegisterResolver("Query", tt.fieldName, func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				resp, err := http.Get(targetURL)
				if err != nil {
					return nil, err
				}
				defer resp.Body.Close()
				if resp.StatusCode >= 400 {
					return nil, fmt.Errorf("upstream error: status %d", resp.StatusCode)
				}
				return nil, nil
			})

			gqlServer := httptest.NewServer(handler)
			defer gqlServer.Close()

			body := fmt.Sprintf(`{"query":"query { %s }"}`, tt.fieldName)
			resp, err := http.Post(gqlServer.URL, "application/json", bytes.NewReader([]byte(body)))
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			respBody := readBody(t, resp)

			// GraphQL layer returns errors in the body
			assertContains(t, respBody, tt.wantInBody)
		})
	}
}

// TestREST2GQL_NestedQuery verifies nested field resolution from a REST
// upstream that returns a single user by ID.
func TestREST2GQL_NestedQuery(t *testing.T) {
	restSrv := startMockRESTServer()
	defer restSrv.Close()

	handler := graphql.NewHandler(graphql.HandlerConfig{
		MaxDepth:      10,
		MaxComplexity: 1000,
	})

	handler.RegisterResolver("Query", "user", func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
		id, _ := args["id"].(string)
		resp, err := http.Get(restSrv.URL + "/users/" + id)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("user not found")
		}
		var user mockUser
		if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"id": user.ID, "name": user.Name, "email": user.Email,
		}, nil
	})

	gqlServer := httptest.NewServer(handler)
	defer gqlServer.Close()

	body := `{"query":"query { user(id: \"1\") { id name email } }"}`
	resp, err := http.Post(gqlServer.URL, "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	respBody := readBody(t, resp)

	assertStatusCode(t, resp, http.StatusOK)
	assertContains(t, respBody, "Alice")
	assertJSONNoErrors(t, respBody)
}

// TestREST2GQL_HTTPMethodNotAllowed verifies GET requests to GraphQL endpoint
// are rejected with 405.
func TestREST2GQL_HTTPMethodNotAllowed(t *testing.T) {
	handler := graphql.NewHandler(graphql.HandlerConfig{})

	req := httptest.NewRequest("GET", "/graphql", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// TestREST2GQL_InvalidJSON verifies invalid JSON body returns 400.
func TestREST2GQL_InvalidJSON(t *testing.T) {
	handler := graphql.NewHandler(graphql.HandlerConfig{})

	req := httptest.NewRequest("POST", "/graphql", bytes.NewReader([]byte(`{not json`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
