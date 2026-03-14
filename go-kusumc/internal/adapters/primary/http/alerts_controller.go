package http

import (
	"ingestion-go/internal/adapters/secondary"

	"github.com/gofiber/fiber/v2"
)

type AlertsController struct {
	repo *secondary.PostgresRepo
}

func NewAlertsController(repo *secondary.PostgresRepo) *AlertsController {
	return &AlertsController{repo: repo}
}

func (c *AlertsController) GetAlerts(ctx *fiber.Ctx) error {
	projectId := alertsProjectIDQuery(ctx)
	status := alertsStatusQuery(ctx)

	// If projectId missing, maybe try to derive from user context?
	// For now, require it or return empty/all (if admin).
	if projectId == "" {
		// Mock default or error
		// return ctx.Status(400).SendString("projectId required")
		// Assume "global" context for admin
	}

	alerts, err := c.repo.GetAlerts(projectId, status)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(normalizeToSnakeKeys(alerts))
}

func alertsProjectIDQuery(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "project_id", "projectId")
}

func alertsStatusQuery(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "status", "status_filter")
}

func (c *AlertsController) AckAlert(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	// Get User ID from JWT context (Phase 11)
	user := ctx.Locals("user").(map[string]interface{})
	userId := user["id"].(string)

	err := c.repo.AckAlert(id, userId)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(fiber.Map{"status": "acknowledged"})
}
