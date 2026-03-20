package graphql

import (
	"strings"
	"testing"

	"github.com/ersinkoc/tentaserve/internal/schema"
)

func TestNewSDLPrinter(t *testing.T) {
	p := NewSDLPrinter()
	if p == nil {
		t.Fatal("expected non-nil printer")
	}
	if p.indent != "  " {
		t.Errorf("expected indent '  ', got %q", p.indent)
	}
}

func TestSDLPrinter_PrintNil(t *testing.T) {
	p := NewSDLPrinter()
	result := p.Print(nil)
	if result != "" {
		t.Errorf("expected empty string for nil schema, got %q", result)
	}
}

func TestSDLPrinter_PrintEmptySchema(t *testing.T) {
	p := NewSDLPrinter()
	s := schema.NewSchemaDefinition()
	result := p.Print(s)
	if result != "" {
		t.Errorf("expected empty string for empty schema, got %q", result)
	}
}

func TestSDLPrinter_PrintSchemaDefinition(t *testing.T) {
	p := NewSDLPrinter()
	s := schema.NewSchemaDefinition()
	s.Query = &schema.OperationDef{Name: "Query"}
	s.Mutation = &schema.OperationDef{Name: "Mutation"}
	s.Subscription = &schema.OperationDef{Name: "Subscription"}

	result := p.Print(s)
	if !strings.Contains(result, "schema {") {
		t.Errorf("expected schema block, got %q", result)
	}
	if !strings.Contains(result, "query: Query") {
		t.Errorf("expected query type, got %q", result)
	}
	if !strings.Contains(result, "mutation: Mutation") {
		t.Errorf("expected mutation type, got %q", result)
	}
	if !strings.Contains(result, "subscription: Subscription") {
		t.Errorf("expected subscription type, got %q", result)
	}
}

func TestSDLPrinter_PrintScalar(t *testing.T) {
	p := NewSDLPrinter()
	s := schema.NewSchemaDefinition()
	s.AddType(&schema.TypeDef{
		Name:        "DateTime",
		Kind:        schema.TypeKindScalar,
		Description: "A date-time scalar",
	})

	result := p.Print(s)
	if !strings.Contains(result, "scalar DateTime") {
		t.Errorf("expected scalar declaration, got %q", result)
	}
	if !strings.Contains(result, "A date-time scalar") {
		t.Errorf("expected description, got %q", result)
	}
}

func TestSDLPrinter_PrintScalar_NoDescription(t *testing.T) {
	p := NewSDLPrinter()
	s := schema.NewSchemaDefinition()
	s.AddType(&schema.TypeDef{
		Name: "JSON",
		Kind: schema.TypeKindScalar,
	})

	result := p.Print(s)
	if !strings.Contains(result, "scalar JSON") {
		t.Errorf("expected scalar JSON, got %q", result)
	}
}

func TestSDLPrinter_PrintEnum(t *testing.T) {
	p := NewSDLPrinter()
	s := schema.NewSchemaDefinition()
	s.AddType(&schema.TypeDef{
		Name:        "Status",
		Kind:        schema.TypeKindEnum,
		Description: "Status enum",
		EnumValues: []schema.EnumValueDef{
			{Name: "ACTIVE"},
			{Name: "INACTIVE", Deprecated: true, DepReason: "Use DISABLED instead"},
			{Name: "DISABLED"},
		},
	})

	result := p.Print(s)
	if !strings.Contains(result, "enum Status") {
		t.Errorf("expected enum declaration, got %q", result)
	}
	if !strings.Contains(result, "ACTIVE") {
		t.Errorf("expected ACTIVE value, got %q", result)
	}
	if !strings.Contains(result, "@deprecated") {
		t.Errorf("expected @deprecated directive, got %q", result)
	}
	if !strings.Contains(result, "Use DISABLED instead") {
		t.Errorf("expected deprecation reason, got %q", result)
	}
}

func TestSDLPrinter_PrintEnum_DeprecatedNoReason(t *testing.T) {
	p := NewSDLPrinter()
	s := schema.NewSchemaDefinition()
	s.AddType(&schema.TypeDef{
		Name: "Color",
		Kind: schema.TypeKindEnum,
		EnumValues: []schema.EnumValueDef{
			{Name: "RED"},
			{Name: "BLUE", Deprecated: true},
		},
	})

	result := p.Print(s)
	if !strings.Contains(result, "BLUE @deprecated") {
		t.Errorf("expected deprecated without reason, got %q", result)
	}
}

