package services_test

import (
	"ingestion-go/internal/core/services"
	"testing"
)

func TestTransformer_Virtual(t *testing.T) {
	trans := services.NewGovaluateTransformer()

	// Config with Virtual Sensor
	// HeatIndex = T + H
	config := []interface{}{
		map[string]interface{}{
			"id":            "heat_index",
			"param":         "heat_index",
			"transformMode": "virtual",
			"expression":    "temp + hum",
		},
	}

	raw := map[string]interface{}{
		"temp": 30.0,
		"hum":  10.0,
	}

	result, err := trans.Apply(raw, config)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	// Expect heat_index = 40
	val, ok := result["heat_index"]
	if !ok {
		t.Fatal("heat_index missing from result")
	}
	if val.(float64) != 40.0 {
		t.Errorf("Expected 40.0, got %v", val)
	}
}

func TestTransformer_Linear(t *testing.T) {
	trans := services.NewGovaluateTransformer()
	// y = x (Identity)
	config := []interface{}{
		map[string]interface{}{
			"id": "temp", "param": "temp", "transformMode": "linear",
			"raw_min": 0.0, "raw_max": 100.0,
			"min": 0.0, "max": 100.0,
		},
	}
	raw := map[string]interface{}{"temp": 50.0}
	res, _ := trans.Apply(raw, config)
	if res["temp"].(float64) != 50.0 {
		t.Errorf("Linear failed")
	}
}

func TestTransformer_Virtual_Combined(t *testing.T) {
	trans := services.NewGovaluateTransformer()
	// Combined: D1 (Door Status 0-1) + A1 (Volts 0-5)
	// Expr: D1 * 10 + A1
	// Case: D1=1 (Open), A1=2.5 -> 12.5
	config := []interface{}{
		map[string]interface{}{
			"id":            "combo_val",
			"param":         "combo_val",
			"transformMode": "virtual",
			"expression":    "D1 * 10 + A1",
		},
	}
	raw := map[string]interface{}{
		"D1": 1,
		"A1": 2.5,
	}

	res, err := trans.Apply(raw, config)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	val, ok := res["combo_val"]
	if !ok {
		t.Fatal("combo_val missing")
	}
	if val.(float64) != 12.5 {
		t.Errorf("Expected 12.5, got %v", val)
	}
}
