package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"ingestion-go/internal/adapters/secondary"
	"ingestion-go/internal/core/ports"
	"ingestion-go/internal/engine"
)

type DeviceService struct {
	repo        ports.DeviceRepo
	redisStore  *secondary.RedisStore
	emqx        *secondary.EmqxAdapter
	dnaRepo     *secondary.PostgresDNARepo
	rulesEngine *engine.RulesEngine
	mqtt        mqttPublisher
}

const identityCacheTTL = 20 * time.Minute

type identityCacheEntry struct {
	DeviceID  string `json:"device_id"`
	ProjectID string `json:"project_id"`
}

func NewDeviceService(repo ports.DeviceRepo, redisStore *secondary.RedisStore, emqx *secondary.EmqxAdapter, dnaRepo *secondary.PostgresDNARepo) *DeviceService {
	return &DeviceService{
		repo:        repo,
		redisStore:  redisStore,
		emqx:        emqx,
		dnaRepo:     dnaRepo,
		rulesEngine: engine.NewRulesEngine(),
	}
}

// SetMqttClient lets composition root wire the shared MQTT client.
func (s *DeviceService) SetMqttClient(client mqttPublisher) {
	s.mqtt = client
}

// EvaluateRules checks incoming telemetry against project automation flows
func (s *DeviceService) EvaluateRules(projectId string, telemetry map[string]interface{}) {
	flow, err := s.loadAutomationFlow(projectId)
	if err != nil {
		fmt.Printf("Rule Load Error: %v\n", err)
		return
	}
	if flow == nil {
		return // No rules
	}

	// 2. Evaluate
	actions := s.rulesEngine.Evaluate(telemetry, flow)

	// 3. Act
	deviceID, _ := telemetry["device_id"].(string)
	imei, _ := telemetry["imei"].(string)
	for _, action := range actions {
		s.executeAutomationAction(projectId, deviceID, imei, action)
	}
}

func (s *DeviceService) executeAutomationAction(projectId, deviceID, imei string, action engine.Action) {
	actionType := strings.ToLower(strings.TrimSpace(action.Type))
	if actionType == "" {
		log.Printf("[Automation] action missing type: %+v", action.Payload)
		return
	}

	payload := action.Payload
	if payload == nil {
		payload = map[string]interface{}{}
	}

	switch actionType {
	case "alert":
		message, _ := payload["message"].(string)
		if strings.TrimSpace(message) == "" {
			message = "Automation rule triggered"
		}
		severity, _ := payload["severity"].(string)
		if strings.TrimSpace(severity) == "" {
			severity = "warning"
		}

		if deviceID != "" && projectId != "" {
			if alertsRepo, ok := s.repo.(interface {
				CreateAlert(deviceId, projectId, msg, severity string) error
			}); ok {
				if err := alertsRepo.CreateAlert(deviceID, projectId, message, severity); err != nil {
					log.Printf("[Automation] failed to create alert: %v", err)
				}
			}
		}

		if s.mqtt != nil && projectId != "" {
			alertPayload := map[string]interface{}{
				"type":       "ALERT",
				"project_id": projectId,
				"device_id":  deviceID,
				"imei":       imei,
				"severity":   severity,
				"message":    message,
				"timestamp":  time.Now().UTC().Format(time.RFC3339),
				"details":    payload,
			}
			topic := fmt.Sprintf("channels/%s/alerts", projectId)
			s.mqtt.Publish(topic, 1, false, alertPayload)
		}

		log.Printf("[Automation] alert action executed: %s", message)

	case "mqtt_command", "mqtt", "command":
		topic, _ := payload["topic"].(string)
		if strings.TrimSpace(topic) == "" {
			topic, _ = payload["target"].(string)
		}
		if strings.TrimSpace(topic) == "" && projectId != "" && imei != "" {
			topic = fmt.Sprintf("%s/ondemand", imei)
		}

		if strings.TrimSpace(topic) == "" {
			log.Printf("[Automation] missing mqtt topic for action: %+v", payload)
			return
		}

		commandPayload := interface{}(payload)
		if nested, ok := payload["payload"]; ok {
			commandPayload = nested
		}

		if s.mqtt != nil {
			s.mqtt.Publish(topic, 1, false, commandPayload)
			log.Printf("[Automation] mqtt command published: %s", topic)
			return
		}

		log.Printf("[Automation] mqtt client unavailable for topic: %s", topic)

	default:
		log.Printf("[Automation] unknown action type: %s", action.Type)
	}
}

