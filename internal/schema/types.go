// Package schema provides the unified type system for Tentaserve.
//
// The schema package defines types that can represent both OpenAPI and GraphQL
// constructs, enabling bi-directional translation between REST and GraphQL.
package schema

import (
	"fmt"
	"sync"
)

// TypeKind represents the kind of a type.
type TypeKind int

const (
	// Scalar types
	TypeKindString TypeKind = iota
	TypeKindInt
	TypeKindFloat
	TypeKindBool
	TypeKindID

	// Complex types
	TypeKindObject
	TypeKindInput
	TypeKindInterface
	TypeKindUnion
	TypeKindEnum
	TypeKindList
	TypeKindNonNull

	// Special types
	TypeKindScalar // Custom scalar
)

func (k TypeKind) String() string {
	switch k {
	case TypeKindString:
		return "String"
	case TypeKindInt:
		return "Int"
	case TypeKindFloat:
		return "Float"
	case TypeKindBool:
		return "Boolean"
	case TypeKindID:
		return "ID"
	case TypeKindObject:
		return "Object"
	case TypeKindInput:
		return "Input"
	case TypeKindInterface:
		return "Interface"
	case TypeKindUnion:
		return "Union"
	case TypeKindEnum:
		return "Enum"
	case TypeKindList:
		return "List"
	case TypeKindNonNull:
		return "NonNull"
	case TypeKindScalar:
		return "Scalar"
	default:
		return fmt.Sprintf("TypeKind(%d)", k)
	}
}

// TypeRef is a reference to a type.
type TypeRef struct {
	Kind     TypeKind
	Name     string   // For named types (Object, Enum, Scalar, etc.)
	OfType   *TypeRef // For List and NonNull
	EnumVals []string // For Enum types
}

// IsScalar returns true if the type is a scalar.
func (t *TypeRef) IsScalar() bool {
	switch t.Kind {
	case TypeKindString, TypeKindInt, TypeKindFloat, TypeKindBool, TypeKindID, TypeKindScalar:
		return true
	}
	return false
}

// IsLeaf returns true if the type is a leaf type (scalar or enum).
func (t *TypeRef) IsLeaf() bool {
	return t.IsScalar() || t.Kind == TypeKindEnum
}

// IsList returns true if the type is a list.
func (t *TypeRef) IsList() bool {
	return t.Kind == TypeKindList
}

// IsNonNull returns true if the type is non-null.
func (t *TypeRef) IsNonNull() bool {
	return t.Kind == TypeKindNonNull
}

// Unwrap returns the inner type for List and NonNull.
func (t *TypeRef) Unwrap() *TypeRef {
	if t.OfType != nil {
		return t.OfType
	}
	return t
}

// String returns a string representation of the type.
func (t *TypeRef) String() string {
	switch t.Kind {
	case TypeKindList:
		if t.OfType != nil {
			return "[" + t.OfType.String() + "]"
		}
		return "[]"
	case TypeKindNonNull:
		if t.OfType != nil {
			return t.OfType.String() + "!"
		}
		return "!"
	default:
		return t.Name
	}
}

// FieldDef defines a field in an object or interface type.
type FieldDef struct {
	Name        string
	Description string
	Type        *TypeRef
	Arguments   []*ArgumentDef
	Deprecated  bool
	DepReason   string
}

// ArgumentDef defines an argument to a field.
type ArgumentDef struct {
	Name         string
	Description  string
	Type         *TypeRef
	DefaultValue interface{}
	Required     bool
}

// TypeDef defines a named type in the schema.
type TypeDef struct {
	Name        string
	Description string
	Kind        TypeKind
	Fields      []*FieldDef
	EnumValues  []EnumValueDef
	Interfaces  []string // For object types
	Possible    []string // For union types
}

// IsScalar returns true if the type is a scalar.
func (t *TypeDef) IsScalar() bool {
	switch t.Kind {
	case TypeKindString, TypeKindInt, TypeKindFloat, TypeKindBool, TypeKindID, TypeKindScalar:
		return true
	}
	return false
}

// EnumValueDef defines a value in an enum.
type EnumValueDef struct {
	Name        string
	Description string
	Deprecated  bool
	DepReason   string
}

