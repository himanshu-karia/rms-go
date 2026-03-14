package services

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"ingestion-go/internal/config/payloadschema"
	"ingestion-go/internal/core/domain"
	"ingestion-go/internal/core/ports"
)

type IngestionService struct {
	state       ports.StateStore
	repo        ports.TelemetryRepo
	commands    ports.CommandRepo
	transformer *GovaluateTransformer
	device      *DeviceService
	rules       *RulesService
	batchChan   chan map[string]interface{}
}

type ingestOverflowStore interface {
	PushDeadLetter(queue string, val string, maxLen int, ttl time.Duration) error
	IncrementCounter(key string, ttl time.Duration) (int64, error)
}

var legacyIMEIPrefix = regexp.MustCompile(`^\d{10,20}$`)

func inferIMEIFromTopic(topic string) (projectID string, imei string) {
	parts := strings.Split(strings.TrimSpace(topic), "/")
	if len(parts) >= 4 && parts[0] == "channels" {
		// channels/{project_id}/messages/{imei}
		// channels/{project_id}/commands/{imei}/(resp|ack)
		pid := strings.TrimSpace(parts[1])
		segment := strings.TrimSpace(parts[2])
		if segment == "messages" || segment == "commands" {
			candidate := strings.TrimSpace(parts[3])
			if candidate != "" {
				return pid, candidate
			}
		}
	}
	if len(parts) >= 3 && parts[0] == "devices" {
		// devices/{imei}/telemetry
		// devices/{imei}/errors
		candidate := strings.TrimSpace(parts[1])
		segment := strings.TrimSpace(parts[2])
		if candidate != "" && (segment == "telemetry" || segment == "errors") {
			return "", candidate
		}
	}
	if len(parts) >= 2 {
		// Legacy RMS: <imei>/{heartbeat,data,daq,ondemand}
		candidate := strings.TrimSpace(parts[0])
		if legacyIMEIPrefix.MatchString(candidate) {
			return "", candidate
		}
	}
	return "", ""
}

func normalizeIdentity(raw map[string]interface{}, topic string, fallbackProjectID string) {
	if raw == nil {
		return
	}

	// Carry caller-supplied project when provided (e.g., HTTP ingest with ApiKey scope).
	if raw["project_id"] == nil && strings.TrimSpace(fallbackProjectID) != "" {
		raw["project_id"] = strings.TrimSpace(fallbackProjectID)
	}

	// Normalize msgid variants into msgid (used by dedupe + correlation fallback).
	if raw["msgid"] == nil {
		for _, key := range []string{"MSGID", "msg_id", "MSG_ID"} {
			if v, ok := raw[key].(string); ok && strings.TrimSpace(v) != "" {
				raw["msgid"] = strings.TrimSpace(v)
				break
			}
		}
	}

	// Normalize IMEI variants into imei.
	if raw["imei"] == nil {
		for _, key := range []string{"IMEI", "Imei"} {
			if v, ok := raw[key].(string); ok && strings.TrimSpace(v) != "" {
				raw["imei"] = strings.TrimSpace(v)
				break
			}
		}
	}

	// If still missing, infer from topic for both legacy and channels layouts.
	if raw["imei"] == nil {
		pid, imei := inferIMEIFromTopic(topic)
		if strings.TrimSpace(imei) != "" {
			raw["imei"] = imei
		}
		if raw["project_id"] == nil && strings.TrimSpace(pid) != "" {
			raw["project_id"] = pid
		}
	}
}

type validationReport struct {
	Missing   []string
	Oversized []string
	Unknown   []string
	NilKeys   []string
}

func (r validationReport) ok() bool {
	return len(r.Missing) == 0 && len(r.Oversized) == 0 && len(r.Unknown) == 0 && len(r.NilKeys) == 0
}

func (r validationReport) asMap() map[string]interface{} {
	out := make(map[string]interface{})
	if len(r.Missing) > 0 {
		out["missing"] = r.Missing
	}
	if len(r.Oversized) > 0 {
		out["oversized"] = r.Oversized
	}
	if len(r.Unknown) > 0 {
		out["unknown"] = r.Unknown
	}
	if len(r.NilKeys) > 0 {
		out["nil"] = r.NilKeys
	}
	return out
}

func NewIngestionService(state ports.StateStore, repo ports.TelemetryRepo, commands ports.CommandRepo, transf *GovaluateTransformer, device *DeviceService, rules *RulesService) *IngestionService {
	s := &IngestionService{
		state:       state,
		repo:        repo,
		commands:    commands,
		transformer: transf,
		device:      device,
		rules:       rules,
		batchChan:   make(chan map[string]interface{}, 5000),
	}
	go s.startBatcher()
	return s
}

// startBatcher consumes packets and flushes them in batches
func (s *IngestionService) startBatcher() {
	buffer := make([]interface{}, 0, 1000)
	ticker := time.NewTicker(1 * time.Second)

	for {
		select {
		case item := <-s.batchChan:
			buffer = append(buffer, item)
			if len(buffer) >= 1000 {
				s.flush(buffer)
				buffer = buffer[:0]
			}
		case <-ticker.C:
			if len(buffer) > 0 {
				s.flush(buffer)
				buffer = buffer[:0]
			}
		}
	}
}

func (s *IngestionService) flush(batch []interface{}) {
	// Deep Copy Payload maps? No, they are passed by val (pointer for map)
	// Just pass slice to Repo
	if err := s.repo.SaveBatch(batch); err != nil {
		fmt.Printf("⚠️ Batch Save Error: %v\n", err)
	}
}