// ResolveDeviceIdentity returns device UUID (and project) for a given IMEI when present in the DB.
// This is useful for ingestion paths that only carry imei but still need device_id for persistence.
func (s *DeviceService) ResolveDeviceIdentity(imei string) (string, string, error) {
	imei = strings.TrimSpace(imei)
	if imei == "" {
		return "", "", fmt.Errorf("empty imei")
	}

	cacheKey := "identity:imei:" + imei
	if s.redisStore != nil {
		if raw, found, err := s.redisStore.GetRaw(cacheKey); err == nil && found {
			var cached identityCacheEntry
			if unmarshalErr := json.Unmarshal([]byte(raw), &cached); unmarshalErr == nil {
				if strings.TrimSpace(cached.DeviceID) != "" {
					return cached.DeviceID, strings.TrimSpace(cached.ProjectID), nil
				}
			}
		}
	}

	dev, err := s.repo.GetDeviceByIMEI(imei)
	if err != nil || dev == nil {
		return "", "", err
	}
	m, ok := dev.(map[string]interface{})
	if !ok {
		return "", "", fmt.Errorf("unexpected device shape")
	}
	id, _ := m["id"].(string)
	pid, _ := m["project_id"].(string)

	if s.redisStore != nil && strings.TrimSpace(id) != "" {
		entry := identityCacheEntry{DeviceID: strings.TrimSpace(id), ProjectID: strings.TrimSpace(pid)}
		if encoded, marshalErr := json.Marshal(entry); marshalErr == nil {
			_ = s.redisStore.SetRaw(cacheKey, string(encoded), identityCacheTTL)
		}
	}

	return id, pid, nil
}

func (s *DeviceService) ListDevices(projectId string, search string, status string, includeInactive bool, limit int, offset int) ([]map[string]interface{}, int, error) {
	return s.repo.ListDevices(projectId, search, status, includeInactive, limit, offset)
}

func (s *DeviceService) GetDeviceByIDOrIMEI(idOrIMEI string) (map[string]interface{}, error) {
	return s.repo.GetDeviceByIDOrIMEI(idOrIMEI)
}

func (s *DeviceService) UpdateDeviceByIDOrIMEI(idOrIMEI string, name *string, status *string, projectId *string, attrsPatch map[string]interface{}) (map[string]interface{}, error) {
	return s.repo.UpdateDeviceByIDOrIMEI(idOrIMEI, name, status, projectId, attrsPatch)
}

func (s *DeviceService) DeleteDevice(idOrIMEI string) error {
	return s.repo.SoftDeleteDevice(idOrIMEI)
}

func (s *DeviceService) loadAutomationFlow(projectId string) (map[string]interface{}, error) {
	// Prefer bundled automation if available
	if s.redisStore != nil {
		if bundle, ok := s.redisStore.GetConfigBundle(projectId); ok {
			if flow, ok := bundle["automation"].(map[string]interface{}); ok {
				if len(flow) == 0 {
					return nil, nil
				}
				return flow, nil
			}
		}
	}

	if s.redisStore != nil {
		key := fmt.Sprintf("config:automation:%s", projectId)
		if raw, ok, err := s.redisStore.GetRaw(key); err == nil && ok {
			if raw == "" || raw == "null" {
				return nil, nil
			}

			var cached map[string]interface{}
			if err := json.Unmarshal([]byte(raw), &cached); err == nil {
				if len(cached) == 0 {
					return nil, nil
				}
				return cached, nil
			}

			log.Printf("[DeviceService] Failed to decode cached automation flow for %s: %v", projectId, err)
		} else if err != nil {
			log.Printf("[DeviceService] Redis lookup failed for %s: %v", projectId, err)
		}
	}

	flow, err := s.repo.GetAutomationFlow(projectId)
	if err != nil {
		return nil, err
	}

	if s.redisStore != nil {
		key := fmt.Sprintf("config:automation:%s", projectId)
		if flow == nil || len(flow) == 0 {
			if err := s.redisStore.SetRaw(key, "null", 0); err != nil {
				log.Printf("[DeviceService] Failed to cache empty automation flow for %s: %v", projectId, err)
			}
			return nil, nil
		}

		if data, err := json.Marshal(flow); err == nil {
			if err := s.redisStore.SetRaw(key, string(data), 0); err != nil {
				log.Printf("[DeviceService] Failed to cache automation flow for %s: %v", projectId, err)
			}
		} else {
			log.Printf("[DeviceService] Failed to marshal automation flow for %s: %v", projectId, err)
		}
	}

	return flow, nil
}

