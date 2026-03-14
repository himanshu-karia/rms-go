package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"ingestion-go/internal/repository" // Import the Redis wrapping package
	"log"
	"sync"
	"time"
)

const CacheTTL = 30 * time.Second

type ConfigLoader struct {
	mu            sync.RWMutex
	projectCache  map[string]*ProjectConfig
	ruleCache     map[string][]RuleConfig
	projectExpiry map[string]time.Time
	ruleExpiry    map[string]time.Time
}

var Loader *ConfigLoader

func InitLoader() {
	Loader = &ConfigLoader{
		projectCache:  make(map[string]*ProjectConfig),
		ruleCache:     make(map[string][]RuleConfig),
		projectExpiry: make(map[string]time.Time),
		ruleExpiry:    make(map[string]time.Time),
	}
}

// GetProject retrieves project config from memory or Redis
func (l *ConfigLoader) GetProject(ctx context.Context, projectID string) (*ProjectConfig, error) {
	// 1. Check Memory Cache
	l.mu.RLock()
	config, ok := l.projectCache[projectID]
	expiry, _ := l.projectExpiry[projectID]
	l.mu.RUnlock()

	if ok && time.Now().Before(expiry) {
		return config, nil
	}

	// 2. Fetch from Redis (Cache Miss/Expired)
	return l.refreshProject(ctx, projectID)
}

func (l *ConfigLoader) refreshProject(ctx context.Context, projectID string) (*ProjectConfig, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Double check logic inside lock
	if config, ok := l.projectCache[projectID]; ok && time.Now().Before(l.projectExpiry[projectID]) {
		return config, nil
	}

	key := fmt.Sprintf("config:project:%s", projectID)
	// Use repository.Rdb directly
	val, err := repository.Rdb.Get(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch project config from redis: %v", err)
	}

	var config ProjectConfig
	if err := json.Unmarshal([]byte(val), &config); err != nil {
		return nil, fmt.Errorf("failed to parse project config json: %v", err)
	}

	// Update Cache
	l.projectCache[projectID] = &config
	l.projectExpiry[projectID] = time.Now().Add(CacheTTL)

	return &config, nil
}

// GetRules retrieves active rules from memory or Redis
func (l *ConfigLoader) GetRules(ctx context.Context, projectID string) ([]RuleConfig, error) {
	l.mu.RLock()
	rules, ok := l.ruleCache[projectID]
	expiry, _ := l.ruleExpiry[projectID]
	l.mu.RUnlock()

	if ok && time.Now().Before(expiry) {
		return rules, nil
	}

	return l.refreshRules(ctx, projectID)
}

func (l *ConfigLoader) refreshRules(ctx context.Context, projectID string) ([]RuleConfig, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	key := fmt.Sprintf("config:rules:%s", projectID)
	val, err := repository.Rdb.Get(ctx, key).Result()
	if err != nil {
		// If key doesn't exist, it means either Redis is down or NO rules.
		// Return empty list safely?
		log.Printf("⚠️ No rules found for %s or Redis error: %v", projectID, err)
		return []RuleConfig{}, nil
	}

	var rules []RuleConfig
	if err := json.Unmarshal([]byte(val), &rules); err != nil {
		return nil, fmt.Errorf("failed to parse rules json: %v", err)
	}

	l.ruleCache[projectID] = rules
	l.ruleExpiry[projectID] = time.Now().Add(CacheTTL)

	return rules, nil
}
