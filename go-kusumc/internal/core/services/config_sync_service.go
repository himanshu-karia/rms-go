package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"ingestion-go/internal/adapters/secondary"
	"ingestion-go/internal/config/dna"
	"ingestion-go/internal/config/payloadschema"
	"ingestion-go/internal/models"
)

type projectConfigSource interface {
	GetAutomationFlow(projectId string) (map[string]interface{}, error)
	GetRules(projectId, deviceId string) ([]map[string]interface{}, error)
}

type thresholdRepository interface {
	ListDnaThresholds(ctx context.Context, projectID, scope string, deviceID *string) ([]models.DnaThreshold, error)
	ListDnaThresholdDeviceIDs(ctx context.Context, projectID string) ([]string, error)
}

type thresholdBundle struct {
	Project []models.DnaThreshold            `json:"project,omitempty"`
	Devices map[string][]models.DnaThreshold `json:"devices,omitempty"`
}

type projectConfigBundle struct {
	ProjectID      string                                `json:"projectId"`
	Project        map[string]interface{}                `json:"project,omitempty"`
	PayloadSchemas map[string]payloadschema.PacketSchema `json:"payloadSchemas,omitempty"`
	Thresholds     *thresholdBundle                      `json:"thresholds,omitempty"`
	Rules          []map[string]interface{}              `json:"rules,omitempty"`
	Automation     map[string]interface{}                `json:"automation,omitempty"`
	UpdatedAt      time.Time                             `json:"updatedAt"`
}

type ConfigSyncService struct {
	projRepo      *secondary.PostgresProjectRepo
	redisStore    *secondary.RedisStore
	dnaRepo       dna.Repository
	configRepo    projectConfigSource
	thresholdRepo thresholdRepository
	bundleEnabled bool
	cacheMu       sync.RWMutex
	scopeCache    map[string]payloadschema.ScopeSchema
	cacheLoaded   time.Time
	cacheTTL      time.Duration
}

func NewConfigSyncService(projRepo *secondary.PostgresProjectRepo, redisStore *secondary.RedisStore, dnaRepo dna.Repository, cfgRepo projectConfigSource, thresholdRepo thresholdRepository, bundleEnabled bool) *ConfigSyncService {
	return &ConfigSyncService{projRepo: projRepo, redisStore: redisStore, dnaRepo: dnaRepo, configRepo: cfgRepo, thresholdRepo: thresholdRepo, bundleEnabled: bundleEnabled, cacheTTL: 2 * time.Second}
}

func (s *ConfigSyncService) SyncAll() {
	log.Println("[ConfigSync] 🚀 Starting Full Sync...")

	scopeSchemas := s.resolveScopeSchemas()

	// 1. Fetch from Postgres
	projects, err := s.projRepo.GetAllProjectsWithConfig()
	if err != nil {
		log.Printf("[ConfigSync] Error fetching projects: %v", err)
		return
	}

	// 2. Sync to Redis
	count := 0
	for _, p := range projects {
		rawID, _ := p["id"].(string)
		projectID := strings.TrimSpace(rawID)
		if projectID == "" {
			continue
		}

		if s.syncProjectPayload(p, scopeSchemas) {
			count++
		}
		s.syncAutomationFlowCache(projectID)
		s.syncRulesCache(projectID)
		s.syncThresholdCache(projectID)
		s.syncConfigBundle(p, scopeSchemas)
	}

	log.Printf("[ConfigSync] ✅ Synced %d Projects to Redis.", count)
}

// SyncProject refreshes a single project record in Redis.
func (s *ConfigSyncService) SyncProject(projectID string) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return
	}

	scopeSchemas := s.resolveScopeSchemas()

	p, err := s.projRepo.GetProjectWithConfig(projectID)
	if err != nil {
		log.Printf("[ConfigSync] Error fetching project %s: %v", projectID, err)
		return
	}

	if s.syncProjectPayload(p, scopeSchemas) {
		log.Printf("[ConfigSync] Synced project %s to Redis", projectID)
	}

	s.syncAutomationFlowCache(projectID)
	s.syncRulesCache(projectID)
	s.syncThresholdCache(projectID)
	s.syncConfigBundle(p, scopeSchemas)
}

