package graphql

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ersinkoc/tentaserve/internal/schema"
)

// SDLPrinter prints a schema definition in GraphQL SDL format.
type SDLPrinter struct {
	indent string
}

// NewSDLPrinter creates a new SDL printer.
func NewSDLPrinter() *SDLPrinter {
	return &SDLPrinter{
		indent: "  ",
	}
}

// Print prints a schema definition to SDL format.
func (p *SDLPrinter) Print(s *schema.SchemaDefinition) string {
	if s == nil {
		return ""
	}

	var parts []string

	// Print schema definition if operations are defined
	schemaDef := p.printSchemaDefinition(s)
	if schemaDef != "" {
		parts = append(parts, schemaDef)
	}

	// Get all types sorted alphabetically
	var typeNames []string
	for name := range s.Types {
		// Skip built-in scalars
		if !isBuiltinScalar(name) {
			typeNames = append(typeNames, name)
		}
	}
	sort.Strings(typeNames)

	// Print scalars first
	for _, name := range typeNames {
		t := s.Types[name]
		if t.Kind == schema.TypeKindScalar {
			parts = append(parts, p.printScalar(t))
		}
	}

	// Print enums
	for _, name := range typeNames {
		t := s.Types[name]
		if t.Kind == schema.TypeKindEnum {
			parts = append(parts, p.printEnum(t))
		}
	}

	// Print objects
	for _, name := range typeNames {
		t := s.Types[name]
		if t.Kind == schema.TypeKindObject {
			parts = append(parts, p.printObject(t))
		}
	}

	// Print input types
	for _, name := range typeNames {
		t := s.Types[name]
		if t.Kind == schema.TypeKindInput {
			parts = append(parts, p.printInput(t))
		}
	}

	// Print interfaces
	for _, name := range typeNames {
		t := s.Types[name]
		if t.Kind == schema.TypeKindInterface {
			parts = append(parts, p.printInterface(t))
		}
	}

	// Print unions
	for _, name := range typeNames {
		t := s.Types[name]
		if t.Kind == schema.TypeKindUnion {
			parts = append(parts, p.printUnion(t))
		}
	}

	// Print directives
	for _, d := range s.Directives {
		parts = append(parts, p.printDirective(d))
	}

	return strings.Join(parts, "\n\n")
}

// printSchemaDefinition prints the schema definition block.
func (p *SDLPrinter) printSchemaDefinition(s *schema.SchemaDefinition) string {
	var parts []string

	if s.Query != nil && s.Query.Name != "" {
		parts = append(parts, fmt.Sprintf("%squery: %s", p.indent, s.Query.Name))
	}
	if s.Mutation != nil && s.Mutation.Name != "" {
		parts = append(parts, fmt.Sprintf("%smutation: %s", p.indent, s.Mutation.Name))
	}
	if s.Subscription != nil && s.Subscription.Name != "" {
		parts = append(parts, fmt.Sprintf("%ssubscription: %s", p.indent, s.Subscription.Name))
	}

	if len(parts) == 0 {
		return ""
	}

	return fmt.Sprintf("schema {\n%s\n}", strings.Join(parts, "\n"))
}

// printScalar prints a scalar type.
func (p *SDLPrinter) printScalar(t *schema.TypeDef) string {
	var parts []string

	if t.Description != "" {
		parts = append(parts, p.printDescription(t.Description, false))
	}

	parts = append(parts, fmt.Sprintf("scalar %s", t.Name))

	return strings.Join(parts, "\n")
}

// printEnum prints an enum type.
func (p *SDLPrinter) printEnum(t *schema.TypeDef) string {
	var parts []string

	if t.Description != "" {
		parts = append(parts, p.printDescription(t.Description, false))
	}

	var values []string
	for _, v := range t.EnumValues {
		valueStr := p.printEnumValue(v)
		values = append(values, p.indent+valueStr)
	}

	enumDef := fmt.Sprintf("enum %s {\n%s\n}", t.Name, strings.Join(values, "\n"))
	parts = append(parts, enumDef)

	return strings.Join(parts, "\n")
}