// ProcessPacket is the entry point for every MQTT/HTTP message
// projectID may be empty for MQTT paths that resolve project via device lookup.
func (s *IngestionService) ProcessPacket(topic string, payload []byte, projectID string) error {
	// 1. Parse JSON
	var raw map[string]interface{}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return fmt.Errorf("invalid json: %v", err)
	}

	normalizeIdentity(raw, topic, projectID)

	// 2. Extract Identity (IMEI)
	imei, ok := raw["imei"].(string)
	if !ok {
		return fmt.Errorf("missing imei")
	}

	// Ignore server-published ondemand commands echoed back via subscription. We already
	// persist command requests in the command pipeline; treating them as telemetry can
	// create false "responses" and pollute telemetry history.
	if isServerPublishedOndemandCommand(raw, topic) {
		return nil
	}

	// Resolve device_id/project_id if missing but we have IMEI (so we can persist without UUID errors)
	if raw["device_id"] == nil && s.device != nil {
		if devID, pid, err := s.device.ResolveDeviceIdentity(imei); err == nil {
			if devID != "" {
				raw["device_id"] = devID
			}
			if raw["project_id"] == nil && pid != "" {
				raw["project_id"] = pid
			}
		}
	}

	// 3. Deduplication (Locking)
	msgid := fmt.Sprintf("%s-%d", imei, time.Now().Unix())
	if val, ok := raw["msgid"].(string); ok {
		msgid = val
	}

	isLocked, err := s.state.AcquireLock(msgid, 30) // 30s lock
	if !isLocked || err != nil {
		return fmt.Errorf("duplicate packet or lock error")
	}

	// 4. Fetch Config (Dynamic Profile)
	projectId, _ := raw["project_id"].(string)
	var sensorConfig []interface{}
	var packetSchemas map[string]payloadschema.PacketSchema

	if projectId != "" {
		if bundle, ok := s.state.GetConfigBundle(projectId); ok {
			sensorConfig = extractSensorsFromBundle(bundle)
			packetSchemas = extractPayloadSchemasFromBundle(bundle)
		}

		if sensorConfig == nil || packetSchemas == nil {
			if cfg, found := s.state.GetProjectConfig(projectId); found {
				if pm, ok := cfg.(map[string]interface{}); ok {
					if hw, ok := pm["hardware"].(map[string]interface{}); ok {
						if sensors, ok := hw["sensors"].([]interface{}); ok {
							sensorConfig = sensors
						}
					}
					if packetSchemas == nil {
						packetSchemas = extractPayloadSchemas(pm)
					}
				}
			}
		}
	}

	packetType := detectPacketType(raw, topic, packetSchemas, imei, projectId)
	if !isSupportedPacketType(packetType) {
		return fmt.Errorf("unsupported packet_type")
	}
	if hasUnsupportedTelemetrySuffix(topic) {
		return fmt.Errorf("unsupported topic suffix")
	}
	var packetSchema *payloadschema.PacketSchema
	if packetType != "" && packetSchemas != nil {
		if ps, ok := packetSchemas[packetType]; ok {
			schemaCopy := ps
			packetSchema = &schemaCopy
		}
	}
	// Built-in schema for dedicated device error topic (<imei>/errors).
	// This keeps device_error packets from being marked suspicious just because they carry
	// error_data/open_id fields not present in sensor telemetry schemas.
	if packetSchema == nil && strings.EqualFold(strings.TrimSpace(packetType), "device_error") {
		packetSchema = &payloadschema.PacketSchema{
			PacketType:    "device_error",
			TopicTemplate: "<IMEI>/errors",
			Keys: []payloadschema.KeySpec{
				{Key: "timestamp", Required: true},
				{Key: "error_code", Required: true},
				{Key: "open_id", Required: false},
				{Key: "error_data", Required: false},
			},
		}
	}

	// 5. Transformation
	processed, err := s.transformer.Apply(raw, sensorConfig)
	if err != nil {
		fmt.Println("Transform Warning:", err)
		processed = raw
	}
	if len(processed) == 0 {
		// If no transforms applied (empty sensor config), persist the raw payload
		processed = raw
	}
	normalizeForwardedTelemetryPayload(processed, raw, topic, imei)
	// Persist packet_type consistently. Many downstream queries (history/latest) filter by
	// telemetry.data->>'packet_type', so even raw legacy payloads must carry it.
	if strings.TrimSpace(packetType) != "" {
		if _, exists := processed["packet_type"]; !exists {
			processed["packet_type"] = packetType
		}
	}

	// 6. Validation (Ported from Node PayloadVerifier)
	report := s.validatePacketQuality(processed, sensorConfig, packetSchema)

	status := "verified"
	if !report.ok() {
		status = "suspicious"
	}

	eventTime := resolvePacketEventTime(processed, raw)
	ingestedAt := time.Now().UTC()
	if _, exists := processed["ingested_at"]; !exists {
		processed["ingested_at"] = ingestedAt.Format(time.RFC3339Nano)
	}

	envelope := map[string]interface{}{
		"time":       eventTime,
		"imei":       imei,
		"device_id":  raw["device_id"],
		"project_id": raw["project_id"],
		"payload":    processed,
		"status":     status,
		"hops":       0,
	}

	if !report.ok() {
		envelope["validation"] = report.asMap()
	}

	// Command response correlation (correlation_id -> command_requests/command_responses)
	corrID, _ := processed["correlation_id"].(string)
	if corrID == "" {
		corrID, _ = raw["correlation_id"].(string)
	}
	if corrID == "" {
		// legacy fallback
		corrID, _ = processed["msgid"].(string)
	}
	if corrID == "" && s.commands != nil {
		// Govt legacy ondemand responses may not carry msgid/correlation_id. In that case, we
		// correlate to the most recent outstanding command request for the device.
		if isOndemandTopic(topic) && isOndemandResponsePayload(processed) {
			if deviceID, _ := raw["device_id"].(string); strings.TrimSpace(deviceID) != "" {
				corrID = s.inferLatestCommandCorrelation(deviceID)
			}
		}
	}
	if corrID != "" && s.commands != nil {
		go s.handleCommandResponse(corrID, raw, processed)
	}

	// 7. Persistence (Buffered)
	// Old: s.repo.SaveTelemetry(envelope)
	// New: Push to Channel
	select {
	case s.batchChan <- envelope:
		// Success
	default:
		// Buffer Full - Drop or Block?
		// For IoT, dropping is better than backpressure cascading
		log.Printf("⚠️ Ingestion Buffer Full! Dropping Packet. topic=%s imei=%s", topic, imei)
		s.recordIngestOverflow(topic, raw, imei)
		return fmt.Errorf("buffer full")
	}

	// 8. Rule Evaluation (Critical Logic Injection)
	// Device-originated errors/offline-rule alerts are published on <imei>/errors and should
	// always be surfaced as alerts, independent of verified/suspicious classification.
	if strings.EqualFold(strings.TrimSpace(packetType), "device_error") {
		if s.rules != nil {
			projectId, _ := raw["project_id"].(string)
			deviceId, _ := raw["device_id"].(string)
			errorCode := "device_error"
			if v, ok := processed["error_code"].(string); ok && strings.TrimSpace(v) != "" {
				errorCode = strings.TrimSpace(v)
			}
			severity := "warning"
			if v, ok := processed["severity"].(string); ok && strings.TrimSpace(v) != "" {
				severity = strings.ToLower(strings.TrimSpace(v))
			}
			openID := ""
			for _, key := range []string{"open_id", "device_uuid", "device_id"} {
				if v, ok := processed[key].(string); ok && strings.TrimSpace(v) != "" {
					openID = strings.TrimSpace(v)
					break
				}
			}
			// Prefer existing timestamp fields; fall back to now.
			ts := processed["timestamp"]
			if ts == nil {
				ts = time.Now().UnixMilli()
			}
			occurredAt := time.Now().UTC()
			switch v := ts.(type) {
			case int64:
				if v > 0 {
					occurredAt = time.UnixMilli(v).UTC()
				}
			case int:
				if v > 0 {
					occurredAt = time.UnixMilli(int64(v)).UTC()
				}
			case float64:
				ms := int64(v)
				if ms > 0 {
					occurredAt = time.UnixMilli(ms).UTC()
				}
			case string:
				if t, err := time.Parse(time.RFC3339, strings.TrimSpace(v)); err == nil {
					occurredAt = t.UTC()
				}
			}
			data := map[string]interface{}{
				"source":     "device_error",
				"open_id":    openID,
				"timestamp":  ts,
				"error_code": errorCode,
				"error_data": processed["error_data"],
				"payload":    processed,
				"device": map[string]any{
					"imei": imei,
					"id":   deviceId,
				},
				"first_seen": occurredAt.Format(time.RFC3339),
				"last_seen":  occurredAt.Format(time.RFC3339),
				"count":      1,
			}
			msg := fmt.Sprintf("Device error: %s", errorCode)
			s.rules.EmitDeviceAlert(projectId, deviceId, msg, severity, data)
		}
	}

	if status == "verified" {
		// Asynchronous to avoid blocking ingestion?
		// Node did it synchronously in loop, but here we can go async or sync.
		// For safety/speed balance:
		projectId, _ := raw["project_id"].(string)
		if projectId != "" {
			// Keep rule evaluation payload aligned with Studio fields, but enrich with identifiers.
			rulePayload := make(map[string]interface{}, len(processed)+3)
			for k, v := range processed {
				rulePayload[k] = v
			}
			if deviceID, ok := raw["device_id"].(string); ok && deviceID != "" {
				rulePayload["device_id"] = deviceID
			}
			if imei != "" {
				rulePayload["imei"] = imei
			}
			rulePayload["project_id"] = projectId

			if s.device != nil {
				go s.device.EvaluateRules(projectId, rulePayload)
			}

			if s.rules != nil {
				packet := map[string]interface{}{
					"project_id": projectId,
					"device_id":  rulePayload["device_id"],
					"payload":    processed,
				}
				go s.rules.Evaluate(packet)
			}
		}
	}

	// 9. Hot Cache Push
	if status == "verified" {
		s.state.PushPacket(imei, envelope)

		// [GAP FIX] Update Persistent Shadow (Reported State)
		// Assuming payload IS the reported state.
		// In V1, we just sync the whole payload as 'reported'.
		go s.repo.UpdateDeviceShadow(imei, processed)
	}

	return nil
}

