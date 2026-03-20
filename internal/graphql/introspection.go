package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// IntrospectionQuery is the standard GraphQL introspection query.
const IntrospectionQuery = `
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

// IntrospectionClient fetches GraphQL schemas via introspection.
type IntrospectionClient struct {
	httpClient *http.Client
	url        string
	headers    map[string]string
}

// IntrospectionClientOptions configures the introspection client.
type IntrospectionClientOptions struct {
	URL     string
	Headers map[string]string
	Client  *http.Client
}

// NewIntrospectionClient creates a new introspection client.
func NewIntrospectionClient(opts IntrospectionClientOptions) *IntrospectionClient {
	client := opts.Client
	if client == nil {
		client = http.DefaultClient
	}

	headers := opts.Headers
	if headers == nil {
		headers = make(map[string]string)
	}

	return &IntrospectionClient{
		httpClient: client,
		url:        opts.URL,
		headers:    headers,
	}
}

// IntrospectionRequest is a GraphQL request for introspection.
type IntrospectionRequest struct {
	Query string `json:"query"`
}

// IntrospectionResponse is the response from an introspection query.
type IntrospectionResponse struct {
	Data   *SchemaData    `json:"data"`
	Errors []GraphQLError `json:"errors,omitempty"`
}

// GraphQLError represents a GraphQL error.
type GraphQLError struct {
	Message    string                 `json:"message"`
	Path       []string               `json:"path,omitempty"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

// SchemaData contains the schema information.
type SchemaData struct {
	Schema *Schema `json:"__schema"`
}

// Schema represents the introspected GraphQL schema.
type Schema struct {
	QueryType        *TypeRef                 `json:"queryType"`
	MutationType     *TypeRef                 `json:"mutationType"`
	SubscriptionType *TypeRef                 `json:"subscriptionType"`
	Types            []IntrospectionType      `json:"types"`
	Directives       []IntrospectionDirective `json:"directives"`
}

// IntrospectionType represents a GraphQL type from introspection.
type IntrospectionType struct {
	Kind          string                   `json:"kind"`
	Name          string                   `json:"name"`
	Description   string                   `json:"description"`
	Fields        []IntrospectionField     `json:"fields"`
	InputFields   []InputValue             `json:"inputFields"`
	Interfaces    []TypeRef                `json:"interfaces"`
	EnumValues    []IntrospectionEnumValue `json:"enumValues"`
	PossibleTypes []TypeRef                `json:"possibleTypes"`
}

// IntrospectionField represents a field in a type.
type IntrospectionField struct {
	Name              string       `json:"name"`
	Description       string       `json:"description"`
	Args              []InputValue `json:"args"`
	Type              TypeRef      `json:"type"`
	IsDeprecated      bool         `json:"isDeprecated"`
	DeprecationReason string       `json:"deprecationReason"`
}

// InputValue represents an input value (argument or input field).
type InputValue struct {
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	Type         TypeRef `json:"type"`
	DefaultValue *string `json:"defaultValue"`
}

// TypeRef represents a type reference (supports nested wrapping types).
type TypeRef struct {
	Kind   string   `json:"kind"`
	Name   string   `json:"name"`
	OfType *TypeRef `json:"ofType"`
}

// IntrospectionEnumValue represents an enum value.
type IntrospectionEnumValue struct {
	Name              string `json:"name"`
	Description       string `json:"description"`
	IsDeprecated      bool   `json:"isDeprecated"`
	DeprecationReason string `json:"deprecationReason"`
}

// IntrospectionDirective represents a directive.
type IntrospectionDirective struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Locations   []string     `json:"locations"`
	Args        []InputValue `json:"args"`
}

// Introspect fetches the schema from a GraphQL endpoint.
func (c *IntrospectionClient) Introspect(ctx context.Context) (*Schema, error) {
	reqBody, err := json.Marshal(IntrospectionRequest{
		Query: IntrospectionQuery,
	})
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Add custom headers
	for key, value := range c.headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status: %d, body: %s", resp.StatusCode, string(body))
	}

	var introspectionResp IntrospectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&introspectionResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if len(introspectionResp.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL errors: %v", introspectionResp.Errors)
	}

	if introspectionResp.Data == nil || introspectionResp.Data.Schema == nil {
		return nil, fmt.Errorf("no schema in response")
	}

	return introspectionResp.Data.Schema, nil
}

// GetType looks up a type by name in the schema.
func (s *Schema) GetType(name string) *IntrospectionType {
	for i, t := range s.Types {
		if t.Name == name {
			return &s.Types[i]
		}
	}
	return nil
}

// UnwrapType unwraps nested type wrappers (NonNull, List) to get the named type.
func UnwrapType(t *TypeRef) *TypeRef {
	if t == nil {
		return nil
	}
	if t.Kind == "NON_NULL" || t.Kind == "LIST" {
		return UnwrapType(t.OfType)
	}
	return t
}

// GetTypeName returns the name of the innermost named type.
func GetTypeName(t *TypeRef) string {
	unwrapped := UnwrapType(t)
	if unwrapped == nil {
		return ""
	}
	return unwrapped.Name
}

// IsNonNull returns true if the type is wrapped in NonNull.
func IsNonNull(t *TypeRef) bool {
	if t == nil {
		return false
	}
	return t.Kind == "NON_NULL"
}

// IsList returns true if the type is a List.
func IsList(t *TypeRef) bool {
	if t == nil {
		return false
	}
	return t.Kind == "LIST"
}

// IsScalar returns true if the type is a scalar.
func (t *IntrospectionType) IsScalar() bool {
	return t.Kind == "SCALAR"
}

// IsObject returns true if the type is an object type.
func (t *IntrospectionType) IsObject() bool {
	return t.Kind == "OBJECT"
}

// IsEnum returns true if the type is an enum.
func (t *IntrospectionType) IsEnum() bool {
	return t.Kind == "ENUM"
}

// IsInputObject returns true if the type is an input object.
func (t *IntrospectionType) IsInputObject() bool {
	return t.Kind == "INPUT_OBJECT"
}

// IsInterface returns true if the type is an interface.
func (t *IntrospectionType) IsInterface() bool {
	return t.Kind == "INTERFACE"
}

// IsUnion returns true if the type is a union.
func (t *IntrospectionType) IsUnion() bool {
	return t.Kind == "UNION"
}
