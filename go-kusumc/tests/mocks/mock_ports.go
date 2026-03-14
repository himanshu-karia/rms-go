package mocks

import (
	"time"

	"ingestion-go/internal/core/domain"
)

// --- MockStateStore ---
type MockStateStore struct {
	AcquireLockFunc      func(key string, ttlSeconds int) (bool, error)
	PushPacketFunc       func(deviceId string, packet interface{}) error
	GetPacketsFunc       func(deviceId string) ([]interface{}, error)
	GetProjectConfigFunc func(projectId string) (interface{}, bool)
	SetProjectConfigFunc func(projectId string, config interface{}) error
	GetConfigBundleFunc  func(projectId string) (map[string]interface{}, bool)
	GetDeviceShadowFunc  func(deviceId string) (map[string]interface{}, error)
}

type MockArchiver struct {
	RestoreDataFunc func(start, end time.Time, projectId string) (string, error)
}

func (m *MockArchiver) RestoreData(start, end time.Time, projectId string) (string, error) {
	if m.RestoreDataFunc != nil {
		return m.RestoreDataFunc(start, end, projectId)
	}
	return "mock_table", nil
}

func (m *MockStateStore) AcquireLock(key string, ttlSeconds int) (bool, error) {
	if m.AcquireLockFunc != nil {
		return m.AcquireLockFunc(key, ttlSeconds)
	}
	return true, nil
}
func (m *MockStateStore) PushPacket(deviceId string, packet interface{}) error {
	if m.PushPacketFunc != nil {
		return m.PushPacketFunc(deviceId, packet)
	}
	return nil
}
func (m *MockStateStore) GetPackets(deviceId string) ([]interface{}, error) {
	if m.GetPacketsFunc != nil {
		return m.GetPacketsFunc(deviceId)
	}
	return nil, nil
}
func (m *MockStateStore) GetProjectConfig(projectId string) (interface{}, bool) {
	if m.GetProjectConfigFunc != nil {
		return m.GetProjectConfigFunc(projectId)
	}
	return nil, false
}
func (m *MockStateStore) SetProjectConfig(projectId string, config interface{}) error {
	if m.SetProjectConfigFunc != nil {
		return m.SetProjectConfigFunc(projectId, config)
	}
	return nil
}
func (m *MockStateStore) GetConfigBundle(projectId string) (map[string]interface{}, bool) {
	if m.GetConfigBundleFunc != nil {
		return m.GetConfigBundleFunc(projectId)
	}
	return nil, false
}
func (m *MockStateStore) GetDeviceShadow(deviceId string) (map[string]interface{}, error) {
	if m.GetDeviceShadowFunc != nil {
		return m.GetDeviceShadowFunc(deviceId)
	}
	return nil, nil
}

// --- MockTelemetryRepo ---
type MockTelemetryRepo struct {
	SaveTelemetryFunc                         func(telemetry interface{}) error
	SaveBatchFunc                             func(batch []interface{}) error
	UpdateDeviceShadowFunc                    func(imei string, reported map[string]interface{}) error
	GetTelemetryHistoryFromTableFunc          func(table, deviceOrIMEI, packetType string, start, end time.Time, limit, offset int) ([]map[string]interface{}, error)
	GetTelemetryHistoryByProjectFromTableFunc func(table string, start, end time.Time, projectId, packetType, quality, excludeQuality string, limit, offset int) ([]map[string]interface{}, int, error)
	GetLatestTelemetryFunc                    func(deviceOrIMEI, packetType string) (map[string]interface{}, error)
	GetAnomaliesFunc                          func(limit int) ([]map[string]interface{}, error)
}

