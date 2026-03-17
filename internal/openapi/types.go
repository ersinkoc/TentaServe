package openapi

import (
	"fmt"
)

// Version represents the OpenAPI specification version.
type Version string

const (
	// Version3_0 represents OpenAPI 3.0.x
	Version3_0 Version = "3.0"
	// Version3_1 represents OpenAPI 3.1.x
	Version3_1 Version = "3.1"
)

// OpenAPISpec represents a parsed OpenAPI specification.
type OpenAPISpec struct {
	// OpenAPI is the version string (e.g., "3.0.0", "3.1.0")
	OpenAPI string `json:"openapi"`
	// Info provides metadata about the API
	Info Info `json:"info"`
	// Servers is an array of server base URLs
	Servers []Server `json:"servers,omitempty"`
	// Paths are the available paths and operations for the API
	Paths map[string]*PathItem `json:"paths"`
	// Components holds reusable schema objects
	Components *Components `json:"components,omitempty"`
	// Security describes security requirements
	Security []SecurityRequirement `json:"security,omitempty"`
	// Tags provides metadata for grouping operations
	Tags []Tag `json:"tags,omitempty"`
	// ExternalDocs links to external documentation
	ExternalDocs *ExternalDocumentation `json:"externalDocs,omitempty"`
}

// Info provides metadata about the API.
type Info struct {
	// Title is the API title
	Title string `json:"title"`
	// Description is a short description of the API
	Description string `json:"description,omitempty"`
	// TermsOfService is a URL to the terms of service
	TermsOfService string `json:"termsOfService,omitempty"`
	// Contact information for the API
	Contact *Contact `json:"contact,omitempty"`
	// License information for the API
	License *License `json:"license,omitempty"`
	// Version is the API version
	Version string `json:"version"`
}