func (s *ConfigSyncService) syncThresholdCache(projectID string) {
	if s.redisStore == nil || s.thresholdRepo == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	defaults, err := s.thresholdRepo.ListDnaThresholds(ctx, projectID, "project", nil)
	if err != nil {
		log.Printf("[ConfigSync] Failed to load project thresholds for %s: %v", projectID, err)
		return
	}

	data, err := json.Marshal(defaults)
	if err != nil {
		log.Printf("[ConfigSync] Failed to marshal project thresholds for %s: %v", projectID, err)
	} else {
		key := fmt.Sprintf("config:thresholds:%s", projectID)
		if err := s.redisStore.SetRaw(key, string(data), 0); err != nil {
			log.Printf("[ConfigSync] Failed to cache thresholds for %s: %v", projectID, err)
		}
	}

	deviceIDs, err := s.thresholdRepo.ListDnaThresholdDeviceIDs(ctx, projectID)
	if err != nil {
		log.Printf("[ConfigSync] Failed to list device overrides for %s: %v", projectID, err)
		return
	}
	for _, devID := range deviceIDs {
		dev := devID
		rows, err := s.thresholdRepo.ListDnaThresholds(ctx, projectID, "device", &dev)
		if err != nil {
			log.Printf("[ConfigSync] Failed to load device thresholds for %s/%s: %v", projectID, dev, err)
			continue
		}
		payload, err := json.Marshal(rows)
		if err != nil {
			log.Printf("[ConfigSync] Failed to marshal device thresholds for %s/%s: %v", projectID, dev, err)
			continue
		}
		key := fmt.Sprintf("config:thresholds:%s:%s", projectID, dev)
		if err := s.redisStore.SetRaw(key, string(payload), 0); err != nil {
			log.Printf("[ConfigSync] Failed to cache device thresholds for %s/%s: %v", projectID, dev, err)
		}
	}
}

func (s *ConfigSyncService) syncConfigBundle(p map[string]interface{}, scopeSchemas map[string]payloadschema.ScopeSchema) {
	if !s.bundleEnabled || s.redisStore == nil {
		return
	}

	projectID, ok := p["id"].(string)
	if !ok {
		return
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return
	}

	bundle := projectConfigBundle{
		ProjectID: projectID,
		UpdatedAt: time.Now().UTC(),
	}

	meta := map[string]interface{}{"id": projectID}
	if v, exists := p["type"]; exists {
		meta["type"] = v
	}
	if v, exists := p["hardware"]; exists {
		meta["hardware"] = v
	}
	if len(meta) > 0 {
		bundle.Project = meta
	}

	merged := aggregatePacketSchemas(payloadschema.ScopeKeyForProject(projectID), scopeSchemas)
	if merged == nil || len(merged) == 0 {
		if t, ok := p["type"].(string); ok {
			fallback := strings.TrimSpace(t)
			if fallback != "" {
				merged = aggregatePacketSchemas(payloadschema.ScopeKeyForProject(fallback), scopeSchemas)
			}
		}
	}
	if merged != nil {
		bundle.PayloadSchemas = merged
	}

	if s.thresholdRepo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		thresholds := &thresholdBundle{}

		if defaults, err := s.thresholdRepo.ListDnaThresholds(ctx, projectID, "project", nil); err != nil {
			log.Printf("[ConfigSync] Failed to load project thresholds for bundle %s: %v", projectID, err)
		} else if len(defaults) > 0 {
			thresholds.Project = defaults
		}

		if ids, err := s.thresholdRepo.ListDnaThresholdDeviceIDs(ctx, projectID); err != nil {
			log.Printf("[ConfigSync] Failed to list device overrides for bundle %s: %v", projectID, err)
		} else if len(ids) > 0 {
			thresholds.Devices = make(map[string][]models.DnaThreshold)
			for _, devID := range ids {
				dev := devID
				rows, err := s.thresholdRepo.ListDnaThresholds(ctx, projectID, "device", &dev)
				if err != nil {
					log.Printf("[ConfigSync] Failed to load device thresholds for bundle %s/%s: %v", projectID, dev, err)
					continue
				}
				if len(rows) > 0 {
					thresholds.Devices[dev] = rows
				}
			}
		}

		if len(thresholds.Project) > 0 || len(thresholds.Devices) > 0 {
			bundle.Thresholds = thresholds
		}
	}

	if s.configRepo != nil {
		if rules, err := s.configRepo.GetRules(projectID, ""); err != nil {
			log.Printf("[ConfigSync] Failed to load rules for bundle %s: %v", projectID, err)
		} else if len(rules) > 0 {
			bundle.Rules = rules
		}

		if flow, err := s.configRepo.GetAutomationFlow(projectID); err != nil {
			log.Printf("[ConfigSync] Failed to load automation flow for bundle %s: %v", projectID, err)
		} else if len(flow) > 0 {
			bundle.Automation = flow
		}
	}

	key := fmt.Sprintf("config:bundle:%s", projectID)
	data, err := json.Marshal(bundle)
	if err != nil {
		log.Printf("[ConfigSync] Failed to marshal config bundle for %s: %v", projectID, err)
		return
	}

	if err := s.redisStore.SetRaw(key, string(data), 0); err != nil {
		log.Printf("[ConfigSync] Failed to cache config bundle for %s: %v", projectID, err)
	}
}