func (m *MockTelemetryRepo) SaveTelemetry(telemetry interface{}) error {
	if m.SaveTelemetryFunc != nil {
		return m.SaveTelemetryFunc(telemetry)
	}
	return nil
}
func (m *MockTelemetryRepo) SaveBatch(batch []interface{}) error {
	if m.SaveBatchFunc != nil {
		return m.SaveBatchFunc(batch)
	}
	return nil
}
func (m *MockTelemetryRepo) UpdateDeviceShadow(imei string, reported map[string]interface{}) error {
	if m.UpdateDeviceShadowFunc != nil {
		return m.UpdateDeviceShadowFunc(imei, reported)
	}
	return nil
}
func (m *MockTelemetryRepo) GetTelemetryHistoryFromTable(table, deviceOrIMEI, packetType string, start, end time.Time, limit, offset int) ([]map[string]interface{}, error) {
	if m.GetTelemetryHistoryFromTableFunc != nil {
		return m.GetTelemetryHistoryFromTableFunc(table, deviceOrIMEI, packetType, start, end, limit, offset)
	}
	return nil, nil
}
func (m *MockTelemetryRepo) GetTelemetryHistoryByProjectFromTable(table string, start, end time.Time, projectId, packetType, quality, excludeQuality string, limit, offset int) ([]map[string]interface{}, int, error) {
	if m.GetTelemetryHistoryByProjectFromTableFunc != nil {
		return m.GetTelemetryHistoryByProjectFromTableFunc(table, start, end, projectId, packetType, quality, excludeQuality, limit, offset)
	}
	return nil, 0, nil
}
func (m *MockTelemetryRepo) GetLatestTelemetry(deviceOrIMEI, packetType string) (map[string]interface{}, error) {
	if m.GetLatestTelemetryFunc != nil {
		return m.GetLatestTelemetryFunc(deviceOrIMEI, packetType)
	}
	return nil, nil
}
func (m *MockTelemetryRepo) GetAnomalies(limit int) ([]map[string]interface{}, error) {
	if m.GetAnomaliesFunc != nil {
		return m.GetAnomaliesFunc(limit)
	}
	return nil, nil
}

// --- MockTransformer ---
type MockTransformer struct {
	ApplyFunc func(raw map[string]interface{}, config interface{}) (map[string]interface{}, error)
}

func (m *MockTransformer) Apply(raw map[string]interface{}, config interface{}) (map[string]interface{}, error) {
	if m.ApplyFunc != nil {
		return m.ApplyFunc(raw, config)
	}
	return raw, nil // Identity transform
}

// --- MockDeviceRepo ---
type MockDeviceRepo struct {
	GetDeviceByIMEIFunc            func(imei string) (interface{}, error)
	GetDeviceByIDFunc              func(id string) (map[string]interface{}, error)
	GetDeviceByIDOrIMEIFunc        func(idOrIMEI string) (map[string]interface{}, error)
	ListDevicesFunc                func(projectId string, search string, status string, includeInactive bool, limit int, offset int) ([]map[string]interface{}, int, error)
	UpdateDeviceByIDOrIMEIFunc     func(idOrIMEI string, name *string, status *string, projectId *string, attrsPatch map[string]interface{}) (map[string]interface{}, error)
	CreateDeviceStructFunc         func(projectId, name, imei string, mqttBundle map[string]interface{}, attrs map[string]interface{}) (string, error)
	SoftDeleteDeviceFunc           func(idOrIMEI string) error
	InsertCredentialHistoryFunc    func(deviceID string, bundle map[string]interface{}) (string, error)
	ListCredentialHistoryFunc      func(deviceID string) ([]map[string]interface{}, error)
	GetAutomationFlowFunc          func(projectId string) (map[string]interface{}, error)
	GetInstallationByDeviceFunc    func(deviceId string) (map[string]interface{}, error)
	GetBeneficiaryFunc             func(id string) (map[string]interface{}, error)
	CreateMqttProvisioningJobFunc  func(deviceId string, credHistoryId *string) error
	GetLatestCredentialHistoryFunc func(deviceId string) (map[string]interface{}, error)
	GetPendingCommandsFunc         func(deviceId string) ([]map[string]interface{}, error)
}

