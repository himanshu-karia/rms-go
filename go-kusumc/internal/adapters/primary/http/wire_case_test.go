package http

import "testing"

func TestNormalizeToSnakeKeys_ConvertsNestedMaps(t *testing.T) {
	in := map[string]any{
		"lastUpdated": "2026-01-01T00:00:00Z",
		"productId": map[string]any{
			"vehicleCount": 3,
			"_id":          "p1",
		},
		"already_snake": 1,
	}

	outAny := normalizeToSnakeKeys(in)
	out, ok := outAny.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", outAny)
	}
	if _, ok := out["last_updated"]; !ok {
		t.Fatalf("expected last_updated key, got keys=%v", keysOf(out))
	}
	prod, _ := out["product_id"].(map[string]any)
	if prod == nil {
		t.Fatalf("expected product_id map, got %T", out["product_id"])
	}
	if _, ok := prod["vehicle_count"]; !ok {
		t.Fatalf("expected vehicle_count inside product_id, got %v", keysOf(prod))
	}
	if _, ok := prod["_id"]; !ok {
		t.Fatalf("expected _id preserved, got %v", keysOf(prod))
	}
	if _, ok := out["already_snake"]; !ok {
		t.Fatalf("expected already_snake preserved")
	}
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
