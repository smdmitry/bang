package bang

import "reflect"

// FilterSafe extracts only fields tagged with `expose:"true"` from a struct.
// Maps are returned as-is (considered safe when passed as Safe data).
func FilterSafe(v any) map[string]any {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	switch rv.Kind() {
	case reflect.Map:
		return mapToStringAny(rv)
	case reflect.Struct:
		return filterStructSafe(rv)
	default:
		return nil
	}
}

// ToMap converts any struct or map to map[string]any (all fields, no filtering).
func ToMap(v any) map[string]any {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	switch rv.Kind() {
	case reflect.Map:
		return mapToStringAny(rv)
	case reflect.Struct:
		return structToMap(rv)
	default:
		return map[string]any{"value": v}
	}
}

func filterStructSafe(rv reflect.Value) map[string]any {
	rt := rv.Type()
	result := make(map[string]any)
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if !f.IsExported() || f.Tag.Get("expose") != "true" {
			continue
		}
		result[jsonFieldName(f)] = rv.Field(i).Interface()
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func structToMap(rv reflect.Value) map[string]any {
	rt := rv.Type()
	result := make(map[string]any)
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if !f.IsExported() {
			continue
		}
		result[jsonFieldName(f)] = rv.Field(i).Interface()
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func mapToStringAny(rv reflect.Value) map[string]any {
	result := make(map[string]any, rv.Len())
	for _, key := range rv.MapKeys() {
		if k, ok := key.Interface().(string); ok {
			result[k] = rv.MapIndex(key).Interface()
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func jsonFieldName(f reflect.StructField) string {
	tag := f.Tag.Get("json")
	if tag == "" || tag == "-" {
		return f.Name
	}
	for i := 0; i < len(tag); i++ {
		if tag[i] == ',' {
			return tag[:i]
		}
	}
	return tag
}
