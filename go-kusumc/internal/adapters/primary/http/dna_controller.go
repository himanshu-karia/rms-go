package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"ingestion-go/internal/config/dna"
	"ingestion-go/internal/core/services"

	"github.com/gofiber/fiber/v2"
)

// DNAController exposes CRUD endpoints for project DNA records.
type DNAController struct {
	service *services.DNAService
}

func NewDNAController(service *services.DNAService) *DNAController {
	return &DNAController{service: service}
}

func (c *DNAController) List(ctx *fiber.Ctx) error {
	records, err := c.service.ListAll(ctx.Context())
	if err != nil {
		return ctx.Status(http.StatusInternalServerError).SendString(err.Error())
	}
	return ctx.JSON(normalizeToSnakeKeys(records))
}

func (c *DNAController) Get(ctx *fiber.Ctx) error {
	projectID := ctx.Params("projectId")
	if projectID == "" {
		return ctx.Status(http.StatusBadRequest).SendString("project_id required")
	}

	record, err := c.service.Get(ctx.Context(), projectID)
	if err != nil {
		return ctx.Status(http.StatusInternalServerError).SendString(err.Error())
	}
	if record == nil {
		return ctx.SendStatus(http.StatusNotFound)
	}
	return ctx.JSON(normalizeToSnakeKeys(record))
}

func (c *DNAController) Upsert(ctx *fiber.Ctx) error {
	projectID := ctx.Params("projectId")
	if projectID == "" {
		return ctx.Status(http.StatusBadRequest).SendString("project_id required")
	}

	bodyBytes := ctx.Body()
	var payload dna.ProjectPayloadSchema
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		return ctx.Status(http.StatusBadRequest).SendString(err.Error())
	}
	// Top-level compatibility: accept snake_case project_id as well as legacy projectId.
	// Note: deeper nested payload sections (e.g. payload rows) still follow their
	// existing schema; changing those requires a dedicated migration.
	if payload.ProjectID == "" {
		var raw map[string]any
		if err := json.Unmarshal(bodyBytes, &raw); err == nil {
			if v, ok := raw["project_id"].(string); ok {
				payload.ProjectID = v
			} else if v, ok := raw["projectId"].(string); ok {
				payload.ProjectID = v
			}
		}
	}
	if payload.ProjectID == "" {
		payload.ProjectID = projectID
	}
	if payload.ProjectID != projectID {
		return ctx.Status(http.StatusBadRequest).SendString("payload project_id mismatch")
	}

	if err := c.service.Save(payload); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, services.ErrInvalidDNA) {
			status = http.StatusBadRequest
		}
		return ctx.Status(status).SendString(err.Error())
	}
	return ctx.SendStatus(http.StatusNoContent)
}
