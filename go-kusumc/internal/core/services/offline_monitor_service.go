package services

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"ingestion-go/internal/adapters/secondary"
)

type OfflineMonitorService struct {
	repo             *secondary.PostgresRepo
	protocols        *ProtocolService
	interval         time.Duration
	offlineThreshold time.Duration
	batchLimit       int
}

func NewOfflineMonitorService(repo *secondary.PostgresRepo, protocols *ProtocolService) *OfflineMonitorService {
	interval := 5 * time.Minute
	threshold := 24 * time.Hour
	batchLimit := 200

	if v := os.Getenv("OFFLINE_MONITOR_INTERVAL_MINUTES"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			interval = time.Duration(parsed) * time.Minute
		}
	}
	if v := os.Getenv("OFFLINE_THRESHOLD_MINUTES"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			threshold = time.Duration(parsed) * time.Minute
		}
	}
	if v := os.Getenv("OFFLINE_MONITOR_BATCH_LIMIT"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			batchLimit = parsed
		}
	}

	return &OfflineMonitorService{repo: repo, protocols: protocols, interval: interval, offlineThreshold: threshold, batchLimit: batchLimit}
}

func (s *OfflineMonitorService) Start() {
	if s.repo == nil {
		return
	}
	log.Printf("[OfflineMonitor] Started (interval=%s threshold=%s)", s.interval, s.offlineThreshold)
	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			s.runOnce()
			<-ticker.C
		}
	}()
}

func (s *OfflineMonitorService) runOnce() {
	defaultThresholdSeconds := int64(s.offlineThreshold.Seconds())
	devices, err := s.repo.ListOfflineCandidates(defaultThresholdSeconds, s.batchLimit)
	if err != nil {
		log.Printf("[OfflineMonitor] Error fetching candidates: %v", err)
		return
	}
	if len(devices) == 0 {
		return
	}

	policyCache := map[string]offlinePolicy{}

	for _, device := range devices {
		deviceID, _ := device["id"].(string)
		deviceUUID, _ := device["id"].(string)
		imei, _ := device["imei"].(string)
		projectID, _ := device["project_id"].(string)
		if deviceID == "" {
			continue
		}

		policy := s.resolvePolicy(projectID, policyCache)
		thresholdSeconds := policy.ThresholdSec
		if thresholdSeconds <= 0 {
			thresholdSeconds = defaultThresholdSeconds
		}

		lastSeen, ok := device["last_seen"].(time.Time)
		if ok && !lastSeen.IsZero() {
			if time.Since(lastSeen) < time.Duration(thresholdSeconds)*time.Second {
				continue
			}
		}

		if err := s.repo.MarkDeviceOffline(deviceID); err != nil {
			log.Printf("[OfflineMonitor] Failed to mark device %s offline: %v", deviceID, err)
			continue
		}

		payload := map[string]interface{}{
			"reason":       "connectivity-offline",
			"device_id":    deviceID,
			"device_uuid":  deviceUUID,
			"imei":         imei,
			"project_id":   projectID,
			"thresholdSec": thresholdSeconds,
			"policy": map[string]interface{}{
				"source":        policy.Source,
				"protocol_kind": policy.ProtocolKind,
			},
		}
		metadata := map[string]interface{}{
			"event":         "device_offline",
			"project_id":    projectID,
			"policy_source": policy.Source,
			"protocol_kind": policy.ProtocolKind,
		}

		channel := policy.Channel
		if channel == "" {
			channel = "queue"
		}
		target := policy.Target
		if target == "" {
			target = "default"
		}

		if err := s.repo.EnqueueNotification(deviceID, deviceUUID, channel, target, "offline-monitor", payload, time.Now(), nil, metadata); err != nil {
			log.Printf("[OfflineMonitor] Failed to enqueue notification for %s: %v", deviceID, err)
		}
	}
}

type offlinePolicy struct {
	ThresholdSec int64
	Channel      string
	Target       string
	Source       string
	ProtocolKind string
}

func (s *OfflineMonitorService) resolvePolicy(projectID string, cache map[string]offlinePolicy) offlinePolicy {
	if strings.TrimSpace(projectID) == "" {
		return offlinePolicy{Source: "default"}
	}
	if existing, ok := cache[projectID]; ok {
		return existing
	}

	policy := offlinePolicy{Source: "default"}
	if s.protocols == nil {
		cache[projectID] = policy
		return policy
	}

	profiles, err := s.protocols.ListByProject(context.Background(), projectID)
	if err != nil || len(profiles) == 0 {
		cache[projectID] = policy
		return policy
	}

	selected := profiles[0]
	for _, profile := range profiles {
		if strings.EqualFold(strings.TrimSpace(profile.Kind), "primary") {
			selected = profile
			break
		}
	}
	policy.ProtocolKind = strings.TrimSpace(selected.Kind)

	if selected.Metadata != nil {
		if v, ok := readInt64(selected.Metadata, "offline_threshold_sec", "offlineThresholdSec"); ok && v > 0 {
			policy.ThresholdSec = v
			policy.Source = "protocol_metadata"
		}
		if ch, ok := readString(selected.Metadata, "offline_channel", "offlineChannel"); ok {
			policy.Channel = ch
			if policy.Source == "default" {
				policy.Source = "protocol_metadata"
			}
		}
		if t, ok := readString(selected.Metadata, "offline_target", "offlineTarget"); ok {
			policy.Target = t
			if policy.Source == "default" {
				policy.Source = "protocol_metadata"
			}
		}
	}

	cache[projectID] = policy
	return policy
}

func readString(meta map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		if v, ok := meta[key]; ok {
			if s, ok := v.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					return s, true
				}
			}
		}
	}
	return "", false
}

func readInt64(meta map[string]any, keys ...string) (int64, bool) {
	for _, key := range keys {
		if v, ok := meta[key]; ok {
			switch t := v.(type) {
			case int:
				return int64(t), true
			case int32:
				return int64(t), true
			case int64:
				return t, true
			case float64:
				return int64(t), true
			case string:
				parsed, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
				if err == nil {
					return parsed, true
				}
			}
		}
	}
	return 0, false
}