// InvalidateScopes clears any cached scope schemas so the next sync reloads fresh data.
func (s *ConfigSyncService) InvalidateScopes() {
	s.cacheMu.Lock()
	s.scopeCache = nil
	s.cacheLoaded = time.Time{}
	s.cacheMu.Unlock()
}

func (s *ConfigSyncService) loadDNAScopes() map[string]payloadschema.ScopeSchema {
	if s.dnaRepo == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	records, err := s.dnaRepo.ListAll(ctx)
	if err != nil {
		log.Printf("[ConfigSync] Failed to load payload schema from DNA repository: %v", err)
		return nil
	}
	if len(records) == 0 {
		return nil
	}

	scopes, err := dna.AssembleScopes(records)
	if err != nil {
		log.Printf("[ConfigSync] Failed to assemble DNA payload schema: %v", err)
		return nil
	}
	if len(scopes) == 0 {
		return nil
	}

	log.Printf("[ConfigSync] Loaded payload schema from DNA repository (%d scope entries)", len(scopes))
	return scopes
}

func (s *ConfigSyncService) loadCSVScopes() map[string]payloadschema.ScopeSchema {
	schemaPath := resolveSchemaPath()
	if schemaPath == "" {
		log.Printf("[ConfigSync] No payload schema CSV resolved; skipping schema injection")
		return nil
	}

	scopes, err := payloadschema.LoadAndGroup(schemaPath)
	if err != nil {
		log.Printf("[ConfigSync] Failed to load payload schema from %s: %v", schemaPath, err)
		return nil
	}

	log.Printf("[ConfigSync] Loaded payload schema from %s (%d scope entries)", schemaPath, len(scopes))
	return scopes
}

func (s *ConfigSyncService) resolveScopeSchemas() map[string]payloadschema.ScopeSchema {
	s.cacheMu.RLock()
	if s.scopeCache != nil && time.Since(s.cacheLoaded) < s.cacheTTL {
		cached := s.scopeCache
		s.cacheMu.RUnlock()
		return cached
	}
	s.cacheMu.RUnlock()

	scopeSchemas := s.loadDNAScopes()
	if scopeSchemas == nil {
		scopeSchemas = s.loadCSVScopes()
	}

	s.cacheMu.Lock()
	s.scopeCache = scopeSchemas
	s.cacheLoaded = time.Now()
	s.cacheMu.Unlock()

	return scopeSchemas
}

