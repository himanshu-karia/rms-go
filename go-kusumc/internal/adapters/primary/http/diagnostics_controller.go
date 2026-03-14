package http

import (
	"ingestion-go/internal/core/workers"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type deadLetterReplayRunner interface {
	ReplayNow(limit int) workers.DeadLetterReplayResult
	QueueLength() int64
}

type DeadLetterDiagnosticsController struct {
	replay deadLetterReplayRunner
}

func NewDeadLetterDiagnosticsController(replay deadLetterReplayRunner) *DeadLetterDiagnosticsController {
	return &DeadLetterDiagnosticsController{replay: replay}
}

// GET /api/diagnostics/ingest/deadletter
func (c *DeadLetterDiagnosticsController) GetStatus(ctx *fiber.Ctx) error {
	if c.replay == nil {
		return ctx.Status(503).JSON(fiber.Map{"message": "dead-letter replay unavailable"})
	}
	return ctx.JSON(fiber.Map{
		"queue":       "ingest:deadletter",
		"queue_len":   c.replay.QueueLength(),
		"replay_mode": "manual_or_worker",
	})
}

// POST /api/diagnostics/ingest/deadletter/replay
func (c *DeadLetterDiagnosticsController) Replay(ctx *fiber.Ctx) error {
	if c.replay == nil {
		return ctx.Status(503).JSON(fiber.Map{"message": "dead-letter replay unavailable"})
	}

	limit := 50
	if q := strings.TrimSpace(ctx.Query("limit")); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			limit = n
		}
	}
	if len(ctx.Body()) > 0 {
		var body struct {
			Limit int `json:"limit"`
		}
		if err := ctx.BodyParser(&body); err == nil && body.Limit > 0 {
			limit = body.Limit
		}
	}
	if limit > 1000 {
		limit = 1000
	}

	res := c.replay.ReplayNow(limit)
	return ctx.JSON(res)
}
