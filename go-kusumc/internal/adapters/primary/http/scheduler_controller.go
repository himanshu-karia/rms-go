package http

import (
	"ingestion-go/internal/adapters/secondary"

	"github.com/gofiber/fiber/v2"
)

type SchedulerController struct {
	repo *secondary.PostgresRepo
}

func NewSchedulerController(repo *secondary.PostgresRepo) *SchedulerController {
	return &SchedulerController{repo: repo}
}

func (c *SchedulerController) GetSchedules(ctx *fiber.Ctx) error {
	list, err := c.repo.GetSchedules()
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(normalizeToSnakeKeys(list))
}

func (c *SchedulerController) CreateSchedule(ctx *fiber.Ctx) error {
	var body map[string]interface{}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString("Bad JSON")
	}
	if normalized, ok := normalizeToSnakeKeys(body).(map[string]interface{}); ok {
		body = normalized
	}

	if err := c.repo.CreateSchedule(body); err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.SendStatus(201)
}

func (c *SchedulerController) ToggleSchedule(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	if err := c.repo.ToggleSchedule(id); err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(fiber.Map{"status": "toggled"})
}
