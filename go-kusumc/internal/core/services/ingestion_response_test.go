package services

import (
	"testing"

	"ingestion-go/internal/core/domain"
)

func TestClassifyResponseRegex(t *testing.T) {
	payload := map[string]interface{}{"msg": "ok", "code": "200"}
	patterns := []domain.ResponsePattern{{
		ID:          "p1",
		CommandID:   "c1",
		PatternType: "regex",
		Pattern:     `\"code\":\s*\"200\"`,
		Success:     true,
	}}
	id, status, parsed := classifyResponse(payload, patterns)
	if status != "acked" {
		t.Fatalf("expected acked, got %s", status)
	}
	if id == nil || *id != "p1" {
		t.Fatalf("expected pattern p1 matched")
	}
	if parsed["code"] != "200" {
		t.Fatalf("expected extracted code 200, got %v", parsed["code"])
	}
}

func TestClassifyResponseJsonPathFail(t *testing.T) {
	payload := map[string]interface{}{"status": "error"}
	patterns := []domain.ResponsePattern{{
		ID:          "p2",
		CommandID:   "c1",
		PatternType: "jsonpath",
		Pattern:     "$.status",
		Success:     false,
	}}
	id, status, _ := classifyResponse(payload, patterns)
	if status != "failed" {
		t.Fatalf("expected failed, got %s", status)
	}
	if id == nil || *id != "p2" {
		t.Fatalf("expected pattern p2 matched")
	}
}

func TestClassifyResponseNoPatterns(t *testing.T) {
	status := classifyStatusOnly(map[string]interface{}{})
	if status != "acked" {
		t.Fatalf("expected acked default, got %s", status)
	}
}

// classifyStatusOnly is a small helper invoking classifyResponse with no patterns.
func classifyStatusOnly(payload map[string]interface{}) string {
	_, status, _ := classifyResponse(payload, nil)
	return status
}