// Contact provides contact information for the API.
type Contact struct {
	Name  string `json:"name,omitempty"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

// License provides license information for the API.
type License struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// Server provides connectivity information.
type Server struct {
	URL         string                    `json:"url"`
	Description string                    `json:"description,omitempty"`
	Variables   map[string]*ServerVariable `json:"variables,omitempty"`
}

// ServerVariable represents a server variable for URL template substitution.
type ServerVariable struct {
	Enum        []string `json:"enum,omitempty"`
	Default     string   `json:"default"`
	Description string   `json:"description,omitempty"`
}

// PathItem describes the operations available on a single path.
type PathItem struct {
	Ref         string      `json:"$ref,omitempty"`
	Summary     string      `json:"summary,omitempty"`
	Description string      `json:"description,omitempty"`
	Get         *Operation  `json:"get,omitempty"`
	Put         *Operation  `json:"put,omitempty"`
	Post        *Operation  `json:"post,omitempty"`
	Delete      *Operation  `json:"delete,omitempty"`
	Options     *Operation  `json:"options,omitempty"`
	Head        *Operation  `json:"head,omitempty"`
	Patch       *Operation  `json:"patch,omitempty"`
	Trace       *Operation  `json:"trace,omitempty"`
	Servers     []Server    `json:"servers,omitempty"`
	Parameters  []*Parameter `json:"parameters,omitempty"`
}

// Operation describes a single API operation on a path.
type Operation struct {
	Tags         []string              `json:"tags,omitempty"`
	Summary      string                `json:"summary,omitempty"`
	Description  string                `json:"description,omitempty"`
	ExternalDocs *ExternalDocumentation `json:"externalDocs,omitempty"`
	OperationID  string                `json:"operationId,omitempty"`
	Parameters   []*Parameter          `json:"parameters,omitempty"`
	RequestBody  *RequestBody          `json:"requestBody,omitempty"`
	Responses    map[string]*Response  `json:"responses"`
	Deprecated   bool                  `json:"deprecated,omitempty"`
	Security     []SecurityRequirement `json:"security,omitempty"`
	Servers      []Server              `json:"servers,omitempty"`
}

// HTTPMethod represents an HTTP method.
type HTTPMethod string

const (
	MethodGet     HTTPMethod = "GET"
	MethodPut     HTTPMethod = "PUT"
	MethodPost    HTTPMethod = "POST"
	MethodDelete  HTTPMethod = "DELETE"
	MethodOptions HTTPMethod = "OPTIONS"
	MethodHead    HTTPMethod = "HEAD"
	MethodPatch   HTTPMethod = "PATCH"
	MethodTrace   HTTPMethod = "TRACE"
)

// GetOperations returns a map of HTTP method to operation for this path item.
func (p *PathItem) GetOperations() map[HTTPMethod]*Operation {
	ops := make(map[HTTPMethod]*Operation)
	if p.Get != nil {
		ops[MethodGet] = p.Get
	}
	if p.Put != nil {
		ops[MethodPut] = p.Put
	}
	if p.Post != nil {
		ops[MethodPost] = p.Post
	}
	if p.Delete != nil {
		ops[MethodDelete] = p.Delete
	}
	if p.Options != nil {
		ops[MethodOptions] = p.Options
	}
	if p.Head != nil {
		ops[MethodHead] = p.Head
	}
	if p.Patch != nil {
		ops[MethodPatch] = p.Patch
	}
	if p.Trace != nil {
		ops[MethodTrace] = p.Trace
	}
	return ops
}

// ExternalDocumentation allows referencing an external resource.
type ExternalDocumentation struct {
	Description string `json:"description,omitempty"`
	URL         string `json:"url"`
}

// Parameter describes a single operation parameter.
type Parameter struct {
	Ref         string          `json:"$ref,omitempty"`
	Name        string          `json:"name"`
	In          string          `json:"in"`
	Description string          `json:"description,omitempty"`
	Required    bool            `json:"required,omitempty"`
	Deprecated  bool            `json:"deprecated,omitempty"`
	AllowEmptyValue bool        `json:"allowEmptyValue,omitempty"`
	Schema      *SchemaObject   `json:"schema,omitempty"`
	Example     interface{}     `json:"example,omitempty"`
	Examples    map[string]*Example `json:"examples,omitempty"`
}

// ParameterLocation represents where a parameter is located.
type ParameterLocation string

const (
	LocationQuery  ParameterLocation = "query"
	LocationHeader ParameterLocation = "header"
	LocationPath   ParameterLocation = "path"
	LocationCookie ParameterLocation = "cookie"
)

// RequestBody describes the request body.
type RequestBody struct {
	Ref         string               `json:"$ref,omitempty"`
	Description string               `json:"description,omitempty"`
	Content     map[string]*MediaType `json:"content"`
	Required    bool                 `json:"required,omitempty"`
}

// Response describes a single response from an API operation.
type Response struct {
	Ref         string               `json:"$ref,omitempty"`
	Description string               `json:"description"`
	Headers     map[string]*Header   `json:"headers,omitempty"`
	Content     map[string]*MediaType `json:"content,omitempty"`
	Links       map[string]*Link     `json:"links,omitempty"`
}

// MediaType provides schema and examples for a media type.
type MediaType struct {
	Schema   *SchemaObject        `json:"schema,omitempty"`
	Example  interface{}          `json:"example,omitempty"`
	Examples map[string]*Example  `json:"examples,omitempty"`
	Encoding map[string]*Encoding `json:"encoding,omitempty"`
}

// Example represents an example value.
type Example struct {
	Ref         string      `json:"$ref,omitempty"`
	Summary     string      `json:"summary,omitempty"`
	Description string      `json:"description,omitempty"`
	Value       interface{} `json:"value,omitempty"`
	ExternalValue string    `json:"externalValue,omitempty"`
}

// Encoding defines an encoding for a single property.
type Encoding struct {
	ContentType   string            `json:"contentType,omitempty"`
	Headers       map[string]*Header `json:"headers,omitempty"`
	Style         string            `json:"style,omitempty"`
	Explode       bool              `json:"explode,omitempty"`
	AllowReserved bool              `json:"allowReserved,omitempty"`
}

// Header follows the structure of Parameter but with different fields.
type Header struct {
	Ref         string          `json:"$ref,omitempty"`
	Description string          `json:"description,omitempty"`
	Required    bool            `json:"required,omitempty"`
	Deprecated  bool            `json:"deprecated,omitempty"`
	Schema      *SchemaObject   `json:"schema,omitempty"`
	Example     interface{}     `json:"example,omitempty"`
}

// Link represents a possible design-time link for a response.
type Link struct {
	Ref         string      `json:"$ref,omitempty"`
	OperationID string      `json:"operationId,omitempty"`
	OperationRef string     `json:"operationRef,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	RequestBody interface{} `json:"requestBody,omitempty"`
	Description string      `json:"description,omitempty"`
	Server      *Server     `json:"server,omitempty"`
}