// CreateDevice handles the full lifecycle
func (s *DeviceService) CreateDevice(projectId, name, imei string, attrs map[string]interface{}) (map[string]string, error) {
	// 1. Generate Creds
	mqttUser := imei
	bytes := make([]byte, 8)
	rand.Read(bytes)
	mqttPass := hex.EncodeToString(bytes)
	mqttClientID := fmt.Sprintf("dev-%s", imei)

	publishTopics, subscribeTopics := s.resolveTopics(projectId, imei)

	publicHost := os.Getenv("MQTT_PUBLIC_HOST")
	if publicHost == "" {
		publicHost = "localhost"
	}
	publicPort := os.Getenv("MQTT_PUBLIC_PORT")
	if publicPort == "" {
		publicPort = "8883"
	}
	publicProto := os.Getenv("MQTT_PUBLIC_PROTOCOL")
	if publicProto == "" {
		publicProto = "mqtts"
	}
	publicEndpoint := fmt.Sprintf("%s://%s:%s", publicProto, publicHost, publicPort)

	// 2. Save to DB (Postgres)
	// We pass attrs to support flexibility (e.g. govt_creds, shadow_init)
	mqttBundle := map[string]interface{}{
		"username":  mqttUser,
		"password":  mqttPass,
		"client_id": mqttClientID,
	}

	// Ensure attributes map exists and include mqtt bundle.
	if attrs == nil {
		attrs = make(map[string]interface{})
	}
	attrs["mqtt"] = mqttBundle

	// CreateDeviceStruct now returns the ID (string)
	id, err := s.repo.CreateDeviceStruct(projectId, name, imei, mqttBundle, attrs)
	if err != nil {
		return nil, fmt.Errorf("db failure: %v", err)
	}

	credID, err := s.repo.InsertCredentialHistory(id, mqttBundle)
	if err != nil {
		return nil, fmt.Errorf("cred history failure: %v", err)
	}

	// 3. Queue MQTT Provisioning (Async Worker)
	// This matches Node.js architecture (reliable, retriable)
	err = s.repo.CreateMqttProvisioningJob(id, &credID)
	if err != nil {
		// Log error but don't fail the request?
		// Or fail request? Ideally we transactionally rollback, but for now log it.
		fmt.Printf("⚠️ Failed to queue MQTT Job for device %s: %v\n", imei, err)
	}

	// Async queue implies we return success immediately
	return map[string]string{
		"id":                    id,
		"imei":                  imei,
		"mqtt_user":             mqttUser,
		"mqtt_pass":             mqttPass,
		"client_id":             mqttClientID,
		"endpoint":              publicEndpoint,
		"publish_topics":        strings.Join(publishTopics, ","),
		"subscribe_topics":      strings.Join(subscribeTopics, ","),
		"credential_history_id": credID,
		"provisioning_status":   "pending",
	}, nil
}