func (s *IngestionService) recordIngestOverflow(topic string, raw map[string]interface{}, imei string) {
	overflowStore, ok := s.state.(ingestOverflowStore)
	if !ok {
		return
	}

	entry := map[string]interface{}{
		"topic":       topic,
		"imei":        imei,
		"payload":     raw,
		"reason":      "ingestion_buffer_full",
		"occurred_at": time.Now().UTC().Format(time.RFC3339Nano),
	}
	if blob, err := json.Marshal(entry); err == nil {
		_ = overflowStore.PushDeadLetter("ingest:deadletter", string(blob), 10000, 7*24*time.Hour)
	}
	_, _ = overflowStore.IncrementCounter("metrics:ingest:buffer_overflow_total", 0)
	_, _ = overflowStore.IncrementCounter("metrics:ingest:buffer_overflow_hourly", time.Hour)
}

// handleCommandResponse persists command response and updates request status with basic pattern classification.
func (s *IngestionService) handleCommandResponse(corrID string, raw map[string]interface{}, processed map[string]interface{}) {
	req, err := s.commands.GetCommandRequestByCorrelation(corrID)
	if err != nil || req == nil {
		return
	}

	patterns, _ := s.commands.GetResponsePatterns(req.CommandID)
	matchedID, status, parsed := classifyResponse(processed, patterns)
	if mapped := commandStatusFromPayload(processed); mapped != "" {
		status = mapped
	}

	deviceID, _ := raw["device_id"].(string)
	projectID, _ := raw["project_id"].(string)
	completed := time.Now()
	resp := domain.CommandResponse{
		CorrelationID:    corrID,
		DeviceID:         deviceID,
		ProjectID:        projectID,
		RawResponse:      processed,
		Parsed:           parsed,
		MatchedPatternID: matchedID,
		ReceivedAt:       completed,
	}
	_ = s.commands.SaveCommandResponse(resp)
	_ = s.commands.UpdateCommandRequestStatus(corrID, status, nil, &completed, nil)

	// If this is a device configuration apply command, finalize the corresponding device_configurations row.
	if cmd, err := s.commands.GetCommandByID(req.CommandID); err == nil && cmd != nil {
		if strings.EqualFold(strings.TrimSpace(cmd.Name), "apply_device_configuration") {
			cfgStatus := ""
			switch status {
			case "acked":
				cfgStatus = "acknowledged"
			case "failed", "timeout":
				cfgStatus = "failed"
			default:
				// keep pending for non-terminal states
				cfgStatus = ""
			}
			if cfgStatus != "" {
				type configFinalizer interface {
					FinalizeDeviceConfiguration(configID string, status string, ack map[string]any) error
				}
				if fin, ok := s.repo.(configFinalizer); ok {
					_ = fin.FinalizeDeviceConfiguration(corrID, cfgStatus, processed)
				}
			}
		}
	}
}