func TestSDLPrinter_PrintEnum_WithDescription(t *testing.T) {
	p := NewSDLPrinter()
	s := schema.NewSchemaDefinition()
	s.AddType(&schema.TypeDef{
		Name: "Direction",
		Kind: schema.TypeKindEnum,
		EnumValues: []schema.EnumValueDef{
			{Name: "NORTH", Description: "Points north"},
		},
	})

	result := p.Print(s)
	if !strings.Contains(result, "NORTH") {
		t.Errorf("expected NORTH, got %q", result)
	}
}

func TestSDLPrinter_PrintObject(t *testing.T) {
	p := NewSDLPrinter()
	s := schema.NewSchemaDefinition()
	s.AddType(&schema.TypeDef{
		Name:        "User",
		Kind:        schema.TypeKindObject,
		Description: "A user type",
		Interfaces:  []string{"Node"},
		Fields: []*schema.FieldDef{
			{
				Name: "id",
				Type: &schema.TypeRef{Kind: schema.TypeKindNonNull, OfType: &schema.TypeRef{Kind: schema.TypeKindID, Name: "ID"}},
			},
			{
				Name:        "name",
				Type:        &schema.TypeRef{Kind: schema.TypeKindString, Name: "String"},
				Description: "The user name",
			},
			{
				Name:       "oldField",
				Type:       &schema.TypeRef{Kind: schema.TypeKindString, Name: "String"},
				Deprecated: true,
				DepReason:  "Use newField instead",
			},
			{
				Name:       "anotherOld",
				Type:       &schema.TypeRef{Kind: schema.TypeKindString, Name: "String"},
				Deprecated: true,
			},
			{
				Name: "posts",
				Type: &schema.TypeRef{Kind: schema.TypeKindList, OfType: &schema.TypeRef{Kind: schema.TypeKindObject, Name: "Post"}},
				Arguments: []*schema.ArgumentDef{
					{Name: "first", Type: &schema.TypeRef{Kind: schema.TypeKindInt, Name: "Int"}},
					{Name: "offset", Type: &schema.TypeRef{Kind: schema.TypeKindInt, Name: "Int"}, DefaultValue: 0, Required: false},
				},
			},
		},
	})

	result := p.Print(s)
	if !strings.Contains(result, "type User implements Node") {
		t.Errorf("expected type with interface, got %q", result)
	}
	if !strings.Contains(result, "id: ID!") {
		t.Errorf("expected id field, got %q", result)
	}
	if !strings.Contains(result, "name: String") {
		t.Errorf("expected name field, got %q", result)
	}
	if !strings.Contains(result, `@deprecated(reason: "Use newField instead")`) {
		t.Errorf("expected deprecated with reason, got %q", result)
	}
	if !strings.Contains(result, "posts(first: Int, offset: Int = 0): [Post]") {
		t.Errorf("expected posts field with args, got %q", result)
	}
}

func TestSDLPrinter_PrintInput(t *testing.T) {
	p := NewSDLPrinter()
	s := schema.NewSchemaDefinition()
	s.AddType(&schema.TypeDef{
		Name:        "CreateUserInput",
		Kind:        schema.TypeKindInput,
		Description: "Input for creating a user",
		Fields: []*schema.FieldDef{
			{Name: "name", Type: &schema.TypeRef{Kind: schema.TypeKindNonNull, OfType: &schema.TypeRef{Kind: schema.TypeKindString, Name: "String"}}},
			{Name: "email", Type: &schema.TypeRef{Kind: schema.TypeKindString, Name: "String"}},
			{Name: "bio", Type: &schema.TypeRef{Kind: schema.TypeKindString, Name: "String"}, Description: "User bio"},
		},
	})

	result := p.Print(s)
	if !strings.Contains(result, "input CreateUserInput") {
		t.Errorf("expected input declaration, got %q", result)
	}
	if !strings.Contains(result, "name: String!") {
		t.Errorf("expected name field, got %q", result)
	}
}

func TestSDLPrinter_PrintInterface(t *testing.T) {
	p := NewSDLPrinter()
	s := schema.NewSchemaDefinition()
	s.AddType(&schema.TypeDef{
		Name:        "Node",
		Kind:        schema.TypeKindInterface,
		Description: "Node interface",
		Fields: []*schema.FieldDef{
			{Name: "id", Type: &schema.TypeRef{Kind: schema.TypeKindNonNull, OfType: &schema.TypeRef{Kind: schema.TypeKindID, Name: "ID"}}},
		},
	})

	result := p.Print(s)
	if !strings.Contains(result, "interface Node") {
		t.Errorf("expected interface declaration, got %q", result)
	}
}

