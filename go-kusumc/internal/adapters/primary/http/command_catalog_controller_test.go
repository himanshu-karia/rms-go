package http

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"ingestion-go/internal/core/domain"
	"ingestion-go/internal/core/services"
	"ingestion-go/tests/mocks"
)

func TestCommandCatalogController_List_ValidatesParams(t *testing.T) {
	svc := services.NewCommandsService(&mocks.MockCommandRepo{}, &mocks.MockDeviceRepo{})
	ctrl := NewCommandCatalogController(svc)

	app := fiber.New()
	app.Get("/api/commands/catalog-admin", ctrl.List)

	// missing project_id
	req := httptest.NewRequest("GET", "/api/commands/catalog-admin", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("fiber test error: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for missing project_id, got %d", resp.StatusCode)
	}

	// missing device_id
	req = httptest.NewRequest("GET", "/api/commands/catalog-admin?project_id=proj-1", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("fiber test error: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for missing device_id, got %d", resp.StatusCode)
	}
}

func TestCommandCatalogController_List_Success(t *testing.T) {
	repo := &mocks.MockCommandRepo{
		ListCommandsForDeviceFunc: func(deviceID, projectID string, protocolID, modelID *string) ([]domain.CommandCatalog, error) {
			return []domain.CommandCatalog{{ID: "cmd-1", Name: "Ping", Scope: "project"}}, nil
		},
	}
	devices := &mocks.MockDeviceRepo{
		GetDeviceByIDOrIMEIFunc: func(idOrIMEI string) (map[string]interface{}, error) {
			return map[string]interface{}{"id": "dev-1", "imei": "123", "project_id": "proj-1"}, nil
		},
	}
	svc := services.NewCommandsService(repo, devices)
	ctrl := NewCommandCatalogController(svc)

	app := fiber.New()
	app.Get("/api/commands/catalog-admin", ctrl.List)

	req := httptest.NewRequest("GET", "/api/commands/catalog-admin?project_id=proj-1&device_id=dev-1", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("fiber test error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var out []domain.CommandCatalog
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != 1 || out[0].ID != "cmd-1" {
		t.Fatalf("unexpected response: %+v", out)
	}
}

func TestCommandCatalogController_List_SnakeCaseAliases(t *testing.T) {
	repo := &mocks.MockCommandRepo{
		ListCommandsForDeviceFunc: func(deviceID, projectID string, protocolID, modelID *string) ([]domain.CommandCatalog, error) {
			if projectID != "proj-snake" {
				t.Fatalf("expected projectID proj-snake, got %s", projectID)
			}
			if deviceID != "dev-snake" {
				t.Fatalf("expected deviceID dev-snake, got %s", deviceID)
			}
			return []domain.CommandCatalog{}, nil
		},
	}
	devices := &mocks.MockDeviceRepo{
		GetDeviceByIDOrIMEIFunc: func(idOrIMEI string) (map[string]interface{}, error) {
			return map[string]interface{}{"id": "dev-snake", "imei": "123", "project_id": "proj-snake"}, nil
		},
	}
	svc := services.NewCommandsService(repo, devices)
	ctrl := NewCommandCatalogController(svc)

	app := fiber.New()
	app.Get("/api/commands/catalog-admin", ctrl.List)

	req := httptest.NewRequest("GET", "/api/commands/catalog-admin?project_id=proj-snake&device_id=dev-snake", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("fiber test error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestCommandCatalogController_List_DeviceAliasPrecedence(t *testing.T) {
	repo := &mocks.MockCommandRepo{
		ListCommandsForDeviceFunc: func(deviceID, projectID string, protocolID, modelID *string) ([]domain.CommandCatalog, error) {
			if deviceID != "resolved-dev" {
				t.Fatalf("unexpected resolved device id, got %s", deviceID)
			}
			return []domain.CommandCatalog{}, nil
		},
	}
	devices := &mocks.MockDeviceRepo{
		GetDeviceByIDOrIMEIFunc: func(idOrIMEI string) (map[string]interface{}, error) {
			if idOrIMEI != "dev-snake" {
				t.Fatalf("expected snake_case precedence to use device_id for lookup, got %s", idOrIMEI)
			}
			return map[string]interface{}{"id": "resolved-dev", "imei": "123", "project_id": "proj-1"}, nil
		},
	}
	svc := services.NewCommandsService(repo, devices)
	ctrl := NewCommandCatalogController(svc)

	app := fiber.New()
	app.Get("/api/commands/catalog-admin", ctrl.List)

	req := httptest.NewRequest("GET", "/api/commands/catalog-admin?project_id=proj-1&deviceId=dev-camel&device_id=dev-snake&imei=imei-fallback", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("fiber test error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestCommandCatalogController_Upsert_Success(t *testing.T) {
	repo := &mocks.MockCommandRepo{
		UpsertCommandCatalogFunc: func(rec domain.CommandCatalog) (string, error) {
			if rec.Name != "Ping" || rec.Scope != "project" {
				t.Fatalf("unexpected catalog payload: %+v", rec)
			}
			return "cmd-123", nil
		},
		UpsertDeviceCapabilitiesFunc: func(commandID string, deviceIDs []string) error {
			if commandID != "cmd-123" {
				t.Fatalf("unexpected commandID: %s", commandID)
			}
			if len(deviceIDs) != 1 || deviceIDs[0] != "dev-1" {
				t.Fatalf("unexpected deviceIDs: %+v", deviceIDs)
			}
			return nil
		},
	}
	svc := services.NewCommandsService(repo, &mocks.MockDeviceRepo{})
	ctrl := NewCommandCatalogController(svc)

	app := fiber.New()
	app.Post("/api/commands/catalog", ctrl.Upsert)

	payload := map[string]any{
		"name":           "Ping",
		"scope":          "project",
		"project_id":     "proj-1",
		"transport":      "mqtt",
		"device_ids":     []string{"dev-1"},
		"payload_schema": map[string]any{"type": "object"},
	}
	buf, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/commands/catalog", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("fiber test error: %v", err)
	}
	if resp.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var out map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out["id"] != "cmd-123" {
		t.Fatalf("unexpected id: %v", out)
	}
}

func TestCommandCatalogController_Upsert_Invalid(t *testing.T) {
	svc := services.NewCommandsService(&mocks.MockCommandRepo{}, &mocks.MockDeviceRepo{})
	ctrl := NewCommandCatalogController(svc)

	app := fiber.New()
	app.Post("/api/commands/catalog", ctrl.Upsert)

	payload := map[string]any{
		"scope":     "project",
		"projectId": "proj-1",
	}
	buf, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/commands/catalog", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("fiber test error: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for missing name, got %d", resp.StatusCode)
	}
}
