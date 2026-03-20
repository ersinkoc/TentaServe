package schema

// FieldMapper provides bidirectional field name mapping for REST↔GraphQL translation.
// It supports explicit mappings from configuration and falls back to naming conventions.
type FieldMapper struct {
	// explicitMappings stores configured mappings: externalName -> internalName
	// For REST→GraphQL: REST field name -> GraphQL field name
	// For GraphQL→REST: GraphQL field name -> REST field name
	explicitMappings map[string]string

	// reverseMappings stores the inverse for fast lookup
	reverseMappings map[string]string

	// defaultConvention is applied when no explicit mapping exists
	// "camel" for camelCase, "snake" for snake_case, "pascal" for PascalCase
	defaultConvention string
}

// FieldMapperOptions configures the field mapper.
type FieldMapperOptions struct {
	// Mappings is a map of field name mappings
	// Key: source field name (REST or GraphQL)
	// Value: target field name (GraphQL or REST)
	Mappings map[string]string

	// DefaultConvention applied when no explicit mapping exists
	// Options: "camel", "snake", "pascal", "kebab"
	DefaultConvention string
}

// NewFieldMapper creates a new field mapper with the given options.
func NewFieldMapper(opts FieldMapperOptions) *FieldMapper {
	fm := &FieldMapper{
		explicitMappings:  make(map[string]string),
		reverseMappings:   make(map[string]string),
		defaultConvention: opts.DefaultConvention,
	}

	// Set default convention if not specified
	if fm.defaultConvention == "" {
		fm.defaultConvention = "camel"
	}

	// Add explicit mappings
	for source, target := range opts.Mappings {
		fm.AddMapping(source, target)
	}

	return fm
}

// AddMapping adds a bidirectional field mapping.
// source: field name as it appears in the source (REST for input, GraphQL for output)
// target: field name as it should appear in the target (GraphQL for input, REST for output)
func (fm *FieldMapper) AddMapping(source, target string) {
	if source == "" || target == "" {
		return
	}

	fm.explicitMappings[source] = target
	fm.reverseMappings[target] = source
}

// Map maps a field name from source to target.
// If an explicit mapping exists, it is used; otherwise naming convention is applied.
func (fm *FieldMapper) Map(sourceName string) string {
	// Check explicit mapping first
	if target, ok := fm.explicitMappings[sourceName]; ok {
		return target
	}

	// Fall back to naming convention
	return fm.applyConvention(sourceName)
}

// MapReverse maps a field name from target back to source.
// Used for reverse transformations.
func (fm *FieldMapper) MapReverse(targetName string) string {
	// Check reverse mapping first
	if source, ok := fm.reverseMappings[targetName]; ok {
		return source
	}

	// Fall back to reverse naming convention
	return fm.applyReverseConvention(targetName)
}

// HasMapping checks if a field has an explicit mapping.
func (fm *FieldMapper) HasMapping(sourceName string) bool {
	_, ok := fm.explicitMappings[sourceName]
	return ok
}

// HasReverseMapping checks if a target field has an explicit reverse mapping.
func (fm *FieldMapper) HasReverseMapping(targetName string) bool {
	_, ok := fm.reverseMappings[targetName]
	return ok
}

// applyConvention applies the default naming convention.
func (fm *FieldMapper) applyConvention(name string) string {
	switch fm.defaultConvention {
	case "snake":
		return ToSnakeCase(name)
	case "pascal":
		return ToPascalCase(name)
	case "kebab":
		return ToKebabCase(name)
	case "camel":
		fallthrough
	default:
		return ToCamelCase(name)
	}
}

// applyReverseConvention applies the reverse of the default naming convention.
func (fm *FieldMapper) applyReverseConvention(name string) string {
	// Reverse conventions:
	// camel -> snake (GraphQL camelCase -> REST snake_case)
	// snake -> camel (REST snake_case -> GraphQL camelCase)
	// pascal -> snake
	// kebab -> camel
	switch fm.defaultConvention {
	case "snake":
		return ToCamelCase(name)
	case "pascal":
		return ToSnakeCase(name)
	case "kebab":
		return ToCamelCase(name)
	case "camel":
		fallthrough
	default:
		return ToSnakeCase(name)
	}
}

// MapFields maps all fields in a map using the mapper.
// Returns a new map with mapped keys, or nil if input is nil.
func (fm *FieldMapper) MapFields(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return nil
	}

	result := make(map[string]interface{}, len(data))
	for key, value := range data {
		mappedKey := fm.Map(key)
		result[mappedKey] = fm.mapValue(value)
	}
	return result
}

// MapFieldsReverse maps all fields back from target to source naming.
func (fm *FieldMapper) MapFieldsReverse(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return nil
	}

	result := make(map[string]interface{}, len(data))
	for key, value := range data {
		mappedKey := fm.MapReverse(key)
		result[mappedKey] = fm.mapValueReverse(value)
	}
	return result
}

// mapValue recursively maps field names in nested structures.
func (fm *FieldMapper) mapValue(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		return fm.MapFields(v)
	case []interface{}:
		return fm.mapSlice(v)
	default:
		return value
	}
}

// mapValueReverse recursively maps field names back in nested structures.
func (fm *FieldMapper) mapValueReverse(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		return fm.MapFieldsReverse(v)
	case []interface{}:
		return fm.mapSliceReverse(v)
	default:
		return value
	}
}

// mapSlice maps field names in a slice.
func (fm *FieldMapper) mapSlice(slice []interface{}) []interface{} {
	result := make([]interface{}, len(slice))
	for i, item := range slice {
		result[i] = fm.mapValue(item)
	}
	return result
}

// mapSliceReverse maps field names back in a slice.
func (fm *FieldMapper) mapSliceReverse(slice []interface{}) []interface{} {
	result := make([]interface{}, len(slice))
	for i, item := range slice {
		result[i] = fm.mapValueReverse(item)
	}
	return result
}

// PerUpstreamFieldMapper manages field mappers for multiple upstreams.
type PerUpstreamFieldMapper struct {
	mappers       map[string]*FieldMapper
	defaultMapper *FieldMapper
}

// NewPerUpstreamFieldMapper creates a new per-upstream field mapper manager.
func NewPerUpstreamFieldMapper() *PerUpstreamFieldMapper {
	return &PerUpstreamFieldMapper{
		mappers: make(map[string]*FieldMapper),
		defaultMapper: NewFieldMapper(FieldMapperOptions{
			DefaultConvention: "camel",
		}),
	}
}

// RegisterMapper registers a field mapper for a specific upstream.
func (pm *PerUpstreamFieldMapper) RegisterMapper(upstreamName string, mapper *FieldMapper) {
	pm.mappers[upstreamName] = mapper
}

// GetMapper gets the field mapper for an upstream, or the default if none registered.
func (pm *PerUpstreamFieldMapper) GetMapper(upstreamName string) *FieldMapper {
	if mapper, ok := pm.mappers[upstreamName]; ok {
		return mapper
	}
	return pm.defaultMapper
}

// MapForUpstream maps a field name for a specific upstream.
func (pm *PerUpstreamFieldMapper) MapForUpstream(upstreamName, fieldName string) string {
	return pm.GetMapper(upstreamName).Map(fieldName)
}

// MapReverseForUpstream maps a field name back for a specific upstream.
func (pm *PerUpstreamFieldMapper) MapReverseForUpstream(upstreamName, fieldName string) string {
	return pm.GetMapper(upstreamName).MapReverse(fieldName)
}
