// Package graphql provides GraphQL-to-GraphQL proxy functionality.
// It forwards GraphQL queries to upstream GraphQL endpoints and handles
// response mapping, error translation, and schema introspection.
package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is a GraphQL proxy client.
type Client struct {
	endpoint   string
	httpClient *http.Client
	headers    map[string]string
}

// ClientOptions configures the GraphQL client.
type ClientOptions struct {
	Endpoint string
	Timeout  time.Duration
	Headers  map[string]string
}

// NewClient creates a new GraphQL proxy client.
func NewClient(opts ClientOptions) *Client {
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &Client{
		endpoint: opts.Endpoint,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		headers: opts.Headers,
	}
}

// QueryRequest represents a GraphQL query request.
type QueryRequest struct {
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
	OperationName string                 `json:"operationName,omitempty"`
}

// QueryResponse represents a GraphQL query response.
type QueryResponse struct {
	Data   json.RawMessage `json:"data,omitempty"`
	Errors []GraphQLError  `json:"errors,omitempty"`
}

// GraphQLError represents a GraphQL error.
type GraphQLError struct {
	Message    string                 `json:"message"`
	Path       []interface{}          `json:"path,omitempty"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

// Error returns the error message.
func (e GraphQLError) Error() string {
	return e.Message
}

// Execute sends a GraphQL query to the upstream endpoint.
func (c *Client) Execute(ctx context.Context, req QueryRequest) (*QueryResponse, error) {
	// Build request body
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	// Add custom headers
	for key, value := range c.headers {
		httpReq.Header.Set(key, value)
	}

	// Forward auth headers from context
	c.addHeadersFromContext(ctx, httpReq)

	// Execute request
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer httpResp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	// Check status code
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream error: status=%d, body=%s", httpResp.StatusCode, string(respBody))
	}

	// Parse response
	var resp QueryResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return &resp, nil
}

// addHeadersFromContext forwards headers from context (auth, etc.).
func (c *Client) addHeadersFromContext(ctx context.Context, req *http.Request) {
	// Extract auth token from context if present
	if authHeader := ctx.Value("Authorization"); authHeader != nil {
		if str, ok := authHeader.(string); ok {
			req.Header.Set("Authorization", str)
		}
	}

	// Extract other common headers
	for _, header := range []string{"X-Request-ID", "X-User-ID", "X-Tenant-ID"} {
		if val := ctx.Value(header); val != nil {
			if str, ok := val.(string); ok {
				req.Header.Set(header, str)
			}
		}
	}
}

// IntrospectionResponse represents a GraphQL introspection response.
type IntrospectionResponse struct {
	Data   *IntrospectionData `json:"data,omitempty"`
	Errors []GraphQLError     `json:"errors,omitempty"`
}

// IntrospectionData contains the introspection result.
type IntrospectionData struct {
	Schema *IntrospectionSchema `json:"__schema,omitempty"`
}

// IntrospectionSchema represents the GraphQL schema from introspection.
type IntrospectionSchema struct {
	QueryType        *IntrospectionType        `json:"queryType,omitempty"`
	MutationType     *IntrospectionType        `json:"mutationType,omitempty"`
	SubscriptionType *IntrospectionType        `json:"subscriptionType,omitempty"`
	Types            []*IntrospectionFullType  `json:"types,omitempty"`
	Directives       []*IntrospectionDirective `json:"directives,omitempty"`
}

// IntrospectionType represents a type reference.
type IntrospectionType struct {
	Name string `json:"name,omitempty"`
	Kind string `json:"kind,omitempty"`
}

// IntrospectionFullType represents a full type definition.
type IntrospectionFullType struct {
	Kind          string                     `json:"kind"`
	Name          string                     `json:"name,omitempty"`
	Description   string                     `json:"description,omitempty"`
	Fields        []*IntrospectionField      `json:"fields,omitempty"`
	InputFields   []*IntrospectionInputValue `json:"inputFields,omitempty"`
	Interfaces    []*IntrospectionType       `json:"interfaces,omitempty"`
	EnumValues    []*IntrospectionEnumValue  `json:"enumValues,omitempty"`
	PossibleTypes []*IntrospectionType       `json:"possibleTypes,omitempty"`
}

// IntrospectionField represents a field definition.
type IntrospectionField struct {
	Name              string                     `json:"name"`
	Description       string                     `json:"description,omitempty"`
	Args              []*IntrospectionInputValue `json:"args,omitempty"`
	Type              *IntrospectionTypeRef      `json:"type"`
	IsDeprecated      bool                       `json:"isDeprecated"`
	DeprecationReason string                     `json:"deprecationReason,omitempty"`
}

// IntrospectionInputValue represents an input value.
type IntrospectionInputValue struct {
	Name         string                `json:"name"`
	Description  string                `json:"description,omitempty"`
	Type         *IntrospectionTypeRef `json:"type"`
	DefaultValue string                `json:"defaultValue,omitempty"`
}

// IntrospectionTypeRef represents a type reference.
type IntrospectionTypeRef struct {
	Kind   string                `json:"kind"`
	Name   string                `json:"name,omitempty"`
	OfType *IntrospectionTypeRef `json:"ofType,omitempty"`
}

// IntrospectionEnumValue represents an enum value.
type IntrospectionEnumValue struct {
	Name              string `json:"name"`
	Description       string `json:"description,omitempty"`
	IsDeprecated      bool   `json:"isDeprecated"`
	DeprecationReason string `json:"deprecationReason,omitempty"`
}

// IntrospectionDirective represents a directive.
type IntrospectionDirective struct {
	Name        string                     `json:"name"`
	Description string                     `json:"description,omitempty"`
	Locations   []string                   `json:"locations"`
	Args        []*IntrospectionInputValue `json:"args,omitempty"`
}

// Introspect performs a GraphQL introspection query.
func (c *Client) Introspect(ctx context.Context) (*IntrospectionSchema, error) {
	req := QueryRequest{
		Query: introspectionQuery,
	}

	resp, err := c.Execute(ctx, req)
	if err != nil {
		return nil, err
	}

	// Check for GraphQL errors
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("introspection error: %s", resp.Errors[0].Message)
	}

	// Parse introspection response
	var introResp IntrospectionResponse
	if err := json.Unmarshal(resp.Data, &introResp.Data); err != nil {
		return nil, fmt.Errorf("parsing introspection data: %w", err)
	}

	if introResp.Data == nil || introResp.Data.Schema == nil {
		return nil, fmt.Errorf("no schema in introspection response")
	}

	return introResp.Data.Schema, nil
}

// introspectionQuery is the standard GraphQL introspection query.
const introspectionQuery = `
  query IntrospectionQuery {
    __schema {
      queryType { name }
      mutationType { name }
      subscriptionType { name }
      types {
        ...FullType
      }
      directives {
        name
        description
        locations
        args {
          ...InputValue
        }
      }
    }
  }

  fragment FullType on __Type {
    kind
    name
    description
    fields(includeDeprecated: true) {
      name
      description
      args {
        ...InputValue
      }
      type {
        ...TypeRef
      }
      isDeprecated
      deprecationReason
    }
    inputFields {
      ...InputValue
    }
    interfaces {
      ...TypeRef
    }
    enumValues(includeDeprecated: true) {
      name
      description
      isDeprecated
      deprecationReason
    }
    possibleTypes {
      ...TypeRef
    }
  }

  fragment InputValue on __InputValue {
    name
    description
    type { ...TypeRef }
    defaultValue
  }

  fragment TypeRef on __Type {
    kind
    name
    ofType {
      kind
      name
      ofType {
        kind
        name
        ofType {
          kind
          name
          ofType {
            kind
            name
            ofType {
              kind
              name
              ofType {
                kind
                name
                ofType {
                  kind
                  name
                }
              }
            }
          }
        }
      }
    }
  }
`