func (m *MockDeviceRepo) GetDeviceByIMEI(imei string) (interface{}, error) {
	if m.GetDeviceByIMEIFunc != nil {
		return m.GetDeviceByIMEIFunc(imei)
	}
	return nil, nil
}
func (m *MockDeviceRepo) GetDeviceByID(id string) (map[string]interface{}, error) {
	if m.GetDeviceByIDFunc != nil {
		return m.GetDeviceByIDFunc(id)
	}
	return nil, nil
}
func (m *MockDeviceRepo) GetDeviceByIDOrIMEI(idOrIMEI string) (map[string]interface{}, error) {
	if m.GetDeviceByIDOrIMEIFunc != nil {
		return m.GetDeviceByIDOrIMEIFunc(idOrIMEI)
	}
	return nil, nil
}
func (m *MockDeviceRepo) ListDevices(projectId string, search string, status string, includeInactive bool, limit int, offset int) ([]map[string]interface{}, int, error) {
	if m.ListDevicesFunc != nil {
		return m.ListDevicesFunc(projectId, search, status, includeInactive, limit, offset)
	}
	return nil, 0, nil
}
func (m *MockDeviceRepo) UpdateDeviceByIDOrIMEI(idOrIMEI string, name *string, status *string, projectId *string, attrsPatch map[string]interface{}) (map[string]interface{}, error) {
	if m.UpdateDeviceByIDOrIMEIFunc != nil {
		return m.UpdateDeviceByIDOrIMEIFunc(idOrIMEI, name, status, projectId, attrsPatch)
	}
	return nil, nil
}
func (m *MockDeviceRepo) CreateDeviceStruct(projectId, name, imei string, mqttBundle map[string]interface{}, attrs map[string]interface{}) (string, error) {
	if m.CreateDeviceStructFunc != nil {
		return m.CreateDeviceStructFunc(projectId, name, imei, mqttBundle, attrs)
	}
	return "", nil
}
func (m *MockDeviceRepo) SoftDeleteDevice(idOrIMEI string) error {
	if m.SoftDeleteDeviceFunc != nil {
		return m.SoftDeleteDeviceFunc(idOrIMEI)
	}
	return nil
}
func (m *MockDeviceRepo) InsertCredentialHistory(deviceID string, bundle map[string]interface{}) (string, error) {
	if m.InsertCredentialHistoryFunc != nil {
		return m.InsertCredentialHistoryFunc(deviceID, bundle)
	}
	return "", nil
}
func (m *MockDeviceRepo) ListCredentialHistory(deviceID string) ([]map[string]interface{}, error) {
	if m.ListCredentialHistoryFunc != nil {
		return m.ListCredentialHistoryFunc(deviceID)
	}
	return nil, nil
}
func (m *MockDeviceRepo) GetAutomationFlow(projectId string) (map[string]interface{}, error) {
	if m.GetAutomationFlowFunc != nil {
		return m.GetAutomationFlowFunc(projectId)
	}
	return nil, nil
}
func (m *MockDeviceRepo) GetInstallationByDevice(deviceId string) (map[string]interface{}, error) {
	if m.GetInstallationByDeviceFunc != nil {
		return m.GetInstallationByDeviceFunc(deviceId)
	}
	return nil, nil
}
func (m *MockDeviceRepo) GetBeneficiary(id string) (map[string]interface{}, error) {
	if m.GetBeneficiaryFunc != nil {
		return m.GetBeneficiaryFunc(id)
	}
	return nil, nil
}
func (m *MockDeviceRepo) CreateMqttProvisioningJob(deviceId string, credHistoryId *string) error {
	if m.CreateMqttProvisioningJobFunc != nil {
		return m.CreateMqttProvisioningJobFunc(deviceId, credHistoryId)
	}
	return nil
}
func (m *MockDeviceRepo) GetLatestCredentialHistory(deviceId string) (map[string]interface{}, error) {
	if m.GetLatestCredentialHistoryFunc != nil {
		return m.GetLatestCredentialHistoryFunc(deviceId)
	}
	return nil, nil
}
func (m *MockDeviceRepo) GetPendingCommands(deviceId string) ([]map[string]interface{}, error) {
	if m.GetPendingCommandsFunc != nil {
		return m.GetPendingCommandsFunc(deviceId)
	}
	return nil, nil
}

// --- MockCommandRepo ---
type MockCommandRepo struct {
	ListCommandsForDeviceFunc          func(deviceID, projectID string, protocolID, modelID *string) ([]domain.CommandCatalog, error)
	GetCommandByIDFunc                 func(commandID string) (*domain.CommandCatalog, error)
	UpsertCommandCatalogFunc           func(rec domain.CommandCatalog) (string, error)
	DeleteCommandCatalogFunc           func(commandID string) error
	UpsertDeviceCapabilitiesFunc       func(commandID string, deviceIDs []string) error
	GetCommandRequestByCorrelationFunc func(correlationID string) (*domain.CommandRequest, error)
	GetResponsePatternsFunc            func(commandID string) ([]domain.ResponsePattern, error)
	InsertCommandRequestFunc           func(req domain.CommandRequest) (string, error)
	UpdateCommandRequestStatusFunc     func(correlationID, status string, publishedAt, completedAt *time.Time, retries *int) error
	SaveCommandResponseFunc            func(resp domain.CommandResponse) error
	ListCommandRequestsFunc            func(deviceID string, limit int) ([]domain.CommandRequest, error)
	ListCommandResponsesFunc           func(deviceID string, limit int) ([]domain.CommandResponse, error)
	ListPendingRetriesFunc             func(cutoff time.Time, limit int) ([]domain.CommandRequest, error)
	GetCommandStatsFunc                func(deviceID string, cutoff time.Time) (domain.CommandStats, error)
}

