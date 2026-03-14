package engine

import "strings"

// VerifyPayload checks whether the incoming payload keys comply with the configured project DNA.
func VerifyPayload(project *ProjectConfig, data map[string]interface{}) (bool, []string) {
	if project == nil {
		return false, []string{"Missing Project Config"}
	}

	// Minimal required envelope: identifiers and timestamp must be present.
	required := []string{"imei", "project_id", "timestamp"}
	var missing []string
	for _, key := range required {
		if _, ok := data[key]; !ok {
			// allow case variants
			if _, okUpper := data[strings.ToUpper(key)]; !okUpper {
				if _, okLower := data[strings.ToLower(key)]; !okLower {
					missing = append(missing, key)
				}
			}
		}
	}

	if len(missing) > 0 {
		return false, missing
	}

	allowed := make(map[string]bool)

	// Envelope defaults that we always accept
	for _, builtin := range []string{"timestamp", "TIMESTAMP", "msgid", "MSGID", "imei", "IMEI", "project_id", "PROJECT_ID", "device_uuid", "DEVICE_UUID"} {
		allowed[builtin] = true
	}

	// Prefer packet schema when available
	if project.PayloadSchemas != nil {
		for _, schema := range project.PayloadSchemas {
			// Accept envelope keys declared in schema
			for _, spec := range schema.EnvelopeKeys {
				allowKey(allowed, spec.Key)
			}
			// Accept packet keys declared in schema
			for _, spec := range schema.Keys {
				allowKey(allowed, spec.Key)
			}
		}
	}

	// Fallback to legacy sensors list (+ ensures backwards compatibility)
	for _, sensor := range project.Hardware.Sensors {
		allowKey(allowed, sensor.Param)
	}

	var unknown []string
	for key := range data {
		if !allowed[key] {
			unknown = append(unknown, key)
		}
	}

	if len(unknown) > 0 {
		return false, unknown
	}
	return true, nil
}

func allowKey(allowed map[string]bool, key string) {
	if key == "" {
		return
	}
	allowed[key] = true
	if upper := strings.ToUpper(key); upper != key {
		allowed[upper] = true
	}
	if lower := strings.ToLower(key); lower != key {
		allowed[lower] = true
	}
}
