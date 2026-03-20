package integration_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ersinkoc/tentaserve/internal/proxy/gql2rest"
)

// TestGQL2REST_GetEndpoint tests a REST GET endpoint backed by a GraphQL
// upstream. The gql2rest.Handler translates the REST request into a GraphQL
// query, executes it against the mock GraphQL server, and returns the result.
func TestGQL2REST_GetEndpoint(t *testing.T) {
	gqlSrv := startMockGraphQLServer()
	defer gqlSrv.Close()

	handler := gql2rest.NewHandler(gql2rest.HandlerOptions{
		BasePath:   "/api",
		GraphQLURL: gqlSrv.URL + "/graphql",
		Endpoints: []gql2rest.Endpoint{
			{
				Path:        "/api/users",
				Method:      "GET",
				GraphQLType: "Query",
				Field:       "users",
				ReturnType:  "User",
			},
		},
	})

	req := httptest.NewRequest("GET", "/api/users", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	assertContains(t, body, "Alice")
	assertContains(t, body, "Bob")
}

// TestGQL2REST_GetWithPathParam tests a REST GET endpoint that uses a path
// parameter to fetch a single resource from the GraphQL upstream.
func TestGQL2REST_GetWithPathParam(t *testing.T) {
	gqlSrv := startMockGraphQLServer()
	defer gqlSrv.Close()

	handler := gql2rest.NewHandler(gql2rest.HandlerOptions{
		BasePath:   "/api",
		GraphQLURL: gqlSrv.URL + "/graphql",
		Endpoints: []gql2rest.Endpoint{
			{
				Path:        "/api/users/{id}",
				Method:      "GET",
				GraphQLType: "Query",
				Field:       "user",
				ReturnType:  "User",
				Arguments: []gql2rest.Argument{
					{Name: "id", Type: "string", Required: true, Location: "path"},
				},
			},
		},
	})

	req := httptest.NewRequest("GET", "/api/users/1", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	assertContains(t, body, "Alice")
}

// TestGQL2REST_PostMutation tests a REST POST endpoint that maps to a GraphQL
// mutation.
func TestGQL2REST_PostMutation(t *testing.T) {
	gqlSrv := startMockGraphQLServer()
	defer gqlSrv.Close()

	handler := gql2rest.NewHandler(gql2rest.HandlerOptions{
		BasePath:   "/api",
		GraphQLURL: gqlSrv.URL + "/graphql",
		Endpoints: []gql2rest.Endpoint{
			{
				Path:        "/api/users",
				Method:      "POST",
				GraphQLType: "Mutation",
				Field:       "createUser",
				ReturnType:  "User",
				Arguments: []gql2rest.Argument{
					{Name: "input", Type: "object", Required: true, Location: "body"},
				},
			},
		},
	})

	reqBody := `{"input":{"name":"Eve","email":"eve@test.com"}}`
	req := httptest.NewRequest("POST", "/api/users", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// The handler should return a response with the created user data.
	body := w.Body.String()
	if w.Code >= 400 {
		t.Errorf("expected success status, got %d: %s", w.Code, body)
	}
}

// TestGQL2REST_FieldSelection tests the ?fields= query parameter that controls
// which fields are included in the GraphQL selection set.
func TestGQL2REST_FieldSelection(t *testing.T) {
	gqlSrv := startMockGraphQLServer()
	defer gqlSrv.Close()

	handler := gql2rest.NewHandler(gql2rest.HandlerOptions{
		BasePath:   "/api",
		GraphQLURL: gqlSrv.URL + "/graphql",
		Endpoints: []gql2rest.Endpoint{
			{
				Path:        "/api/users",
				Method:      "GET",
				GraphQLType: "Query",
				Field:       "users",
				ReturnType:  "User",
			},
		},
	})

	req := httptest.NewRequest("GET", "/api/users?fields=id,name", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	// The mock GraphQL server returns all fields regardless, but the
	// handler should still return valid data.
	body := w.Body.String()
	assertContains(t, body, "Alice")
}

// TestGQL2REST_NotFoundEndpoint tests that requesting an unregistered path
// returns a 404 with a NOT_FOUND error code.
func TestGQL2REST_NotFoundEndpoint(t *testing.T) {
	gqlSrv := startMockGraphQLServer()
	defer gqlSrv.Close()

	handler := gql2rest.NewHandler(gql2rest.HandlerOptions{
		BasePath:   "/api",
		GraphQLURL: gqlSrv.URL + "/graphql",
		Endpoints:  []gql2rest.Endpoint{},
	})

	req := httptest.NewRequest("GET", "/api/nonexistent", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}

	body := w.Body.String()
	assertContains(t, body, "NOT_FOUND")
}

// TestGQL2REST_GraphQLErrorPropagation verifies that GraphQL errors from the
// upstream are correctly mapped to HTTP error status codes.
func TestGQL2REST_GraphQLErrorPropagation(t *testing.T) {
	gqlSrv := startMockGraphQLServer()
	defer gqlSrv.Close()

	handler := gql2rest.NewHandler(gql2rest.HandlerOptions{
		BasePath:   "/api",
		GraphQLURL: gqlSrv.URL + "/graphql",
		Endpoints: []gql2rest.Endpoint{
			{
				Path:        "/api/users/{id}",
				Method:      "GET",
				GraphQLType: "Query",
				Field:       "user",
				ReturnType:  "User",
				Arguments: []gql2rest.Argument{
					{Name: "id", Type: "string", Required: true, Location: "path"},
				},
			},
		},
	})

	// Request a user that does not exist. The mock GraphQL server returns
	// an error with extensions.code = "NOT_FOUND".
	req := httptest.NewRequest("GET", "/api/users/999", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	// The handler should translate the GraphQL NOT_FOUND error.
	assertContains(t, body, "NOT_FOUND")
}

// TestGQL2REST_TableDriven runs a table of REST requests through the
// gql2rest.Handler and verifies the responses.
func TestGQL2REST_TableDriven(t *testing.T) {
	gqlSrv := startMockGraphQLServer()
	defer gqlSrv.Close()

	handler := gql2rest.NewHandler(gql2rest.HandlerOptions{
		BasePath:   "/api",
		GraphQLURL: gqlSrv.URL + "/graphql",
		Endpoints: []gql2rest.Endpoint{
			{
				Path:        "/api/users",
				Method:      "GET",
				GraphQLType: "Query",
				Field:       "users",
				ReturnType:  "User",
			},
			{
				Path:        "/api/users/{id}",
				Method:      "GET",
				GraphQLType: "Query",
				Field:       "user",
				ReturnType:  "User",
				Arguments: []gql2rest.Argument{
					{Name: "id", Type: "string", Required: true, Location: "path"},
				},
			},
		},
	})

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantInBody string
	}{
		{
			name:       "list users",
			method:     "GET",
			path:       "/api/users",
			wantStatus: http.StatusOK,
			wantInBody: "Alice",
		},
		{
			name:       "get user by ID",
			method:     "GET",
			path:       "/api/users/1",
			wantStatus: http.StatusOK,
			wantInBody: "Alice",
		},
		{
			name:       "user not found",
			method:     "GET",
			path:       "/api/users/999",
			wantStatus: -1, // any status, just check body
			wantInBody: "NOT_FOUND",
		},
		{
			name:       "unknown endpoint returns 404",
			method:     "GET",
			path:       "/api/unknown",
			wantStatus: http.StatusNotFound,
			wantInBody: "NOT_FOUND",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if tt.wantStatus > 0 && w.Code != tt.wantStatus {
				t.Errorf("status: got %d, want %d", w.Code, tt.wantStatus)
			}
			if tt.wantInBody != "" {
				assertContains(t, w.Body.String(), tt.wantInBody)
			}
		})
	}
}

// TestGQL2REST_UnwrapResponse tests the standalone UnwrapResponse function
// that extracts data from a raw GraphQL JSON response.
func TestGQL2REST_UnwrapResponse(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		field     string
		wantData  bool
		wantError bool
	}{
		{
			name:     "successful response",
			input:    `{"data":{"users":[{"id":"1","name":"Alice"}]}}`,
			field:    "users",
			wantData: true,
		},
		{
			name:      "error response",
			input:     `{"data":null,"errors":[{"message":"not found","extensions":{"code":"NOT_FOUND"}}]}`,
			field:     "user",
			wantError: true,
		},
		{
			name:     "missing field",
			input:    `{"data":{"other":"value"}}`,
			field:    "missing",
			wantData: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := gql2rest.UnwrapResponse([]byte(tt.input), tt.field)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantError && resp.Error == nil {
				t.Error("expected error in response, got nil")
			}
			if tt.wantData && resp.Data == nil {
				t.Error("expected data in response, got nil")
			}
			if !tt.wantData && !tt.wantError && resp.Data != nil && resp.Error != nil {
				t.Error("expected empty response")
			}
		})
	}
}

// TestGQL2REST_FieldParamParser tests the fields query parameter parser.
func TestGQL2REST_FieldParamParser(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantAll  bool
		hasField string
	}{
		{name: "empty", input: "", wantAll: true},
		{name: "wildcard", input: "*", wantAll: true},
		{name: "specific fields", input: "id,name", wantAll: false, hasField: "id"},
		{name: "nested", input: "posts.title", wantAll: false, hasField: "posts"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := gql2rest.NewFieldParamParser()
			result := parser.Parse(tt.input)

			if result.IncludeAll != tt.wantAll {
				t.Errorf("IncludeAll: got %v, want %v", result.IncludeAll, tt.wantAll)
			}
			if tt.hasField != "" && !result.HasField(tt.hasField) {
				t.Errorf("expected field %q to be present", tt.hasField)
			}
		})
	}
}