// printEnumValue prints an enum value.
func (p *SDLPrinter) printEnumValue(v schema.EnumValueDef) string {
	var parts []string

	if v.Description != "" {
		parts = append(parts, p.printDescription(v.Description, true))
		parts = append(parts, p.indent)
	}

	value := v.Name
	if v.Deprecated {
		if v.DepReason != "" {
			value += fmt.Sprintf(" @deprecated(reason: %q)", v.DepReason)
		} else {
			value += " @deprecated"
		}
	}

	parts = append(parts, value)
	return strings.Join(parts, "")
}

// printObject prints an object type.
func (p *SDLPrinter) printObject(t *schema.TypeDef) string {
	var parts []string

	if t.Description != "" {
		parts = append(parts, p.printDescription(t.Description, false))
	}

	// Build type header
	header := fmt.Sprintf("type %s", t.Name)

	// Add interfaces
	if len(t.Interfaces) > 0 {
		header += fmt.Sprintf(" implements %s", strings.Join(t.Interfaces, " & "))
	}

	// Add fields
	var fieldStrs []string
	for _, f := range t.Fields {
		fieldStrs = append(fieldStrs, p.indent+p.printField(f))
	}

	objectDef := fmt.Sprintf("%s {\n%s\n}", header, strings.Join(fieldStrs, "\n"))
	parts = append(parts, objectDef)

	return strings.Join(parts, "\n")
}

// printInput prints an input type.
func (p *SDLPrinter) printInput(t *schema.TypeDef) string {
	var parts []string

	if t.Description != "" {
		parts = append(parts, p.printDescription(t.Description, false))
	}

	var fieldStrs []string
	for _, f := range t.Fields {
		fieldStrs = append(fieldStrs, p.indent+p.printInputField(f))
	}

	inputDef := fmt.Sprintf("input %s {\n%s\n}", t.Name, strings.Join(fieldStrs, "\n"))
	parts = append(parts, inputDef)

	return strings.Join(parts, "\n")
}

// printInterface prints an interface type.
func (p *SDLPrinter) printInterface(t *schema.TypeDef) string {
	var parts []string

	if t.Description != "" {
		parts = append(parts, p.printDescription(t.Description, false))
	}

	var fieldStrs []string
	for _, f := range t.Fields {
		fieldStrs = append(fieldStrs, p.indent+p.printField(f))
	}

	interfaceDef := fmt.Sprintf("interface %s {\n%s\n}", t.Name, strings.Join(fieldStrs, "\n"))
	parts = append(parts, interfaceDef)

	return strings.Join(parts, "\n")
}

// printUnion prints a union type.
func (p *SDLPrinter) printUnion(t *schema.TypeDef) string {
	var parts []string

	if t.Description != "" {
		parts = append(parts, p.printDescription(t.Description, false))
	}

	unionDef := fmt.Sprintf("union %s = %s", t.Name, strings.Join(t.Possible, " | "))
	parts = append(parts, unionDef)

	return strings.Join(parts, "\n")
}

// printField prints a field definition.
func (p *SDLPrinter) printField(f *schema.FieldDef) string {
	var parts []string

	if f.Description != "" {
		parts = append(parts, p.printDescription(f.Description, true))
		parts = append(parts, p.indent)
	}

	// Field name
	field := f.Name

	// Arguments
	if len(f.Arguments) > 0 {
		var argStrs []string
		for _, a := range f.Arguments {
			argStrs = append(argStrs, p.printArgument(a))
		}
		field += fmt.Sprintf("(%s)", strings.Join(argStrs, ", "))
	}

	// Type
	field += fmt.Sprintf(": %s", p.printTypeRef(f.Type))

	// Deprecated directive
	if f.Deprecated {
		if f.DepReason != "" {
			field += fmt.Sprintf(" @deprecated(reason: %q)", f.DepReason)
		} else {
			field += " @deprecated"
		}
	}

	parts = append(parts, field)
	return strings.Join(parts, "")
}

