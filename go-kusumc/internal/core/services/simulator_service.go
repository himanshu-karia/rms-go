package services

import (
	"fmt"
	"strings"
	"time"

	"ingestion-go/internal/adapters/secondary"

	"github.com/google/uuid"
)

type SimulatorService struct {
	repo *secondary.PostgresRepo
}

func NewSimulatorService(repo *secondary.PostgresRepo) *SimulatorService {
	return &SimulatorService{repo: repo}
}

type SimulatorSessionStatus string

const (
	SimulatorSessionActive  SimulatorSessionStatus = "active"
	SimulatorSessionRevoked SimulatorSessionStatus = "revoked"
	SimulatorSessionExpired SimulatorSessionStatus = "expired"
)

type SimulatorSession struct {
	ID                 string                 `json:"id"`
	Token              string                 `json:"token"`
	Status             SimulatorSessionStatus `json:"status"`
	DeviceUUID         *string                `json:"device_uuid"`
	DeviceID           *string                `json:"device_id"`
	CreatedAt          time.Time              `json:"created_at"`
	ExpiresAt          time.Time              `json:"expires_at"`
	EndedAt            *time.Time             `json:"ended_at"`
	RequestedBy        *string                `json:"requested_by"`
	RevokedBy          *string                `json:"revoked_by"`
	LastActivityAt     time.Time              `json:"last_activity_at"`
	CredentialSnapshot map[string]interface{} `json:"credential_snapshot"`
	CommandQuota       map[string]interface{} `json:"command_quota"`
	Device             map[string]interface{} `json:"device"`
}

type CreateSimulatorSessionInput struct {
	DeviceUUID       string
	ExpiresInMinutes int
	RequestedBy      *string
}

type CreateSimulatorSessionResult struct {
	Session     SimulatorSession       `json:"session"`
	Credentials map[string]interface{} `json:"credentials"`
}

type ListSimulatorSessionsParams struct {
	Limit  int
	Cursor string
	Status string
}

func (s *SimulatorService) GenerateScript(projectId string) []byte {
	// Template for Python Simulator
	// In real V1.1, this would fetch Project Config -> Sensors -> Generate Python Code
	// For V1 MVP, we return a generic comprehensive script.

	script := fmt.Sprintf(`# Unified IoT Simulator
# Project: %s
import time
import json
import random
import requests

SERVER_URL = "http://localhost:8081/api/ingest"
IMEI = "SIM-%s-001"
PROJECT_ID = "%s"

def generate_telemetry():
    return {
        "imei": IMEI,
        "project_id": PROJECT_ID,
        "temp": round(random.uniform(20.0, 30.0), 2),
        "hum": round(random.uniform(40.0, 60.0), 2),
        "voltage": round(random.uniform(3.3, 4.2), 2),
        "msgid": str(int(time.time()))
    }

while True:
    data = generate_telemetry()
    try:
        res = requests.post(SERVER_URL, json=data)
        print(f"Sent: {data} -> {res.status_code}")
    except Exception as e:
        print(f"Error: {e}")
    time.sleep(5)
`, projectId, projectId, projectId)

	return []byte(script)
}

func (s *SimulatorService) CreateSession(input CreateSimulatorSessionInput) (*CreateSimulatorSessionResult, error) {
	if input.DeviceUUID == "" {
		return nil, fmt.Errorf("device_uuid is required")
	}
	minutes := clampDuration(input.ExpiresInMinutes)

	device, err := s.repo.GetDeviceByIDOrUUID(input.DeviceUUID)
	if err != nil {
		return nil, err
	}
	if device == nil {
		return nil, fmt.Errorf("device not found")
	}

	deviceID, _ := device["id"].(string)
	deviceUUID, _ := device["uuid"].(string)
	if deviceUUID == "" {
		deviceUUID = deviceID
	}
	imei, _ := device["imei"].(string)

	if deviceID == "" {
		return nil, fmt.Errorf("device id missing")
	}

	cred, err := s.repo.GetLatestCredentialHistory(deviceID)
	if err != nil || cred == nil {
		return nil, fmt.Errorf("credential history not found")
	}

	bundle, _ := cred["bundle"].(map[string]interface{})
	if bundle == nil {
		bundle = map[string]interface{}{}
	}

	username, _ := bundle["username"].(string)
	password, _ := bundle["password"].(string)
	clientID, _ := bundle["client_id"].(string)

	credentials := map[string]interface{}{
		"clientId":  clientID,
		"username":  username,
		"password":  password,
		"endpoints": bundle["endpoints"],
		"topics": map[string]interface{}{
			"publish":   bundle["publish_topics"],
			"subscribe": bundle["subscribe_topics"],
		},
	}

	passwordMasked := maskSecret(password)

	credentialSnapshot := map[string]interface{}{
		"clientId":       clientID,
		"username":       username,
		"passwordMasked": passwordMasked,
		"endpoints":      bundle["endpoints"],
		"topics": map[string]interface{}{
			"publish":   bundle["publish_topics"],
			"subscribe": bundle["subscribe_topics"],
		},
	}

	commandQuota := map[string]interface{}{
		"limit":        20,
		"active_count": 0,
	}

	now := time.Now()
	expiresAt := now.Add(time.Duration(minutes) * time.Minute)
	requestedBy := ""
	if input.RequestedBy != nil {
		requestedBy = *input.RequestedBy
	}

	sessionID := uuid.NewString()
	token := uuid.NewString()

	if err := s.repo.CreateSimulatorSession(secondary.SimulatorSessionRecord{
		ID:                 sessionID,
		Token:              token,
		Status:             string(SimulatorSessionActive),
		DeviceID:           &deviceID,
		DeviceUUID:         &deviceUUID,
		CreatedAt:          now,
		ExpiresAt:          expiresAt,
		LastActivityAt:     now,
		CredentialSnapshot: credentialSnapshot,
		CommandQuota:       commandQuota,
		RequestedBy:        nullableString(requestedBy),
	}); err != nil {
		return nil, err
	}

	session := SimulatorSession{
		ID:                 sessionID,
		Token:              token,
		Status:             SimulatorSessionActive,
		DeviceUUID:         nullableString(deviceUUID),
		DeviceID:           nullableString(deviceID),
		CreatedAt:          now,
		ExpiresAt:          expiresAt,
		LastActivityAt:     now,
		CredentialSnapshot: credentialSnapshot,
		CommandQuota:       commandQuota,
		Device: map[string]interface{}{
			"uuid": deviceUUID,
			"imei": imei,
		},
	}

	return &CreateSimulatorSessionResult{Session: session, Credentials: credentials}, nil
}

