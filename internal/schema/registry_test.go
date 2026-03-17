package schema

import (
	"testing"
)

func TestNewTypeRegistry(t *testing.T) {
	r := NewTypeRegistry()
	if r == nil {
		t.Fatal("NewTypeRegistry returned nil")
	}
	if r.Count() != 0 {
		t.Errorf("Expected empty registry, got %d types", r.Count())
	}
}

func TestNewTypeRegistryWithBuiltins(t *testing.T) {
	r := NewTypeRegistryWithBuiltins()
	if r == nil {
		t.Fatal("NewTypeRegistryWithBuiltins returned nil")
	}
	if r.Count() != 5 {
		t.Errorf("Expected 5 builtin types, got %d", r.Count())
	}

	// Check builtins exist
	for _, name := range []string{"String", "Int", "Float", "Boolean", "ID"} {
		if !r.Has(name) {
			t.Errorf("Expected builtin type %s", name)
		}
	}
}

func TestTypeRegistry_Register(t *testing.T) {
	r := NewTypeRegistry()

	// Register valid type
	user := &TypeDef{Name: "User", Kind: TypeKindObject}
	err := r.Register(user)
	if err != nil {
		t.Errorf("Register failed: %v", err)
	}

	// Register nil
	err = r.Register(nil)
	if err == nil {
		t.Error("Expected error for nil type")
	}

	// Register with empty name
	err = r.Register(&TypeDef{Kind: TypeKindObject})
	if err == nil {
		t.Error("Expected error for empty name")
	}
}

func TestTypeRegistry_RegisterAll(t *testing.T) {
	r := NewTypeRegistry()

	types := []*TypeDef{
		{Name: "User", Kind: TypeKindObject},
		{Name: "Post", Kind: TypeKindObject},
		{Name: "Comment", Kind: TypeKindObject},
	}

	err := r.RegisterAll(types)
	if err != nil {
		t.Errorf("RegisterAll failed: %v", err)
	}

	if r.Count() != 3 {
		t.Errorf("Expected 3 types, got %d", r.Count())
	}
}

func TestTypeRegistry_Lookup(t *testing.T) {
	r := NewTypeRegistry()
	r.Register(&TypeDef{Name: "User", Kind: TypeKindObject})

	// Existing type
	typ, ok := r.Lookup("User")
	if !ok {
		t.Error("Expected to find User")
	}
	if typ.Name != "User" {
		t.Errorf("Expected User, got %s", typ.Name)
	}

	// Non-existing type
	_, ok = r.Lookup("NonExistent")
	if ok {
		t.Error("Expected not to find NonExistent")
	}
}

func TestTypeRegistry_MustLookup(t *testing.T) {
	r := NewTypeRegistry()
	r.Register(&TypeDef{Name: "User", Kind: TypeKindObject})

	// Existing type
	typ := r.MustLookup("User")
	if typ.Name != "User" {
		t.Errorf("Expected User, got %s", typ.Name)
	}

	// Non-existing type should panic
	panicCalled := false
	func() {
		defer func() {
			if recover() != nil {
				panicCalled = true
			}
		}()
		r.MustLookup("NonExistent")
	}()

	if !panicCalled {
		t.Error("Expected panic for non-existing type")
	}
}

func TestTypeRegistry_Has(t *testing.T) {
	r := NewTypeRegistry()
	r.Register(&TypeDef{Name: "User", Kind: TypeKindObject})

	if !r.Has("User") {
		t.Error("Expected Has(User) to be true")
	}
	if r.Has("NonExistent") {
		t.Error("Expected Has(NonExistent) to be false")
	}
}

func TestTypeRegistry_Remove(t *testing.T) {
	r := NewTypeRegistry()
	r.Register(&TypeDef{Name: "User", Kind: TypeKindObject})

	r.Remove("User")
	if r.Has("User") {
		t.Error("Expected User to be removed")
	}

	// Removing non-existing should not panic
	r.Remove("NonExistent")
}

func TestTypeRegistry_Names(t *testing.T) {
	r := NewTypeRegistry()
	r.Register(&TypeDef{Name: "User", Kind: TypeKindObject})
	r.Register(&TypeDef{Name: "Post", Kind: TypeKindObject})

	names := r.Names()
	if len(names) != 2 {
		t.Errorf("Expected 2 names, got %d", len(names))
	}

	// Check both names exist
	hasUser, hasPost := false, false
	for _, name := range names {
		if name == "User" {
			hasUser = true
		}
		if name == "Post" {
			hasPost = true
		}
	}
	if !hasUser || !hasPost {
		t.Error("Expected both User and Post in names")
	}
}

func TestTypeRegistry_Clear(t *testing.T) {
	r := NewTypeRegistryWithBuiltins()
	r.Register(&TypeDef{Name: "User", Kind: TypeKindObject})

	r.Clear()

	if r.Count() != 5 {
		t.Errorf("Expected 5 builtins after clear, got %d", r.Count())
	}
	if r.Has("User") {
		t.Error("Expected User to be cleared")
	}
}

func TestTypeRegistry_All(t *testing.T) {
	r := NewTypeRegistry()
	r.Register(&TypeDef{Name: "User", Kind: TypeKindObject})
	r.Register(&TypeDef{Name: "Post", Kind: TypeKindObject})

	all := r.All()
	if len(all) != 2 {
		t.Errorf("Expected 2 types, got %d", len(all))
	}
}