// RotateCredentials generates new MQTT credentials, records history, and queues provisioning.
func (s *DeviceService) RotateCredentials(deviceID string) (map[string]string, error) {
	dev, err := s.repo.GetDeviceByID(deviceID)
	if err != nil || dev == nil {
		// Fallback for environments where UUID lookups may fail; try IMEI-or-ID query.
		dev, err = s.repo.GetDeviceByIDOrIMEI(deviceID)
		if err != nil || dev == nil {
			return nil, fmt.Errorf("device not found")
		}
	}

	pid, _ := dev["project_id"].(string)
	imei, _ := dev["imei"].(string)
	if pid == "" || imei == "" {
		return nil, fmt.Errorf("device missing project_id or imei")
	}

	// Generate new password; keep same username/client_id convention
	mqttUser := imei
	bytes := make([]byte, 8)
	rand.Read(bytes)
	mqttPass := hex.EncodeToString(bytes)
	mqttClientID := fmt.Sprintf("dev-%s", imei)

	publishTopics, subscribeTopics := s.resolveTopics(pid, imei)

	publicHost := os.Getenv("MQTT_PUBLIC_HOST")
	if publicHost == "" {
		publicHost = "localhost"
	}
	publicPort := os.Getenv("MQTT_PUBLIC_PORT")
	if publicPort == "" {
		publicPort = "8883"
	}
	publicProto := os.Getenv("MQTT_PUBLIC_PROTOCOL")
	if publicProto == "" {
		publicProto = "mqtts"
	}
	publicEndpoint := fmt.Sprintf("%s://%s:%s", publicProto, publicHost, publicPort)

	bundle := map[string]interface{}{
		"username":  mqttUser,
		"password":  mqttPass,
		"client_id": mqttClientID,
	}

	credID, err := s.repo.InsertCredentialHistory(deviceID, bundle)
	if err != nil {
		return nil, fmt.Errorf("cred history failure: %v", err)
	}

	if err := s.repo.CreateMqttProvisioningJob(deviceID, &credID); err != nil {
		log.Printf("⚠️ Failed to queue MQTT Job for device %s: %v", imei, err)
	}

	return map[string]string{
		"credential_history_id": credID,
		"provisioning_status":   "pending",
		"mqtt_user":             mqttUser,
		"mqtt_pass":             mqttPass,
		"client_id":             mqttClientID,
		"endpoint":              publicEndpoint,
		"publish_topics":        strings.Join(publishTopics, ","),
		"subscribe_topics":      strings.Join(subscribeTopics, ","),
	}, nil
}

func (s *DeviceService) ListCredentialHistory(deviceID string) ([]map[string]interface{}, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("device id required")
	}
	return s.repo.ListCredentialHistory(deviceID)
}

func (s *DeviceService) RetryProvisioning(deviceID string, credentialHistoryID *string) error {
	if deviceID == "" {
		return fmt.Errorf("device id required")
	}
	var credID string
	if credentialHistoryID != nil && strings.TrimSpace(*credentialHistoryID) != "" {
		credID = strings.TrimSpace(*credentialHistoryID)
	} else {
		latest, err := s.repo.GetLatestCredentialHistory(deviceID)
		if err != nil || latest == nil {
			return fmt.Errorf("credential history not found")
		}
		if id, ok := latest["id"].(string); ok {
			credID = id
		}
	}
	if credID == "" {
		return fmt.Errorf("credential history id required")
	}
	return s.repo.CreateMqttProvisioningJob(deviceID, &credID)
}

func (s *DeviceService) GetLatestCredentialHistory(deviceID string) (map[string]interface{}, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("device id required")
	}
	return s.repo.GetLatestCredentialHistory(deviceID)
}

func (s *DeviceService) ResolveTopics(projectId, imei string) ([]string, []string) {
	return s.resolveTopics(projectId, imei)
}