func TestSDLPrinter_PrintUnion(t *testing.T) {
	p := NewSDLPrinter()
	s := schema.NewSchemaDefinition()
	s.AddType(&schema.TypeDef{
		Name:        "SearchResult",
		Kind:        schema.TypeKindUnion,
		Description: "Search result union",
		Possible:    []string{"User", "Post", "Comment"},
	})

	result := p.Print(s)
	if !strings.Contains(result, "union SearchResult = User | Post | Comment") {
		t.Errorf("expected union declaration, got %q", result)
	}
}

func TestSDLPrinter_PrintDirective(t *testing.T) {
	p := NewSDLPrinter()
	s := schema.NewSchemaDefinition()
	s.Directives = []*schema.DirectiveDef{
		{
			Name:        "cacheControl",
			Description: "Cache control directive",
			Locations:   []string{"FIELD_DEFINITION", "OBJECT"},
			Arguments: []*schema.ArgumentDef{
				{Name: "maxAge", Type: &schema.TypeRef{Kind: schema.TypeKindInt, Name: "Int"}, Required: true},
			},
		},
	}

	result := p.Print(s)
	if !strings.Contains(result, "directive @cacheControl") {
		t.Errorf("expected directive declaration, got %q", result)
	}
	if !strings.Contains(result, "on FIELD_DEFINITION | OBJECT") {
		t.Errorf("expected directive locations, got %q", result)
	}
}

func TestSDLPrinter_PrintDirective_NoArgs(t *testing.T) {
	p := NewSDLPrinter()
	s := schema.NewSchemaDefinition()
	s.Directives = []*schema.DirectiveDef{
		{
			Name:      "deprecated",
			Locations: []string{"FIELD_DEFINITION"},
		},
	}

	result := p.Print(s)
	if !strings.Contains(result, "directive @deprecated on FIELD_DEFINITION") {
		t.Errorf("expected directive without args, got %q", result)
	}
}

func TestSDLPrinter_PrintDescription_MultiLine(t *testing.T) {
	p := NewSDLPrinter()
	desc := p.printDescription("line1\nline2", false)
	if !strings.Contains(desc, `"""`) {
		t.Errorf("expected block string for multiline description, got %q", desc)
	}
}

func TestSDLPrinter_PrintDescription_WithQuotes(t *testing.T) {
	p := NewSDLPrinter()
	desc := p.printDescription(`hello "world"`, false)
	if !strings.Contains(desc, `"""`) {
		t.Errorf("expected block string for description with quotes, got %q", desc)
	}
}

func TestSDLPrinter_PrintDescription_Inline(t *testing.T) {
	p := NewSDLPrinter()
	desc := p.printDescription("simple description", true)
	if !strings.HasPrefix(desc, "# ") {
		t.Errorf("expected inline comment, got %q", desc)
	}
}

func TestSDLPrinter_PrintDescription_NotInline(t *testing.T) {
	p := NewSDLPrinter()
	desc := p.printDescription("simple", false)
	if desc != `"simple"` {
		t.Errorf("expected quoted string, got %q", desc)
	}
}

func TestSDLPrinter_PrintTypeRef_Nil(t *testing.T) {
	p := NewSDLPrinter()
	result := p.printTypeRef(nil)
	if result != "" {
		t.Errorf("expected empty string for nil type ref, got %q", result)
	}
}

func TestIsBuiltinScalar(t *testing.T) {
	builtins := []string{"String", "Int", "Float", "Boolean", "ID"}
	for _, name := range builtins {
		if !isBuiltinScalar(name) {
			t.Errorf("expected %q to be builtin scalar", name)
		}
	}

	nonBuiltins := []string{"DateTime", "JSON", "User", ""}
	for _, name := range nonBuiltins {
		if isBuiltinScalar(name) {
			t.Errorf("expected %q to NOT be builtin scalar", name)
		}
	}
}

func TestHasDefaultValue(t *testing.T) {
	f := &schema.FieldDef{Name: "test"}
	if hasDefaultValue(f) {
		t.Error("expected false for hasDefaultValue")
	}
}

func TestFormatDefaultValue(t *testing.T) {
	tests := []struct {
		kind     schema.TypeKind
		expected string
	}{
		{schema.TypeKindString, `""`},
		{schema.TypeKindInt, "0"},
		{schema.TypeKindFloat, "0"},
		{schema.TypeKindBool, "false"},
		{schema.TypeKindObject, "null"},
	}

	for _, tt := range tests {
		result := formatDefaultValue(&schema.TypeRef{Kind: tt.kind})
		if result != tt.expected {
			t.Errorf("formatDefaultValue(%v) = %q, want %q", tt.kind, result, tt.expected)
		}
	}
}

