package http

import (
	"strings"

	"ingestion-go/internal/core/services"

	"github.com/gofiber/fiber/v2"
)

type ApiKeysController struct {
	svc *services.ApiKeyService
}

func NewApiKeyController(svc *services.ApiKeyService) *ApiKeysController {
	return &ApiKeysController{svc: svc}
}

func (c *ApiKeysController) List(ctx *fiber.Ctx) error {
	keys, err := c.svc.List()
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.JSON(normalizeToSnakeKeys(keys))
}

func (c *ApiKeysController) Create(ctx *fiber.Ctx) error {
	var body struct {
		Name      string   `json:"name"`
		Scopes    []string `json:"scopes"`
		ProjectID *string  `json:"project_id"`
		OrgID     *string  `json:"org_id"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}
	if strings.TrimSpace(body.Name) == "" {
		return ctx.Status(400).JSON(fiber.Map{"error": "name is required"})
	}
	if body.Scopes == nil {
		body.Scopes = []string{}
	}

	var createdBy *string
	if val, ok := ctx.Locals("user_id").(string); ok && strings.TrimSpace(val) != "" {
		createdBy = &val
	}

	res, err := c.svc.Create(services.ApiKeyCreateInput{
		Name:      body.Name,
		Scopes:    body.Scopes,
		ProjectID: body.ProjectID,
		OrgID:     body.OrgID,
		CreatedBy: createdBy,
	})
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(201).JSON(fiber.Map{
		"secret": res.Secret,
	})
}

func (c *ApiKeysController) Revoke(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	if strings.TrimSpace(id) == "" {
		return ctx.Status(400).JSON(fiber.Map{"error": "id is required"})
	}
	if err := c.svc.Revoke(id); err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.SendStatus(204)
}