// resolveTopics derives publish/subscribe topics for a device using Project DNA metadata when available.
func (s *DeviceService) resolveTopics(projectId, imei string) ([]string, []string) {
	// defaults (KUSUMC/RMS legacy)
	defaultPub := []string{
		fmt.Sprintf("%s/heartbeat", imei),
		fmt.Sprintf("%s/data", imei),
		fmt.Sprintf("%s/daq", imei),
		fmt.Sprintf("%s/ondemand", imei),
		fmt.Sprintf("%s/errors", imei),
	}
	defaultPub = filterUnsupportedPublishTopics(defaultPub)
	defaultSub := []string{fmt.Sprintf("%s/ondemand", imei)}

	compatEnabled := false
	if v := strings.TrimSpace(os.Getenv("MQTT_COMPAT_TOPICS_ENABLED")); v != "" {
		compatEnabled = strings.EqualFold(v, "true") || strings.EqualFold(v, "1") || strings.EqualFold(v, "yes")
	}
	compatPub := []string{
		fmt.Sprintf("devices/%s/telemetry", imei),
		fmt.Sprintf("devices/%s/errors", imei),
	}
	if strings.TrimSpace(projectId) != "" {
		compatPub = append(compatPub, fmt.Sprintf("channels/%s/messages/%s", strings.TrimSpace(projectId), imei))
	}

	if s.dnaRepo == nil {
		if compatEnabled {
			return filterUnsupportedPublishTopics(uniqueStrings(append(defaultPub, compatPub...))), defaultSub
		}
		return defaultPub, defaultSub
	}

	record, err := s.dnaRepo.GetByProjectID(context.Background(), projectId)
	if err != nil || record == nil || record.Metadata == nil {
		if compatEnabled {
			return filterUnsupportedPublishTopics(uniqueStrings(append(defaultPub, compatPub...))), defaultSub
		}
		return defaultPub, defaultSub
	}

	meta := record.Metadata
	if mqttMeta, ok := meta["mqtt"].(map[string]interface{}); ok {
		pub := extractStringSlice(mqttMeta["publish_topics"])
		sub := extractStringSlice(mqttMeta["subscribe_topics"])
		if len(pub) > 0 {
			pub = substituteIMEI(pub, projectId, imei)
			pub = filterUnsupportedPublishTopics(pub)
		}
		if len(sub) > 0 {
			sub = substituteIMEI(sub, projectId, imei)
		}
		if len(pub) > 0 || len(sub) > 0 {
			if len(pub) == 0 {
				pub = defaultPub
			}
			if len(sub) == 0 {
				sub = defaultSub
			}
			if compatEnabled {
				pub = uniqueStrings(append(pub, compatPub...))
			}
			return filterUnsupportedPublishTopics(pub), sub
		}
	}

	if compatEnabled {
		return filterUnsupportedPublishTopics(uniqueStrings(append(defaultPub, compatPub...))), defaultSub
	}
	return defaultPub, defaultSub
}

func filterUnsupportedPublishTopics(topics []string) []string {
	if len(topics) == 0 {
		return topics
	}
	allowedSuffixes := map[string]bool{
		"heartbeat": true,
		"data":      true,
		"daq":       true,
		"ondemand":  true,
		"errors":    true,
	}
	out := make([]string, 0, len(topics))
	for _, topic := range topics {
		trimmed := strings.TrimSpace(topic)
		if trimmed == "" {
			continue
		}
		parts := strings.Split(strings.ToLower(trimmed), "/")
		if len(parts) == 2 {
			suffix := strings.TrimSpace(parts[1])
			if suffix != "" && !allowedSuffixes[suffix] {
				continue
			}
		}
		out = append(out, trimmed)
	}
	return out
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		s := strings.TrimSpace(v)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func extractStringSlice(v interface{}) []string {
	if v == nil {
		return nil
	}
	result := []string{}
	switch val := v.(type) {
	case []interface{}:
		for _, item := range val {
			if s, ok := item.(string); ok && s != "" {
				result = append(result, s)
			}
		}
	case []string:
		for _, s := range val {
			if s != "" {
				result = append(result, s)
			}
		}
	}
	return result
}

func substituteIMEI(topics []string, projectId, imei string) []string {
	out := make([]string, 0, len(topics))
	for _, t := range topics {
		val := t
		val = strings.ReplaceAll(val, "<IMEI>", imei)
		val = strings.ReplaceAll(val, "{imei}", imei)
		val = strings.ReplaceAll(val, "{IMEI}", imei)
		val = strings.ReplaceAll(val, "{project_id}", projectId)
		val = strings.ReplaceAll(val, "{PROJECT_ID}", projectId)
		out = append(out, val)
	}
	return out
}
