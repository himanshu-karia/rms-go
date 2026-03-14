package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"ingestion-go/internal/core/domain"
	"ingestion-go/internal/core/ports"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
)

type CommandsService struct {
	repo    ports.CommandRepo
	devices ports.DeviceRepo
	mqtt    mqttPublisher
	http    *http.Client
	httpURL string
}

// mqttPublisher is the minimal subset we need for publishing commands.
type mqttPublisher interface {
	Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token
}

func NewCommandsService(repo ports.CommandRepo, devices ports.DeviceRepo) *CommandsService {
	return &CommandsService{
		repo:    repo,
		devices: devices,
		http:    &http.Client{Timeout: 8 * time.Second},
		httpURL: strings.TrimSpace(os.Getenv("COMMAND_HTTP_ENDPOINT")),
	}
}

// SetMqttClient lets composition root wire the shared MQTT client.
func (s *CommandsService) SetMqttClient(client mqttPublisher) {
	s.mqtt = client
}

// SetHTTPClient wires the HTTP transport endpoint for command dispatch.
func (s *CommandsService) SetHTTPClient(client *http.Client, endpoint string) {
	if client != nil {
		s.http = client
	}
	if strings.TrimSpace(endpoint) != "" {
		s.httpURL = strings.TrimSpace(endpoint)
	}
}

func (s *CommandsService) resolveDevice(deviceRef, projectOverride string) (deviceID, projectID, imei string, err error) {
	dev, derr := s.devices.GetDeviceByIDOrIMEI(deviceRef)
	if derr != nil {
		err = fmt.Errorf("device lookup failed: %w", derr)
		return
	}
	if dev == nil {
		err = fmt.Errorf("device not found")
		return
	}
	if val, ok := dev["id"].(string); ok {
		deviceID = val
	}
	if val, ok := dev["imei"].(string); ok {
		imei = val
	}
	projectID = projectOverride
	if projectID == "" {
		if pid, ok := dev["project_id"].(string); ok {
			projectID = pid
		}
	}
	if strings.TrimSpace(deviceID) == "" || strings.TrimSpace(projectID) == "" || strings.TrimSpace(imei) == "" {
		err = fmt.Errorf("device metadata incomplete")
	}
	return
}

func (s *CommandsService) ListCatalog(deviceRef, projectOverride string) ([]domain.CommandCatalog, error) {
	deviceID, projectID, _, err := s.resolveDevice(deviceRef, projectOverride)
	if err != nil {
		return nil, err
	}
	// Protocol/model scoping can be added once those attributes exist on device/model tables.
	return s.repo.ListCommandsForDevice(deviceID, projectID, nil, nil)
}

// UpsertCatalog creates or updates a command definition and optionally assigns device capabilities.
func (s *CommandsService) UpsertCatalog(rec domain.CommandCatalog, deviceIDs []string) (string, error) {
	if strings.TrimSpace(rec.Name) == "" {
		return "", fmt.Errorf("name is required")
	}
	if strings.TrimSpace(rec.Scope) == "" {
		rec.Scope = "project"
	}
	if rec.Transport == "" {
		rec.Transport = "mqtt"
	}
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now()
	}
	id, err := s.repo.UpsertCommandCatalog(rec)
	if err != nil {
		return "", err
	}
	if err := s.repo.UpsertDeviceCapabilities(id, deviceIDs); err != nil {
		return "", err
	}
	return id, nil
}

func (s *CommandsService) DeleteCatalog(commandID string) error {
	if strings.TrimSpace(commandID) == "" {
		return fmt.Errorf("commandID required")
	}
	return s.repo.DeleteCommandCatalog(commandID)
}