// SchemaObject defines a data schema.
type SchemaObject struct {
	Ref                  string                 `json:"$ref,omitempty"`
	Title                string                 `json:"title,omitempty"`
	MultipleOf           float64                `json:"multipleOf,omitempty"`
	Maximum              float64                `json:"maximum,omitempty"`
	ExclusiveMaximum     bool                   `json:"exclusiveMaximum,omitempty"`
	Minimum              float64                `json:"minimum,omitempty"`
	ExclusiveMinimum     bool                   `json:"exclusiveMinimum,omitempty"`
	MaxLength            int                    `json:"maxLength,omitempty"`
	MinLength            int                    `json:"minLength,omitempty"`
	Pattern              string                 `json:"pattern,omitempty"`
	MaxItems             int                    `json:"maxItems,omitempty"`
	MinItems             int                    `json:"minItems,omitempty"`
	UniqueItems          bool                   `json:"uniqueItems,omitempty"`
	MaxProperties        int                    `json:"maxProperties,omitempty"`
	MinProperties        int                    `json:"minProperties,omitempty"`
	Required             []string               `json:"required,omitempty"`
	Enum                 []interface{}          `json:"enum,omitempty"`
	Type                 string                 `json:"type,omitempty"`
	AllOf                []*SchemaObject        `json:"allOf,omitempty"`
	OneOf                []*SchemaObject        `json:"oneOf,omitempty"`
	AnyOf                []*SchemaObject        `json:"anyOf,omitempty"`
	Not                  *SchemaObject          `json:"not,omitempty"`
	Items                *SchemaObject          `json:"items,omitempty"`
	Properties           map[string]*SchemaObject `json:"properties,omitempty"`
	AdditionalProperties *SchemaObject          `json:"additionalProperties,omitempty"`
	Description          string                 `json:"description,omitempty"`
	Format               string                 `json:"format,omitempty"`
	Default              interface{}            `json:"default,omitempty"`
	Nullable             bool                   `json:"nullable,omitempty"`
	ReadOnly             bool                   `json:"readOnly,omitempty"`
	WriteOnly            bool                   `json:"writeOnly,omitempty"`
	Example              interface{}            `json:"example,omitempty"`
	Deprecated           bool                   `json:"deprecated,omitempty"`
	// OpenAPI 3.1 only
	Const        interface{}            `json:"const,omitempty"`
	ExclusiveMaximumNum *float64        `json:"exclusiveMaximumNum,omitempty"`
	ExclusiveMinimumNum *float64        `json:"exclusiveMinimumNum,omitempty"`
}

// SchemaType represents the JSON Schema type.
type SchemaType string

const (
	TypeNull    SchemaType = "null"
	TypeBoolean SchemaType = "boolean"
	TypeObject  SchemaType = "object"
	TypeArray   SchemaType = "array"
	TypeNumber  SchemaType = "number"
	TypeString  SchemaType = "string"
	TypeInteger SchemaType = "integer"
)

