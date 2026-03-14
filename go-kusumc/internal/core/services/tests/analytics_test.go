package services_test

import (
	"errors"
	"ingestion-go/internal/core/services"
	"ingestion-go/tests/mocks"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAnalytics_GetHistory_Hot(t *testing.T) {
	// Setup Mocks
	mockRepo := &mocks.MockTelemetryRepo{}
	mockDevRepo := &mocks.MockDeviceRepo{}
	mockArchiver := &mocks.MockArchiver{}

	// Service under test
	svc := services.NewAnalyticsService(mockRepo, mockDevRepo, mockArchiver)

	// Defines
	imei := "123456789012345"
	now := time.Now()
	start := now.Add(-24 * time.Hour) // 1 day ago (Warm)
	end := now

	// Mock Expectations
	mockRepo.GetTelemetryHistoryFromTableFunc = func(table, deviceOrIMEI, packetType string, s, e time.Time, l, o int) ([]map[string]interface{}, error) {
		assert.Equal(t, "telemetry", table) // Default table
		assert.Equal(t, "", packetType)
		return []map[string]interface{}{{"temp": 25.0}}, nil
	}

	// Execution
	result, err := svc.GetHistory(imei, "", start, end, 10, 0)

	// Verification
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, 25.0, result[0]["temp"])
}

func TestAnalytics_GetHistory_Cold(t *testing.T) {
	// Setup Mocks
	mockRepo := &mocks.MockTelemetryRepo{}
	mockDevRepo := &mocks.MockDeviceRepo{}
	mockArchiver := &mocks.MockArchiver{}

	svc := services.NewAnalyticsService(mockRepo, mockDevRepo, mockArchiver)

	// Defines
	imei := "123456789012345"
	now := time.Now()
	start := now.AddDate(0, 0, -200) // 200 days ago (Cold)
	end := now.AddDate(0, 0, -199)
	projId := "proj_alpha"
	tempTable := "telemetry_temp_123"

	// Mock Expectations
	// 1. Get Device (for ProjectID)
	mockDevRepo.GetDeviceByIMEIFunc = func(i string) (interface{}, error) {
		return map[string]interface{}{"project_id": projId}, nil
	}

	// 2. Archive Restore
	mockArchiver.RestoreDataFunc = func(s, e time.Time, pid string) (string, error) {
		assert.Equal(t, projId, pid)
		return tempTable, nil
	}

	// 3. Query Temp Table
	mockRepo.GetTelemetryHistoryFromTableFunc = func(table, deviceOrIMEI, packetType string, s, e time.Time, l, o int) ([]map[string]interface{}, error) {
		assert.Equal(t, tempTable, table)
		assert.Equal(t, "", packetType)
		return []map[string]interface{}{{"temp": 10.0}}, nil
	}

	// Execution
	result, err := svc.GetHistory(imei, "", start, end, 10, 0)

	// Verification
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, 10.0, result[0]["temp"])
}

func TestAnalytics_GetHistory_DeviceNotFound(t *testing.T) {
	mockRepo := &mocks.MockTelemetryRepo{}
	mockDevRepo := &mocks.MockDeviceRepo{}
	mockArchiver := &mocks.MockArchiver{}

	svc := services.NewAnalyticsService(mockRepo, mockDevRepo, mockArchiver)

	start := time.Now().AddDate(0, 0, -200)
	end := time.Now()

	mockDevRepo.GetDeviceByIMEIFunc = func(i string) (interface{}, error) {
		return nil, errors.New("device not found")
	}

	result, err := svc.GetHistory("bad_imei", "", start, end, 10, 0)
	assert.Error(t, err)
	assert.Nil(t, result)
}