func (m *MockCommandRepo) ListCommandsForDevice(deviceID, projectID string, protocolID, modelID *string) ([]domain.CommandCatalog, error) {
	if m.ListCommandsForDeviceFunc != nil {
		return m.ListCommandsForDeviceFunc(deviceID, projectID, protocolID, modelID)
	}
	return nil, nil
}

func (m *MockCommandRepo) GetCommandByID(commandID string) (*domain.CommandCatalog, error) {
	if m.GetCommandByIDFunc != nil {
		return m.GetCommandByIDFunc(commandID)
	}
	return nil, nil
}

func (m *MockCommandRepo) UpsertCommandCatalog(rec domain.CommandCatalog) (string, error) {
	if m.UpsertCommandCatalogFunc != nil {
		return m.UpsertCommandCatalogFunc(rec)
	}
	return rec.ID, nil
}

func (m *MockCommandRepo) DeleteCommandCatalog(commandID string) error {
	if m.DeleteCommandCatalogFunc != nil {
		return m.DeleteCommandCatalogFunc(commandID)
	}
	return nil
}

func (m *MockCommandRepo) UpsertDeviceCapabilities(commandID string, deviceIDs []string) error {
	if m.UpsertDeviceCapabilitiesFunc != nil {
		return m.UpsertDeviceCapabilitiesFunc(commandID, deviceIDs)
	}
	return nil
}

func (m *MockCommandRepo) GetCommandRequestByCorrelation(correlationID string) (*domain.CommandRequest, error) {
	if m.GetCommandRequestByCorrelationFunc != nil {
		return m.GetCommandRequestByCorrelationFunc(correlationID)
	}
	return nil, nil
}

func (m *MockCommandRepo) GetResponsePatterns(commandID string) ([]domain.ResponsePattern, error) {
	if m.GetResponsePatternsFunc != nil {
		return m.GetResponsePatternsFunc(commandID)
	}
	return nil, nil
}

func (m *MockCommandRepo) InsertCommandRequest(req domain.CommandRequest) (string, error) {
	if m.InsertCommandRequestFunc != nil {
		return m.InsertCommandRequestFunc(req)
	}
	return req.ID, nil
}

func (m *MockCommandRepo) UpdateCommandRequestStatus(correlationID, status string, publishedAt, completedAt *time.Time, retries *int) error {
	if m.UpdateCommandRequestStatusFunc != nil {
		return m.UpdateCommandRequestStatusFunc(correlationID, status, publishedAt, completedAt, retries)
	}
	return nil
}

func (m *MockCommandRepo) SaveCommandResponse(resp domain.CommandResponse) error {
	if m.SaveCommandResponseFunc != nil {
		return m.SaveCommandResponseFunc(resp)
	}
	return nil
}

func (m *MockCommandRepo) ListCommandRequests(deviceID string, limit int) ([]domain.CommandRequest, error) {
	if m.ListCommandRequestsFunc != nil {
		return m.ListCommandRequestsFunc(deviceID, limit)
	}
	return nil, nil
}

func (m *MockCommandRepo) ListCommandResponses(deviceID string, limit int) ([]domain.CommandResponse, error) {
	if m.ListCommandResponsesFunc != nil {
		return m.ListCommandResponsesFunc(deviceID, limit)
	}
	return nil, nil
}

func (m *MockCommandRepo) ListPendingRetries(cutoff time.Time, limit int) ([]domain.CommandRequest, error) {
	if m.ListPendingRetriesFunc != nil {
		return m.ListPendingRetriesFunc(cutoff, limit)
	}
	return nil, nil
}

func (m *MockCommandRepo) GetCommandStats(deviceID string, cutoff time.Time) (domain.CommandStats, error) {
	if m.GetCommandStatsFunc != nil {
		return m.GetCommandStatsFunc(deviceID, cutoff)
	}
	return domain.CommandStats{}, nil
}