func (s *ConfigSyncService) syncProjectPayload(p map[string]interface{}, scopeSchemas map[string]payloadschema.ScopeSchema) bool {
	projectID, ok := p["id"].(string)
	if !ok || projectID == "" {
		log.Printf("[ConfigSync] skipping record without project id: %#v", p)
		return false
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		log.Printf("[ConfigSync] skipping record without project id: %#v", p)
		return false
	}

	key := fmt.Sprintf("config:project:%s", projectID)

	payload := map[string]interface{}{
		"id": projectID,
	}

	if v, exists := p["type"]; exists {
		payload["type"] = v
	}
	if v, exists := p["hardware"]; exists {
		payload["hardware"] = v
	}

	rawConfig := p["config"]
	configMap, hasConfig := toConfigMap(rawConfig)
	merged := aggregatePacketSchemas(payloadschema.ScopeKeyForProject(projectID), scopeSchemas)
	if merged == nil || len(merged) == 0 {
		if t, ok := p["type"].(string); ok {
			fallback := strings.TrimSpace(t)
			if fallback != "" {
				merged = aggregatePacketSchemas(payloadschema.ScopeKeyForProject(fallback), scopeSchemas)
			}
		}
	}
	if merged != nil {
		if configMap == nil {
			configMap = make(map[string]interface{})
		}
		configMap["payloadSchemas"] = merged
		hasConfig = true
	}

	if hasConfig && configMap != nil {
		payload["config"] = configMap
	} else if rawConfig != nil {
		payload["config"] = rawConfig
	}

	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[ConfigSync] Failed to marshal config for %s: %v", projectID, err)
		return false
	}

	if err := s.redisStore.SetRaw(key, string(data), 0); err != nil {
		log.Printf("[ConfigSync] Failed to sync %s: %v", projectID, err)
		return false
	}

	return true
}

func (s *ConfigSyncService) syncAutomationFlowCache(projectID string) {
	if s.redisStore == nil || s.configRepo == nil {
		return
	}

	flow, err := s.configRepo.GetAutomationFlow(projectID)
	if err != nil {
		log.Printf("[ConfigSync] Failed to load automation flow for %s: %v", projectID, err)
		return
	}

	key := fmt.Sprintf("config:automation:%s", projectID)
	if flow == nil || len(flow) == 0 {
		if err := s.redisStore.SetRaw(key, "null", 0); err != nil {
			log.Printf("[ConfigSync] Failed to cache automation flow for %s: %v", projectID, err)
		}
		return
	}

	data, err := json.Marshal(flow)
	if err != nil {
		log.Printf("[ConfigSync] Failed to marshal automation flow for %s: %v", projectID, err)
		return
	}

	if err := s.redisStore.SetRaw(key, string(data), 0); err != nil {
		log.Printf("[ConfigSync] Failed to cache automation flow for %s: %v", projectID, err)
	}
}

func (s *ConfigSyncService) syncRulesCache(projectID string) {
	if s.redisStore == nil || s.configRepo == nil {
		return
	}

	rules, err := s.configRepo.GetRules(projectID, "")
	if err != nil {
		log.Printf("[ConfigSync] Failed to load rules for %s: %v", projectID, err)
		return
	}

	key := fmt.Sprintf("config:rules:%s", projectID)
	if len(rules) == 0 {
		if err := s.redisStore.SetRaw(key, "[]", 0); err != nil {
			log.Printf("[ConfigSync] Failed to cache empty rules for %s: %v", projectID, err)
		}
		return
	}

	data, err := json.Marshal(rules)
	if err != nil {
		log.Printf("[ConfigSync] Failed to marshal rules for %s: %v", projectID, err)
		return
	}

	if err := s.redisStore.SetRaw(key, string(data), 0); err != nil {
		log.Printf("[ConfigSync] Failed to cache rules for %s: %v", projectID, err)
	}
}

