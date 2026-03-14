//go:build integration

package services_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"ingestion-go/internal/adapters/secondary"
	"ingestion-go/internal/core/services"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProjectCRUD(t *testing.T) {
	// reuse pool setup from Auth Test or global setup in real suite
	ctx := context.Background()
	// Dynamic Host for Docker vs Local
	host := os.Getenv("TEST_DB_HOST")
	if host == "" {
		host = "localhost"
	}
	pool, _ := pgxpool.New(ctx, fmt.Sprintf("postgres://postgres:password@%s:5432/telemetry?sslmode=disable", host))
	defer pool.Close()

	// Clean
	pool.Exec(ctx, "DELETE FROM projects WHERE id = 'test_proj_01'")

	repo := secondary.NewPostgresProjectRepo(pool)
	service := services.NewProjectService(repo, nil) // Mock Redis as nil for now

	// 1. Create
	config := map[string]interface{}{"sensors": []string{"temp"}}
	err := service.CreateProject("test_proj_01", "Test Project", "demo", "test-location", config)
	if err != nil {
		t.Fatalf("Create Project failed: %v", err)
	}

	// 2. Fetch
	p, err := service.GetProject("test_proj_01")
	if err != nil {
		t.Fatalf("Get Project failed: %v", err)
	}

	if p.Name != "Test Project" {
		t.Errorf("Expected name 'Test Project', got %s", p.Name)
	}

	// 3. List should contain the project with type/location/config
	list, err := service.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}

	found := false
	for _, item := range list {
		if item.ID == "test_proj_01" {
			found = true
			if item.Type != "demo" {
				t.Errorf("expected type 'demo', got %s", item.Type)
			}
			if item.Location != "test-location" {
				t.Errorf("expected location 'test-location', got %s", item.Location)
			}
			if m, ok := item.Config.(map[string]interface{}); ok {
				if _, ok := m["sensors"]; !ok {
					t.Errorf("expected sensors config present")
				}
			} else {
				t.Errorf("expected config map, got %T", item.Config)
			}
			break
		}
	}
	if !found {
		t.Fatalf("created project not found in list")
	}

	// Check JSONB
	// In real implementation we check deep equality
}
