package verticals

import (
	"math"
)

// --- 1. Agriculture (Soil Advisory) ---

type SoilData struct {
	PH float64 `json:"ph"`
	N  float64 `json:"n"`
	P  float64 `json:"p"`
	K  float64 `json:"k"`
}

type Advice struct {
	Parameter string
	Severity  string
	Message   string
}

// GenerateSoilAdvice evaluates static rules (Mocked: in real app load from DB)
func GenerateSoilAdvice(crop string, data SoilData) []Advice {
	advice := []Advice{}

	// Hardcoded logic from Node or DB Rules
	if crop == "Paddy" {
		if data.PH < 6.0 {
			advice = append(advice, Advice{"pH", "MODERATE", "Soil is acidic. Add Lime."})
		}
	}
	return advice
}

// --- 2. Healthcare (Vitals) ---

func AnalyzeVitals(heartRate, spo2 float64) string {
	status := "NORMAL"
	if spo2 < 90 {
		status = "CRITICAL_LOW_SPO2"
	}
	if heartRate > 120 {
		status = "TACHYCARDIA"
	}
	return status
}

// --- 3. Traffic (Congestion) ---

func AnalyzeTraffic(carCount, bikeCount int) string {
	total := carCount + bikeCount
	if total > 50 {
		return "HIGH"
	}
	if total > 20 {
		return "MODERATE"
	}
	return "LOW"
}

// --- 4. Logistics (Geofence) ---

// Haversine
func IsInside(lat1, lon1, lat2, lon2, radiusMeters float64) bool {
	var R = 6371000.0 // meters
	dLat := (lat2 - lat1) * (math.Pi / 180.0)
	dLon := (lon2 - lon1) * (math.Pi / 180.0)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*(math.Pi/180.0))*math.Cos(lat2*(math.Pi/180.0))*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	dist := R * c

	return dist <= radiusMeters
}