// OperationDef defines a query, mutation, or subscription.
type OperationDef struct {
	Name       string
	Type       string // "query", "mutation", "subscription"
	Fields     []*FieldDef
	Deprecated bool
}

// SchemaDefinition holds the complete schema.
type SchemaDefinition struct {
	Types        map[string]*TypeDef
	Query        *OperationDef
	Mutation     *OperationDef
	Subscription *OperationDef
	Directives   []*DirectiveDef
	mu           sync.RWMutex
}

// DirectiveDef defines a directive.
type DirectiveDef struct {
	Name        string
	Description string
	Locations   []string
	Arguments   []*ArgumentDef
}

// NewSchemaDefinition creates a new empty schema.
func NewSchemaDefinition() *SchemaDefinition {
	return &SchemaDefinition{
		Types: make(map[string]*TypeDef),
	}
}

// AddType adds a type to the schema.
func (s *SchemaDefinition) AddType(t *TypeDef) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Types[t.Name] = t
}

// GetType retrieves a type by name.
func (s *SchemaDefinition) GetType(name string) (*TypeDef, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.Types[name]
	return t, ok
}

// HasType checks if a type exists.
func (s *SchemaDefinition) HasType(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.Types[name]
	return ok
}

// RemoveType removes a type from the schema.
func (s *SchemaDefinition) RemoveType(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Types, name)
}

// AllTypes returns all type names.
func (s *SchemaDefinition) AllTypes() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.Types))
	for name := range s.Types {
		names = append(names, name)
	}
	return names
}

// BuiltinScalars returns the built-in scalar types.
func BuiltinScalars() map[string]*TypeDef {
	return map[string]*TypeDef{
		"String": {
			Name: "String",
			Kind: TypeKindString,
		},
		"Int": {
			Name: "Int",
			Kind: TypeKindInt,
		},
		"Float": {
			Name: "Float",
			Kind: TypeKindFloat,
		},
		"Boolean": {
			Name: "Boolean",
			Kind: TypeKindBool,
		},
		"ID": {
			Name: "ID",
			Kind: TypeKindID,
		},
	}
}

// ScalarType creates a scalar type reference.
func ScalarType(name string) *TypeRef {
	return &TypeRef{Kind: TypeKindScalar, Name: name}
}

// StringType creates a String type reference.
func StringType() *TypeRef {
	return &TypeRef{Kind: TypeKindString, Name: "String"}
}

// IntType creates an Int type reference.
func IntType() *TypeRef {
	return &TypeRef{Kind: TypeKindInt, Name: "Int"}
}

// FloatType creates a Float type reference.
func FloatType() *TypeRef {
	return &TypeRef{Kind: TypeKindFloat, Name: "Float"}
}

// BoolType creates a Boolean type reference.
func BoolType() *TypeRef {
	return &TypeRef{Kind: TypeKindBool, Name: "Boolean"}
}

// IDType creates an ID type reference.
func IDType() *TypeRef {
	return &TypeRef{Kind: TypeKindID, Name: "ID"}
}

// ListType creates a list type reference.
func ListType(of *TypeRef) *TypeRef {
	return &TypeRef{Kind: TypeKindList, OfType: of}
}

// NonNullType creates a non-null type reference.
func NonNullType(of *TypeRef) *TypeRef {
	return &TypeRef{Kind: TypeKindNonNull, OfType: of}
}

// NamedType creates a named type reference.
func NamedType(name string) *TypeRef {
	return &TypeRef{Kind: TypeKindObject, Name: name}
}

// EnumType creates an enum type reference.
func EnumType(name string, values []string) *TypeRef {
	return &TypeRef{Kind: TypeKindEnum, Name: name, EnumVals: values}
}

// DeepCopy creates a deep copy of the type reference.
func (t *TypeRef) DeepCopy() *TypeRef {
	if t == nil {
		return nil
	}

	copy := &TypeRef{
		Kind:     t.Kind,
		Name:     t.Name,
		EnumVals: make([]string, len(t.EnumVals)),
	}
	copy.EnumVals = append([]string(nil), t.EnumVals...)

	if t.OfType != nil {
		copy.OfType = t.OfType.DeepCopy()
	}

	return copy
}