func (s *CommandsService) SendCommand(deviceRef, commandID, projectOverride string, payload map[string]any) (*domain.CommandRequest, error) {
	deviceID, projectID, imei, err := s.resolveDevice(deviceRef, projectOverride)
	if err != nil {
		return nil, err
	}

	cmd, err := s.repo.GetCommandByID(commandID)
	if err != nil {
		return nil, fmt.Errorf("unknown command: %w", err)
	}
	transport := strings.ToLower(cmd.Transport)

	allowed, err := s.repo.ListCommandsForDevice(deviceID, projectID, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("list commands: %w", err)
	}
	if !containsCommand(allowed, commandID) {
		return nil, fmt.Errorf("command not allowed for device")
	}

	correlationID := uuid.New().String()
	req := domain.CommandRequest{
		DeviceID:      deviceID,
		ProjectID:     projectID,
		CommandID:     commandID,
		Payload:       payload,
		Status:        "queued",
		Retries:       0,
		CorrelationID: correlationID,
		CreatedAt:     time.Now(),
	}

	if id, err := s.repo.InsertCommandRequest(req); err != nil {
		return nil, fmt.Errorf("persist command: %w", err)
	} else {
		req.ID = id
	}

	var publishErr error

	switch transport {
	case "http":
		publishErr = s.publishHTTP(req)
	case "mqtt", "":
		publishErr = s.publishMQTT(req, imei, projectID, cmd.Name)
	default:
		publishErr = fmt.Errorf("unsupported transport: %s", transport)
	}

	if publishErr != nil {
		req.Status = "failed"
		_ = s.repo.UpdateCommandRequestStatus(correlationID, req.Status, nil, nil, nil)
		return &req, fmt.Errorf("publish command: %w", publishErr)
	}

	now := time.Now()
	req.Status = "published"
	req.PublishedAt = &now
	_ = s.repo.UpdateCommandRequestStatus(correlationID, req.Status, &now, nil, nil)

	return &req, nil
}

// SendCommandWithCorrelation publishes a command using a caller-supplied correlation ID.
// This is useful when a higher-level workflow already has a durable ID (e.g. device_configurations.id)
// and wants device responses to correlate back to that record.
func (s *CommandsService) SendCommandWithCorrelation(
	deviceRef, commandID, projectOverride, correlationID string,
	payload map[string]any,
) (*domain.CommandRequest, error) {
	deviceID, projectID, imei, err := s.resolveDevice(deviceRef, projectOverride)
	if err != nil {
		return nil, err
	}

	correlationID = strings.TrimSpace(correlationID)
	if correlationID == "" {
		return nil, fmt.Errorf("correlation_id is required")
	}
	if _, err := uuid.Parse(correlationID); err != nil {
		return nil, fmt.Errorf("invalid correlation_id")
	}

	cmd, err := s.repo.GetCommandByID(commandID)
	if err != nil {
		return nil, fmt.Errorf("unknown command: %w", err)
	}
	transport := strings.ToLower(cmd.Transport)

	allowed, err := s.repo.ListCommandsForDevice(deviceID, projectID, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("list commands: %w", err)
	}
	if !containsCommand(allowed, commandID) {
		return nil, fmt.Errorf("command not allowed for device")
	}

	req := domain.CommandRequest{
		DeviceID:      deviceID,
		ProjectID:     projectID,
		CommandID:     commandID,
		Payload:       payload,
		Status:        "queued",
		Retries:       0,
		CorrelationID: correlationID,
		CreatedAt:     time.Now(),
	}

	if id, err := s.repo.InsertCommandRequest(req); err != nil {
		// Idempotency: if correlation already exists, return the existing request.
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "duplicate") || strings.Contains(msg, "unique") {
			if existing, getErr := s.repo.GetCommandRequestByCorrelation(correlationID); getErr == nil && existing != nil {
				return existing, nil
			}
		}
		return nil, fmt.Errorf("persist command: %w", err)
	} else {
		req.ID = id
	}

	var publishErr error

	switch transport {
	case "http":
		publishErr = s.publishHTTP(req)
	case "mqtt", "":
		publishErr = s.publishMQTT(req, imei, projectID, cmd.Name)
	default:
		publishErr = fmt.Errorf("unsupported transport: %s", transport)
	}

	if publishErr != nil {
		req.Status = "failed"
		_ = s.repo.UpdateCommandRequestStatus(correlationID, req.Status, nil, nil, nil)
		return &req, fmt.Errorf("publish command: %w", publishErr)
	}

	now := time.Now()
	req.Status = "published"
	req.PublishedAt = &now
	_ = s.repo.UpdateCommandRequestStatus(correlationID, req.Status, &now, nil, nil)

	return &req, nil
}

func (s *CommandsService) ListHistory(deviceRef string, limit int) ([]domain.CommandRequest, error) {
	deviceID, _, _, err := s.resolveDevice(deviceRef, "")
	if err != nil {
		return nil, err
	}
	return s.repo.ListCommandRequests(deviceID, limit)
}

