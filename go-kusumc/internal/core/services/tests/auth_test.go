//go:build integration

package services_test

import (
	"context"
	"os"
	"testing"

	"ingestion-go/internal/adapters/secondary"
	"ingestion-go/internal/core/services"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TEST SEED
const TestDBURL = "postgres://postgres:password@localhost:5432/telemetry"

func TestAuthFlow(t *testing.T) {
	if os.Getenv("TEST_LIVE_DB") != "true" {
		t.Skip("Skipping Live DB Test. Set TEST_LIVE_DB=true to run.")
	}

	ctx := context.Background()

	// 1. Setup DB
	pool, err := pgxpool.New(ctx, TestDBURL)
	if err != nil {
		t.Fatal("Failed to connect to Live DB:", err)
	}
	defer pool.Close()

	// Cleanup
	pool.Exec(ctx, "DELETE FROM users WHERE username = 'test_admin'")

	// 2. Setup Service
	// Need a UserRepo adapter
	userRepo := secondary.NewPostgresUserRepo(pool)
	authService := services.NewAuthService(userRepo, "secret_salt")

	// 3. Register (Or Seed)
	err = authService.Register("test_admin", "securePass123", "admin")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// 4. Test Login Success
	token, err := authService.Login("test_admin", "securePass123")
	if err != nil {
		t.Fatalf("Login valid failed: %v", err)
	}
	if token == "" {
		t.Fatal("Expected JWT token, got empty")
	}

	// 5. Test Login Fail
	_, err = authService.Login("test_admin", "wrongpass")
	if err == nil {
		t.Fatal("Expected error on wrong pass, got success")
	}
}
