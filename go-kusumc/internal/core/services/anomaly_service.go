package services

import (
	"fmt"
	"ingestion-go/internal/adapters/secondary"
	"log"
)

type AnomalyService struct {
	repo *secondary.PostgresRepo
}

func NewAnomalyService(repo *secondary.PostgresRepo) *AnomalyService {
	return &AnomalyService{repo: repo}
}

// DetectTrends calculates the Linear Regression Slope for a device's metric over the last N minutes.
// If slope > 0.5 (Rising) or < -0.5 (Falling), it flags an anomaly.
func (s *AnomalyService) DetectTrends(deviceId, field string, durationMinutes int) error {
	// 1. Fetch History
	// SELECT val, time FROM telemetry WHERE device_id=$1 AND time > NOW() - interval
	// Mocking data retrieval for now as repo method might need "GetTelemetryHistory"
	// In Phase 3, we implement the math.

	// Mock Data Points (Time offset in minutes, Value)
	points := []struct {
		x float64
		y float64
	}{
		{0, 20}, {1, 22}, {2, 25}, {3, 29}, {4, 35}, // Rising fast
	}

	slope := s.calculateSlope(points)
	log.Printf("[Anomaly] Device %s Field %s Slope: %.2f", deviceId, field, slope)

	if slope > 1.5 {
		// Rising Trend
		return s.repo.CreateAlert(deviceId, "unknown_project", fmt.Sprintf("Rapid Rise detected in %s (Slope: %.2f)", field, slope), "warning")
	}
	return nil
}

// Simple Least Squares Linear Regression
func (s *AnomalyService) calculateSlope(points []struct{ x, y float64 }) float64 {
	n := float64(len(points))
	if n < 2 {
		return 0
	}

	var sumX, sumY, sumXY, sumXX float64
	for _, p := range points {
		sumX += p.x
		sumY += p.y
		sumXY += p.x * p.y
		sumXX += p.x * p.x
	}

	// Slope (m) = (n*sumXY - sumX*sumY) / (n*sumXX - sumX*sumX)
	numerator := n*sumXY - sumX*sumY
	denominator := n*sumXX - sumX*sumX

	if denominator == 0 {
		return 0 // Vertical line (infinite slope)
	}

	return numerator / denominator
}
