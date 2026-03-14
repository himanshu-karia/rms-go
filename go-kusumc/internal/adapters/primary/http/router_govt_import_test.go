package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"ingestion-go/internal/models"
)

type mockGovtCredsAPI struct {
	bulkInput []models.GovtCredentialBundle
	bulkErr   error
}

func (m *mockGovtCredsAPI) Upsert(ctx context.Context, deviceID, protocolID, clientID, username, password string, metadata map[string]any) (models.GovtCredentialBundle, error) {
	return models.GovtCredentialBundle{}, nil
}

func (m *mockGovtCredsAPI) ListByDevice(ctx context.Context, deviceID string) ([]models.GovtCredentialBundle, error) {
	return nil, nil
}

func (m *mockGovtCredsAPI) BulkUpsert(ctx context.Context, bundles []models.GovtCredentialBundle) ([]models.GovtCredentialBundle, error) {
	m.bulkInput = bundles
	if m.bulkErr != nil {
		return nil, m.bulkErr
	}
	return bundles, nil
}

type mockRouterRepo struct {
	devices map[string]map[string]interface{}
	lastJob map[string]interface{}
}

func (m *mockRouterRepo) GetDeviceByIDOrIMEI(idOrIMEI string) (map[string]interface{}, error) {
	if m.devices == nil {
		return nil, nil
	}
	if device, ok := m.devices[idOrIMEI]; ok {
		return device, nil
	}
	return nil, nil
}

func (m *mockRouterRepo) CreateImportJob(jobType, projectID string, total, success, errorCount int, errors []map[string]any) (map[string]any, error) {
	m.lastJob = map[string]interface{}{
		"id":            "job-1",
		"job_type":      jobType,
		"project_id":    projectID,
		"total_count":   total,
		"success_count": success,
		"error_count":   errorCount,
		"errors":        errors,
	}
	return m.lastJob, nil
}

func TestHandleBulkUpsertGovtCreds_JSONArray_Success(t *testing.T) {
	app := fiber.New()
	mockGovt := &mockGovtCredsAPI{}
	mockRepo := &mockRouterRepo{}
	router := &Router{govtCreds: mockGovt, repo: mockRepo}
	app.Post("/import", func(c *fiber.Ctx) error {
		c.Locals("project_id", "proj-1")
		return router.handleBulkUpsertGovtCreds(c)
	})

	body := []byte(`[{"device_id":"dev-1","protocol_id":"proto-1","username":"u1","password":"p1"}]`)
	req := httptest.NewRequest("POST", "/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	if len(mockGovt.bulkInput) != 1 {
		t.Fatalf("expected 1 bundle, got %d", len(mockGovt.bulkInput))
	}
	if resp.Header.Get("X-Import-Job-Id") != "job-1" {
		t.Fatalf("expected X-Import-Job-Id header job-1, got %q", resp.Header.Get("X-Import-Job-Id"))
	}
	if mockRepo.lastJob == nil {
		t.Fatalf("expected import job to be created")
	}
	if mockRepo.lastJob["job_type"] != "government_credentials_import" {
		t.Fatalf("unexpected job type: %v", mockRepo.lastJob["job_type"])
	}
	if mockRepo.lastJob["total_count"] != 1 || mockRepo.lastJob["success_count"] != 1 || mockRepo.lastJob["error_count"] != 0 {
		t.Fatalf("unexpected import counts: %+v", mockRepo.lastJob)
	}
}

