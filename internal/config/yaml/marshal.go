package yaml

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Unmarshal converts a map[string]any (from Parse) into a typed struct.
// The target struct must have `yaml:"fieldname"` tags to map fields.
func Unmarshal(data map[string]any, target any) error {
	return unmarshalToStruct(data, reflect.ValueOf(target))
}

// unmarshalToStruct recursively unmarshals data into a reflected value.
func unmarshalToStruct(data any, v reflect.Value) error {
	// Handle nil
	if data == nil {
		return nil
	}

	// Dereference pointer
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}

	// Ensure we can set the value
	if !v.CanSet() {
		return fmt.Errorf("cannot set value of type %v", v.Type())
	}

	switch v.Kind() {
	case reflect.Struct:
		return unmarshalStruct(data, v)
	case reflect.Map:
		return unmarshalMap(data, v)
	case reflect.Slice:
		return unmarshalSlice(data, v)
	case reflect.String:
		return unmarshalString(data, v)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// Special handling for time.Duration
		if v.Type() == reflect.TypeOf(time.Duration(0)) {
			return unmarshalDuration(data, v)
		}
		return unmarshalInt(data, v)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return unmarshalUint(data, v)
	case reflect.Float32, reflect.Float64:
		return unmarshalFloat(data, v)
	case reflect.Bool:
		return unmarshalBool(data, v)
	default:
		return fmt.Errorf("unsupported kind: %v", v.Kind())
	}
}

// unmarshalStruct unmarshals data into a struct based on yaml tags.
func unmarshalStruct(data any, v reflect.Value) error {
	dataMap, ok := data.(map[string]any)
	if !ok {
		return fmt.Errorf("expected map for struct, got %T", data)
	}

	typ := v.Type()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldValue := v.Field(i)

		// Get field name from yaml tag or use field name
		yamlName := field.Name
		tag := field.Tag.Get("yaml")
		if tag != "" {
			// Handle "name" and "name,opt" format
			parts := strings.Split(tag, ",")
			yamlName = parts[0]
		}

		// Skip unexported fields
		if !fieldValue.CanSet() {
			continue
		}

		// Get value from map
		rawValue, exists := dataMap[yamlName]
		if !exists {
			// Check for lowercase key
			for k, val := range dataMap {
				if strings.EqualFold(k, yamlName) {
					rawValue = val
					exists = true
					break
				}
			}
		}

		if !exists {
			continue // Field not in data, leave at zero value
		}

		if err := unmarshalToStruct(rawValue, fieldValue); err != nil {
			return fmt.Errorf("field %q: %w", yamlName, err)
		}
	}

	return nil
}

// unmarshalMap unmarshals data into a map.
func unmarshalMap(data any, v reflect.Value) error {
	dataMap, ok := data.(map[string]any)
	if !ok {
		return fmt.Errorf("expected map, got %T", data)
	}

	// Initialize map if nil
	if v.IsNil() {
		v.Set(reflect.MakeMap(v.Type()))
	}

	mapType := v.Type()
	keyType := mapType.Key()
	elemType := mapType.Elem()

	for key, val := range dataMap {
		// Create key
		mapKey := reflect.New(keyType).Elem()
		if err := setSimpleValue(key, mapKey); err != nil {
			return fmt.Errorf("map key %q: %w", key, err)
		}

		// Create value
		mapVal := reflect.New(elemType).Elem()
		if err := unmarshalToStruct(val, mapVal); err != nil {
			return fmt.Errorf("map value for key %q: %w", key, err)
		}

		v.SetMapIndex(mapKey, mapVal)
	}

	return nil
}

// unmarshalSlice unmarshals data into a slice.
func unmarshalSlice(data any, v reflect.Value) error {
	dataSlice, ok := data.([]any)
	if !ok {
		return fmt.Errorf("expected slice, got %T", data)
	}

	slice := reflect.MakeSlice(v.Type(), len(dataSlice), len(dataSlice))

	for i, item := range dataSlice {
		elem := slice.Index(i)
		if err := unmarshalToStruct(item, elem); err != nil {
			return fmt.Errorf("slice element %d: %w", i, err)
		}
	}

	v.Set(slice)
	return nil
}

// unmarshalString unmarshals data into a string.
func unmarshalString(data any, v reflect.Value) error {
	switch val := data.(type) {
	case string:
		v.SetString(val)
	case nil:
		v.SetString("")
	default:
		v.SetString(fmt.Sprintf("%v", val))
	}
	return nil
}

