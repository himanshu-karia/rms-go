package http

import (
	"context"
	"strings"

	"ingestion-go/internal/adapters/secondary"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

// ApiKeyMiddleware validates x-api-key header
func ApiKeyMiddleware(repo *secondary.PostgresRepo) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. Check Header
		apiKey := c.Get("x-api-key")
		if apiKey == "" {
			return c.Next() // No API Key, pass to JWT Middleware (Fallthrough)
		}

		// 2. Format: "ak_prefix_secret"
		// We use prefix "ak_" and first 7 chars is the ID or prefix stored in DB
		// Actually, standard practice: ak_test_12345
		// We stored 'prefix' in DB.
		// Let's assume key is just a string.

		// For Performance: We should cache this in Redis.
		// For V1: DB Lookup.

		// Optimization: We need to find WHICH key this is to verify hash.
		// Usually keys are "prefix.secret".
		parts := strings.Split(apiKey, ".")
		if len(parts) != 2 {
			// Invalid format, maybe it's legacy?
			// If invalid format, we can't look it up efficiently if we hashed it.
			// Unless we iterate all keys? No.
			// Assumption: Keys are generated as "prefix.secret"
			return WriteAPIError(c, fiber.StatusUnauthorized, "api_key_invalid_format", "Invalid API Key format. Expected prefix.secret", nil)
		}

		prefix := parts[0]
		secret := parts[1]

		ctx := context.Background()

		// 3. Lookup by Prefix
		var id, hash, pid, scopesJson string
		var isActive bool

		query := `SELECT id, key_hash, project_id, scopes, is_active FROM api_keys WHERE prefix=$1`
		err := repo.Pool.QueryRow(ctx, query, prefix).Scan(&id, &hash, &pid, &scopesJson, &isActive)

		if err != nil {
			// Not found
			return WriteAPIError(c, fiber.StatusUnauthorized, "api_key_invalid", "Invalid API Key", nil)
		}

		if !isActive {
			return WriteAPIError(c, fiber.StatusForbidden, "api_key_revoked", "API Key Revoked", nil)
		}

		// 4. Verify Hash
		err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(secret))
		if err != nil {
			return WriteAPIError(c, fiber.StatusUnauthorized, "api_key_invalid_secret", "Invalid API Key Secret", nil)
		}

		// 5. Success - Inject User Context
		// We inject a "ServiceUser" context
		c.Locals("user_id", "service_account")
		c.Locals("role", "service_role") // Or derive from scopes
		c.Locals("project_id", pid)
		c.Locals("scopes", scopesJson)

		// Skip JWT Middleware if it follows
		c.Locals("auth_method", "api_key")

		return c.Next()
	}
}
