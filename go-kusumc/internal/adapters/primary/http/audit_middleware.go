package http

import (
	"ingestion-go/internal/adapters/secondary"
	"ingestion-go/internal/pkg/logger"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// AuditMiddleware logs administrative actions to DB and structured logs
func AuditMiddleware(repo *secondary.PostgresRepo) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Only audit mutating methods or critical reads?
		// "Log every admin action" usually implies changes
		method := c.Method()
		if method == "GET" || method == "HEAD" {
			// Skip GETs to reduce noise, unless it's strictly required.
			// Revisit if "Read Audit" is needed.
			return c.Next()
		}

		// 1. Get User
		user := c.Locals("user")
		var userId string

		if claims, ok := user.(jwt.MapClaims); ok {
			userId, _ = claims["id"].(string)
		} else {
			userId = "anonymous"
		}

		path := c.Path()
		ip := c.IP()
		metadata := map[string]any{}
		if strings.HasPrefix(path, "/api/mobile/") {
			metadata["actor_type"] = "mobile_user"
			metadata["actor_id"] = userId
		} else {
			metadata["actor_type"] = "user"
			metadata["actor_id"] = userId
		}
		if reqID, ok := c.Locals("request_id").(string); ok && strings.TrimSpace(reqID) != "" {
			metadata["request_id"] = reqID
		}
		if sessionID, ok := c.Locals("session_id").(string); ok && strings.TrimSpace(sessionID) != "" {
			metadata["session_id"] = sessionID
		}
		segments := strings.Split(strings.Trim(path, "/"), "/")
		if len(segments) > 1 {
			metadata["resource_kind"] = segments[1]
		}
		switch method {
		case "POST":
			metadata["action_kind"] = "create"
		case "PATCH", "PUT":
			metadata["action_kind"] = "update"
		case "DELETE":
			metadata["action_kind"] = "delete"
		default:
			metadata["action_kind"] = strings.ToLower(method)
		}

		// 2. Log Start (Info)
		logger.Info("Audit", "user", userId, "method", method, "path", path, "ip", ip)

		// 3. Next
		err := c.Next()

		// 4. Log Result (Repo)
		status := "success"
		if err != nil {
			status = "error"
		} else if c.Response().StatusCode() >= 400 {
			status = "failed"
		}

		// Fire and Forget DB Log (Non-blocking)
		if repo == nil {
			return err
		}
		go func() {
			repoErr := repo.LogAudit(userId, method, path, ip, status, metadata)
			if repoErr != nil {
				logger.Error("Failed to write Audit Log", "error", repoErr)
			}
		}()

		return err
	}
}
