package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

type mockImportJobsRepo struct {
	lastJobType string
	jobs        []map[string]any
	jobsByID    map[string]map[string]any
	lastRetryID string
	lastStatus  string
}

func (m *mockImportJobsRepo) ListImportJobs(jobType string) ([]map[string]any, error) {
	m.lastJobType = jobType
	if m.jobs == nil {
		return []map[string]any{}, nil
	}
	return m.jobs, nil
}

func (m *mockImportJobsRepo) GetImportJob(jobID string) (map[string]any, error) {
	if m.jobsByID == nil {
		return nil, nil
	}
	if job, ok := m.jobsByID[jobID]; ok {
		return job, nil
	}
	return nil, nil
}

func (m *mockImportJobsRepo) UpdateImportJobStatus(jobID, status string) error {
	m.lastRetryID = jobID
	m.lastStatus = status
	return nil
}

func TestListGovtImportJobs_DefaultType(t *testing.T) {
	app := fiber.New()
	mockRepo := &mockImportJobsRepo{jobs: []map[string]any{{"id": "job-1"}}}
	controller := &DeviceController{importJobs: mockRepo}
	app.Get("/jobs", controller.ListGovtImportJobs)

	req := httptest.NewRequest("GET", "/jobs", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if mockRepo.lastJobType != "government_credentials_import" {
		t.Fatalf("expected default job type government_credentials_import, got %q", mockRepo.lastJobType)
	}

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	jobs, ok := payload["jobs"].([]any)
	if !ok || len(jobs) != 1 {
		t.Fatalf("expected one job in response, got %v", payload["jobs"])
	}
}

func TestListGovtImportJobs_OverrideType(t *testing.T) {
	app := fiber.New()
	mockRepo := &mockImportJobsRepo{}
	controller := &DeviceController{importJobs: mockRepo}
	app.Get("/jobs", controller.ListGovtImportJobs)

	req := httptest.NewRequest("GET", "/jobs?type=device_import", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if mockRepo.lastJobType != "device_import" {
		t.Fatalf("expected overridden job type device_import, got %q", mockRepo.lastJobType)
	}
}

func TestListImportJobs_ImportTypeAlias(t *testing.T) {
	app := fiber.New()
	mockRepo := &mockImportJobsRepo{}
	controller := &DeviceController{importJobs: mockRepo}
	app.Get("/jobs", controller.ListImportJobs)

	req := httptest.NewRequest("GET", "/jobs?importType=device_configuration_import", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if mockRepo.lastJobType != "device_configuration_import" {
		t.Fatalf("expected importType alias to resolve to device_configuration_import, got %q", mockRepo.lastJobType)
	}
}

func TestListGovtImportJobs_ImportTypeAlias(t *testing.T) {
	app := fiber.New()
	mockRepo := &mockImportJobsRepo{}
	controller := &DeviceController{importJobs: mockRepo}
	app.Get("/jobs", controller.ListGovtImportJobs)

	req := httptest.NewRequest("GET", "/jobs?importType=device_import", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if mockRepo.lastJobType != "device_import" {
		t.Fatalf("expected importType alias override device_import, got %q", mockRepo.lastJobType)
	}
}

func TestListImportJobs_AliasPrecedence(t *testing.T) {
	app := fiber.New()
	mockRepo := &mockImportJobsRepo{}
	controller := &DeviceController{importJobs: mockRepo}
	app.Get("/jobs", controller.ListImportJobs)

	req := httptest.NewRequest("GET", "/jobs?type=device_import&jobType=device_configuration_import&importType=government_credentials_import", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if mockRepo.lastJobType != "device_import" {
		t.Fatalf("expected precedence to prefer type=device_import, got %q", mockRepo.lastJobType)
	}
}

func TestListImportJobs_SnakeCaseAliases(t *testing.T) {
	app := fiber.New()
	mockRepo := &mockImportJobsRepo{}
	controller := &DeviceController{importJobs: mockRepo}
	app.Get("/jobs", controller.ListImportJobs)

	req := httptest.NewRequest("GET", "/jobs?job_type=device_import", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if mockRepo.lastJobType != "device_import" {
		t.Fatalf("expected job_type alias device_import, got %q", mockRepo.lastJobType)
	}

	req2 := httptest.NewRequest("GET", "/jobs?import_type=device_configuration_import", nil)
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp2.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
	if mockRepo.lastJobType != "device_configuration_import" {
		t.Fatalf("expected import_type alias device_configuration_import, got %q", mockRepo.lastJobType)
	}
}

func TestListImportJobs_AliasPrecedence_SnakeCase(t *testing.T) {
	app := fiber.New()
	mockRepo := &mockImportJobsRepo{}
	controller := &DeviceController{importJobs: mockRepo}
	app.Get("/jobs", controller.ListImportJobs)

	req := httptest.NewRequest("GET", "/jobs?jobType=device_configuration_import&job_type=device_import&type=government_credentials_import", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if mockRepo.lastJobType != "government_credentials_import" {
		t.Fatalf("expected precedence to keep type first, got %q", mockRepo.lastJobType)
	}
}

func TestGovtImportJobsRouteFamily_ListGetErrorsRetry(t *testing.T) {
	app := fiber.New()
	mockRepo := &mockImportJobsRepo{
		jobs: []map[string]any{{"id": "job-1", "job_type": "government_credentials_import"}},
		jobsByID: map[string]map[string]any{
			"job-1": {
				"id":       "job-1",
				"job_type": "government_credentials_import",
				"errors":   []map[string]any{{"row": 2, "error": "protocol_id required"}},
			},
		},
	}
	controller := &DeviceController{importJobs: mockRepo}

	app.Get("/api/devices/government-credentials/import/jobs", controller.ListGovtImportJobs)
	app.Get("/api/v1/devices/government-credentials/import/jobs", controller.ListGovtImportJobs)
	app.Get("/api/devices/government-credentials/import/jobs/:jobId", controller.GetImportJob)
	app.Get("/api/devices/government-credentials/import/jobs/:jobId/errors.csv", controller.GetImportJobErrorsCSV)
	app.Post("/api/devices/government-credentials/import/jobs/:jobId/retry", controller.RetryImportJob)

	cases := []struct {
		method       string
		url          string
		expectStatus int
		validate     func(t *testing.T, body []byte, headers map[string]string)
	}{
		{
			method:       "GET",
			url:          "/api/devices/government-credentials/import/jobs",
			expectStatus: 200,
			validate: func(t *testing.T, body []byte, headers map[string]string) {
				if mockRepo.lastJobType != "government_credentials_import" {
					t.Fatalf("expected default job type, got %q", mockRepo.lastJobType)
				}
				var payload map[string]any
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("decode list payload: %v", err)
				}
				jobs, ok := payload["jobs"].([]any)
				if !ok || len(jobs) != 1 {
					t.Fatalf("expected one job, got %v", payload["jobs"])
				}
			},
		},
		{
			method:       "GET",
			url:          "/api/devices/government-credentials/import/jobs/job-1",
			expectStatus: 200,
			validate: func(t *testing.T, body []byte, headers map[string]string) {
				var payload map[string]any
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("decode get payload: %v", err)
				}
				if fmt.Sprintf("%v", payload["id"]) != "job-1" {
					t.Fatalf("expected id job-1, got %v", payload["id"])
				}
			},
		},
		{
			method:       "GET",
			url:          "/api/devices/government-credentials/import/jobs/job-1/errors.csv",
			expectStatus: 200,
			validate: func(t *testing.T, body []byte, headers map[string]string) {
				if !strings.HasPrefix(headers["Content-Type"], "text/csv") {
					t.Fatalf("expected text/csv content-type, got %q", headers["Content-Type"])
				}
				text := string(body)
				if text == "" || text == "row,error\n" {
					t.Fatalf("expected csv with error row, got %q", text)
				}
			},
		},
		{
			method:       "POST",
			url:          "/api/devices/government-credentials/import/jobs/job-1/retry",
			expectStatus: 200,
			validate: func(t *testing.T, body []byte, headers map[string]string) {
				if mockRepo.lastRetryID != "job-1" || mockRepo.lastStatus != "retry_requested" {
					t.Fatalf("unexpected retry invocation id=%q status=%q", mockRepo.lastRetryID, mockRepo.lastStatus)
				}
			},
		},
		{
			method:       "GET",
			url:          "/api/v1/devices/government-credentials/import/jobs?jobType=device_configuration_import",
			expectStatus: 200,
			validate: func(t *testing.T, body []byte, headers map[string]string) {
				if mockRepo.lastJobType != "device_configuration_import" {
					t.Fatalf("expected v1 alias to honor jobType override, got %q", mockRepo.lastJobType)
				}
			},
		},
		{
			method:       "GET",
			url:          "/api/v1/devices/government-credentials/import/jobs?importType=device_import",
			expectStatus: 200,
			validate: func(t *testing.T, body []byte, headers map[string]string) {
				if mockRepo.lastJobType != "device_import" {
					t.Fatalf("expected v1 alias to honor importType override, got %q", mockRepo.lastJobType)
				}
			},
		},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.url, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("%s %s failed: %v", tc.method, tc.url, err)
		}
		if resp.StatusCode != tc.expectStatus {
			t.Fatalf("%s %s expected %d, got %d", tc.method, tc.url, tc.expectStatus, resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if tc.validate != nil {
			tc.validate(t, body, map[string]string{"Content-Type": resp.Header.Get("Content-Type")})
		}
	}
}
