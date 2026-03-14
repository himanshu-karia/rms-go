package http

import (
	"encoding/json"
	"time"

	"ingestion-go/internal/core/domain"
	"ingestion-go/internal/core/services"

	"github.com/gofiber/fiber/v2"
)

// CommandCatalogController manages authoring of command definitions.
type CommandCatalogController struct {
	svc *services.CommandsService
}

func NewCommandCatalogController(svc *services.CommandsService) *CommandCatalogController {
	return &CommandCatalogController{svc: svc}
}

type commandCatalogWire struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Scope         string         `json:"scope"`
	ProtocolID    *string        `json:"protocol_id,omitempty"`
	ModelID       *string        `json:"model_id,omitempty"`
	ProjectID     *string        `json:"project_id,omitempty"`
	PayloadSchema map[string]any `json:"payload_schema,omitempty"`
	Transport     string         `json:"transport"`
	CreatedAt     time.Time      `json:"created_at"`
}

// GET /api/commands/catalog-admin?project_id=...&device_id=...
func (c *CommandCatalogController) List(ctx *fiber.Ctx) error {
	projectID := commandCatalogProjectQuery(ctx)
	if projectID == "" {
		return ctx.Status(400).JSON(fiber.Map{"error": "project_id is required"})
	}
	deviceID := commandCatalogDeviceQuery(ctx)
	if deviceID == "" {
		return ctx.Status(400).JSON(fiber.Map{"error": "device_id is required to scope capabilities"})
	}
	items, err := c.svc.ListCatalog(deviceID, projectID)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	out := make([]commandCatalogWire, 0, len(items))
	for _, item := range items {
		out = append(out, commandCatalogWire{
			ID:            item.ID,
			Name:          item.Name,
			Scope:         item.Scope,
			ProtocolID:    item.ProtocolID,
			ModelID:       item.ModelID,
			ProjectID:     item.ProjectID,
			PayloadSchema: item.PayloadSchema,
			Transport:     item.Transport,
			CreatedAt:     item.CreatedAt,
		})
	}
	return ctx.JSON(out)
}

func commandCatalogProjectQuery(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "project_id", "projectId")
}

func commandCatalogDeviceQuery(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "device_id", "deviceId", "imei")
}

// POST /api/commands/catalog
func (c *CommandCatalogController) Upsert(ctx *fiber.Ctx) error {
	var body struct {
		ID                 string         `json:"id"`
		Name               string         `json:"name"`
		Scope              string         `json:"scope"`
		ProtocolID         *string        `json:"protocol_id"`
		ProtocolIDCamel    *string        `json:"protocolId"`
		ModelID            *string        `json:"model_id"`
		ModelIDCamel       *string        `json:"modelId"`
		ProjectID          *string        `json:"project_id"`
		ProjectIDCamel     *string        `json:"projectId"`
		PayloadSchema      map[string]any `json:"payload_schema"`
		PayloadSchemaCamel map[string]any `json:"payloadSchema"`
		Transport          string         `json:"transport"`
		DeviceIDs          []string       `json:"device_ids"`
		DeviceIDsCamel     []string       `json:"deviceIds"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}
	if body.ProtocolID == nil {
		body.ProtocolID = body.ProtocolIDCamel
	}
	if body.ModelID == nil {
		body.ModelID = body.ModelIDCamel
	}
	if body.ProjectID == nil {
		body.ProjectID = body.ProjectIDCamel
	}
	if body.PayloadSchema == nil {
		body.PayloadSchema = body.PayloadSchemaCamel
	}
	if len(body.DeviceIDs) == 0 {
		body.DeviceIDs = body.DeviceIDsCamel
	}
	rec := domain.CommandCatalog{
		ID:            body.ID,
		Name:          body.Name,
		Scope:         body.Scope,
		ProtocolID:    body.ProtocolID,
		ModelID:       body.ModelID,
		ProjectID:     body.ProjectID,
		PayloadSchema: body.PayloadSchema,
		Transport:     body.Transport,
	}
	if rec.PayloadSchema == nil {
		// allow raw JSON schema payload as string
		if raw := ctx.Body(); len(raw) > 0 {
			var parsed map[string]any
			if err := json.Unmarshal(raw, &parsed); err == nil {
				if v, ok := parsed["payload_schema"].(map[string]any); ok {
					rec.PayloadSchema = v
				} else if v, ok := parsed["payloadSchema"].(map[string]any); ok {
					rec.PayloadSchema = v
				}
			}
		}
	}
	id, err := c.svc.UpsertCatalog(rec, body.DeviceIDs)
	if err != nil {
		return ctx.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(201).JSON(fiber.Map{"id": id})
}

// DELETE /api/commands/catalog/:id
func (c *CommandCatalogController) Delete(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	if id == "" {
		return ctx.Status(400).JSON(fiber.Map{"error": "id required"})
	}
	if err := c.svc.DeleteCatalog(id); err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.SendStatus(204)
}
