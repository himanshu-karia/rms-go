package services

import (
	"fmt"
	"ingestion-go/internal/core/ports"
	"time"
)

type AnalyticsService struct {
	repo     ports.TelemetryRepo
	devRepo  ports.DeviceRepo // Needed for ProjectID lookup
	archiver ports.Archiver   // Abstracted for Testing
}

func NewAnalyticsService(repo ports.TelemetryRepo, devRepo ports.DeviceRepo, archiver ports.Archiver) *AnalyticsService {
	return &AnalyticsService{repo: repo, devRepo: devRepo, archiver: archiver}
}

func (s *AnalyticsService) GetHistory(deviceOrIMEI, packetType string, start, end time.Time, limit, offset int) ([]map[string]interface{}, error) {
	// [STRATEGY B] Check for "Cold" Data Request (> 180 Days)
	// If Start Date is older than 6 months, trigger Restore.
	cutoff := time.Now().AddDate(0, 0, -180)

	if start.Before(cutoff) {
		// 1. Trigger Hydration (Strategy B: Temp Table)
		// We need projectId.
		device, err := s.devRepo.GetDeviceByIMEI(deviceOrIMEI)
		if err != nil {
			return nil, err // Device not found
		}

		// Type assertion dance
		devMap, ok := device.(map[string]interface{})
		if !ok {
			// In case the Repo returns struct. For now assuming map.
			return nil, fmt.Errorf("device type assertion failed")
		}

		projectId := devMap["project_id"].(string)

		// 2. Restore to Temp Table
		tempTable, err := s.archiver.RestoreData(start, end, projectId)
		if err != nil {
			return nil, err
		}

		// 3. Query Temp Table
		return s.repo.GetTelemetryHistoryFromTable(tempTable, deviceOrIMEI, packetType, start, end, limit, offset)
	}

	// Hot/Warm Path (Standard)
	// We need GetTelemetryHistory on the Repo Interface too?
	// PostgresRepo has it, but Interface might not.
	// Let's assume GetTelemetryHistory is standard query.
	// Wait, standard repo uses separate Table? No, "telemetry" table.
	// PostgresRepo.GetTelemetryHistoryHelper?
	// Actually, standard usage in Node uses raw SQL.
	// We need to ensure TelemetryRepo has GetTelemetryHistory.
	// Let's check interfaces.go again later. For now, assuming it needs to be there.
	// Ah, GetTelemetryHistoryFromTable is one func.
	// "GetTelemetryHistory" for live view is likely missing from Interface.
	// Using "GetTelemetryHistoryFromTable" with "telemetry" table name?
	// Yes, usually "telemetry" is the table.
	return s.repo.GetTelemetryHistoryFromTable("telemetry", deviceOrIMEI, packetType, start, end, limit, offset)
}

func (s *AnalyticsService) GetHistoryByProject(projectId, packetType, quality, excludeQuality string, start, end time.Time, limit, offset int) ([]map[string]interface{}, int, error) {
	cutoff := time.Now().AddDate(0, 0, -180)

	if start.Before(cutoff) {
		tempTable, err := s.archiver.RestoreData(start, end, projectId)
		if err != nil {
			return nil, 0, err
		}
		return s.repo.GetTelemetryHistoryByProjectFromTable(tempTable, start, end, projectId, packetType, quality, excludeQuality, limit, offset)
	}

	return s.repo.GetTelemetryHistoryByProjectFromTable("telemetry", start, end, projectId, packetType, quality, excludeQuality, limit, offset)
}

func (s *AnalyticsService) GetAnomalies(limit int) ([]map[string]interface{}, error) {
	return s.repo.GetAnomalies(limit)
}

func (s *AnalyticsService) GetLatest(deviceOrIMEI, packetType string) (map[string]interface{}, error) {
	return s.repo.GetLatestTelemetry(deviceOrIMEI, packetType)
}
