package services

import (
	"encoding/json"
	"fmt"
	"testing"

	"ingestion-go/internal/adapters/secondary"

	"github.com/alicebob/miniredis/v2"
)

type stubRulesRepo struct {
	rules        []map[string]interface{}
	err          error
	getRulesCall int
}

func (s *stubRulesRepo) GetRules(projectId, deviceId string) ([]map[string]interface{}, error) {
	s.getRulesCall++
	if s.err != nil {
		return nil, s.err
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

func (s *stubRulesRepo) CreateRule(rule map[string]interface{}) (string, error) { return "", nil }
func (s *stubRulesRepo) DeleteRule(id string) error                             { return nil }
func (s *stubRulesRepo) CreateWorkOrder(wo map[string]interface{}) error        { return nil }
func (s *stubRulesRepo) CreateAlert(deviceId, projectId, msg, severity string) error {
	return nil
}

func (s *stubRulesRepo) CreateAlertWithData(deviceId, projectId, msg, severity string, data interface{}) error {
	return nil
}

func TestRulesServiceLoadRulesFromCache(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := secondary.NewRedisStore(fmt.Sprintf("redis://%s", redisServer.Addr()))

	cached := []map[string]interface{}{{"id": "rule1", "trigger": "temp > 10"}}
	bytes, err := json.Marshal(cached)
	if err != nil {
		t.Fatalf("marshal cache: %v", err)
	}

	if err := store.SetRaw("config:rules:proj123", string(bytes), 0); err != nil {
		t.Fatalf("set cache: %v", err)
	}

	repo := &stubRulesRepo{}
	svc := NewRulesService(repo, store, nil)

	got, err := svc.loadRules("proj123")
	if err != nil {
		t.Fatalf("loadRules returned error: %v", err)
	}

	if repo.getRulesCall != 0 {
		t.Fatalf("expected repo not called, got %d", repo.getRulesCall)
	}

	if len(got) != 1 || got[0]["id"] != "rule1" {
		t.Fatalf("unexpected cached rules: %#v", got)
	}
}

func TestRulesServiceLoadRulesCachesMisses(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := secondary.NewRedisStore(fmt.Sprintf("redis://%s", redisServer.Addr()))

	repo := &stubRulesRepo{rules: []map[string]interface{}{{"id": "rule-db", "trigger": "temp > 5"}}}
	svc := NewRulesService(repo, store, nil)

	got, err := svc.loadRules("projABC")
	if err != nil {
		t.Fatalf("loadRules returned error: %v", err)
	}

	if repo.getRulesCall != 1 {
		t.Fatalf("expected repo called once, got %d", repo.getRulesCall)
	}

	if len(got) != 1 || got[0]["id"] != "rule-db" {
		t.Fatalf("unexpected rules from repo: %#v", got)
	}

	raw, ok, err := store.GetRaw("config:rules:projABC")
	if err != nil {
		t.Fatalf("get raw: %v", err)
	}
	if !ok {
		t.Fatalf("expected cache to be written")
	}

	var cached []map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &cached); err != nil {
		t.Fatalf("decode cache: %v", err)
	}
	if len(cached) != 1 || cached[0]["id"] != "rule-db" {
		t.Fatalf("unexpected cached rules: %#v", cached)
	}

	// Subsequent call should hit cache, not repo again
	got, err = svc.loadRules("projABC")
	if err != nil {
		t.Fatalf("second loadRules error: %v", err)
	}
	if repo.getRulesCall != 1 {
		t.Fatalf("expected repo still called once, got %d", repo.getRulesCall)
	}
	if len(got) != 1 || got[0]["id"] != "rule-db" {
		t.Fatalf("unexpected rules on second call: %#v", got)
	}
}
