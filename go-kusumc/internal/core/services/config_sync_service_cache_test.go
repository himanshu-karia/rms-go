package services

import (
	"encoding/json"
	"fmt"
	"testing"

	"ingestion-go/internal/adapters/secondary"

	"github.com/alicebob/miniredis/v2"
)

type stubConfigRepo struct {
	flow       map[string]interface{}
	rules      []map[string]interface{}
	flowErr    error
	rulesErr   error
	flowCalls  int
	rulesCalls int
}

func (s *stubConfigRepo) GetAutomationFlow(projectId string) (map[string]interface{}, error) {
	s.flowCalls++
	if s.flowErr != nil {
		return nil, s.flowErr
	}
	if s.flow == nil {
		return nil, nil
	}
	copy := make(map[string]interface{}, len(s.flow))
	for k, v := range s.flow {
		copy[k] = v
	}
	return copy, nil
}

func (s *stubConfigRepo) GetRules(projectId, deviceId string) ([]map[string]interface{}, error) {
	s.rulesCalls++
	if s.rulesErr != nil {
		return nil, s.rulesErr
	}
	if len(s.rules) == 0 {
		return nil, nil
	}
	out := make([]map[string]interface{}, len(s.rules))
	for i, rule := range s.rules {
		cp := make(map[string]interface{}, len(rule))
		for k, v := range rule {
			cp[k] = v
		}
		out[i] = cp
	}
	return out, nil
}

func TestConfigSyncServiceSyncAutomationFlowCache(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := secondary.NewRedisStore(fmt.Sprintf("redis://%s", redisServer.Addr()))

	repo := &stubConfigRepo{
		flow: map[string]interface{}{
			"nodes": []interface{}{},
			"edges": []interface{}{},
		},
	}

	svc := &ConfigSyncService{
		redisStore: store,
		configRepo: repo,
	}

	svc.syncAutomationFlowCache("proj-1")

	if repo.flowCalls != 1 {
		t.Fatalf("expected flow repo to be called once, got %d", repo.flowCalls)
	}

	raw, ok, err := store.GetRaw("config:automation:proj-1")
	if err != nil {
		t.Fatalf("get raw: %v", err)
	}
	if !ok {
		t.Fatalf("expected automation flow to be cached")
	}

	var cached map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &cached); err != nil {
		t.Fatalf("decode cached flow: %v", err)
	}
	if len(cached) != 2 {
		t.Fatalf("expected cached flow to contain nodes and edges, got %#v", cached)
	}
}

func TestConfigSyncServiceSyncRulesCache(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := secondary.NewRedisStore(fmt.Sprintf("redis://%s", redisServer.Addr()))

	repo := &stubConfigRepo{
		rules: []map[string]interface{}{{
			"id":      "rule-1",
			"trigger": "temp > 5",
		}},
	}

	svc := &ConfigSyncService{
		redisStore: store,
		configRepo: repo,
	}

	svc.syncRulesCache("proj-99")

	if repo.rulesCalls != 1 {
		t.Fatalf("expected rules repo to be called once, got %d", repo.rulesCalls)
	}

	raw, ok, err := store.GetRaw("config:rules:proj-99")
	if err != nil {
		t.Fatalf("get raw: %v", err)
	}
	if !ok {
		t.Fatalf("expected rules to be cached")
	}

	var cached []map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &cached); err != nil {
		t.Fatalf("decode cached rules: %v", err)
	}
	if len(cached) != 1 || cached[0]["id"] != "rule-1" {
		t.Fatalf("unexpected cached rules: %#v", cached)
	}
}

func TestConfigSyncServiceSyncRulesCacheEmpty(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := secondary.NewRedisStore(fmt.Sprintf("redis://%s", redisServer.Addr()))

	repo := &stubConfigRepo{}

	svc := &ConfigSyncService{
		redisStore: store,
		configRepo: repo,
	}

	svc.syncRulesCache("proj-empty")

	raw, ok, err := store.GetRaw("config:rules:proj-empty")
	if err != nil {
		t.Fatalf("get raw: %v", err)
	}
	if !ok {
		t.Fatalf("expected empty rules marker to be cached")
	}
	if raw != "[]" {
		t.Fatalf("expected empty rules slice, got %s", raw)
	}
}
