package http

import (
	"ingestion-go/internal/core/services"

	"github.com/gofiber/fiber/v2"
)

type ConfigController struct {
	service *services.ConfigService
}

func NewConfigController(service *services.ConfigService) *ConfigController {
	return &ConfigController{service: service}
}

// --- Automation Flows ---
func (c *ConfigController) SaveAutomationFlow(ctx *fiber.Ctx) error {
	var body map[string]interface{}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString(err.Error())
	}
	// Accept legacy camelCase for *top-level* fields only. Do not normalize nested
	// nodes/edges, since the ReactFlow payload includes camelCase keys.
	if _, ok := body["project_id"]; !ok {
		if v, ok := body["projectId"]; ok {
			body["project_id"] = v
		}
	}
	if _, ok := body["schema_version"]; !ok {
		if v, ok := body["schemaVersion"]; ok {
			body["schema_version"] = v
		}
	}
	if _, ok := body["compiled_rules"]; !ok {
		if v, ok := body["compiledRules"]; ok {
			body["compiled_rules"] = v
		}
	}
	diagnostics, err := c.service.SaveAutomationFlow(body)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	if len(diagnostics.Errors) > 0 {
		return ctx.Status(400).JSON(diagnostics)
	}
	return ctx.JSON(diagnostics)
}

func (c *ConfigController) GetAutomationFlow(ctx *fiber.Ctx) error {
	projectId := ctx.Params("projectId")
	data, err := c.service.GetAutomationFlow(projectId)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	// If nil (not found), return empty object 200 OK for builder init
	if data == nil {
		return ctx.JSON(map[string]interface{}{"nodes": []interface{}{}, "edges": []interface{}{}, "compiled_rules": []interface{}{}, "schema_version": "1.0.0"})
	}
	if _, ok := data["compiled_rules"]; !ok {
		data["compiled_rules"] = []interface{}{}
	}
	if _, ok := data["schema_version"]; !ok {
		data["schema_version"] = "1.0.0"
	}
	// Normalize top-level keys for the wire format while keeping ReactFlow
	// node/edge payloads unchanged.
	out := map[string]any{}
	for k, v := range data {
		switch k {
		case "nodes", "edges":
			out[k] = v
		default:
			out[toSnakeKey(k)] = normalizeToSnakeKeys(v)
		}
	}
	return ctx.JSON(out)
}

// --- Device Profiles ---
func (c *ConfigController) CreateDeviceProfile(ctx *fiber.Ctx) error {
	var body map[string]interface{}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString(err.Error())
	}
	if err := c.service.CreateDeviceProfile(body); err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.SendStatus(201)
}

func (c *ConfigController) GetDeviceProfiles(ctx *fiber.Ctx) error {
	data, err := c.service.GetDeviceProfiles()
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(normalizeToSnakeKeys(data))
}
