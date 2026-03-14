package ports

import (
	"time"

	"ingestion-go/internal/core/domain"
)

// StateStore handles ephemeral, high-speed state (Redis/Memory)
type StateStore interface {
	// Deduplication
	AcquireLock(key string, ttlSeconds int) (bool, error)

	// Hot Cache (Last 50 packets)
	PushPacket(deviceId string, packet interface{}) error
	GetPackets(deviceId string) ([]interface{}, error)

	// Phase 10: Virtual Sensor Support
	GetProjectConfig(projectId string) (interface{}, bool)
	SetProjectConfig(projectId string, config interface{}) error

	// Config bundles (feature-flagged, merged payload schemas/thresholds/rules/automation)
	GetConfigBundle(projectId string) (map[string]interface{}, bool)

	// device shadow
	GetDeviceShadow(deviceId string) (map[string]interface{}, error)
}

type TelemetryRepo interface {
	SaveTelemetry(telemetry interface{}) error
	SaveBatch(batch []interface{}) error
	UpdateDeviceShadow(imei string, reported map[string]interface{}) error
	GetTelemetryHistoryFromTable(table, deviceOrIMEI, packetType string, start, end time.Time, limit, offset int) ([]map[string]interface{}, error)
	GetTelemetryHistoryByProjectFromTable(table string, start, end time.Time, projectId, packetType, quality, excludeQuality string, limit, offset int) ([]map[string]interface{}, int, error)
	GetLatestTelemetry(deviceOrIMEI, packetType string) (map[string]interface{}, error)
	GetAnomalies(limit int) ([]map[string]interface{}, error)
}

type DeviceRepo interface {
	GetDeviceByIMEI(imei string) (interface{}, error)
	GetDeviceByID(id string) (map[string]interface{}, error)
	GetDeviceByIDOrIMEI(idOrIMEI string) (map[string]interface{}, error)
	ListDevices(projectId string, search string, status string, includeInactive bool, limit int, offset int) ([]map[string]interface{}, int, error)
	UpdateDeviceByIDOrIMEI(idOrIMEI string, name *string, status *string, projectId *string, attrsPatch map[string]interface{}) (map[string]interface{}, error)
	CreateDeviceStruct(projectId, name, imei string, mqttBundle map[string]interface{}, attrs map[string]interface{}) (string, error)
	SoftDeleteDevice(idOrIMEI string) error
	InsertCredentialHistory(deviceID string, bundle map[string]interface{}) (string, error)
	GetAutomationFlow(projectId string) (map[string]interface{}, error)
	GetInstallationByDevice(deviceId string) (map[string]interface{}, error)
	GetBeneficiary(id string) (map[string]interface{}, error)
	CreateMqttProvisioningJob(deviceId string, credHistoryId *string) error
	GetLatestCredentialHistory(deviceId string) (map[string]interface{}, error)
	ListCredentialHistory(deviceId string) ([]map[string]interface{}, error)
	GetPendingCommands(deviceId string) ([]map[string]interface{}, error)
}

// CommandRepo handles catalog, requests, and responses for command/control.
type CommandRepo interface {
	// Catalog & capabilities
	ListCommandsForDevice(deviceID, projectID string, protocolID, modelID *string) ([]domain.CommandCatalog, error)
	GetCommandByID(commandID string) (*domain.CommandCatalog, error)
	UpsertCommandCatalog(rec domain.CommandCatalog) (string, error)
	DeleteCommandCatalog(commandID string) error
	UpsertDeviceCapabilities(commandID string, deviceIDs []string) error
	GetCommandRequestByCorrelation(correlationID string) (*domain.CommandRequest, error)
	GetResponsePatterns(commandID string) ([]domain.ResponsePattern, error)

	// Requests & responses
	InsertCommandRequest(req domain.CommandRequest) (string, error)
	UpdateCommandRequestStatus(correlationID, status string, publishedAt, completedAt *time.Time, retries *int) error
	SaveCommandResponse(resp domain.CommandResponse) error
	ListCommandRequests(deviceID string, limit int) ([]domain.CommandRequest, error)
	ListCommandResponses(deviceID string, limit int) ([]domain.CommandResponse, error)
	ListPendingRetries(cutoff time.Time, limit int) ([]domain.CommandRequest, error)
	GetCommandStats(deviceID string, cutoff time.Time) (domain.CommandStats, error)
}

type Transformer interface {
	Apply(raw map[string]interface{}, config interface{}) (map[string]interface{}, error)
}

type Archiver interface {
	RestoreData(start, end time.Time, projectId string) (string, error)
}