func resolveSchemaPath() string {
	if custom := os.Getenv("PAYLOAD_SCHEMA_CSV_PATH"); custom != "" {
		return custom
	}
	defaultCandidates := []string{
		"docs/rms-payload-parameters.csv",
		"../docs/rms-payload-parameters.csv",
	}
	for _, candidate := range defaultCandidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func toConfigMap(value interface{}) (map[string]interface{}, bool) {
	switch v := value.(type) {
	case nil:
		return nil, false
	case map[string]interface{}:
		return v, true
	case []byte:
		if len(v) == 0 {
			return nil, false
		}
		var out map[string]interface{}
		if err := json.Unmarshal(v, &out); err == nil {
			return out, true
		}
	case string:
		if v == "" {
			return nil, false
		}
		var out map[string]interface{}
		if err := json.Unmarshal([]byte(v), &out); err == nil {
			return out, true
		}
	default:
		if raw, err := json.Marshal(v); err == nil {
			var out map[string]interface{}
			if json.Unmarshal(raw, &out) == nil {
				return out, true
			}
		}
	}
	return nil, false
}

func aggregatePacketSchemas(scopeKey string, scopes map[string]payloadschema.ScopeSchema) map[string]payloadschema.PacketSchema {
	if len(scopes) == 0 {
		return nil
	}
	result := make(map[string]payloadschema.PacketSchema)

	if globalScope, ok := scopes["global"]; ok {
		for key, packet := range globalScope.PacketSchemas {
			result[key] = clonePacketSchema(packet)
		}
	}

	if scopeKey != "" {
		if scope, ok := scopes[scopeKey]; ok {
			for key, packet := range scope.PacketSchemas {
				if existing, exists := result[key]; exists {
					result[key] = mergePacketSchemas(existing, packet)
				} else {
					result[key] = clonePacketSchema(packet)
				}
			}
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

func clonePacketSchema(src payloadschema.PacketSchema) payloadschema.PacketSchema {
	clone := payloadschema.PacketSchema{
		PacketType:    src.PacketType,
		TopicTemplate: src.TopicTemplate,
		Keys:          cloneKeySpecs(src.Keys),
		EnvelopeKeys:  cloneKeySpecs(src.EnvelopeKeys),
	}
	return clone
}

func cloneKeySpecs(src []payloadschema.KeySpec) []payloadschema.KeySpec {
	if len(src) == 0 {
		return nil
	}
	out := make([]payloadschema.KeySpec, len(src))
	for i, spec := range src {
		out[i] = cloneKeySpec(spec)
	}
	return out
}

func cloneKeySpec(src payloadschema.KeySpec) payloadschema.KeySpec {
	clone := src
	if src.MaxLength != nil {
		value := *src.MaxLength
		clone.MaxLength = &value
	}
	if src.ValueMin != nil {
		value := *src.ValueMin
		clone.ValueMin = &value
	}
	if src.ValueMax != nil {
		value := *src.ValueMax
		clone.ValueMax = &value
	}
	if src.Resolution != nil {
		value := *src.Resolution
		clone.Resolution = &value
	}
	return clone
}

func mergePacketSchemas(base payloadschema.PacketSchema, override payloadschema.PacketSchema) payloadschema.PacketSchema {
	merged := clonePacketSchema(base)
	if override.TopicTemplate != "" {
		merged.TopicTemplate = override.TopicTemplate
	}
	merged.Keys = mergeKeySpecs(merged.Keys, override.Keys)
	merged.EnvelopeKeys = mergeKeySpecs(merged.EnvelopeKeys, override.EnvelopeKeys)
	return merged
}

func mergeKeySpecs(base, override []payloadschema.KeySpec) []payloadschema.KeySpec {
	if len(base) == 0 {
		return cloneKeySpecs(override)
	}
	result := cloneKeySpecs(base)
	index := make(map[string]int, len(result))
	for i, spec := range result {
		index[spec.Key] = i
	}
	for _, spec := range override {
		if pos, exists := index[spec.Key]; exists {
			result[pos] = mergeKeySpec(result[pos], spec)
		} else {
			result = append(result, cloneKeySpec(spec))
			index[spec.Key] = len(result) - 1
		}
	}
	return result
}

func mergeKeySpec(base payloadschema.KeySpec, override payloadschema.KeySpec) payloadschema.KeySpec {
	merged := cloneKeySpec(base)
	if override.Description != "" {
		merged.Description = override.Description
	}
	if override.Unit != "" {
		merged.Unit = override.Unit
	}
	if override.Required {
		merged.Required = true
	}
	if override.MaxLength != nil {
		value := *override.MaxLength
		merged.MaxLength = &value
	}
	if override.ValueMin != nil {
		value := *override.ValueMin
		merged.ValueMin = &value
	}
	if override.ValueMax != nil {
		value := *override.ValueMax
		merged.ValueMax = &value
	}
	if override.Resolution != nil {
		value := *override.Resolution
		merged.Resolution = &value
	}
	if override.Notes != "" {
		merged.Notes = override.Notes
	}
	return merged
}