// commandStatusFromPayload maps standardized device ack/resp fields into command_requests.status.
// Supported statuses (DB constraint): queued, published, acked, failed, timeout
func commandStatusFromPayload(payload map[string]interface{}) string {
	if payload == nil {
		return ""
	}

	// Numeric code wins when present.
	if raw, ok := payload["code"]; ok {
		switch v := raw.(type) {
		case int:
			return mapCommandCode(v)
		case int32:
			return mapCommandCode(int(v))
		case int64:
			return mapCommandCode(int(v))
		case float64:
			if float64(int(v)) == v {
				return mapCommandCode(int(v))
			}
		}
	}

	status, _ := payload["status"].(string)
	s := strings.ToLower(strings.TrimSpace(status))
	if s == "" {
		return ""
	}
	// Common firmware statuses.
	switch s {
	case "ack", "acked", "ok", "success", "done", "completed":
		return "acked"
	case "wait", "pending", "retry":
		return "published"
	case "error", "failed", "reject", "rejected":
		return "failed"
	default:
		return ""
	}
}

func mapCommandCode(code int) string {
	// Contract:
	// 0 = accepted/acked, 1 = failed, 2 = wait
	switch code {
	case 0:
		return "acked"
	case 1:
		return "failed"
	case 2:
		return "published"
	default:
		return ""
	}
}

// classifyResponse returns matched pattern ID and status based on simple regex/jsonpath checks.
func classifyResponse(payload map[string]interface{}, patterns []domain.ResponsePattern) (*string, string, map[string]interface{}) {
	status := "acked"
	var matched *string
	parsed := payload

	if len(patterns) == 0 {
		return matched, status, parsed
	}

	blob, _ := json.Marshal(payload)
	for _, p := range patterns {
		switch strings.ToLower(p.PatternType) {
		case "regex":
			re, err := regexp.Compile(p.Pattern)
			if err != nil {
				continue
			}
			if re.Match(blob) {
				matched = &p.ID
				status = ternaryString(p.Success, "acked", "failed")
				parsed = applyExtract(payload, p.Extract)
				return matched, status, parsed
			}
		case "jsonpath":
			if _, ok := matchJSONPath(payload, p.Pattern); ok {
				matched = &p.ID
				status = ternaryString(p.Success, "acked", "failed")
				parsed = applyExtract(payload, p.Extract)
				return matched, status, parsed
			}
		}
	}

	return matched, "failed", parsed
}

// applyExtract builds a parsed payload from extraction rules (destKey -> jsonpath).
func applyExtract(payload map[string]interface{}, rules map[string]any) map[string]interface{} {
	if len(rules) == 0 {
		return payload
	}
	out := make(map[string]interface{}, len(rules))
	for k, v := range rules {
		path, ok := v.(string)
		if !ok {
			continue
		}
		if val, ok := matchJSONPath(payload, path); ok {
			out[k] = val
		}
	}
	if len(out) == 0 {
		return payload
	}
	return out
}