// printInputField prints an input field definition.
func (p *SDLPrinter) printInputField(f *schema.FieldDef) string {
	var parts []string

	if f.Description != "" {
		parts = append(parts, p.printDescription(f.Description, true))
		parts = append(parts, p.indent)
	}

	field := f.Name
	field += fmt.Sprintf(": %s", p.printTypeRef(f.Type))

	// Default value
	if f.Type.Kind != schema.TypeKindNonNull && f.Type.Kind != schema.TypeKindList {
		// Simple check - in production would need proper value serialization
		if hasDefaultValue(f) {
			field += fmt.Sprintf(" = %v", formatDefaultValue(f.Type))
		}
	}

	parts = append(parts, field)
	return strings.Join(parts, "")
}

// printArgument prints an argument definition.
func (p *SDLPrinter) printArgument(a *schema.ArgumentDef) string {
	arg := a.Name
	arg += fmt.Sprintf(": %s", p.printTypeRef(a.Type))

	// Default value (simplified)
	if !a.Required && a.DefaultValue != nil {
		arg += fmt.Sprintf(" = %v", a.DefaultValue)
	}

	return arg
}

// printTypeRef prints a type reference.
func (p *SDLPrinter) printTypeRef(t *schema.TypeRef) string {
	if t == nil {
		return ""
	}
	return t.String()
}

// printDescription prints a description string.
func (p *SDLPrinter) printDescription(desc string, inline bool) string {
	if strings.Contains(desc, "\n") || strings.Contains(desc, `"`) {
		// Multi-line or contains quotes, use block string
		return fmt.Sprintf(`"""\n%s\n"""`, desc)
	}

	if inline {
		return fmt.Sprintf("# %s", desc)
	}
	return fmt.Sprintf(`"%s"`, desc)
}

// printDirective prints a directive definition.
func (p *SDLPrinter) printDirective(d *schema.DirectiveDef) string {
	var parts []string

	if d.Description != "" {
		parts = append(parts, p.printDescription(d.Description, false))
	}

	// Arguments
	var argStr string
	if len(d.Arguments) > 0 {
		var argStrs []string
		for _, a := range d.Arguments {
			argStrs = append(argStrs, p.printArgument(a))
		}
		argStr = fmt.Sprintf("(%s)", strings.Join(argStrs, ", "))
	}

	// Locations
	locations := fmt.Sprintf("on %s", strings.Join(d.Locations, " | "))

	directiveDef := fmt.Sprintf("directive @%s%s %s", d.Name, argStr, locations)
	parts = append(parts, directiveDef)

	return strings.Join(parts, "\n")
}

// isBuiltinScalar checks if a type is a built-in scalar.
func isBuiltinScalar(name string) bool {
	builtins := []string{"String", "Int", "Float", "Boolean", "ID"}
	for _, b := range builtins {
		if b == name {
			return true
		}
	}
	return false
}

// hasDefaultValue checks if a field has a default value.
func hasDefaultValue(f *schema.FieldDef) bool {
	// Simplified - would need actual default value tracking
	return false
}

// formatDefaultValue formats a default value for SDL output.
func formatDefaultValue(t *schema.TypeRef) string {
	switch t.Kind {
	case schema.TypeKindString:
		return `""`
	case schema.TypeKindInt, schema.TypeKindFloat:
		return "0"
	case schema.TypeKindBool:
		return "false"
	default:
		return "null"
	}
}

// PrintSDL is a convenience function to print a schema definition to SDL.
func PrintSDL(s *schema.SchemaDefinition) string {
	printer := NewSDLPrinter()
	return printer.Print(s)
}
