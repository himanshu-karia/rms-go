package services

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/google/uuid"

	"ingestion-go/internal/adapters/secondary"
	"ingestion-go/internal/models"
)

// GovtCredsService manages per-device government broker credentials.
type GovtCredsService struct {
	repo *secondary.PostgresGovtCredsRepo
	key  []byte
}

func NewGovtCredsService(repo *secondary.PostgresGovtCredsRepo) *GovtCredsService {
	key := os.Getenv("GOVT_CREDS_KEY")
	if key == "" {
		key = "dev-govt-creds-default-key-32-bytes!!"
	}
	norm := normalizeKey(key)
	return &GovtCredsService{repo: repo, key: norm}
}

func (s *GovtCredsService) Upsert(ctx context.Context, deviceID, protocolID, clientID, username, password string, metadata map[string]any) (models.GovtCredentialBundle, error) {
	enc, err := s.encrypt(password)
	if err != nil {
		return models.GovtCredentialBundle{}, err
	}

	rec := models.GovtCredentialBundle{
		ID:         uuid.NewString(),
		DeviceID:   deviceID,
		ProtocolID: protocolID,
		ClientID:   clientID,
		Username:   username,
		Password:   enc,
		Metadata:   metadata,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := s.repo.Upsert(ctx, rec); err != nil {
		return models.GovtCredentialBundle{}, fmt.Errorf("govt creds upsert: %w", err)
	}
	rec.Password = password
	return rec, nil
}

func (s *GovtCredsService) ListByDevice(ctx context.Context, deviceID string) ([]models.GovtCredentialBundle, error) {
	list, err := s.repo.GetByDevice(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if plain, err := s.decrypt(list[i].Password); err == nil {
			list[i].Password = plain
		}
	}
	return list, nil
}

func (s *GovtCredsService) GetByDeviceAndProtocol(ctx context.Context, deviceID, protocolID string) (*models.GovtCredentialBundle, error) {
	rec, err := s.repo.GetByDeviceAndProtocol(ctx, deviceID, protocolID)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, nil
	}
	if plain, err := s.decrypt(rec.Password); err == nil {
		rec.Password = plain
	}
	return rec, nil
}

func (s *GovtCredsService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// BulkUpsert inserts or updates multiple govt credential bundles atomically.
func (s *GovtCredsService) BulkUpsert(ctx context.Context, bundles []models.GovtCredentialBundle) ([]models.GovtCredentialBundle, error) {
	if len(bundles) == 0 {
		return nil, nil
	}
	now := time.Now()
	toStore := make([]models.GovtCredentialBundle, 0, len(bundles))
	for i := range bundles {
		b := bundles[i]
		if b.ID == "" {
			b.ID = uuid.NewString()
		}
		b.CreatedAt = now
		b.UpdatedAt = now
		enc, err := s.encrypt(b.Password)
		if err != nil {
			return nil, err
		}
		b.Password = enc
		toStore = append(toStore, b)
	}
	if err := s.repo.BulkUpsert(ctx, toStore); err != nil {
		return nil, fmt.Errorf("govt creds bulk upsert: %w", err)
	}
	// Return with plaintext preserved from input slices
	return bundles, nil
}

func (s *GovtCredsService) encrypt(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (s *GovtCredsService) decrypt(enc string) (string, error) {
	if enc == "" {
		return "", nil
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	data, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce := data[:gcm.NonceSize()]
	ciphertext := data[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func normalizeKey(raw string) []byte {
	if raw == "" {
		raw = "dev-govt-creds-default-key-32-bytes!!"
	}
	if len(raw) >= 32 {
		return []byte(raw[:32])
	}
	buf := make([]byte, 32)
	copy(buf, []byte(raw))
	for i := len(raw); i < 32; i++ {
		buf[i] = '0'
	}
	return buf
}