func TestTypeRegistry_Filter(t *testing.T) {
	r := NewTypeRegistry()
	r.Register(&TypeDef{Name: "User", Kind: TypeKindObject})
	r.Register(&TypeDef{Name: "Post", Kind: TypeKindObject})
	r.Register(&TypeDef{Name: "Status", Kind: TypeKindEnum})
	r.Register(&TypeDef{Name: "DateTime", Kind: TypeKindScalar})

	// Filter objects
	objects := r.Filter(func(t *TypeDef) bool {
		return t.Kind == TypeKindObject
	})
	if len(objects) != 2 {
		t.Errorf("Expected 2 objects, got %d", len(objects))
	}

	// Filter enums
	enums := r.Filter(func(t *TypeDef) bool {
		return t.Kind == TypeKindEnum
	})
	if len(enums) != 1 {
		t.Errorf("Expected 1 enum, got %d", len(enums))
	}
}

func TestTypeRegistry_Objects(t *testing.T) {
	r := NewTypeRegistry()
	r.Register(&TypeDef{Name: "User", Kind: TypeKindObject})
	r.Register(&TypeDef{Name: "Post", Kind: TypeKindInput})
	r.Register(&TypeDef{Name: "Status", Kind: TypeKindEnum})

	objects := r.Objects()
	if len(objects) != 2 {
		t.Errorf("Expected 2 objects (Object + Input), got %d", len(objects))
	}
}

func TestTypeRegistry_Enums(t *testing.T) {
	r := NewTypeRegistry()
	r.Register(&TypeDef{Name: "User", Kind: TypeKindObject})
	r.Register(&TypeDef{Name: "Status", Kind: TypeKindEnum})

	enums := r.Enums()
	if len(enums) != 1 {
		t.Errorf("Expected 1 enum, got %d", len(enums))
	}
}

func TestTypeRegistry_Scalars(t *testing.T) {
	r := NewTypeRegistryWithBuiltins()
	r.Register(&TypeDef{Name: "DateTime", Kind: TypeKindScalar})

	scalars := r.Scalars()
	if len(scalars) != 6 { // 5 builtins + DateTime
		t.Errorf("Expected 6 scalars, got %d", len(scalars))
	}
}

func TestTypeRegistry_IsBuiltin(t *testing.T) {
	r := NewTypeRegistry()
	if !r.IsBuiltin("String") {
		t.Error("Expected String to be builtin")
	}
	if r.IsBuiltin("CustomType") {
		t.Error("Expected CustomType not to be builtin")
	}
}

func TestTypeRegistry_Merge(t *testing.T) {
	r1 := NewTypeRegistry()
	r1.Register(&TypeDef{Name: "User", Kind: TypeKindObject})

	r2 := NewTypeRegistry()
	r2.Register(&TypeDef{Name: "Post", Kind: TypeKindObject})
	r2.Register(&TypeDef{Name: "User", Kind: TypeKindEnum}) // Different User

	// Merge without overwrite
	err := r1.Merge(r2, false)
	if err != nil {
		t.Errorf("Merge failed: %v", err)
	}

	if r1.Count() != 2 {
		t.Errorf("Expected 2 types, got %d", r1.Count())
	}

	// User should still be Object (not overwritten)
	user, _ := r1.Lookup("User")
	if user.Kind != TypeKindObject {
		t.Error("Expected User to remain Object")
	}

	// Merge with overwrite
	err = r1.Merge(r2, true)
	if err != nil {
		t.Errorf("Merge failed: %v", err)
	}

	user, _ = r1.Lookup("User")
	if user.Kind != TypeKindEnum {
		t.Error("Expected User to be overwritten to Enum")
	}
}

func TestTypeRegistry_Clone(t *testing.T) {
	r := NewTypeRegistry()
	r.Register(&TypeDef{Name: "User", Kind: TypeKindObject})

	clone := r.Clone()

	if clone.Count() != 1 {
		t.Errorf("Expected 1 type in clone, got %d", clone.Count())
	}

	// Modify original
	r.Register(&TypeDef{Name: "Post", Kind: TypeKindObject})

	// Clone should be unaffected
	if clone.Count() != 1 {
		t.Error("Clone should be independent")
	}
}

func TestTypeRegistry_ResolveField(t *testing.T) {
	r := NewTypeRegistry()

	// Create types
	user := &TypeDef{
		Name: "User",
		Kind: TypeKindObject,
		Fields: []*FieldDef{
			{
				Name: "id",
				Type: IDType(),
			},
			{
				Name: "posts",
				Type: ListType(NamedType("Post")),
			},
		},
	}

	post := &TypeDef{
		Name: "Post",
		Kind: TypeKindObject,
		Fields: []*FieldDef{
			{Name: "id", Type: IDType()},
			{Name: "title", Type: StringType()},
		},
	}

	r.Register(user)
	r.Register(post)

	// Resolve simple field
	_, field, err := r.ResolveField("User", []string{"id"})
	if err != nil {
		t.Errorf("ResolveField failed: %v", err)
	}
	if field.Name != "id" {
		t.Errorf("Expected id field, got %s", field.Name)
	}

	// Resolve nested field
	_, field, err = r.ResolveField("User", []string{"posts", "title"})
	if err != nil {
		t.Errorf("ResolveField failed: %v", err)
	}
	if field.Name != "title" {
		t.Errorf("Expected title field, got %s", field.Name)
	}

	// Non-existent type
	_, _, err = r.ResolveField("NonExistent", []string{"id"})
	if err == nil {
		t.Error("Expected error for non-existent type")
	}

	// Non-existent field
	_, _, err = r.ResolveField("User", []string{"nonexistent"})
	if err == nil {
		t.Error("Expected error for non-existent field")
	}
}
