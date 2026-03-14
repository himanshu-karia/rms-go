package services

import (
	"ingestion-go/internal/core/ports"
)

// Interface for Rule Persistence
type RuleRepo interface {
	CreateRule(projectId, name string, trigger, actions interface{}) error
	GetRules(projectId string) ([]map[string]interface{}, error)
}

type RuleService struct {
	repo  RuleRepo
	state ports.StateStore
}

func NewRuleService(repo RuleRepo, state ports.StateStore) *RuleService {
	return &RuleService{repo: repo, state: state}
}

func (s *RuleService) CreateRule(projectId, deviceId, name string, trigger, actions interface{}) error {
	// 1. Save to DB
	err := s.repo.CreateRule(projectId, name, trigger, actions)
	if err != nil {
		return err
	}

	// 2. Sync
	return s.SyncRules(projectId)
}

func (s *RuleService) SyncRules(projectId string) error {
	// Fetch all rules from DB
	_, err := s.repo.GetRules(projectId)
	if err != nil {
		return err
	}

	// Push to Redis
	// s.state.SetRules(projectId, rules)
	return nil
}

// --- Postgres Repo Extension (rules) ---
// func (r *PostgresRepo) CreateRule(...) error
// func (r *PostgresRepo) GetRules(...) ([]Rule, error)