func (s *SimulatorService) ListSessions(params ListSimulatorSessionsParams) ([]SimulatorSession, string, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	records, nextCursor, err := s.repo.ListSimulatorSessions(limit, params.Cursor, params.Status)
	if err != nil {
		return nil, "", err
	}

	sessions := make([]SimulatorSession, 0, len(records))
	for _, rec := range records {
		device := map[string]interface{}{}
		if rec.DeviceUUID != nil {
			device["uuid"] = *rec.DeviceUUID
		}
		if rec.DeviceID != nil {
			if dev, err := s.repo.GetDeviceByID(*rec.DeviceID); err == nil && dev != nil {
				if imei, ok := dev["imei"].(string); ok {
					device["imei"] = imei
				}
			}
		}

		sessions = append(sessions, SimulatorSession{
			ID:                 rec.ID,
			Token:              rec.Token,
			Status:             SimulatorSessionStatus(rec.Status),
			DeviceUUID:         rec.DeviceUUID,
			DeviceID:           rec.DeviceID,
			CreatedAt:          rec.CreatedAt,
			ExpiresAt:          rec.ExpiresAt,
			EndedAt:            rec.EndedAt,
			RequestedBy:        rec.RequestedBy,
			RevokedBy:          rec.RevokedBy,
			LastActivityAt:     rec.LastActivityAt,
			CredentialSnapshot: rec.CredentialSnapshot,
			CommandQuota:       rec.CommandQuota,
			Device:             device,
		})
	}

	return sessions, nextCursor, nil
}

func (s *SimulatorService) RevokeSession(sessionID string, requestedBy *string) (*SimulatorSession, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("sessionId required")
	}
	user := ""
	if requestedBy != nil {
		user = *requestedBy
	}
	if err := s.repo.RevokeSimulatorSession(sessionID, nullableString(user)); err != nil {
		return nil, err
	}

	rec, err := s.repo.GetSimulatorSessionByID(sessionID)
	if err != nil || rec == nil {
		return nil, fmt.Errorf("session not found")
	}

	device := map[string]interface{}{}
	if rec.DeviceUUID != nil {
		device["uuid"] = *rec.DeviceUUID
	}
	if rec.DeviceID != nil {
		if dev, err := s.repo.GetDeviceByID(*rec.DeviceID); err == nil && dev != nil {
			if imei, ok := dev["imei"].(string); ok {
				device["imei"] = imei
			}
		}
	}

	session := SimulatorSession{
		ID:                 rec.ID,
		Token:              rec.Token,
		Status:             SimulatorSessionStatus(rec.Status),
		DeviceUUID:         rec.DeviceUUID,
		DeviceID:           rec.DeviceID,
		CreatedAt:          rec.CreatedAt,
		ExpiresAt:          rec.ExpiresAt,
		EndedAt:            rec.EndedAt,
		RequestedBy:        rec.RequestedBy,
		RevokedBy:          rec.RevokedBy,
		LastActivityAt:     rec.LastActivityAt,
		CredentialSnapshot: rec.CredentialSnapshot,
		CommandQuota:       rec.CommandQuota,
		Device:             device,
	}

	return &session, nil
}

func clampDuration(value int) int {
	if value <= 0 {
		return 120
	}
	if value < 5 {
		return 5
	}
	if value > 480 {
		return 480
	}
	return value
}

func nullableString(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func maskSecret(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 4 {
		return strings.Repeat("*", len(trimmed))
	}
	return strings.Repeat("*", len(trimmed)-4) + trimmed[len(trimmed)-4:]
}