// unmarshalInt unmarshals data into an integer.
func unmarshalInt(data any, v reflect.Value) error {
	switch val := data.(type) {
	case int:
		v.SetInt(int64(val))
	case int64:
		v.SetInt(val)
	case float64:
		v.SetInt(int64(val))
	case string:
		// Try to parse as memory size first (e.g., "256MB", "1GB")
		if size, err := ParseMemorySize(val); err == nil {
			v.SetInt(size)
			return nil
		}
		// Otherwise parse as regular integer
		i, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return fmt.Errorf("cannot parse %q as int or memory size: %w", val, err)
		}
		v.SetInt(i)
	case nil:
		v.SetInt(0)
	default:
		return fmt.Errorf("cannot convert %T to int", data)
	}
	return nil
}

// unmarshalUint unmarshals data into an unsigned integer.
func unmarshalUint(data any, v reflect.Value) error {
	switch val := data.(type) {
	case int:
		if val < 0 {
			return fmt.Errorf("cannot convert negative int to uint")
		}
		v.SetUint(uint64(val))
	case int64:
		if val < 0 {
			return fmt.Errorf("cannot convert negative int64 to uint")
		}
		v.SetUint(uint64(val))
	case uint:
		v.SetUint(uint64(val))
	case uint64:
		v.SetUint(val)
	case float64:
		if val < 0 {
			return fmt.Errorf("cannot convert negative float to uint")
		}
		v.SetUint(uint64(val))
	case string:
		u, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			return fmt.Errorf("cannot parse %q as uint: %w", val, err)
		}
		v.SetUint(u)
	case nil:
		v.SetUint(0)
	default:
		return fmt.Errorf("cannot convert %T to uint", data)
	}
	return nil
}

// unmarshalFloat unmarshals data into a float.
func unmarshalFloat(data any, v reflect.Value) error {
	switch val := data.(type) {
	case float64:
		v.SetFloat(val)
	case float32:
		v.SetFloat(float64(val))
	case int:
		v.SetFloat(float64(val))
	case int64:
		v.SetFloat(float64(val))
	case string:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return fmt.Errorf("cannot parse %q as float: %w", val, err)
		}
		v.SetFloat(f)
	case nil:
		v.SetFloat(0)
	default:
		return fmt.Errorf("cannot convert %T to float", data)
	}
	return nil
}

// unmarshalBool unmarshals data into a bool.
func unmarshalBool(data any, v reflect.Value) error {
	switch val := data.(type) {
	case bool:
		v.SetBool(val)
	case nil:
		v.SetBool(false)
	case string:
		b, err := strconv.ParseBool(val)
		if err != nil {
			return fmt.Errorf("cannot parse %q as bool: %w", val, err)
		}
		v.SetBool(b)
	default:
		return fmt.Errorf("cannot convert %T to bool", data)
	}
	return nil
}

// unmarshalDuration unmarshals data into a time.Duration.
func unmarshalDuration(data any, v reflect.Value) error {
	switch val := data.(type) {
	case string:
		d, err := time.ParseDuration(val)
		if err != nil {
			return fmt.Errorf("cannot parse %q as duration: %w", val, err)
		}
		v.SetInt(int64(d))
	case int:
		v.SetInt(int64(time.Duration(val) * time.Second))
	case int64:
		v.SetInt(int64(time.Duration(val) * time.Second))
	case float64:
		v.SetInt(int64(time.Duration(val) * time.Second))
	case nil:
		v.SetInt(0)
	default:
		return fmt.Errorf("cannot convert %T to duration", data)
	}
	return nil
}

// setSimpleValue sets a simple value (string, int, etc.) from a string.
func setSimpleValue(s string, v reflect.Value) error {
	switch v.Kind() {
	case reflect.String:
		v.SetString(s)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		v.SetInt(i)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return err
		}
		v.SetUint(u)
		return nil
	default:
		return fmt.Errorf("unsupported key type: %v", v.Kind())
	}
}

// ParseDuration parses a duration string (e.g., "30s", "5m", "1h").
func ParseDuration(s string) (time.Duration, error) {
	return time.ParseDuration(s)
}

// ParseMemorySize parses a memory size string (e.g., "256MB", "1GB", "512KB").
func ParseMemorySize(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))

	multipliers := map[string]int64{
		"KB": 1024,
		"MB": 1024 * 1024,
		"GB": 1024 * 1024 * 1024,
		"TB": 1024 * 1024 * 1024 * 1024,
	}

	for suffix, multiplier := range multipliers {
		if strings.HasSuffix(s, suffix) {
			numStr := strings.TrimSuffix(s, suffix)
			num, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid memory size %q: %w", s, err)
			}
			return int64(num * float64(multiplier)), nil
		}
	}

	// No suffix, assume bytes
	return strconv.ParseInt(s, 10, 64)
}