func (s *CommandsService) ListResponses(deviceRef string, limit int) ([]domain.CommandResponse, error) {
	deviceID, _, _, err := s.resolveDevice(deviceRef, "")
	if err != nil {
		return nil, err
	}
	return s.repo.ListCommandResponses(deviceID, limit)
}

// GetStats returns aggregated command counters and retry posture for a device.
func (s *CommandsService) GetStats(deviceRef, projectOverride string) (domain.CommandStats, error) {
	deviceID, projectID, _, err := s.resolveDevice(deviceRef, projectOverride)
	if err != nil {
		return domain.CommandStats{}, err
	}

	config := commandWorkerConfigFromEnv()
	cutoff := time.Now().Add(-time.Duration(config.AgeSeconds) * time.Second)

	stats, err := s.repo.GetCommandStats(deviceID, cutoff)
	if err != nil {
		return stats, err
	}
	stats.ProjectID = projectID
	stats.WorkerConfig = config
	return stats, nil
}

// AcknowledgeCommand records a command response and updates request status.
func (s *CommandsService) AcknowledgeCommand(deviceRef, correlationID, status string, payload map[string]any, receivedAt *time.Time) (domain.CommandResponse, error) {
	if strings.TrimSpace(correlationID) == "" {
		return domain.CommandResponse{}, fmt.Errorf("correlationId required")
	}
	deviceID, projectID, _, err := s.resolveDevice(deviceRef, "")
	if err != nil {
		return domain.CommandResponse{}, err
	}
	req, err := s.repo.GetCommandRequestByCorrelation(correlationID)
	if err != nil {
		return domain.CommandResponse{}, err
	}
	if req != nil {
		if req.DeviceID != "" {
			deviceID = req.DeviceID
		}
		if req.ProjectID != "" {
			projectID = req.ProjectID
		}
	}
	ackTime := time.Now()
	if receivedAt != nil {
		ackTime = *receivedAt
	}
	resp := domain.CommandResponse{
		CorrelationID: correlationID,
		DeviceID:      deviceID,
		ProjectID:     projectID,
		RawResponse:   payload,
		ReceivedAt:    ackTime,
	}
	if err := s.repo.SaveCommandResponse(resp); err != nil {
		return domain.CommandResponse{}, err
	}
	nextStatus := strings.ToLower(strings.TrimSpace(status))
	if nextStatus == "" {
		nextStatus = "acked"
	}
	_ = s.repo.UpdateCommandRequestStatus(correlationID, nextStatus, nil, &ackTime, nil)
	return resp, nil
}

// RetryPublish republishes an existing command request and increments retries.
func (s *CommandsService) RetryPublish(req domain.CommandRequest) error {
	cmd, err := s.repo.GetCommandByID(req.CommandID)
	if err != nil {
		return fmt.Errorf("command lookup failed: %w", err)
	}

	transport := strings.ToLower(cmd.Transport)

	switch transport {
	case "http":
		if err := s.publishHTTP(req); err != nil {
			retries := req.Retries + 1
			_ = s.repo.UpdateCommandRequestStatus(req.CorrelationID, "failed", nil, nil, &retries)
			return err
		}
		now := time.Now()
		retries := req.Retries + 1
		_ = s.repo.UpdateCommandRequestStatus(req.CorrelationID, "published", &now, nil, &retries)
		return nil
	case "mqtt", "":
		dev, derr := s.devices.GetDeviceByID(req.DeviceID)
		if derr != nil || dev == nil {
			return fmt.Errorf("device lookup failed")
		}
		imei, _ := dev["imei"].(string)
		projectID := req.ProjectID
		if projectID == "" {
			if pid, ok := dev["project_id"].(string); ok {
				projectID = pid
			}
		}
		if strings.TrimSpace(imei) == "" || strings.TrimSpace(projectID) == "" {
			return fmt.Errorf("device metadata incomplete")
		}

		if err := s.publishMQTT(req, imei, projectID, cmd.Name); err != nil {
			retries := req.Retries + 1
			_ = s.repo.UpdateCommandRequestStatus(req.CorrelationID, "failed", nil, nil, &retries)
			return err
		}

		now := time.Now()
		retries := req.Retries + 1
		_ = s.repo.UpdateCommandRequestStatus(req.CorrelationID, "published", &now, nil, &retries)
		return nil
	default:
		return fmt.Errorf("unsupported transport: %s", transport)
	}
}

