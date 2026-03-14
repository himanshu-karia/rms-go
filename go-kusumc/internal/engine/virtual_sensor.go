package engine

import (
	"math"

	"github.com/Knetic/govaluate"
)

// TransformPayload applies the Virtual Sensor logic defined in the Project Config
// It returns a new map with the transformed values.
// Original raw values are preserved if no transformation is defined.
func TransformPayload(project *ProjectConfig, data map[string]interface{}) map[string]interface{} {
	if project == nil {
		return data
	}

	transformed := make(map[string]interface{})

	// Create lookup for Sensor Config
	sensorMap := make(map[string]SensorConfig)
	for _, s := range project.Hardware.Sensors {
		sensorMap[s.Param] = s
	}

	for key, value := range data {
		// Default: Copy raw value
		transformed[key] = value

		config, exists := sensorMap[key]
		if !exists {
			continue
		}

		// Apply Transformation if enabled
		if config.TransformMode != "none" && config.TransformMode != "" {
			// Convert generic interface{} to float64 for math
			floatVal, isNum := toFloat(value)

			if isNum {
				if config.TransformMode == "linear" {
					// Linear Scaling: y = (x - rawMin) * (outMax - outMin) / (rawMax - rawMin) + outMin
					if config.RawMax != config.RawMin { // Avoid division by zero
						slope := (config.Max - config.Min) / (config.RawMax - config.RawMin)
						val := (floatVal-config.RawMin)*slope + config.Min
						// Round to 2 decimal places?
						transformed[key] = math.Round(val*100) / 100
					}
				} else if config.TransformMode == "digital" {
					// Digital Mapping: 0 -> Label0, 1 -> Label1
					intVal := int(floatVal)
					if intVal == 0 && config.Digital0Label != "" {
						transformed[key] = config.Digital0Label
					} else if intVal == 1 && config.Digital1Label != "" {
						transformed[key] = config.Digital1Label
					}
				} else if config.TransformMode == "expression" && config.Expression != "" {
					// Flexible Math Logic: "(current * voltage) / 1000"
					// We need to pass the ENTIRE payload as context or at least relevant local keys?
					// Usually virtual sensors depend on OTHER sensors.
					// Context: The 'data' map + 'raw' (self)

					// 1. Build Context
					context := make(map[string]interface{})
					for k, v := range data {
						context[k] = v
					}
					context["raw"] = floatVal // 'raw' references the current value being transformed
					context["self"] = floatVal

					// 2. Evaluate
					expr, err := govaluate.NewEvaluableExpression(config.Expression)
					if err == nil {
						result, err := expr.Evaluate(context)
						if err == nil {
							// Round result
							if fRes, ok := toFloat(result); ok {
								transformed[key] = math.Round(fRes*100) / 100
							} else {
								transformed[key] = result
							}
						} else {
							// Log logic error? Keep raw value or set error?
							// set key_error?
							transformed[key+"_error"] = err.Error()
						}
					}
				}
			}
		}
	}

	return transformed
}

// Helper to safely convert interface{} to float64
func toFloat(unk interface{}) (float64, bool) {
	switch i := unk.(type) {
	case float64:
		return i, true
	case float32:
		return float64(i), true
	case int64:
		return float64(i), true
	case int32:
		return float64(i), true
	case int:
		return float64(i), true
	default:
		return 0, false
	}
}
