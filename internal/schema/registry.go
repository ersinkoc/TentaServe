package schema

import (
	"fmt"
	"sync"
)

// TypeRegistry provides thread-safe storage and lookup of types.
type TypeRegistry struct {
	types map[string]*TypeDef
	mu    sync.RWMutex
}

// NewTypeRegistry creates a new type registry.
func NewTypeRegistry() *TypeRegistry {
	return &TypeRegistry{
		types: make(map[string]*TypeDef),
	}
}

// NewTypeRegistryWithBuiltins creates a registry with built-in scalar types.
func NewTypeRegistryWithBuiltins() *TypeRegistry {
	r := NewTypeRegistry()
	for name, t := range BuiltinScalars() {
		r.types[name] = t
	}
	return r
}

// Register adds a type to the registry.
func (r *TypeRegistry) Register(t *TypeDef) error {
	if t == nil {
		return fmt.Errorf("cannot register nil type")
	}
	if t.Name == "" {
		return fmt.Errorf("cannot register type with empty name")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.types[t.Name] = t
	return nil
}

// RegisterAll registers multiple types.
func (r *TypeRegistry) RegisterAll(types []*TypeDef) error {
	for _, t := range types {
		if err := r.Register(t); err != nil {
			return err
		}
	}
	return nil
}

// Lookup retrieves a type by name.
func (r *TypeRegistry) Lookup(name string) (*TypeDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.types[name]
	return t, ok
}

// MustLookup retrieves a type by name, panicking if not found.
func (r *TypeRegistry) MustLookup(name string) *TypeDef {
	t, ok := r.Lookup(name)
	if !ok {
		panic(fmt.Sprintf("type %q not found in registry", name))
	}
	return t
}

// Remove removes a type from the registry.
func (r *TypeRegistry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.types, name)
}

// Has checks if a type exists.
func (r *TypeRegistry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.types[name]
	return ok
}

// Names returns all registered type names.
func (r *TypeRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.types))
	for name := range r.types {
		names = append(names, name)
	}
	return names
}

// Count returns the number of registered types.
func (r *TypeRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.types)
}

// Clear removes all types except built-in scalars.
func (r *TypeRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Keep only built-in scalars
	builtins := BuiltinScalars()
	r.types = make(map[string]*TypeDef, len(builtins))
	for name, t := range builtins {
		r.types[name] = t
	}
}

// All returns a copy of all registered types.
func (r *TypeRegistry) All() []*TypeDef {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]*TypeDef, 0, len(r.types))
	for _, t := range r.types {
		types = append(types, t)
	}
	return types
}

// Filter returns types matching a predicate.
func (r *TypeRegistry) Filter(predicate func(*TypeDef) bool) []*TypeDef {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*TypeDef
	for _, t := range r.types {
		if predicate(t) {
			result = append(result, t)
		}
	}
	return result
}

// Objects returns all object types.
func (r *TypeRegistry) Objects() []*TypeDef {
	return r.Filter(func(t *TypeDef) bool {
		return t.Kind == TypeKindObject || t.Kind == TypeKindInput
	})
}

// Enums returns all enum types.
func (r *TypeRegistry) Enums() []*TypeDef {
	return r.Filter(func(t *TypeDef) bool {
		return t.Kind == TypeKindEnum
	})
}

// Scalars returns all scalar types (including built-ins).
func (r *TypeRegistry) Scalars() []*TypeDef {
	return r.Filter(func(t *TypeDef) bool {
		return t.IsScalar()
	})
}

// Interfaces returns all interface types.
func (r *TypeRegistry) Interfaces() []*TypeDef {
	return r.Filter(func(t *TypeDef) bool {
		return t.Kind == TypeKindInterface
	})
}

// Unions returns all union types.
func (r *TypeRegistry) Unions() []*TypeDef {
	return r.Filter(func(t *TypeDef) bool {
		return t.Kind == TypeKindUnion
	})
}

// IsBuiltin returns true if the type is a built-in scalar.
func (r *TypeRegistry) IsBuiltin(name string) bool {
	builtins := BuiltinScalars()
	_, ok := builtins[name]
	return ok
}

// Merge merges another registry into this one.
// If overwrite is true, existing types are replaced.
func (r *TypeRegistry) Merge(other *TypeRegistry, overwrite bool) error {
	if other == nil {
		return nil
	}

	other.mu.RLock()
	defer other.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	for name, t := range other.types {
		if _, exists := r.types[name]; exists && !overwrite {
			continue
		}
		r.types[name] = t
	}

	return nil
}

// Clone creates a deep copy of the registry.
func (r *TypeRegistry) Clone() *TypeRegistry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	clone := NewTypeRegistry()
	for name, t := range r.types {
		// Note: TypeDef is not deeply copied here for performance
		// If mutation is needed, deep copy should be implemented
		clone.types[name] = t
	}
	return clone
}

// ResolveField resolves a field path like "User.posts.title".
func (r *TypeRegistry) ResolveField(typeName string, path []string) (*TypeDef, *FieldDef, error) {
	currentType, ok := r.Lookup(typeName)
	if !ok {
		return nil, nil, fmt.Errorf("type %q not found", typeName)
	}

	for i, fieldName := range path {
		var found bool
		var field *FieldDef
		for _, f := range currentType.Fields {
			if f.Name == fieldName {
				field = f
				found = true
				break
			}
		}
		if !found {
			return nil, nil, fmt.Errorf("field %q not found on type %q", fieldName, currentType.Name)
		}

		if i == len(path)-1 {
			return currentType, field, nil
		}

		// Move to next type
		nextTypeName := field.Type.Unwrap().Name
		nextType, ok := r.Lookup(nextTypeName)
		if !ok {
			return nil, nil, fmt.Errorf("type %q not found", nextTypeName)
		}
		currentType = nextType
	}

	return currentType, nil, nil
}