func TestPrintSDL_ConvenienceFunction(t *testing.T) {
	s := schema.NewSchemaDefinition()
	s.AddType(&schema.TypeDef{
		Name: "User",
		Kind: schema.TypeKindObject,
		Fields: []*schema.FieldDef{
			{Name: "id", Type: &schema.TypeRef{Kind: schema.TypeKindID, Name: "ID"}},
		},
	})

	result := PrintSDL(s)
	if !strings.Contains(result, "type User") {
		t.Errorf("expected type User, got %q", result)
	}
}

func TestSDLPrinter_PrintAllTypesOrdered(t *testing.T) {
	p := NewSDLPrinter()
	s := schema.NewSchemaDefinition()

	// Add different type kinds to verify ordering
	s.AddType(&schema.TypeDef{Name: "MyScalar", Kind: schema.TypeKindScalar})
	s.AddType(&schema.TypeDef{Name: "MyEnum", Kind: schema.TypeKindEnum, EnumValues: []schema.EnumValueDef{{Name: "A"}}})
	s.AddType(&schema.TypeDef{
		Name: "MyObject",
		Kind: schema.TypeKindObject,
		Fields: []*schema.FieldDef{
			{Name: "id", Type: &schema.TypeRef{Kind: schema.TypeKindID, Name: "ID"}},
		},
	})
	s.AddType(&schema.TypeDef{
		Name: "MyInput",
		Kind: schema.TypeKindInput,
		Fields: []*schema.FieldDef{
			{Name: "name", Type: &schema.TypeRef{Kind: schema.TypeKindString, Name: "String"}},
		},
	})
	s.AddType(&schema.TypeDef{
		Name: "MyInterface",
		Kind: schema.TypeKindInterface,
		Fields: []*schema.FieldDef{
			{Name: "id", Type: &schema.TypeRef{Kind: schema.TypeKindID, Name: "ID"}},
		},
	})
	s.AddType(&schema.TypeDef{
		Name:     "MyUnion",
		Kind:     schema.TypeKindUnion,
		Possible: []string{"A", "B"},
	})

	result := p.Print(s)

	// Verify all types appear
	if !strings.Contains(result, "scalar MyScalar") {
		t.Errorf("missing scalar, got %q", result)
	}
	if !strings.Contains(result, "enum MyEnum") {
		t.Errorf("missing enum, got %q", result)
	}
	if !strings.Contains(result, "type MyObject") {
		t.Errorf("missing object, got %q", result)
	}
	if !strings.Contains(result, "input MyInput") {
		t.Errorf("missing input, got %q", result)
	}
	if !strings.Contains(result, "interface MyInterface") {
		t.Errorf("missing interface, got %q", result)
	}
	if !strings.Contains(result, "union MyUnion") {
		t.Errorf("missing union, got %q", result)
	}
}

func TestSDLPrinter_BuiltinScalarsExcluded(t *testing.T) {
	p := NewSDLPrinter()
	s := schema.NewSchemaDefinition()

	// Add built-in scalars - they should be excluded
	for _, name := range []string{"String", "Int", "Float", "Boolean", "ID"} {
		s.AddType(&schema.TypeDef{Name: name, Kind: schema.TypeKindScalar})
	}

	result := p.Print(s)
	if result != "" {
		t.Errorf("expected empty output for builtin scalars only, got %q", result)
	}
}

func TestSDLPrinter_SchemaDefinitionNoOps(t *testing.T) {
	p := NewSDLPrinter()
	s := schema.NewSchemaDefinition()
	// No query/mutation/subscription set
	result := p.printSchemaDefinition(s)
	if result != "" {
		t.Errorf("expected empty schema def, got %q", result)
	}
}

func TestSDLPrinter_PrintArgument_Required(t *testing.T) {
	p := NewSDLPrinter()
	arg := &schema.ArgumentDef{
		Name:     "id",
		Type:     &schema.TypeRef{Kind: schema.TypeKindNonNull, OfType: &schema.TypeRef{Kind: schema.TypeKindID, Name: "ID"}},
		Required: true,
	}
	result := p.printArgument(arg)
	if !strings.Contains(result, "id: ID!") {
		t.Errorf("expected 'id: ID!', got %q", result)
	}
}