// IsPrimitive returns true if the schema represents a primitive type.
func (s *SchemaObject) IsPrimitive() bool {
	if s == nil {
		return false
	}
	switch s.Type {
	case "string", "integer", "number", "boolean":
		return true
	default:
		return false
	}
}

// IsArray returns true if the schema represents an array type.
func (s *SchemaObject) IsArray() bool {
	return s != nil && s.Type == "array"
}

// IsObject returns true if the schema represents an object type.
func (s *SchemaObject) IsObject() bool {
	return s != nil && s.Type == "object"
}

// GetEffectiveType returns the type, resolving $ref if necessary.
func (s *SchemaObject) GetEffectiveType() string {
	if s == nil {
		return ""
	}
	return s.Type
}

// Components holds a set of reusable objects.
type Components struct {
	Schemas         map[string]*SchemaObject  `json:"schemas,omitempty"`
	Responses       map[string]*Response      `json:"responses,omitempty"`
	Parameters      map[string]*Parameter     `json:"parameters,omitempty"`
	Examples        map[string]*Example       `json:"examples,omitempty"`
	RequestBodies   map[string]*RequestBody   `json:"requestBodies,omitempty"`
	Headers         map[string]*Header        `json:"headers,omitempty"`
	SecuritySchemes map[string]*SecurityScheme `json:"securitySchemes,omitempty"`
	Links           map[string]*Link          `json:"links,omitempty"`
	Callbacks       map[string]interface{}    `json:"callbacks,omitempty"`
}

// SecurityScheme defines a security scheme.
type SecurityScheme struct {
	Ref              string `json:"$ref,omitempty"`
	Type             string `json:"type"`
	Description      string `json:"description,omitempty"`
	Name             string `json:"name,omitempty"`
	In               string `json:"in,omitempty"`
	Scheme           string `json:"scheme,omitempty"`
	BearerFormat     string `json:"bearerFormat,omitempty"`
	Flows            *OAuthFlows `json:"flows,omitempty"`
	OpenIDConnectURL string `json:"openIdConnectUrl,omitempty"`
}

// SecuritySchemeType represents the type of security scheme.
type SecuritySchemeType string

const (
	SecurityHTTP         SecuritySchemeType = "http"
	SecurityAPIKey       SecuritySchemeType = "apiKey"
	SecurityOAuth2       SecuritySchemeType = "oauth2"
	SecurityOpenIDConnect SecuritySchemeType = "openIdConnect"
)

// OAuthFlows allows configuration of supported OAuth flows.
type OAuthFlows struct {
	Implicit          *OAuthFlow `json:"implicit,omitempty"`
	Password          *OAuthFlow `json:"password,omitempty"`
	ClientCredentials *OAuthFlow `json:"clientCredentials,omitempty"`
	AuthorizationCode *OAuthFlow `json:"authorizationCode,omitempty"`
}

// OAuthFlow describes an OAuth flow.
type OAuthFlow struct {
	AuthorizationURL string `json:"authorizationUrl,omitempty"`
	TokenURL         string `json:"tokenUrl,omitempty"`
	RefreshURL       string `json:"refreshUrl,omitempty"`
	Scopes           map[string]string `json:"scopes"`
}

// SecurityRequirement defines a security scheme and scopes.
type SecurityRequirement map[string][]string

// Tag adds metadata to a single tag.
type Tag struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description,omitempty"`
	ExternalDocs *ExternalDocumentation `json:"externalDocs,omitempty"`
}

// ParseError represents an error during parsing.
type ParseError struct {
	Path    string
	Message string
	Cause   error
}

func (e *ParseError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("parse error at %s: %s", e.Path, e.Message)
	}
	return fmt.Sprintf("parse error: %s", e.Message)
}

func (e *ParseError) Unwrap() error {
	return e.Cause
}

// ValidationError represents a validation error in the spec.
type ValidationError struct {
	Path    string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("validation error at %s: %s", e.Path, e.Message)
	}
	return fmt.Sprintf("validation error: %s", e.Message)
}