// RetryByCorrelation loads the request and republishes it, bumping retries.
func (s *CommandsService) RetryByCorrelation(correlationID string) (*domain.CommandRequest, error) {
	req, err := s.repo.GetCommandRequestByCorrelation(correlationID)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, fmt.Errorf("command request not found")
	}

	if err := s.RetryPublish(*req); err != nil {
		return nil, err
	}

	// refresh view after retry bump
	return s.repo.GetCommandRequestByCorrelation(correlationID)
}

func containsCommand(items []domain.CommandCatalog, id string) bool {
	for _, c := range items {
		if c.ID == id {
			return true
		}
	}
	return false
}

func (s *CommandsService) publishMQTT(req domain.CommandRequest, imei, projectID, commandName string) error {
	if s.mqtt == nil {
		return fmt.Errorf("mqtt client not configured")
	}
	name := strings.TrimSpace(commandName)
	if name == "" {
		name = "unknown"
	}

	// Govt/legacy frozen protocol expects OnDemandCommand fields at the top level:
	// { msgid, timestamp, type, cmd, DO1, ... }
	// Avoid adding extra keys because some firmwares treat unknown fields as an error.
	msg := map[string]any{}
	for k, v := range req.Payload {
		msg[k] = v
	}
	msg["msgid"] = req.CorrelationID
	msg["timestamp"] = time.Now().UnixMilli()
	msg["type"] = "ondemand_cmd"
	msg["cmd"] = name

	data, _ := json.Marshal(msg)
	// KUSUMC/RMS legacy protocol uses a per-device ondemand topic for downlink commands.
	topic := fmt.Sprintf("%s/ondemand", imei)
	token := s.mqtt.Publish(topic, 1, false, data)
	token.Wait()
	if err := token.Error(); err != nil {
		// mark failed
		_ = s.repo.UpdateCommandRequestStatus(req.CorrelationID, "failed", nil, nil, nil)
		return err
	}

	// on success mark published+pushed timestamp
	now := time.Now()
	_ = s.repo.UpdateCommandRequestStatus(req.CorrelationID, "published", &now, nil, nil)
	return nil
}

func (s *CommandsService) publishHTTP(req domain.CommandRequest) error {
	if s.http == nil {
		return fmt.Errorf("http client not configured")
	}
	if strings.TrimSpace(s.httpURL) == "" {
		return fmt.Errorf("http endpoint not configured")
	}

	body := map[string]any{
		"deviceId":      req.DeviceID,
		"projectId":     req.ProjectID,
		"commandId":     req.CommandID,
		"payload":       req.Payload,
		"correlationId": req.CorrelationID,
	}
	data, _ := json.Marshal(body)

	httpReq, err := http.NewRequest(http.MethodPost, s.httpURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.http.Do(httpReq)
	if err != nil {
		_ = s.repo.UpdateCommandRequestStatus(req.CorrelationID, "failed", nil, nil, nil)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		_ = s.repo.UpdateCommandRequestStatus(req.CorrelationID, "failed", nil, nil, nil)
		return fmt.Errorf("http publish failed: %s", resp.Status)
	}

	now := time.Now()
	_ = s.repo.UpdateCommandRequestStatus(req.CorrelationID, "published", &now, nil, nil)

	return nil
}

func commandWorkerConfigFromEnv() domain.CommandWorkerConfig {
	cfg := domain.CommandWorkerConfig{IntervalMs: 5000, AgeSeconds: 15, Batch: 10, MaxRetries: 3}

	if v := os.Getenv("COMMAND_RETRY_INTERVAL_MS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			cfg.IntervalMs = parsed
		}
	}
	if v := os.Getenv("COMMAND_RETRY_AGE_SECONDS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			cfg.AgeSeconds = parsed
		}
	}
	if v := os.Getenv("COMMAND_RETRY_BATCH"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			cfg.Batch = parsed
		}
	}
	if v := os.Getenv("COMMAND_RETRY_MAX"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			cfg.MaxRetries = parsed
		}
	}

	return cfg
}
