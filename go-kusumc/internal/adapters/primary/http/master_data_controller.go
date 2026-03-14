package http

import (
	"ingestion-go/internal/adapters/secondary"

	"github.com/gofiber/fiber/v2"
)

type MasterDataController struct {
	repo *secondary.PostgresRepo
}

func NewMasterDataController(repo *secondary.PostgresRepo) *MasterDataController {
	return &MasterDataController{repo: repo}
}

// GET /api/master-data/:type?project_id=...
func (c *MasterDataController) List(ctx *fiber.Ctx) error {
	mdType := ctx.Params("type")
	if mdType == "" {
		return ctx.Status(400).JSON(fiber.Map{"error": "type is required"})
	}
	projectId := masterDataProjectQuery(ctx)

	items, err := c.repo.ListMasterData(mdType, projectId)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	wired := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		wired = append(wired, toWireMasterDataItem(item))
	}
	return ctx.JSON(wired)
}

func masterDataProjectQuery(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "project_id", "projectId")
}

func toWireMasterDataItem(item map[string]interface{}) map[string]interface{} {
	if item == nil {
		return map[string]interface{}{}
	}
	out := map[string]interface{}{}
	for k, v := range item {
		out[k] = v
	}
	if pid, ok := out["projectId"]; ok {
		out["project_id"] = pid
		delete(out, "projectId")
	}
	return out
}

// POST /api/master-data/:type
func (c *MasterDataController) Create(ctx *fiber.Ctx) error {
	mdType := ctx.Params("type")
	if mdType == "" {
		return ctx.Status(400).JSON(fiber.Map{"error": "type is required"})
	}

	var body struct {
		Name           string `json:"name"`
		Code           string `json:"code"`
		Value          string `json:"value"`
		ProjectID      string `json:"project_id"`
		ProjectIDCamel string `json:"projectId"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}
	if body.Name == "" {
		return ctx.Status(400).JSON(fiber.Map{"error": "name is required"})
	}
	if body.ProjectID == "" {
		body.ProjectID = body.ProjectIDCamel
	}

	code := body.Code
	if code == "" {
		code = body.Value
	}

	item, err := c.repo.CreateMasterData(mdType, body.Name, code, body.ProjectID)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(201).JSON(toWireMasterDataItem(item))
}

// PUT /api/master-data/:type/:id
func (c *MasterDataController) Update(ctx *fiber.Ctx) error {
	mdType := ctx.Params("type")
	id := ctx.Params("id")
	if mdType == "" || id == "" {
		return ctx.Status(400).JSON(fiber.Map{"error": "type and id are required"})
	}

	var body struct {
		Name           string `json:"name"`
		Code           string `json:"code"`
		Value          string `json:"value"`
		ProjectID      string `json:"project_id"`
		ProjectIDCamel string `json:"projectId"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}
	if body.Name == "" {
		return ctx.Status(400).JSON(fiber.Map{"error": "name is required"})
	}
	if body.ProjectID == "" {
		body.ProjectID = body.ProjectIDCamel
	}

	code := body.Code
	if code == "" {
		code = body.Value
	}

	item, err := c.repo.UpdateMasterData(mdType, id, body.Name, code, body.ProjectID)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.JSON(toWireMasterDataItem(item))
}

// DELETE /api/master-data/:type/:id
func (c *MasterDataController) Delete(ctx *fiber.Ctx) error {
	mdType := ctx.Params("type")
	id := ctx.Params("id")
	if mdType == "" || id == "" {
		return ctx.Status(400).JSON(fiber.Map{"error": "type and id are required"})
	}

	if err := c.repo.DeleteMasterData(mdType, id); err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.SendStatus(204)
}
