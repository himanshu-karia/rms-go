package http

import (
	"time"

	"ingestion-go/internal/core/services"

	"github.com/gofiber/fiber/v2"
)

type MeshNodesController struct {
	svc *services.MeshNodesService
}

func NewMeshNodesController(svc *services.MeshNodesService) *MeshNodesController {
	return &MeshNodesController{svc: svc}
}

// GET /api/devices/:device_uuid/nodes?project_id=...&include_disabled=false
func (c *MeshNodesController) ListForGateway(ctx *fiber.Ctx) error {
	deviceRef := pathParamAlias(ctx, "device_uuid", "deviceUuid")
	if deviceRef == "" {
		return ctx.Status(400).JSON(fiber.Map{"error": "device_uuid is required"})
	}
	projectID := queryAlias(ctx, "project_id", "projectId")
	includeDisabled := queryAliasBool(ctx, false, "include_disabled", "includeDisabled")
	items, err := c.svc.ListGatewayNodes(deviceRef, projectID, includeDisabled)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	wired := make([]map[string]any, 0, len(items))
	for _, item := range items {
		wired = append(wired, map[string]any{
			"id":            item["id"],
			"project_id":    firstNonEmpty(asString(item["projectId"]), asString(item["project_id"])),
			"node_id":       firstNonEmpty(asString(item["nodeId"]), asString(item["node_id"])),
			"label":         item["label"],
			"kind":          item["kind"],
			"attributes":    item["attributes"],
			"enabled":       item["enabled"],
			"discovered":    item["discovered"],
			"last_seen":     firstNonEmpty(asTimeString(item["lastSeen"]), asTimeString(item["last_seen"])),
			"link_metadata": item["linkMetadata"],
		})
	}
	return ctx.JSON(fiber.Map{"device": fiber.Map{"ref": deviceRef}, "nodes": wired})
}

// POST /api/devices/:device_uuid/nodes/attach
func (c *MeshNodesController) Attach(ctx *fiber.Ctx) error {
	deviceRef := pathParamAlias(ctx, "device_uuid", "deviceUuid")
	if deviceRef == "" {
		return ctx.Status(400).JSON(fiber.Map{"error": "device_uuid is required"})
	}
	var body struct {
		ProjectID      string         `json:"project_id"`
		ProjectIDCamel string         `json:"projectId"`
		NodeID         string         `json:"node_id"`
		NodeIDCamel    string         `json:"nodeId"`
		Label          string         `json:"label"`
		Kind           string         `json:"kind"`
		Attributes     map[string]any `json:"attributes"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}
	nodeID := firstNonEmpty(body.NodeID, body.NodeIDCamel)
	projectID := firstNonEmpty(body.ProjectID, body.ProjectIDCamel)
	if nodeID == "" {
		return ctx.Status(400).JSON(fiber.Map{"error": "nodeId is required"})
	}
	out, err := c.svc.AttachNode(deviceRef, projectID, nodeID, body.Label, body.Kind, body.Attributes)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(201).JSON(fiber.Map{
		"node_uuid": out["nodeUuid"],
		"node_id":   out["nodeId"],
	})
}

// POST /api/devices/:device_uuid/nodes/detach
func (c *MeshNodesController) Detach(ctx *fiber.Ctx) error {
	deviceRef := pathParamAlias(ctx, "device_uuid", "deviceUuid")
	if deviceRef == "" {
		return ctx.Status(400).JSON(fiber.Map{"error": "device_uuid is required"})
	}
	var body struct {
		ProjectID      string `json:"project_id"`
		ProjectIDCamel string `json:"projectId"`
		NodeUUID       string `json:"node_uuid"`
		NodeUUIDCamel  string `json:"nodeUuid"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}
	projectID := firstNonEmpty(body.ProjectID, body.ProjectIDCamel)
	nodeUUID := firstNonEmpty(body.NodeUUID, body.NodeUUIDCamel)
	if err := c.svc.DetachNode(deviceRef, projectID, nodeUUID); err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.SendStatus(204)
}

// POST /api/devices/:device_uuid/nodes/discovery
func (c *MeshNodesController) Discovery(ctx *fiber.Ctx) error {
	deviceRef := pathParamAlias(ctx, "device_uuid", "deviceUuid")
	if deviceRef == "" {
		return ctx.Status(400).JSON(fiber.Map{"error": "device_uuid is required"})
	}
	var body struct {
		ProjectID      string   `json:"project_id"`
		ProjectIDCamel string   `json:"projectId"`
		NodeIDs        []string `json:"node_ids"`
		NodeIDsCamel   []string `json:"nodeIds"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}
	projectID := firstNonEmpty(body.ProjectID, body.ProjectIDCamel)
	nodeIDs := body.NodeIDs
	if len(nodeIDs) == 0 {
		nodeIDs = body.NodeIDsCamel
	}
	count, err := c.svc.ReportDiscovery(deviceRef, projectID, nodeIDs)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.JSON(fiber.Map{"attached": count})
}

func asString(value any) string {
	if value == nil {
		return ""
	}
	if v, ok := value.(string); ok {
		return v
	}
	return ""
}

func asTimeString(value any) string {
	if value == nil {
		return ""
	}
	if t, ok := value.(*time.Time); ok {
		if t == nil {
			return ""
		}
		return t.Format(time.RFC3339)
	}
	if t, ok := value.(time.Time); ok {
		return t.Format(time.RFC3339)
	}
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}
