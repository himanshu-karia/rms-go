package workers

import (
	"context"
	"fmt"
	"ingestion-go/internal/adapters/secondary"
	"ingestion-go/internal/core/services"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type MqttWorker struct {
	repo       *secondary.PostgresRepo
	emqx       *secondary.EmqxAdapter
	protocols  *services.ProtocolService
	pollTicker *time.Ticker
	statsMu    sync.Mutex
	stats      MqttWorkerStats
}

type MqttWorkerStats struct {
	PollIntervalMS      int64            `json:"poll_interval_ms"`
	JobsProcessed       int64            `json:"jobs_processed"`
	JobsCompleted       int64            `json:"jobs_completed"`
	JobsFailed          int64            `json:"jobs_failed"`
	JobsRetried         int64            `json:"jobs_retried"`
	LastJobID           string           `json:"last_job_id,omitempty"`
	LastDeviceID        string           `json:"last_device_id,omitempty"`
	LastTrigger         string           `json:"last_trigger,omitempty"`
	LastErrorClass      string           `json:"last_error_class,omitempty"`
	ConsecutiveFailures int64            `json:"consecutive_failures"`
	TriggerBreakdown    map[string]int64 `json:"trigger_breakdown,omitempty"`
	LastStatus          string           `json:"last_status,omitempty"`
	LastError           string           `json:"last_error,omitempty"`
	LastUpdatedAt       string           `json:"last_updated_at,omitempty"`
}

func NewMqttWorker(repo *secondary.PostgresRepo, protocols *services.ProtocolService) *MqttWorker {
	return &MqttWorker{
		repo:      repo,
		emqx:      secondary.NewEmqxAdapter(),
		protocols: protocols,
	}
}

func (w *MqttWorker) Start() {
	log.Println("[MqttWorker] Started Background Worker")
	pollInterval := 5 * time.Second
	if env := os.Getenv("MQTT_JOB_POLL_INTERVAL_MS"); env != "" {
		if v, err := strconv.Atoi(env); err == nil && v > 0 {
			pollInterval = time.Duration(v) * time.Millisecond
		}
	}
	w.pollTicker = time.NewTicker(pollInterval)
	w.statsMu.Lock()
	w.stats.PollIntervalMS = pollInterval.Milliseconds()
	w.statsMu.Unlock()

	go func() {
		for range w.pollTicker.C {
			w.processNextJob()
		}
	}()
}

func (w *MqttWorker) RuntimeStats() MqttWorkerStats {
	w.statsMu.Lock()
	defer w.statsMu.Unlock()
	return w.stats
}

func (w *MqttWorker) processNextJob() {
	// 1. Fetch Job (Atomically Locked)
	job, err := w.repo.GetNextProvisioningJob()
	if err != nil {
		log.Printf("[MqttWorker] Error fetching job: %v", err)
		return
	}
	if job == nil {
		return // No jobs
	}

	jobID := job["id"].(string)
	deviceID := job["device_id"].(string)
	attempts := job["attempt_count"].(int)
	triggerKind, _ := job["trigger_kind"].(string)
	credHistoryID, _ := job["credential_history_id"].(string)
	w.recordJobStart(jobID, deviceID, triggerKind, attempts)

	log.Printf("[MqttWorker] Processing Job %s for Device %s", jobID, deviceID)

	// 2. Fetch Device Details
	dev, err := w.repo.GetDeviceByID(deviceID)
	if err != nil || dev == nil {
		w.failJob(jobID, credHistoryID, "Device not found", attempts)
		return
	}

	// Track prior credentials so we can disable them on rotation
	var oldUsername string
	if authMap, ok := dev["auth"].(map[string]interface{}); ok {
		oldUsername, _ = authMap["username"].(string)
	}

	// 3. Extract Creds & ACLs
	var username, password string
	var clientID string
	var pubTopics, subTopics []string

	if credHistoryID != "" {
		if cred, err := w.repo.GetCredentialHistory(credHistoryID); err == nil {
			if bundle, ok := cred["bundle"].(map[string]interface{}); ok {
				username, _ = bundle["username"].(string)
				password, _ = bundle["password"].(string)
				clientID, _ = bundle["client_id"].(string)
				pubTopics = extractTopicList(bundle, "publish_topics")
				subTopics = extractTopicList(bundle, "subscribe_topics")
			}
		}
	}

	if username == "" || password == "" {
		// Fallback to device attributes
		if authMap, ok := dev["auth"].(map[string]interface{}); ok {
			username, _ = authMap["username"].(string)
			password, _ = authMap["password"].(string)
			clientID, _ = authMap["client_id"].(string)
			pubTopics = extractTopicList(authMap, "publish_topics")
			subTopics = extractTopicList(authMap, "subscribe_topics")
		}
	}

	pid, _ := dev["project_id"].(string)
	imei, _ := dev["imei"].(string)

	ensureTopic := func(topics []string, topic string) []string {
		want := strings.TrimSpace(topic)
		if want == "" {
			return topics
		}
		for _, t := range topics {
			if strings.TrimSpace(t) == want {
				return topics
			}
		}
		return append(topics, want)
	}

	if username == "" || password == "" {
		w.failJob(jobID, credHistoryID, "Missing credentials", attempts)
		return
	}

	expandTopicTemplates := func(topics []string, projectID, imei string) []string {
		if len(topics) == 0 {
			return topics
		}
		out := make([]string, 0, len(topics))
		for _, t := range topics {
			if strings.TrimSpace(t) == "" {
				continue
			}
			x := t
			x = strings.ReplaceAll(x, "<IMEI>", imei)
			x = strings.ReplaceAll(x, "{imei}", imei)
			x = strings.ReplaceAll(x, "{IMEI}", imei)
			x = strings.ReplaceAll(x, "{project_id}", projectID)
			x = strings.ReplaceAll(x, "{PROJECT_ID}", projectID)
			x = strings.ReplaceAll(x, "{projectId}", projectID)
			out = append(out, x)
		}
		return out
	}

	// Derive topics at runtime (prefer protocol profile topics when available), so
	// we don't rely on persisted topic lists in credential_history/device attrs.
	if w.protocols != nil && pid != "" {
		if list, err := w.protocols.ListByProject(context.Background(), pid); err == nil {
			for i := range list {
				p := list[i]
				if p.Kind == "primary" {
					if len(pubTopics) == 0 {
						pubTopics = expandTopicTemplates(p.PublishTopics, pid, imei)
						pubTopics = filterUnsupportedPublishTopics(pubTopics)
					}
					if len(subTopics) == 0 {
						subTopics = expandTopicTemplates(p.SubscribeTopics, pid, imei)
					}
					break
				}
			}
		}
	}

	// Final fallback topics.
	if len(pubTopics) == 0 {
		pubTopics = []string{
			fmt.Sprintf("%s/heartbeat", imei),
			fmt.Sprintf("%s/data", imei),
			fmt.Sprintf("%s/daq", imei),
			fmt.Sprintf("%s/ondemand", imei),
			fmt.Sprintf("%s/errors", imei),
		}
	}
	pubTopics = filterUnsupportedPublishTopics(pubTopics)
	if len(subTopics) == 0 {
		subTopics = []string{fmt.Sprintf("%s/ondemand", imei)}
	}

	// Safety: always allow the dedicated errors lane in legacy topic mode.
	// This keeps ACLs aligned with the firmware contract even if protocol profiles omit it.
	pubTopics = ensureTopic(pubTopics, fmt.Sprintf("%s/errors", imei))

	// 4. Disable old credentials if rotating to a new username
	if oldUsername != "" && oldUsername != username {
		clients, err := w.emqx.ListClientsByUsername(oldUsername)
		if err != nil {
			w.failJob(jobID, credHistoryID, fmt.Sprintf("EMQX ListClients (old user) Error: %v", err), attempts)
			return
		}
		for _, cid := range clients {
			if err := w.emqx.KillSession(cid); err != nil {
				w.failJob(jobID, credHistoryID, fmt.Sprintf("EMQX KillSession (old user) Error: %v", err), attempts)
				return
			}
		}
		if err := w.emqx.DeleteACL(oldUsername); err != nil {
			w.failJob(jobID, credHistoryID, fmt.Sprintf("EMQX DeleteACL (old user) Error: %v", err), attempts)
			return
		}
		if err := w.emqx.DeleteUser(oldUsername); err != nil {
			w.failJob(jobID, credHistoryID, fmt.Sprintf("EMQX DeleteUser (old user) Error: %v", err), attempts)
			return
		}
	}

	// 5. Call EMQX for new credentials
	// A. User
	if err := w.emqx.ProvisionDevice(username, password); err != nil {
		w.failJob(jobID, credHistoryID, fmt.Sprintf("EMQX Auth Error: %v", err), attempts)
		return
	}

	// B. ACL
	if err := w.emqx.UpdateACL(username, pubTopics, subTopics); err != nil {
		w.failJob(jobID, credHistoryID, fmt.Sprintf("EMQX ACL Error: %v", err), attempts)
		return
	}

	// C. Kill existing sessions so reconnect requires new creds.
	// Always kill all currently connected clients for the effective username,
	// because devices may reconnect with custom/ephemeral client IDs.
	clients, err := w.emqx.ListClientsByUsername(username)
	if err != nil {
		w.failJob(jobID, credHistoryID, fmt.Sprintf("EMQX ListClients Error: %v", err), attempts)
		return
	}

	if clientID != "" {
		clients = append(clients, clientID)
	}

	seen := make(map[string]struct{}, len(clients))
	for _, cid := range clients {
		cid = strings.TrimSpace(cid)
		if cid == "" {
			continue
		}
		if _, ok := seen[cid]; ok {
			continue
		}
		seen[cid] = struct{}{}
		if err := w.emqx.KillSession(cid); err != nil {
			w.failJob(jobID, credHistoryID, fmt.Sprintf("EMQX KillSession Error: %v", err), attempts)
			return
		}
	}

	// 6. Success
	log.Printf("[MqttWorker] Job %s Complete. Device %s provisioned.", jobID, imei)
	if credHistoryID != "" {
		_ = w.repo.MarkCredentialApplied(credHistoryID)
	}
	if err := w.repo.UpdateMqttJob(jobID, "completed", "", nil, ""); err != nil {
		log.Printf("[MqttWorker] Failed to mark completed for job %s: %v", jobID, err)
	}
	w.recordJobComplete(jobID, deviceID)
}

func (w *MqttWorker) failJob(id, credHistoryID, reason string, attempts int) {
	log.Printf("[MqttWorker] Job %s Failed: %s", id, reason)
	errCategory := classifyProvisioningError(reason)
	w.recordJobFailure(id, reason, errCategory)

	// Backoff: 5s, 10s, 20s...
	delay := time.Duration(math.Pow(2, float64(attempts))) * 5 * time.Second
	next := time.Now().Add(delay)

	if err := w.repo.UpdateMqttJob(id, "failed", reason, &next, errCategory); err != nil {
		log.Printf("[MqttWorker] Failed to mark failed for job %s: %v", id, err)
	}
	if credHistoryID != "" {
		_ = w.repo.MarkCredentialFailure(credHistoryID, reason)
	}
}

func (w *MqttWorker) recordJobStart(jobID, deviceID, triggerKind string, attempts int) {
	w.statsMu.Lock()
	defer w.statsMu.Unlock()
	if w.stats.TriggerBreakdown == nil {
		w.stats.TriggerBreakdown = map[string]int64{}
	}
	trigger := strings.TrimSpace(triggerKind)
	if trigger == "" {
		trigger = "initial"
	}
	w.stats.JobsProcessed++
	w.stats.TriggerBreakdown[trigger]++
	if attempts > 0 {
		w.stats.JobsRetried++
	}
	w.stats.LastJobID = jobID
	w.stats.LastDeviceID = deviceID
	w.stats.LastTrigger = trigger
	w.stats.LastStatus = "processing"
	w.stats.LastError = ""
	w.stats.LastErrorClass = ""
	w.stats.LastUpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

func (w *MqttWorker) recordJobComplete(jobID, deviceID string) {
	w.statsMu.Lock()
	defer w.statsMu.Unlock()
	w.stats.JobsCompleted++
	w.stats.LastJobID = jobID
	w.stats.LastDeviceID = deviceID
	w.stats.LastStatus = "completed"
	w.stats.LastError = ""
	w.stats.LastErrorClass = ""
	w.stats.ConsecutiveFailures = 0
	w.stats.LastUpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

func (w *MqttWorker) recordJobFailure(jobID, reason, errClass string) {
	w.statsMu.Lock()
	defer w.statsMu.Unlock()
	w.stats.JobsFailed++
	w.stats.ConsecutiveFailures++
	w.stats.LastJobID = jobID
	w.stats.LastStatus = "failed"
	w.stats.LastError = strings.TrimSpace(reason)
	w.stats.LastErrorClass = strings.TrimSpace(errClass)
	w.stats.LastUpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

func classifyProvisioningError(reason string) string {
	r := strings.ToLower(strings.TrimSpace(reason))
	switch {
	case strings.Contains(r, "auth") || strings.Contains(r, "not authorized") || strings.Contains(r, "password"):
		return "auth"
	case strings.Contains(r, "acl"):
		return "acl"
	case strings.Contains(r, "session") || strings.Contains(r, "client"):
		return "session"
	case strings.Contains(r, "device not found"):
		return "device"
	case strings.Contains(r, "timeout") || strings.Contains(r, "connect") || strings.Contains(r, "network"):
		return "network"
	default:
		return "unknown"
	}
}

func extractTopicList(src map[string]interface{}, key string) []string {
	vals := []string{}
	if raw, ok := src[key]; ok {
		switch v := raw.(type) {
		case []interface{}:
			for _, item := range v {
				if s, ok := item.(string); ok {
					vals = append(vals, s)
				}
			}
		case []string:
			vals = append(vals, v...)
		}
	}
	return vals
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
