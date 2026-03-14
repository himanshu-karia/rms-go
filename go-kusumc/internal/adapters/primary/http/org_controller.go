package http

import (
	"strings"

	"ingestion-go/internal/core/services"

	"github.com/gofiber/fiber/v2"
)

type OrgController struct {
	svc *services.OrgService
}

func NewOrgController(svc *services.OrgService) *OrgController {
	return &OrgController{svc: svc}
}

func (c *OrgController) List(ctx *fiber.Ctx) error {
	orgs, err := c.svc.List()
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.JSON(normalizeToSnakeKeys(orgs))
}

func (c *OrgController) Create(ctx *fiber.Ctx) error {
	var body struct {
		Name     string                 `json:"name"`
		Type     string                 `json:"type"`
		Path     string                 `json:"path"`
		ParentID *string                `json:"parent_id"`
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}
	if strings.TrimSpace(body.Name) == "" || strings.TrimSpace(body.Type) == "" {
		return ctx.Status(400).JSON(fiber.Map{"error": "name and type are required"})
	}
	if body.Metadata == nil {
		body.Metadata = map[string]interface{}{}
	}
	org, err := c.svc.Create(body.Name, body.Type, body.Path, body.ParentID, body.Metadata)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(201).JSON(normalizeToSnakeKeys(org))
}

func (c *OrgController) Update(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	if strings.TrimSpace(id) == "" {
		return ctx.Status(400).JSON(fiber.Map{"error": "id is required"})
	}
	var body struct {
		Name     string                 `json:"name"`
		Type     string                 `json:"type"`
		Path     string                 `json:"path"`
		ParentID *string                `json:"parent_id"`
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}
	if strings.TrimSpace(body.Name) == "" || strings.TrimSpace(body.Type) == "" {
		return ctx.Status(400).JSON(fiber.Map{"error": "name and type are required"})
	}
	if body.Metadata == nil {
		body.Metadata = map[string]interface{}{}
	}
	org, err := c.svc.Update(id, body.Name, body.Type, body.Path, body.ParentID, body.Metadata)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.JSON(normalizeToSnakeKeys(org))
}