func TestHandleBulkUpsertGovtCreds_JSONWrappedCSV_PartialAndJob(t *testing.T) {
	app := fiber.New()
	mockGovt := &mockGovtCredsAPI{}
	mockRepo := &mockRouterRepo{devices: map[string]map[string]interface{}{
		"dev-1": {"id": "resolved-dev-1", "imei": "dev-1"},
	}}
	router := &Router{govtCreds: mockGovt, repo: mockRepo}
	app.Post("/import", func(c *fiber.Ctx) error {
		c.Locals("project_id", "proj-1")
		return router.handleBulkUpsertGovtCreds(c)
	})

	csvPayload := "device_id,protocol_id,username,password\n" +
		"dev-1,proto-1,user1,pass1\n" +
		"dev-2,,user2,pass2\n"
	wrapped := map[string]string{"csv": csvPayload}
	b, _ := json.Marshal(wrapped)
	req := httptest.NewRequest("POST", "/import", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 207 {
		t.Fatalf("expected 207, got %d", resp.StatusCode)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if int(payload["success_count"].(float64)) != 1 {
		t.Fatalf("expected success_count=1, got %v", payload["success_count"])
	}
	if int(payload["error_count"].(float64)) != 1 {
		t.Fatalf("expected error_count=1, got %v", payload["error_count"])
	}
	if payload["job_id"] != "job-1" {
		t.Fatalf("expected job_id job-1, got %v", payload["job_id"])
	}
	if resp.Header.Get("X-Import-Job-Id") != "job-1" {
		t.Fatalf("expected X-Import-Job-Id header job-1, got %q", resp.Header.Get("X-Import-Job-Id"))
	}
	if mockRepo.lastJob == nil || mockRepo.lastJob["job_type"] != "government_credentials_import" {
		t.Fatalf("expected government_credentials_import job, got %+v", mockRepo.lastJob)
	}
}

func TestHandleBulkUpsertGovtCreds_RawCSV_MissingFields(t *testing.T) {
	app := fiber.New()
	mockGovt := &mockGovtCredsAPI{}
	router := &Router{govtCreds: mockGovt}
	app.Post("/import", router.handleBulkUpsertGovtCreds)

	csvPayload := "device_id,protocol_id\n" +
		"dev-1,\n"
	req := httptest.NewRequest("POST", "/import", bytes.NewReader([]byte(csvPayload)))
	req.Header.Set("Content-Type", "text/csv")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if int(payload["success_count"].(float64)) != 0 {
		t.Fatalf("expected success_count=0, got %v", payload["success_count"])
	}
	if int(payload["error_count"].(float64)) == 0 {
		t.Fatalf("expected non-zero error_count")
	}
}

func TestRegisterRoutes_GovtImportCSV_ReturnsJobID(t *testing.T) {
	mockGovt := &mockGovtCredsAPI{}
	mockRepo := &mockRouterRepo{devices: map[string]map[string]interface{}{
		"dev-1": {"id": "resolved-dev-1", "imei": "dev-1"},
	}}
	router := &Router{govtCreds: mockGovt, repo: mockRepo}

	app := fiber.New()
	public := app.Group("/api")
	protected := app.Group("/api", func(c *fiber.Ctx) error {
		c.Locals("capabilities", []string{"devices:credentials"})
		c.Locals("project_id", "proj-1")
		return c.Next()
	})
	router.RegisterRoutes(public, protected)

	body := map[string]string{
		"csv": "device_id,protocol_id,username,password\n" +
			"dev-1,proto-1,user1,pass1\n",
	}
	b, _ := json.Marshal(body)

	paths := []string{
		"/api/devices/government-credentials/import",
		"/api/v1/devices/government-credentials/import",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest("POST", path, bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			if resp.StatusCode != 200 {
				t.Fatalf("expected 200, got %d", resp.StatusCode)
			}

			var payload map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
				t.Fatalf("decode failed: %v", err)
			}
			if payload["job_id"] != "job-1" {
				t.Fatalf("expected job_id job-1, got %v", payload["job_id"])
			}
			if resp.Header.Get("X-Import-Job-Id") != "job-1" {
				t.Fatalf("expected X-Import-Job-Id header job-1, got %q", resp.Header.Get("X-Import-Job-Id"))
			}
		})
	}
}

func TestHandleBulkUpsertGovtCreds_AllInvalidCSV_ReturnsJobID(t *testing.T) {
	app := fiber.New()
	mockGovt := &mockGovtCredsAPI{}
	mockRepo := &mockRouterRepo{}
	router := &Router{govtCreds: mockGovt, repo: mockRepo}
	app.Post("/import", func(c *fiber.Ctx) error {
		c.Locals("project_id", "proj-1")
		return router.handleBulkUpsertGovtCreds(c)
	})

	csvPayload := "device_id,protocol_id\n" +
		",\n"
	req := httptest.NewRequest("POST", "/import", bytes.NewReader([]byte(csvPayload)))
	req.Header.Set("Content-Type", "text/csv")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if payload["job_id"] != "job-1" {
		t.Fatalf("expected job_id job-1, got %v", payload["job_id"])
	}
	if resp.Header.Get("X-Import-Job-Id") != "job-1" {
		t.Fatalf("expected X-Import-Job-Id header job-1, got %q", resp.Header.Get("X-Import-Job-Id"))
	}
	if mockRepo.lastJob == nil || mockRepo.lastJob["error_count"] != 1 {
		t.Fatalf("expected persisted failed import job, got %+v", mockRepo.lastJob)
	}
}

