package services

import (
	"ingestion-go/internal/adapters/secondary"
	"time"
)

type QueryService struct {
	repo *secondary.PostgresRepo
}

func NewQueryService(repo *secondary.PostgresRepo) *QueryService {
	return &QueryService{repo: repo}
}

// GetHistory fetches time-series data
func (s *QueryService) GetHistory(imei string, start, end time.Time, limit int) ([]map[string]interface{}, error) {
	// 1. Resolve IMEI to UUID (Skipped for V1, assuming query by IMEI Join)
	// Actually our Telemetry table uses UUID.
	// Query: SELECT t.time, t.data FROM telemetry t JOIN devices d ON t.device_id = d.id WHERE d.imei = $1 ...

	return s.repo.QueryTelemetryByIMEI(imei, start, end, limit)
}

// -- Add to PostgresRepo --
// func (r *PostgresRepo) QueryTelemetryByIMEI(...)
