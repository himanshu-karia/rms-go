package http

import (
	"strconv"

	"ingestion-go/internal/adapters/secondary"

	"github.com/gofiber/fiber/v2"
)

type AuditController struct {
	repo *secondary.PostgresRepo
}

func NewAuditController(repo *secondary.PostgresRepo) *AuditController {
	return &AuditController{repo: repo}
}

// GET /api/audit
func (c *AuditController) List(ctx *fiber.Ctx) error {
	limit := 100
	if v := auditQuery(ctx, "limit"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed < 1 || parsed > 200 {
			return ctx.Status(400).JSON(fiber.Map{
				"message": "Invalid audit query parameters",
				"issues":  []fiber.Map{{"path": []string{"limit"}, "message": "limit must be between 1 and 200"}},
			})
		}
		limit = parsed
	}

	afterId := auditCursorQuery(ctx)
	actorId := auditQuery(ctx, "actorId")
	action := auditQuery(ctx, "action")
	stateId := auditQuery(ctx, "stateId")
	authorityId := auditQuery(ctx, "authorityId")
	projectId := auditQuery(ctx, "projectId")

	if afterId != "" && !isUUID(afterId) {
		return ctx.Status(400).JSON(fiber.Map{
			"message": "Invalid audit query parameters",
			"issues":  []fiber.Map{{"path": []string{"after_id"}, "message": "Value must be a valid UUID"}},
		})
	}
	if actorId != "" && !isUUID(actorId) {
		return ctx.Status(400).JSON(fiber.Map{
			"message": "Invalid audit query parameters",
			"issues":  []fiber.Map{{"path": []string{"actor_id"}, "message": "Value must be a valid UUID"}},
		})
	}

	events, err := c.repo.ListAuditLogs(limit, afterId, actorId, action, stateId, authorityId, projectId)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"message": err.Error()})
	}

	serialized := make([]fiber.Map, 0, len(events))
	for _, evt := range events {
		actorID := evt["user_id"]
		actorType := "system"
		if actorID != nil && actorID != "" {
			actorType = "user"
		}
		serialized = append(serialized, fiber.Map{
			"id":             evt["id"],
			"correlation_id": nil,
			"actor": fiber.Map{
				"type": actorType,
				"id":   actorID,
			},
			"action": evt["action"],
			"entity": fiber.Map{
				"type": nil,
				"id":   nil,
			},
			"scope":      nil,
			"reason":     nil,
			"metadata":   evt["metadata"],
			"created_at": evt["ts"],
		})
	}

	nextCursor := interface{}(nil)
	if len(serialized) > 0 {
		if last, ok := serialized[len(serialized)-1]["id"]; ok {
			nextCursor = last
		}
	}

	return ctx.JSON(fiber.Map{"events": serialized, "next_cursor": nextCursor})
}

func auditQuery(ctx *fiber.Ctx, key string) string {
	if key == "afterId" {
		return queryAlias(ctx, "after_id", "afterId")
	}
	if key == "actorId" {
		return queryAlias(ctx, "actor_id", "actorId")
	}
	if key == "stateId" {
		return queryAlias(ctx, "state_id", "stateId")
	}
	if key == "authorityId" {
		return queryAlias(ctx, "authority_id", "authorityId")
	}
	if key == "projectId" {
		return queryAlias(ctx, "project_id", "projectId")
	}
	if key == "limit" {
		return queryAlias(ctx, "limit", "pageSize")
	}
	if value := queryAlias(ctx, key); value != "" {
		return value
	}
	return ""
}

func auditCursorQuery(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "cursor", "after_id", "afterId")
}