func TestHandleBulkUpsertGovtCreds_CSVJobCounts_TableDriven(t *testing.T) {
	tests := []struct {
		name            string
		csv             string
		devices         map[string]map[string]interface{}
		expectStatus    int
		expectTotal     int
		expectSuccess   int
		expectError     int
		expectErrorRows int
	}{
		{
			name: "full success",
			csv: "device_id,protocol_id,username,password\n" +
				"dev-1,proto-1,u1,p1\n" +
				"dev-2,proto-2,u2,p2\n",
			devices: map[string]map[string]interface{}{
				"dev-1": {"id": "resolved-dev-1", "imei": "dev-1"},
				"dev-2": {"id": "resolved-dev-2", "imei": "dev-2"},
			},
			expectStatus:    200,
			expectTotal:     2,
			expectSuccess:   2,
			expectError:     0,
			expectErrorRows: 0,
		},
		{
			name: "partial success",
			csv: "device_id,protocol_id,username,password\n" +
				"dev-1,proto-1,u1,p1\n" +
				"dev-2,,u2,p2\n",
			devices: map[string]map[string]interface{}{
				"dev-1": {"id": "resolved-dev-1", "imei": "dev-1"},
				"dev-2": {"id": "resolved-dev-2", "imei": "dev-2"},
			},
			expectStatus:    207,
			expectTotal:     2,
			expectSuccess:   1,
			expectError:     1,
			expectErrorRows: 1,
		},
		{
			name: "full failure",
			csv: "device_id,protocol_id,username,password\n" +
				",proto-1,u1,p1\n" +
				"dev-2,,u2,p2\n",
			devices:         map[string]map[string]interface{}{},
			expectStatus:    400,
			expectTotal:     2,
			expectSuccess:   0,
			expectError:     2,
			expectErrorRows: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New()
			mockGovt := &mockGovtCredsAPI{}
			mockRepo := &mockRouterRepo{devices: tc.devices}
			router := &Router{govtCreds: mockGovt, repo: mockRepo}
			app.Post("/import", func(c *fiber.Ctx) error {
				c.Locals("project_id", "proj-1")
				return router.handleBulkUpsertGovtCreds(c)
			})

			req := httptest.NewRequest("POST", "/import", bytes.NewReader([]byte(tc.csv)))
			req.Header.Set("Content-Type", "text/csv")
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			if resp.StatusCode != tc.expectStatus {
				t.Fatalf("expected status %d, got %d", tc.expectStatus, resp.StatusCode)
			}

			var payload map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
				t.Fatalf("decode failed: %v", err)
			}
			if int(payload["success_count"].(float64)) != tc.expectSuccess {
				t.Fatalf("expected success_count=%d, got %v", tc.expectSuccess, payload["success_count"])
			}
			if int(payload["error_count"].(float64)) != tc.expectError {
				t.Fatalf("expected error_count=%d, got %v", tc.expectError, payload["error_count"])
			}
			errs, _ := payload["errors"].([]interface{})
			if len(errs) != tc.expectErrorRows {
				t.Fatalf("expected errors len=%d, got %d", tc.expectErrorRows, len(errs))
			}
			if payload["job_id"] != "job-1" {
				t.Fatalf("expected job_id job-1, got %v", payload["job_id"])
			}
			if resp.Header.Get("X-Import-Job-Id") != "job-1" {
				t.Fatalf("expected X-Import-Job-Id header job-1, got %q", resp.Header.Get("X-Import-Job-Id"))
			}

			if mockRepo.lastJob == nil {
				t.Fatalf("expected import job to be created")
			}
			if mockRepo.lastJob["job_type"] != "government_credentials_import" {
				t.Fatalf("unexpected job_type: %v", mockRepo.lastJob["job_type"])
			}
			if mockRepo.lastJob["total_count"] != tc.expectTotal {
				t.Fatalf("expected total_count=%d, got %v", tc.expectTotal, mockRepo.lastJob["total_count"])
			}
			if mockRepo.lastJob["success_count"] != tc.expectSuccess {
				t.Fatalf("expected success_count=%d, got %v", tc.expectSuccess, mockRepo.lastJob["success_count"])
			}
			if mockRepo.lastJob["error_count"] != tc.expectError {
				t.Fatalf("expected error_count=%d, got %v", tc.expectError, mockRepo.lastJob["error_count"])
			}
		})
	}
}
