package schema

import (
	"fmt"
	"sync"
	"testing"
)

func TestTypeKind_String(t *testing.T) {
	tests := []struct {
		kind TypeKind
		want string
	}{
		{TypeKindString, "String"},
		{TypeKindInt, "Int"},
		{TypeKindFloat, "Float"},
		{TypeKindBool, "Boolean"},
		{TypeKindID, "ID"},
		{TypeKindObject, "Object"},
		{TypeKindList, "List"},
		{TypeKindNonNull, "NonNull"},
		{TypeKind(999), "TypeKind(999)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.kind.String()
			if got != tt.want {
				t.Errorf("TypeKind.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTypeRef_IsScalar(t *testing.T) {
	tests := []struct {
		name string
		tr   *TypeRef
		want bool
	}{
		{"String", StringType(), true},
		{"Int", IntType(), true},
		{"Float", FloatType(), true},
		{"Boolean", BoolType(), true},
		{"ID", IDType(), true},
		{"Object", NamedType("User"), false},
		{"List", ListType(StringType()), false},
		{"NonNull", NonNullType(StringType()), false},
		{"Custom Scalar", ScalarType("DateTime"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.tr.IsScalar()
			if got != tt.want {
				t.Errorf("IsScalar() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTypeRef_String(t *testing.T) {
	tests := []struct {
		name string
		tr   *TypeRef
		want string
	}{
		{"String", StringType(), "String"},
		{"List of String", ListType(StringType()), "[String]"},
		{"NonNull String", NonNullType(StringType()), "String!"},
		{"List of NonNull String", ListType(NonNullType(StringType())), "[String!]"},
		{"Named", NamedType("User"), "User"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.tr.String()
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTypeRef_Unwrap(t *testing.T) {
	// Unwrap List
	list := ListType(StringType())
	unwrapped := list.Unwrap()
	if unwrapped.Kind != TypeKindString {
		t.Errorf("Expected String, got %s", unwrapped.Kind)
	}

	// Unwrap NonNull
	nonNull := NonNullType(NamedType("User"))
	unwrapped = nonNull.Unwrap()
	if unwrapped.Name != "User" {
		t.Errorf("Expected User, got %s", unwrapped.Name)
	}

	// Unwrap normal type returns itself
	s := StringType()
	if s.Unwrap() != s {
		t.Error("Unwrap of scalar should return itself")
	}
}

func TestTypeRef_DeepCopy(t *testing.T) {
	original := ListType(NonNullType(NamedType("User")))
	copy := original.DeepCopy()

	// Modify original
	original.OfType.OfType.Name = "Changed"

	// Copy should be unchanged
	if copy.OfType.OfType.Name != "User" {
		t.Error("DeepCopy did not create independent copy")
	}
}

func TestSchemaDefinition(t *testing.T) {
	schema := NewSchemaDefinition()

	// Add type
	userType := &TypeDef{
		Name: "User",
		Kind: TypeKindObject,
		Fields: []*FieldDef{
			{Name: "id", Type: IDType()},
			{Name: "name", Type: StringType()},
		},
	}
	schema.AddType(userType)

	// Get type
	got, ok := schema.GetType("User")
	if !ok {
		t.Fatal("Expected to find User type")
	}
	if got.Name != "User" {
		t.Errorf("Expected User, got %s", got.Name)
	}

	// Has type
	if !schema.HasType("User") {
		t.Error("Expected HasType(User) to be true")
	}
	if schema.HasType("NonExistent") {
		t.Error("Expected HasType(NonExistent) to be false")
	}

	// All types
	types := schema.AllTypes()
	if len(types) != 1 {
		t.Errorf("Expected 1 type, got %d", len(types))
	}

	// Remove type
	schema.RemoveType("User")
	if schema.HasType("User") {
		t.Error("Expected User to be removed")
	}
}

func TestSchemaDefinition_Concurrent(t *testing.T) {
	schema := NewSchemaDefinition()

	// Concurrent writes
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			schema.AddType(&TypeDef{
				Name: fmt.Sprintf("Type%d", n),
				Kind: TypeKindObject,
			})
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = schema.AllTypes()
		}()
	}

	wg.Wait()

	if len(schema.AllTypes()) != 100 {
		t.Errorf("Expected 100 types, got %d", len(schema.AllTypes()))
	}
}

func TestBuiltinScalars(t *testing.T) {
	scalars := BuiltinScalars()

	expected := []string{"String", "Int", "Float", "Boolean", "ID"}
	for _, name := range expected {
		if _, ok := scalars[name]; !ok {
			t.Errorf("Expected builtin scalar %s", name)
		}
	}
}

func TestTypeDef_IsScalar(t *testing.T) {
	tests := []struct {
		name string
		kind TypeKind
		want bool
	}{
		{"String", TypeKindString, true},
		{"Int", TypeKindInt, true},
		{"Float", TypeKindFloat, true},
		{"Boolean", TypeKindBool, true},
		{"ID", TypeKindID, true},
		{"Object", TypeKindObject, false},
		{"Enum", TypeKindEnum, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := &TypeDef{Name: tt.name, Kind: tt.kind}
			got := td.IsScalar()
			if got != tt.want {
				t.Errorf("IsScalar() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTypeRef_IsLeaf(t *testing.T) {
	tests := []struct {
		name string
		tr   *TypeRef
		want bool
	}{
		{"String", StringType(), true},
		{"Int", IntType(), true},
		{"Enum", EnumType("Status", []string{"ACTIVE", "INACTIVE"}), true},
		{"Object", NamedType("User"), false},
		{"List", ListType(StringType()), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.tr.IsLeaf()
			if got != tt.want {
				t.Errorf("IsLeaf() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Benchmark type lookup
func BenchmarkSchemaDefinition_GetType(b *testing.B) {
	schema := NewSchemaDefinition()
	for i := 0; i < 1000; i++ {
		schema.AddType(&TypeDef{
			Name: fmt.Sprintf("Type%d", i),
			Kind: TypeKindObject,
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = schema.GetType("Type500")
	}
}
