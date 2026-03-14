package services

import (
	"fmt"
	"strings"

	"github.com/Knetic/govaluate" // Needs go get
)

type GovaluateTransformer struct {
	// Cache compiled expressions: key = "sensorParam:expression"
	exprCache map[string]*govaluate.EvaluableExpression
}

func NewGovaluateTransformer() *GovaluateTransformer {
	return &GovaluateTransformer{
		exprCache: make(map[string]*govaluate.EvaluableExpression),
	}
}

// Config is expected to be the full Project Struct
// But for simplicity here assume it passes the list of Sensors
func (t *GovaluateTransformer) Apply(raw map[string]interface{}, sensorConfig interface{}) (map[string]interface{}, error) {

	sensors, ok := sensorConfig.([]interface{})
	if !ok || len(sensors) == 0 {
		// No transforms configured; keep raw payload intact
		return raw, nil
	}

	computed := make(map[string]interface{})

	for _, s := range sensors {
		sensorMap, ok := s.(map[string]interface{})
		if !ok {
			continue
		}

		mode, _ := sensorMap["transformMode"].(string)
		mode = strings.ToLower(mode)

		param, _ := sensorMap["param"].(string)
		legacyID, _ := sensorMap["id"].(string)
		if param == "" {
			param = legacyID
		}
		if param == "" {
			continue
		}

		// Virtual Mode: Derived from other values, Raw key might not exist
		if mode == "virtual" {
			expr, _ := sensorMap["expression"].(string)
			if expr == "" {
				continue
			}
			cacheKey := param + ":" + expr

			compiled, ok := t.exprCache[cacheKey]
			if !ok {
				var err error
				compiled, err = govaluate.NewEvaluableExpression(expr)
				if err != nil {
					continue
				}
				t.exprCache[cacheKey] = compiled
			}
			res, err := compiled.Evaluate(raw)
			if err == nil {
				computed[param] = res
			}
			continue
		}

		// Find Raw Value (prefer param, fallback to legacy id)
		val, exists := raw[param]
		if !exists && legacyID != "" {
			val, exists = raw[legacyID]
		}
		if !exists {
			continue
		}

		floatVal := convertToFloat(val)

		switch mode {
		case "linear":
			rawMin, okMin := sensorMap["raw_min"].(float64)
			rawMax, okMax := sensorMap["raw_max"].(float64)
			min, okMinOut := sensorMap["min"].(float64)
			max, okMaxOut := sensorMap["max"].(float64)
			if !okMin || !okMax || !okMinOut || !okMaxOut {
				continue
			}
			if rawMax-rawMin == 0 {
				continue
			}
			slope := (max - min) / (rawMax - rawMin)
			res := (floatVal-rawMin)*slope + min
			computed[param] = res

		case "expression":
			expr, _ := sensorMap["expression"].(string)
			if expr == "" {
				continue
			}
			cacheKey := param + ":" + expr

			compiled, ok := t.exprCache[cacheKey]
			if !ok {
				var err error
				compiled, err = govaluate.NewEvaluableExpression(expr)
				if err != nil {
					fmt.Println("Expr Error", err)
					continue
				}
				t.exprCache[cacheKey] = compiled
			}

			params := map[string]interface{}{"x": floatVal}

			res, err := compiled.Evaluate(params)
			if err == nil {
				computed[param] = res
			}

		case "digital":
			if floatVal == 0 {
				computed[param] = sensorMap["digital_0_label"]
			} else {
				computed[param] = sensorMap["digital_1_label"]
			}
		default:
			computed[param] = val
		}
	}

	if len(computed) == 0 {
		// Nothing transformed; keep raw
		return raw, nil
	}

	return computed, nil
}

func convertToFloat(unk interface{}) float64 {
	switch i := unk.(type) {
	case float64:
		return i
	case float32:
		return float64(i)
	case int:
		return float64(i)
	case int64:
		return float64(i)
	default:
		return 0
	}
}
