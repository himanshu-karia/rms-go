package http

import (
	"ingestion-go/internal/core/services"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type SimulatorController struct {
	service *services.SimulatorService
}

func NewSimulatorController(service *services.SimulatorService) *SimulatorController {
	return &SimulatorController{service: service}
}

func (c *SimulatorController) GetScript(ctx *fiber.Ctx) error {
	projectId := ctx.Params("projectId")
	if projectId == "" {
		return ctx.Status(400).SendString("Missing project_id")
	}

	script := c.service.GenerateScript(projectId)

	ctx.Set("Content-Type", "text/x-python")
	ctx.Set("Content-Disposition", "attachment; filename=simulator.py")
	return ctx.Send(script)
}

// StartSimulation (Stub for UI Trigger)
func (c *SimulatorController) StartSimulation(ctx *fiber.Ctx) error {
	// User Requirement: "Device API simulations from UI"
	// In Node, this might spawn a process or just return success if client-side sim.
	// We'll log it and return enabled status.
	var req struct {
		ProjectID      string `json:"project_id"`
		ProjectIDCamel string `json:"projectId"`
		Count          int    `json:"count"`
	}
	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(400).SendString("Bad Request")
	}
	if req.ProjectID == "" {
		req.ProjectID = req.ProjectIDCamel
	}

	// Real implementation would spawn Go routines or Docker containers.
	// For V1 Parity with "Client Sim", we acknowledge.
	return ctx.JSON(fiber.Map{
		"status":  "started",
		"message": "Simulation backend trigger received",
		"target":  req.ProjectID,
	})
}

// CreateSession creates a simulator session for a device.
func (c *SimulatorController) CreateSession(ctx *fiber.Ctx) error {
	var body struct {
		DeviceUUID            string `json:"device_uuid"`
		DeviceUUIDCamel       string `json:"deviceUuid"`
		ExpiresInMinutes      int    `json:"expires_in_minutes"`
		ExpiresInMinutesCamel int    `json:"expiresInMinutes"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).JSON(fiber.Map{
			"message": "Invalid simulator session payload",
			"issues":  []fiber.Map{{"field": "body", "message": "invalid json payload"}},
		})
	}
	if strings.TrimSpace(body.DeviceUUID) == "" {
		body.DeviceUUID = body.DeviceUUIDCamel
	}
	if body.ExpiresInMinutes == 0 {
		body.ExpiresInMinutes = body.ExpiresInMinutesCamel
	}

	issues := []fiber.Map{}
	if strings.TrimSpace(body.DeviceUUID) == "" {
		issues = append(issues, fiber.Map{"field": "device_uuid", "message": "device_uuid is required"})
	}
	if body.ExpiresInMinutes != 0 && (body.ExpiresInMinutes < 5 || body.ExpiresInMinutes > 480) {
		issues = append(issues, fiber.Map{"field": "expires_in_minutes", "message": "expires_in_minutes must be between 5 and 480"})
	}
	if len(issues) > 0 {
		return ctx.Status(400).JSON(fiber.Map{
			"message": "Invalid simulator session payload",
			"issues":  issues,
		})
	}

	userID, _ := ctx.Locals("user_id").(string)
	result, err := c.service.CreateSession(services.CreateSimulatorSessionInput{
		DeviceUUID:       body.DeviceUUID,
		ExpiresInMinutes: body.ExpiresInMinutes,
		RequestedBy:      toNullableString(userID),
	})
	if err != nil {
		return ctx.Status(400).JSON(fiber.Map{"message": err.Error()})
	}

	return ctx.Status(201).JSON(result)
}

// ListSessions lists simulator sessions with optional filters.
func (c *SimulatorController) ListSessions(ctx *fiber.Ctx) error {
	limit := 0
	if raw := simulatorLimitQuery(ctx); raw != "" {
		val, err := strconv.Atoi(raw)
		if err != nil {
			return ctx.Status(400).JSON(fiber.Map{
				"message": "Invalid simulator session query parameters",
				"issues":  []fiber.Map{{"field": "limit", "message": "limit must be an integer"}},
			})
		}
		if val < 1 || val > 100 {
			return ctx.Status(400).JSON(fiber.Map{
				"message": "Invalid simulator session query parameters",
				"issues":  []fiber.Map{{"field": "limit", "message": "limit must be between 1 and 100"}},
			})
		}
		limit = val
	}
	status := simulatorStatusQuery(ctx)
	if status != "" && status != "active" && status != "revoked" && status != "expired" {
		return ctx.Status(400).JSON(fiber.Map{
			"message": "Invalid simulator session query parameters",
			"issues":  []fiber.Map{{"field": "status", "message": "status must be active, revoked, or expired"}},
		})
	}
	cursor := simulatorCursorQuery(ctx)

	sessions, nextCursor, err := c.service.ListSessions(services.ListSimulatorSessionsParams{
		Limit:  limit,
		Cursor: cursor,
		Status: status,
	})
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"message": err.Error()})
	}

	return ctx.JSON(fiber.Map{
		"count":       len(sessions),
		"sessions":    normalizeToSnakeKeys(sessions),
		"next_cursor": toNullableString(nextCursor),
	})
}

// RevokeSession revokes a simulator session.
func (c *SimulatorController) RevokeSession(ctx *fiber.Ctx) error {
	sessionID := ctx.Params("sessionId")
	if sessionID == "" {
		return ctx.Status(400).JSON(fiber.Map{"message": "sessionId is required"})
	}

	userID, _ := ctx.Locals("user_id").(string)
	session, err := c.service.RevokeSession(sessionID, toNullableString(userID))
	if err != nil {
		return ctx.Status(404).JSON(fiber.Map{"message": err.Error()})
	}

	return ctx.JSON(normalizeToSnakeKeys(session))
}

func toNullableString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func simulatorLimitQuery(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "limit", "pageSize")
}

func simulatorCursorQuery(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "cursor", "after_id", "afterId")
}

func simulatorStatusQuery(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "status", "status_filter")
}
