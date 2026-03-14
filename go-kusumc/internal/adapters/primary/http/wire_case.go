package http

import (
	"encoding/json"
	"reflect"
	"unicode"
)

func toSnakeKey(key string) string {
	if key == "" {
		return key
	}
	runes := []rune(key)
	out := make([]rune, 0, len(runes)+4)
	for i, r := range runes {
		if r == '_' {
			out = append(out, r)
			continue
		}
		if unicode.IsUpper(r) {
			prev := rune(0)
			if i > 0 {
				prev = runes[i-1]
			}
			next := rune(0)
			if i < len(runes)-1 {
				next = runes[i+1]
			}

			// Insert '_' for boundaries like: aB -> a_b, 1B -> 1_b
			if i > 0 && prev != '_' && (unicode.IsLower(prev) || unicode.IsDigit(prev)) {
				out = append(out, '_')
			}
			// Insert '_' for acronym boundary like: "IDValue" -> "id_value" (underscore before V)
			if i > 0 && prev != '_' && unicode.IsUpper(prev) && next != 0 && unicode.IsLower(next) {
				out = append(out, '_')
			}

			out = append(out, unicode.ToLower(r))
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

func normalizeToSnakeKeys(value any) any {
	if value == nil {
		return nil
	}

	v := reflect.ValueOf(value)
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Struct:
		// Convert structs (respecting json tags) into generic map/slice shapes,
		// then normalize keys recursively.
		b, err := json.Marshal(v.Interface())
		if err != nil {
			return value
		}
		var decoded any
		if err := json.Unmarshal(b, &decoded); err != nil {
			return value
		}
		return normalizeToSnakeKeys(decoded)
	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String {
			return value
		}
		out := make(map[string]any, v.Len())
		iter := v.MapRange()
		for iter.Next() {
			k := iter.Key().String()
			out[toSnakeKey(k)] = normalizeToSnakeKeys(iter.Value().Interface())
		}
		return out
	case reflect.Slice, reflect.Array:
		out := make([]any, 0, v.Len())
		for i := 0; i < v.Len(); i++ {
			out = append(out, normalizeToSnakeKeys(v.Index(i).Interface()))
		}
		return out
	default:
		return value
	}
}
