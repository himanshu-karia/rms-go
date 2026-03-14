package http

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

type mockOtaService struct {
	startCampaignCalls []struct {
		name        string
		version     string
		s3URL       string
		checksum    string
		projectType string
	}
}

func (m *mockOtaService) UploadFirmware(filename string, content []byte) (string, error) {
	return "https://example.invalid/firmware.bin", nil
}

func (m *mockOtaService) StartCampaign(name, version, s3URL, checksum, projectType string) error {
	m.startCampaignCalls = append(m.startCampaignCalls, struct {
		name        string
		version     string
		s3URL       string
		checksum    string
		projectType string
	}{name: name, version: version, s3URL: s3URL, checksum: checksum, projectType: projectType})
	return nil
}

func TestStartCampaign_SnakeCaseRequestAndStatus(t *testing.T) {
	mockSvc := &mockOtaService{}
	controller := NewOtaController(mockSvc)

	app := fiber.New()
	app.Post("/ota/campaign", controller.StartCampaign)

	req := httptest.NewRequest("POST", "/ota/campaign", strings.NewReader(`{"name":"n1","version":"1.0","s3_url":"s3://x","checksum":"abc","project_type":"dna"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if payload["status"] != "campaign_started" {
		t.Fatalf("expected status campaign_started, got %v", payload["status"])
	}

	if len(mockSvc.startCampaignCalls) != 1 {
		t.Fatalf("expected 1 StartCampaign call, got %d", len(mockSvc.startCampaignCalls))
	}
	call := mockSvc.startCampaignCalls[0]
	if call.s3URL != "s3://x" || call.projectType != "dna" {
		t.Fatalf("expected s3_url and project_type forwarded, got s3URL=%q projectType=%q", call.s3URL, call.projectType)
	}
}

func TestStartCampaign_CamelCaseFallbacks(t *testing.T) {
	mockSvc := &mockOtaService{}
	controller := NewOtaController(mockSvc)

	app := fiber.New()
	app.Post("/ota/campaign", controller.StartCampaign)

	req := httptest.NewRequest("POST", "/ota/campaign", strings.NewReader(`{"name":"n1","version":"1.0","s3Url":"s3://legacy","checksum":"abc","projectType":"legacy"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if len(mockSvc.startCampaignCalls) != 1 {
		t.Fatalf("expected 1 StartCampaign call, got %d", len(mockSvc.startCampaignCalls))
	}
	call := mockSvc.startCampaignCalls[0]
	if call.s3URL != "s3://legacy" || call.projectType != "legacy" {
		t.Fatalf("expected camelCase fallbacks forwarded, got s3URL=%q projectType=%q", call.s3URL, call.projectType)
	}
}
