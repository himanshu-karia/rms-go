package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"ingestion-go/internal/adapters/secondary"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type ApiKeyService struct {
	repo *secondary.PostgresRepo
}

type ApiKeyCreateInput struct {
	Name      string
	Scopes    []string
	ProjectID *string
	OrgID     *string
	CreatedBy *string
}

type ApiKeyCreateResult struct {
	Key    *secondary.ApiKeyRecord
	Secret string
}

func NewApiKeyService(repo *secondary.PostgresRepo) *ApiKeyService {
	return &ApiKeyService{repo: repo}
}

func (s *ApiKeyService) List() ([]secondary.ApiKeyRecord, error) {
	return s.repo.ListApiKeys()
}

func (s *ApiKeyService) Create(input ApiKeyCreateInput) (*ApiKeyCreateResult, error) {
	if strings.TrimSpace(input.Name) == "" {
		return nil, fmt.Errorf("name is required")
	}

	prefix := "ak_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:8]
	secret, err := generateSecret(32)
	if err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	key, err := s.repo.CreateApiKey(input.Name, prefix, string(hash), input.Scopes, input.ProjectID, input.OrgID, input.CreatedBy)
	if err != nil {
		return nil, err
	}

	return &ApiKeyCreateResult{
		Key:    key,
		Secret: prefix + "." + secret,
	}, nil
}

func (s *ApiKeyService) Revoke(id string) error {
	return s.repo.RevokeApiKey(id)
}

func generateSecret(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