// matchJSONPath provides a minimal dotted-path matcher (e.g., $.field.subfield or field.subfield).
func matchJSONPath(payload map[string]interface{}, path string) (interface{}, bool) {
	trimmed := strings.TrimPrefix(path, "$")
	trimmed = strings.TrimPrefix(trimmed, ".")
	parts := strings.Split(trimmed, ".")
	var current interface{} = payload
	for _, p := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = m[p]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func ternaryString(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func extractPayloadSchemas(project map[string]interface{}) map[string]payloadschema.PacketSchema {
	if project == nil {
		return nil
	}

	// Direct payloadSchemas on bundle-like object
	if ps := decodePacketSchemas(project["payloadSchemas"]); ps != nil {
		return ps
	}
	configVal, ok := project["config"]
	if !ok || configVal == nil {
		return nil
	}

	// Attempt direct map first
	if cfgMap, ok := configVal.(map[string]interface{}); ok {
		return decodePacketSchemas(cfgMap["payloadSchemas"])
	}

	// Fallback: json encoded
	switch raw := configVal.(type) {
	case []byte:
		if len(raw) == 0 {
			return nil
		}
		var tmp map[string]interface{}
		if err := json.Unmarshal(raw, &tmp); err == nil {
			return decodePacketSchemas(tmp["payloadSchemas"])
		}
	case string:
		if raw == "" {
			return nil
		}
		var tmp map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &tmp); err == nil {
			return decodePacketSchemas(tmp["payloadSchemas"])
		}
	}

	return nil
}

func extractSensorsFromBundle(bundle map[string]interface{}) []interface{} {
	if bundle == nil {
		return nil
	}
	if proj, ok := bundle["project"].(map[string]interface{}); ok {
		if hw, ok := proj["hardware"].(map[string]interface{}); ok {
			if sensors, ok := hw["sensors"].([]interface{}); ok {
				return sensors
			}
		}
	}
	// Legacy bundle shape could already be the project map
	if hw, ok := bundle["hardware"].(map[string]interface{}); ok {
		if sensors, ok := hw["sensors"].([]interface{}); ok {
			return sensors
		}
	}
	return nil
}

func extractPayloadSchemasFromBundle(bundle map[string]interface{}) map[string]payloadschema.PacketSchema {
	return decodePacketSchemas(bundle["payloadSchemas"])
}

func decodePacketSchemas(value interface{}) map[string]payloadschema.PacketSchema {
	rawMap, ok := value.(map[string]interface{})
	if !ok || len(rawMap) == 0 {
		return nil
	}
	result := make(map[string]payloadschema.PacketSchema, len(rawMap))

	for key, raw := range rawMap {
		packetMap, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}

		packet := payloadschema.PacketSchema{
			PacketType: key,
		}

		if v, ok := packetMap["packetType"].(string); ok && v != "" {
			packet.PacketType = v
		}
		if v, ok := packetMap["topicTemplate"].(string); ok {
			packet.TopicTemplate = v
		}

		packet.Keys = decodeKeySpecs(packetMap["keys"])
		packet.EnvelopeKeys = decodeKeySpecs(packetMap["envelopeKeys"])

		result[key] = packet
		if packet.PacketType != "" && packet.PacketType != key {
			result[packet.PacketType] = packet
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

func decodeKeySpecs(value interface{}) []payloadschema.KeySpec {
	list, ok := value.([]interface{})
	if !ok || len(list) == 0 {
		return nil
	}
	specs := make([]payloadschema.KeySpec, 0, len(list))
	for _, item := range list {
		if specMap, ok := item.(map[string]interface{}); ok {
			specs = append(specs, decodeKeySpec(specMap))
		}
	}
	if len(specs) == 0 {
		return nil
	}
	return specs
}

func decodeKeySpec(entry map[string]interface{}) payloadschema.KeySpec {
	spec := payloadschema.KeySpec{}
	if entry == nil {
		return spec
	}
	if key, ok := entry["key"].(string); ok {
		spec.Key = key
	}
	if desc, ok := entry["description"].(string); ok {
		spec.Description = desc
	}
	if unit, ok := entry["unit"].(string); ok {
		spec.Unit = unit
	}
	if req, ok := entry["required"].(bool); ok {
		spec.Required = req
	} else if reqNum, ok := entry["required"].(float64); ok {
		spec.Required = reqNum != 0
	}
	if ptr := toIntPointer(entry["maxLength"]); ptr != nil {
		spec.MaxLength = ptr
	}
	if notes, ok := entry["notes"].(string); ok {
		spec.Notes = notes
	}
	return spec
}

func toIntPointer(value interface{}) *int {
	switch v := value.(type) {
	case nil:
		return nil
	case int:
		val := v
		return &val
	case int32:
		val := int(v)
		return &val
	case int64:
		val := int(v)
		return &val
	case float64:
		val := int(v)
		return &val
	case json.Number:
		if i, err := v.Int64(); err == nil {
			val := int(i)
			return &val
		}
	}
	return nil
}

func detectPacketType(raw map[string]interface{}, topic string, schemas map[string]payloadschema.PacketSchema, imei, projectID string) string {
	if pt, ok := raw["packet_type"].(string); ok && pt != "" {
		return normalizePacketType(pt)
	}
	if meta, ok := raw["metadata"].(map[string]interface{}); ok {
		if pt, ok := meta["packet_type"].(string); ok && pt != "" {
			return normalizePacketType(pt)
		}
	}
	if inferred := inferPacketTypeFromTopic(topic, raw); inferred != "" {
		return inferred
	}
	for name, schema := range schemas {
		if schema.TopicTemplate == "" {
			continue
		}
		if matchTopic(schema.TopicTemplate, topic, imei, projectID) {
			return normalizePacketType(name)
		}
	}
	return ""
}

func normalizePacketType(packetType string) string {
	pt := strings.ToLower(strings.TrimSpace(packetType))
	switch pt {
	case "data":
		// Canonical external topic/type is data.
		return "data"
	case "ondemand_cmd", "ondemandcommand", "ondemand_command", "ondemandcmd":
		return "ondemand_command"
	case "ondemand_rsp", "ondemandresponse", "ondemand_response", "ondemandresp":
		return "ondemand_response"
	default:
		return strings.TrimSpace(packetType)
	}
}

func inferPacketTypeFromTopic(topic string, raw map[string]interface{}) string {
	parts := strings.Split(strings.TrimSpace(topic), "/")
	if len(parts) == 0 {
		return ""
	}

	suffix := ""
	if len(parts) >= 2 {
		suffix = strings.ToLower(strings.TrimSpace(parts[1]))
	}
	if len(parts) >= 5 && strings.TrimSpace(parts[0]) == "channels" && strings.TrimSpace(parts[2]) == "messages" {
		// channels/{project_id}/messages/{imei}/{suffix}
		suffix = strings.ToLower(strings.TrimSpace(parts[4]))
	}
	if len(parts) >= 4 && strings.TrimSpace(parts[0]) == "devices" {
		// devices/{imei}/telemetry/{suffix}
		// devices/{imei}/errors/{suffix}
		suffix = strings.ToLower(strings.TrimSpace(parts[3]))
	}

	switch suffix {
	case "heartbeat", "daq":
		return suffix
	case "data":
		return "data"
	case "errors":
		return "device_error"
	case "ondemand":
		// Both commands and responses are on the same topic; infer from payload shape.
		if isOndemandResponsePayload(raw) {
			return "ondemand_response"
		}
		return "ondemand_command"
	default:
		return ""
	}
}

func isSupportedPacketType(packetType string) bool {
	pt := strings.ToLower(strings.TrimSpace(packetType))
	switch pt {
	case "", "heartbeat", "data", "daq", "ondemand_command", "ondemand_response", "device_error", "forwarded_data":
		return true
	default:
		return false
	}
}

func hasUnsupportedTelemetrySuffix(topic string) bool {
	parts := strings.Split(strings.TrimSpace(topic), "/")
	if len(parts) == 0 {
		return false
	}

	suffix := ""
	if len(parts) == 2 && legacyIMEIPrefix.MatchString(strings.TrimSpace(parts[0])) {
		// Legacy: <imei>/{suffix}
		suffix = strings.ToLower(strings.TrimSpace(parts[1]))
	}
	if len(parts) >= 5 && strings.TrimSpace(parts[0]) == "channels" && strings.TrimSpace(parts[2]) == "messages" {
		// channels/{project_id}/messages/{imei}/{suffix}
		suffix = strings.ToLower(strings.TrimSpace(parts[4]))
	}
	if len(parts) >= 4 && strings.TrimSpace(parts[0]) == "devices" && (strings.TrimSpace(parts[2]) == "telemetry" || strings.TrimSpace(parts[2]) == "errors") {
		// devices/{imei}/telemetry/{suffix}
		// devices/{imei}/errors/{suffix}
		suffix = strings.ToLower(strings.TrimSpace(parts[3]))
	}

	if suffix == "" {
		return false
	}

	allowedSuffixes := map[string]bool{
		"heartbeat": true,
		"data":      true,
		"daq":       true,
		"ondemand":  true,
		"errors":    true,
	}
	return !allowedSuffixes[suffix]
}

func isOndemandTopic(topic string) bool {
	parts := strings.Split(strings.TrimSpace(topic), "/")
	if len(parts) < 2 {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(parts[1]), "ondemand")
}

func isOndemandResponsePayload(payload map[string]interface{}) bool {
	if payload == nil {
		return false
	}
	// Government legacy responses commonly have status/code fields.
	if _, ok := payload["status"]; ok {
		return true
	}
	if _, ok := payload["code"]; ok {
		return true
	}
	// Server published commands have cmd/command/params and should not be treated as responses.
	if _, ok := payload["cmd"]; ok {
		return false
	}
	if _, ok := payload["command"]; ok {
		return false
	}
	return false
}

func isServerPublishedOndemandCommand(raw map[string]interface{}, topic string) bool {
	if !isOndemandTopic(topic) || raw == nil {
		return false
	}
	// Strict legacy command echo: has cmd/type, but is not a response (no status/code).
	if isOndemandResponsePayload(raw) {
		return false
	}
	cmd, _ := raw["cmd"].(string)
	typ, _ := raw["type"].(string)
	if strings.TrimSpace(cmd) == "" || strings.TrimSpace(typ) == "" {
		return false
	}
	pt := normalizePacketType(typ)
	if pt != "ondemand_command" {
		return false
	}
	// msgid is required for a command; presence strongly indicates a server-issued command.
	if v, ok := raw["msgid"].(string); ok && strings.TrimSpace(v) != "" {
		return true
	}
	return false
}

func (s *IngestionService) inferLatestCommandCorrelation(deviceID string) string {
	if s == nil || s.commands == nil {
		return ""
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return ""
	}
	requests, err := s.commands.ListCommandRequests(deviceID, 5)
	if err != nil || len(requests) == 0 {
		return ""
	}
	now := time.Now()
	for _, req := range requests {
		if strings.TrimSpace(req.CorrelationID) == "" {
			continue
		}
		if req.CompletedAt != nil {
			continue
		}
		if now.Sub(req.CreatedAt) > 15*time.Minute {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(req.Status))
		if status != "published" && status != "queued" && status != "timeout" {
			// only auto-correlate to outstanding-ish commands
			continue
		}
		return req.CorrelationID
	}
	return ""
}

func matchTopic(template, topic, imei, projectID string) bool {
	if template == "" || topic == "" {
		return false
	}
	candidate := strings.ReplaceAll(template, "<IMEI>", imei)
	candidate = strings.ReplaceAll(candidate, "{imei}", imei)
	candidate = strings.ReplaceAll(candidate, "{IMEI}", imei)
	candidate = strings.ReplaceAll(candidate, "{projectId}", projectID)
	candidate = strings.ReplaceAll(candidate, "{PROJECT_ID}", projectID)
	return candidate == topic
}

// validatePacketQuality implements dynamic checks against Project DNA
func (s *IngestionService) validatePacketQuality(data map[string]interface{}, sensors []interface{}, packetSchema *payloadschema.PacketSchema) validationReport {
	report := validationReport{}

	for key, v := range data {
		if v == nil {
			log.Printf("[Ingestion] nil value for key %s", key)
			report.NilKeys = append(report.NilKeys, key)
		}
	}

	builtin := []string{"timestamp", "TIMESTAMP", "imei", "IMEI", "project_id", "PROJECT_ID", "msgid", "MSGID", "packet_type", "PACKET_TYPE", "device_uuid", "DEVICE_UUID", "metadata", "METADATA"}
	allowed := make(map[string]bool)
	for _, key := range builtin {
		allowed[key] = true
	}

	if packetSchema != nil {
		for _, spec := range packetSchema.EnvelopeKeys {
			allowed[spec.Key] = true
		}
		for _, spec := range packetSchema.Keys {
			allowed[spec.Key] = true
			if spec.Required {
				if _, exists := data[spec.Key]; !exists {
					report.Missing = append(report.Missing, spec.Key)
				}
			}
			if spec.MaxLength != nil {
				if value, ok := data[spec.Key].(string); ok {
					if len(value) > *spec.MaxLength {
						report.Oversized = append(report.Oversized, spec.Key)
					}
				}
			}
		}
	} else if len(sensors) > 0 {
		for _, sens := range sensors {
			if sMap, ok := sens.(map[string]interface{}); ok {
				if param, ok := sMap["param"].(string); ok && param != "" {
					allowed[param] = true
				}
				if legacyID, ok := sMap["id"].(string); ok && legacyID != "" {
					allowed[legacyID] = true
				}
			}
		}
	}

	for key := range data {
		if !allowed[key] {
			report.Unknown = append(report.Unknown, key)
		}
	}

	if isForwardedTelemetry(data) {
		validateForwardedRouting(data, &report)
	}

	if !report.ok() {
		log.Printf("[Ingestion] payload schema check failed (missing=%v oversize=%v unknown=%v nil=%v)", report.Missing, report.Oversized, report.Unknown, report.NilKeys)
	}

	return report
}

func isForwardedTelemetry(data map[string]interface{}) bool {
	if pt, ok := data["packet_type"].(string); ok && strings.EqualFold(strings.TrimSpace(pt), "forwarded_data") {
		return true
	}
	meta, ok := data["metadata"].(map[string]interface{})
	if !ok || meta == nil {
		return false
	}
	if forwarded, ok := meta["forwarded"].(bool); ok && forwarded {
		return true
	}
	if pt, ok := meta["packet_type"].(string); ok && strings.EqualFold(strings.TrimSpace(pt), "forwarded_data") {
		return true
	}
	return false
}

func validateForwardedRouting(data map[string]interface{}, report *validationReport) {
	meta, ok := data["metadata"].(map[string]interface{})
	if !ok || meta == nil {
		report.Missing = append(report.Missing, "metadata")
		return
	}

	if forwarded, ok := meta["forwarded"].(bool); !ok || !forwarded {
		report.Missing = append(report.Missing, "metadata.forwarded")
	}

	origin, _ := meta["origin_imei"].(string)
	originNodeID, _ := meta["origin_node_id"].(string)
	if strings.TrimSpace(origin) == "" && strings.TrimSpace(originNodeID) == "" {
		// Backwards: origin_imei. Forwarded-node: origin_node_id.
		report.Missing = append(report.Missing, "metadata.origin_imei|metadata.origin_node_id")
	}

	route, ok := meta["route"].(map[string]interface{})
	if !ok || route == nil {
		report.Missing = append(report.Missing, "metadata.route")
		return
	}

	path, ok := route["path"].([]interface{})
	if !ok || len(path) == 0 {
		report.Missing = append(report.Missing, "metadata.route.path")
	} else {
		for _, hop := range path {
			hopID, ok := hop.(string)
			if !ok || strings.TrimSpace(hopID) == "" {
				report.Missing = append(report.Missing, "metadata.route.path")
				break
			}
		}
	}

	if !hasNonNegativeInt(route["hops"]) {
		report.Missing = append(report.Missing, "metadata.route.hops")
	}

	ingress, ok := route["ingress"].(string)
	if !ok || strings.TrimSpace(ingress) == "" {
		report.Missing = append(report.Missing, "metadata.route.ingress")
	}
}

func hasNonNegativeInt(value interface{}) bool {
	switch typed := value.(type) {
	case int:
		return typed >= 0
	case int32:
		return typed >= 0
	case int64:
		return typed >= 0
	case float32:
		return typed >= 0 && float64(int(typed)) == float64(typed)
	case float64:
		return typed >= 0 && float64(int(typed)) == typed
	default:
		return false
	}
}

func normalizeForwardedTelemetryPayload(processed map[string]interface{}, raw map[string]interface{}, topic, gatewayIMEI string) {
	if processed == nil {
		return
	}

	if !isForwardedTelemetry(processed) && isForwardedTelemetry(raw) {
		if _, ok := processed["packet_type"]; !ok {
			if value, ok := raw["packet_type"]; ok {
				processed["packet_type"] = value
			}
		}
		if _, ok := processed["metadata"]; !ok {
			if metaRaw, ok := raw["metadata"].(map[string]interface{}); ok && metaRaw != nil {
				processed["metadata"] = cloneStringMap(metaRaw)
			}
		}
	}

	if !isForwardedTelemetry(processed) {
		return
	}

	meta := ensureChildMap(processed, "metadata")
	meta["forwarded"] = true

	origin := firstNonEmptyString(
		meta["origin_node_id"],
		processed["origin_node_id"],
		readNestedString(raw, "metadata", "origin_node_id"),
		raw["origin_node_id"],
		meta["origin_imei"],
		processed["origin_imei"],
		readNestedString(raw, "metadata", "origin_imei"),
		raw["origin_imei"],
		meta["origin_node_imei"],
		readNestedString(raw, "metadata", "origin_node_imei"),
	)
	if origin != "" {
		// If it looks like a non-IMEI identifier, store as node_id; otherwise store as origin_imei.
		trimmed := strings.TrimSpace(origin)
		isProbablyImei := true
		for _, r := range trimmed {
			if r < '0' || r > '9' {
				isProbablyImei = false
				break
			}
		}
		if isProbablyImei {
			meta["origin_imei"] = trimmed
		} else {
			meta["origin_node_id"] = trimmed
		}
	}

	route := ensureChildMap(meta, "route")

	path := coercePath(route["path"])
	if len(path) == 0 {
		fallbackPath := coercePath(readNested(raw, "metadata", "route", "path"))
		if len(fallbackPath) > 0 {
			path = fallbackPath
		}
	}
	if len(path) == 0 {
		path = []interface{}{gatewayIMEI}
	}
	route["path"] = path

	if !hasNonNegativeInt(route["hops"]) {
		route["hops"] = len(path) - 1
	}

	ingress := firstNonEmptyString(route["ingress"], readNested(raw, "metadata", "route", "ingress"), meta["ingress"])
	if ingress == "" {
		ingress = inferIngress(topic)
	}
	route["ingress"] = ingress
}

func cloneStringMap(source map[string]interface{}) map[string]interface{} {
	cloned := make(map[string]interface{}, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func ensureChildMap(target map[string]interface{}, key string) map[string]interface{} {
	if current, ok := target[key].(map[string]interface{}); ok && current != nil {
		return current
	}
	child := map[string]interface{}{}
	target[key] = child
	return child
}

func firstNonEmptyString(values ...interface{}) string {
	for _, value := range values {
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func readNested(root map[string]interface{}, keys ...string) interface{} {
	if root == nil {
		return nil
	}
	var current interface{} = root
	for _, key := range keys {
		mapped, ok := current.(map[string]interface{})
		if !ok || mapped == nil {
			return nil
		}
		current = mapped[key]
	}
	return current
}

func readNestedString(root map[string]interface{}, keys ...string) string {
	if value, ok := readNested(root, keys...).(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func coercePath(value interface{}) []interface{} {
	switch typed := value.(type) {
	case []interface{}:
		out := make([]interface{}, 0, len(typed))
		for _, entry := range typed {
			if hop, ok := entry.(string); ok && strings.TrimSpace(hop) != "" {
				out = append(out, strings.TrimSpace(hop))
			}
		}
		return out
	case []string:
		out := make([]interface{}, 0, len(typed))
		for _, hop := range typed {
			if strings.TrimSpace(hop) != "" {
				out = append(out, strings.TrimSpace(hop))
			}
		}
		return out
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		splitter := ","
		if strings.Contains(typed, ">") {
			splitter = ">"
		}
		parts := strings.Split(typed, splitter)
		out := make([]interface{}, 0, len(parts))
		for _, part := range parts {
			if strings.TrimSpace(part) != "" {
				out = append(out, strings.TrimSpace(part))
			}
		}
		return out
	default:
		return nil
	}
}

func inferIngress(topic string) string {
	if strings.HasPrefix(topic, "channels/") {
		return "mqtt/channels"
	}
	if strings.HasPrefix(topic, "devices/") {
		return "mqtt/devices"
	}
	if strings.HasPrefix(topic, "https/") {
		return "https"
	}
	if _, imei := inferIMEIFromTopic(topic); imei != "" {
		return "mqtt/legacy"
	}
	return "legacy/unknown"
}

func resolvePacketEventTime(processed map[string]interface{}, raw map[string]interface{}) time.Time {
	for _, key := range []string{"ts", "timestamp", "TIMESTAMP", "time"} {
		if parsed, ok := parsePacketTimestamp(readMapValue(processed, key)); ok {
			return parsed.UTC()
		}
		if parsed, ok := parsePacketTimestamp(readMapValue(raw, key)); ok {
			return parsed.UTC()
		}
	}
	return time.Now().UTC()
}

func readMapValue(source map[string]interface{}, key string) interface{} {
	if source == nil {
		return nil
	}
	return source[key]
}

func parsePacketTimestamp(value interface{}) (time.Time, bool) {
	switch typed := value.(type) {
	case nil:
		return time.Time{}, false
	case time.Time:
		return typed, true
	case int:
		return epochNumberToTime(float64(typed))
	case int32:
		return epochNumberToTime(float64(typed))
	case int64:
		return epochNumberToTime(float64(typed))
	case float32:
		return epochNumberToTime(float64(typed))
	case float64:
		return epochNumberToTime(typed)
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return time.Time{}, false
		}
		if parsed, err := time.Parse(time.RFC3339Nano, text); err == nil {
			return parsed, true
		}
		if parsed, err := time.Parse(time.RFC3339, text); err == nil {
			return parsed, true
		}
		if n, err := strconv.ParseFloat(text, 64); err == nil {
			return epochNumberToTime(n)
		}
	}
	return time.Time{}, false
}

func epochNumberToTime(n float64) (time.Time, bool) {
	if !isFinitePositive(n) {
		return time.Time{}, false
	}

	// Heuristic by magnitude:
	// >=1e18: nanoseconds, >=1e15: microseconds, >=1e12: milliseconds, else seconds.
	if n >= 1e18 {
		return time.Unix(0, int64(n)).UTC(), true
	}
	if n >= 1e15 {
		return time.UnixMicro(int64(n)).UTC(), true
	}
	if n >= 1e12 {
		return time.UnixMilli(int64(n)).UTC(), true
	}
	return time.Unix(int64(n), 0).UTC(), true
}

func isFinitePositive(value float64) bool {
	return value > 0 && value < 1e20
}
