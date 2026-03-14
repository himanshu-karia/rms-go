package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"ingestion-go/internal/adapters/secondary"
	"ingestion-go/internal/core/services"
	"ingestion-go/internal/models"

	"github.com/gofiber/fiber/v2"
)

// --- COMMANDS ---
type CommandsController struct {
	svc *services.CommandsService
}

func NewCommandsController(svc *services.CommandsService) *CommandsController {
	return &CommandsController{svc: svc}
}

func (c *CommandsController) GetCatalog(ctx *fiber.Ctx) error {
	deviceRef := queryAlias(ctx, "device_id", "deviceId", "imei")
	projectId := queryAlias(ctx, "project_id", "projectId")
	if deviceRef == "" {
		return ctx.Status(400).JSON(fiber.Map{"error": "device_id is required"})
	}
	data, err := c.svc.ListCatalog(deviceRef, projectId)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	out := make([]wireCommandCatalog, 0, len(data))
	for _, item := range data {
		out = append(out, toWireCommandCatalog(item))
	}
	return ctx.JSON(out)
}

func (c *CommandsController) SendCommand(ctx *fiber.Ctx) error {
	var body struct {
		DeviceID       string         `json:"device_id"`
		DeviceIDCamel  string         `json:"deviceId"`
		IMEI           string         `json:"imei"`
		ProjectID      string         `json:"project_id"`
		ProjectIDCamel string         `json:"projectId"`
		CommandID      string         `json:"command_id"`
		CommandIDCamel string         `json:"commandId"`
		Payload        map[string]any `json:"payload"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString("Invalid Body")
	}
	deviceRef := firstNonEmpty(body.DeviceID, body.DeviceIDCamel, body.IMEI)
	commandID := firstNonEmpty(body.CommandID, body.CommandIDCamel)
	projectID := firstNonEmpty(body.ProjectID, body.ProjectIDCamel)
	if deviceRef == "" || commandID == "" {
		return ctx.Status(400).JSON(fiber.Map{"error": "device_id and command_id are required"})
	}
	req, err := c.svc.SendCommand(deviceRef, commandID, projectID, body.Payload)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	if req == nil {
		return ctx.Status(500).JSON(fiber.Map{"error": "command request unavailable"})
	}
	return ctx.JSON(toWireCommandRequest(*req))
}

func (c *CommandsController) ListCommands(ctx *fiber.Ctx) error {
	deviceRef := queryAlias(ctx, "device_id", "deviceId", "imei")
	if deviceRef == "" {
		return ctx.Status(400).SendString("device_id is required")
	}
	limit := ctx.QueryInt("limit", 20)
	items, err := c.svc.ListHistory(deviceRef, limit)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	out := make([]wireCommandRequest, 0, len(items))
	for _, item := range items {
		out = append(out, toWireCommandRequest(item))
	}
	return ctx.JSON(out)
}

func (c *CommandsController) ListResponses(ctx *fiber.Ctx) error {
	deviceRef := queryAlias(ctx, "device_id", "deviceId", "imei")
	if deviceRef == "" {
		return ctx.Status(400).SendString("device_id is required")
	}
	limit := ctx.QueryInt("limit", 20)
	items, err := c.svc.ListResponses(deviceRef, limit)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	out := make([]wireCommandResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toWireCommandResponse(item))
	}
	return ctx.JSON(out)
}

func (c *CommandsController) RetryCommand(ctx *fiber.Ctx) error {
	correlationID := ctx.Params("correlationId")
	if correlationID == "" {
		return ctx.Status(400).JSON(fiber.Map{"error": "correlation_id is required"})
	}
	updated, err := c.svc.RetryByCorrelation(correlationID)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	if updated == nil {
		return ctx.Status(404).JSON(fiber.Map{"error": "command not found"})
	}
	return ctx.JSON(toWireCommandRequest(*updated))
}

func (c *CommandsController) GetStatus(ctx *fiber.Ctx) error {
	deviceRef := queryAlias(ctx, "device_id", "deviceId", "imei")
	projectId := queryAlias(ctx, "project_id", "projectId")
	if deviceRef == "" {
		return ctx.Status(400).SendString("device_id is required")
	}
	stats, err := c.svc.GetStats(deviceRef, projectId)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.JSON(toWireCommandStats(stats))
}

func (c *CommandsController) IssueDeviceCommand(ctx *fiber.Ctx) error {
	deviceRef := pathParamAlias(ctx, "device_uuid", "deviceUuid")
	if deviceRef == "" {
		return ctx.Status(400).JSON(fiber.Map{
			"message": "Invalid ondemand command payload",
			"issues":  []fiber.Map{{"field": "device_uuid", "message": "device_uuid is required"}},
		})
	}
	var body struct {
		ProjectID      string `json:"project_id"`
		ProjectIDCamel string `json:"projectId"`
		CommandID      string `json:"command_id"`
		CommandIDCamel string `json:"commandId"`
		Command        struct {
			Name    string         `json:"name"`
			Payload map[string]any `json:"payload"`
		} `json:"command"`
		Payload map[string]any `json:"payload"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).JSON(fiber.Map{
			"message": "Invalid ondemand command payload",
			"issues":  []fiber.Map{{"field": "body", "message": "invalid json payload"}},
		})
	}
	if strings.TrimSpace(body.ProjectID) == "" {
		body.ProjectID = body.ProjectIDCamel
	}
	if strings.TrimSpace(body.CommandID) == "" {
		body.CommandID = body.CommandIDCamel
	}
	commandName := strings.TrimSpace(body.Command.Name)
	commandID := strings.TrimSpace(body.CommandID)
	if commandID == "" && commandName == "" {
		return ctx.Status(400).JSON(fiber.Map{
			"message": "Invalid ondemand command payload",
			"issues":  []fiber.Map{{"field": "command", "message": "command name is required"}},
		})
	}
	if commandID == "" {
		catalog, err := c.svc.ListCatalog(deviceRef, body.ProjectID)
		if err != nil {
			return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		for _, item := range catalog {
			if strings.EqualFold(item.Name, commandName) {
				commandID = item.ID
				break
			}
		}
		if commandID == "" {
			return ctx.Status(400).JSON(fiber.Map{"message": "Command not found for device"})
		}
	}
	payload := body.Command.Payload
	if payload == nil {
		payload = body.Payload
	}
	if payload == nil {
		payload = map[string]any{}
	}
	req, err := c.svc.SendCommand(deviceRef, commandID, body.ProjectID, payload)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(202).JSON(fiber.Map{
		"correlation_id": req.CorrelationID,
		"msgid":          req.CorrelationID,
		"status":         req.Status,
		"command_id":     req.CommandID,
		"device_id":      req.DeviceID,
	})
}

func (c *CommandsController) IssueSimpleDeviceCommand(ctx *fiber.Ctx, commandName string) error {
	deviceRef := pathParamAlias(ctx, "device_uuid", "deviceUuid")
	if deviceRef == "" {
		return ctx.Status(400).JSON(fiber.Map{"message": "Invalid payload"})
	}
	var body struct {
		Payload        map[string]any `json:"payload"`
		ProjectID      string         `json:"project_id"`
		ProjectIDCamel string         `json:"projectId"`
	}
	_ = ctx.BodyParser(&body)
	if strings.TrimSpace(body.ProjectID) == "" {
		body.ProjectID = body.ProjectIDCamel
	}
	payload := body.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	commandID := ""
	catalog, err := c.svc.ListCatalog(deviceRef, body.ProjectID)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	for _, item := range catalog {
		if strings.EqualFold(item.Name, commandName) {
			commandID = item.ID
			break
		}
	}
	if commandID == "" {
		return ctx.Status(400).JSON(fiber.Map{"message": "Command not found for device"})
	}
	req, err := c.svc.SendCommand(deviceRef, commandID, body.ProjectID, payload)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(202).JSON(fiber.Map{
		"correlation_id": req.CorrelationID,
		"msgid":          req.CorrelationID,
		"status":         req.Status,
		"command_id":     req.CommandID,
		"device_id":      req.DeviceID,
	})
}

func (c *CommandsController) AckDeviceCommand(ctx *fiber.Ctx) error {
	deviceRef := pathParamAlias(ctx, "device_uuid", "deviceUuid")
	if deviceRef == "" {
		return ctx.Status(400).JSON(fiber.Map{"message": "Invalid acknowledgement payload"})
	}
	var body struct {
		MsgID           string         `json:"msgid"`
		Status          string         `json:"status"`
		Payload         map[string]any `json:"payload"`
		ReceivedAt      string         `json:"received_at"`
		ReceivedAtCamel string         `json:"receivedAt"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).JSON(fiber.Map{
			"message": "Invalid acknowledgement payload",
			"issues":  []fiber.Map{{"field": "body", "message": "invalid json payload"}},
		})
	}
	msgid := strings.TrimSpace(body.MsgID)
	if msgid == "" {
		return ctx.Status(400).JSON(fiber.Map{
			"message": "Invalid acknowledgement payload",
			"issues":  []fiber.Map{{"field": "msgid", "message": "msgid is required"}},
		})
	}
	if strings.TrimSpace(body.ReceivedAt) == "" {
		body.ReceivedAt = body.ReceivedAtCamel
	}
	var receivedAt *time.Time
	if body.ReceivedAt != "" {
		if parsed, err := time.Parse(time.RFC3339, body.ReceivedAt); err == nil {
			receivedAt = &parsed
		}
	}
	resp, err := c.svc.AcknowledgeCommand(deviceRef, msgid, body.Status, body.Payload, receivedAt)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.JSON(fiber.Map{
		"correlation_id": resp.CorrelationID,
		"device_id":      resp.DeviceID,
		"project_id":     resp.ProjectID,
		"received_at":    resp.ReceivedAt.Format(time.RFC3339),
		"status":         strings.ToLower(strings.TrimSpace(body.Status)),
	})
}

func (c *CommandsController) GetDeviceCommandHistory(ctx *fiber.Ctx) error {
	deviceRef := pathParamAlias(ctx, "device_uuid", "deviceUuid")
	if deviceRef == "" {
		return ctx.Status(400).JSON(fiber.Map{"message": "Invalid command history query parameters"})
	}
	limit := ctx.QueryInt("limit", 20)
	items, err := c.svc.ListHistory(deviceRef, limit)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.JSON(fiber.Map{
		"device":      fiber.Map{"uuid": deviceRef, "imei": nil},
		"commands":    items,
		"next_cursor": nil,
	})
}

// --- RULES ---
type rulesService interface {
	GetRules(projectId, deviceId string) ([]map[string]interface{}, error)
	CreateRule(rule map[string]interface{}) (string, error)
	DeleteRule(id string) error
}

type RulesController struct {
	svc rulesService
}

func NewRulesController(svc rulesService) *RulesController {
	return &RulesController{svc: svc}
}

func (c *RulesController) GetRules(ctx *fiber.Ctx) error {
	pid := strings.TrimSpace(queryAlias(ctx, "project_id", "projectId"))
	if pid == "" {
		return ctx.Status(400).JSON(fiber.Map{"error": "project_id is required"})
	}
	did := strings.TrimSpace(queryAlias(ctx, "device_id", "deviceId"))
	data, err := c.svc.GetRules(pid, did)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(normalizeToSnakeKeys(data))
}

func (c *RulesController) CreateRule(ctx *fiber.Ctx) error {
	var body map[string]interface{}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString("Invalid Body")
	}

	bodyAny := normalizeToSnakeKeys(body)
	norm, ok := bodyAny.(map[string]any)
	if !ok {
		return ctx.Status(400).SendString("Invalid Body")
	}
	// normalizeToSnakeKeys returns map[string]any; convert to map[string]interface{} for service API.
	bodyNorm := map[string]interface{}{}
	for k, v := range norm {
		bodyNorm[k] = v
	}

	if bodyNorm["project_id"] == nil || bodyNorm["project_id"] == "" {
		return ctx.Status(400).JSON(fiber.Map{"error": "project_id is required"})
	}

	if err := validateRulePayload(bodyNorm); err != nil {
		return ctx.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	id, err := c.svc.CreateRule(bodyNorm)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(fiber.Map{"id": id})
}

func (c *RulesController) DeleteRule(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	err := c.svc.DeleteRule(id)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(fiber.Map{"status": "deleted"})
}

func validateRulePayload(body map[string]interface{}) error {
	name, _ := body["name"].(string)
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("name is required")
	}
	trigger, ok := body["trigger"].(map[string]interface{})
	if !ok || trigger == nil {
		return fmt.Errorf("trigger is required")
	}
	if _, ok := trigger["formula"].(string); !ok {
		field, _ := trigger["field"].(string)
		if strings.TrimSpace(field) == "" {
			return fmt.Errorf("trigger.field is required")
		}
		if op, _ := trigger["operator"].(string); op == "" {
			return fmt.Errorf("trigger.operator is required")
		}
		if _, hasValue := trigger["value"]; !hasValue {
			return fmt.Errorf("trigger.value is required")
		}
	}
	actions, ok := body["actions"].([]interface{})
	if !ok || len(actions) == 0 {
		return fmt.Errorf("at least one action is required")
	}
	for i, a := range actions {
		m, ok := a.(map[string]interface{})
		if !ok {
			return fmt.Errorf("actions[%d] must be an object", i)
		}
		if t, _ := m["type"].(string); strings.TrimSpace(t) == "" {
			return fmt.Errorf("actions[%d].type is required", i)
		}
	}
	return nil
}

// --- DEVICE OPEN (LEGACY) ---
type DeviceOpenController struct {
	devices   *services.DeviceService
	govtCreds *services.GovtCredsService
	protocols *services.ProtocolService
	commands  *services.CommandsService
	rules     *services.RulesService
	repo      *secondary.PostgresRepo
	vfdRepo   *secondary.PostgresVFDRepo
}

func NewDeviceOpenController(devices *services.DeviceService, govt *services.GovtCredsService, protocols *services.ProtocolService, commands *services.CommandsService, rules *services.RulesService, repo *secondary.PostgresRepo, vfdRepo *secondary.PostgresVFDRepo) *DeviceOpenController {
	return &DeviceOpenController{devices: devices, govtCreds: govt, protocols: protocols, commands: commands, rules: rules, repo: repo, vfdRepo: vfdRepo}
}

// GET /api/device-open/nodes?imei=... or ?deviceUuid=...
// Firmware-facing config: which child nodes should be forwarded by this gateway.
func (c *DeviceOpenController) GetMeshNodes(ctx *fiber.Ctx) error {
	lookup := strings.TrimSpace(queryAlias(ctx, "device_uuid", "deviceUuid", "imei", "device_id", "deviceId"))
	if lookup == "" {
		return ctx.Status(400).JSON(fiber.Map{"message": "imei or device_uuid is required"})
	}
	if c.repo == nil {
		return ctx.Status(500).JSON(fiber.Map{"message": "repo unavailable"})
	}
	device, err := c.repo.GetDeviceByIDOrIMEI(lookup)
	if err != nil || device == nil {
		return ctx.Status(404).JSON(fiber.Map{"message": "device not found"})
	}
	deviceID, _ := device["id"].(string)
	if strings.TrimSpace(deviceID) == "" {
		return ctx.Status(500).JSON(fiber.Map{"message": "device metadata incomplete"})
	}
	items, err := c.repo.ListMeshNodesForGateway(deviceID, false)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"message": err.Error()})
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
	return ctx.JSON(fiber.Map{
		"device": c.mapDeviceSummary(device),
		"nodes":  wired,
	})
}

func (c *DeviceOpenController) LegacyBootstrap(ctx *fiber.Ctx) error {
	// Maps /api/device-open/bootstrap -> services.BootstrapService
	// Logic matches BootstrapService
	// We can reuse the existing service logic or call it directly?
	// The BootstrapService already handles this logic.
	// This controller just provides the route alias.
	return ctx.SendString("Use /api/bootstrap instead for V1 Go")
	// Or we can invoke usage.
}

// GET /api/device-open/credentials/local?imei=...
func (c *DeviceOpenController) GetLocalCredentials(ctx *fiber.Ctx) error {
	imei := strings.TrimSpace(ctx.Query("imei"))
	if imei == "" {
		return ctx.Status(400).JSON(fiber.Map{"message": "imei is required"})
	}
	device, err := c.lookupDevice(imei)
	if err != nil || device == nil {
		return ctx.Status(404).JSON(fiber.Map{"message": "device not found"})
	}
	cred, err := c.buildLocalCredential(device)
	if err != nil {
		return ctx.Status(404).JSON(fiber.Map{"message": err.Error()})
	}
	ctx.Set("Cache-Control", "no-store")
	return ctx.JSON(fiber.Map{
		"device":     c.mapDeviceSummary(device),
		"credential": cred,
	})
}

// GET /api/device-open/credentials/government?imei=...
func (c *DeviceOpenController) GetGovernmentCredentials(ctx *fiber.Ctx) error {
	imei := strings.TrimSpace(ctx.Query("imei"))
	if imei == "" {
		return ctx.Status(400).JSON(fiber.Map{"message": "imei is required"})
	}
	device, err := c.lookupDevice(imei)
	if err != nil || device == nil {
		return ctx.Status(404).JSON(fiber.Map{"message": "device not found"})
	}
	cred, err := c.buildGovernmentCredential(ctx.Context(), device)
	if err != nil {
		return ctx.Status(404).JSON(fiber.Map{"message": err.Error()})
	}
	ctx.Set("Cache-Control", "no-store")
	return ctx.JSON(fiber.Map{
		"device":     c.mapDeviceSummary(device),
		"credential": cred,
	})
}

// GET /api/device-open/vfd?imei=...&deviceUuid=...
func (c *DeviceOpenController) GetVFDModels(ctx *fiber.Ctx) error {
	lookup := strings.TrimSpace(queryAlias(ctx, "device_uuid", "deviceUuid", "imei"))
	if lookup == "" {
		return ctx.Status(400).JSON(fiber.Map{"message": "imei or device_uuid is required"})
	}
	device, err := c.lookupDevice(lookup)
	if err != nil || device == nil {
		return ctx.Status(404).JSON(fiber.Map{"message": "device not found"})
	}
	models, err := c.resolveVFDModels(ctx.Context(), device)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"message": err.Error()})
	}
	return ctx.JSON(fiber.Map{
		"device":     c.mapDeviceSummary(device),
		"vfd_models": models,
	})
}

// GET /api/device-open/commands/history?imei=...&deviceUuid=...&limit=20
func (c *DeviceOpenController) GetCommandHistory(ctx *fiber.Ctx) error {
	lookup := c.commandLookup(ctx)
	if lookup == "" {
		return ctx.Status(400).JSON(fiber.Map{"message": "imei or device_uuid is required"})
	}
	if c.commands == nil {
		return ctx.Status(500).JSON(fiber.Map{"message": "commands service unavailable"})
	}
	device, err := c.lookupDevice(lookup)
	if err != nil || device == nil {
		return ctx.Status(404).JSON(fiber.Map{"message": "device not found"})
	}
	deviceRef := strings.TrimSpace(fmt.Sprintf("%v", device["id"]))
	if deviceRef == "" {
		deviceRef = lookup
	}
	limit := clampLimit(ctx.QueryInt("limit", 20), 20, 100)
	items, err := c.commands.ListHistory(deviceRef, limit)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"message": err.Error()})
	}
	wired := make([]wireCommandRequest, 0, len(items))
	for _, item := range items {
		wired = append(wired, toWireCommandRequest(item))
	}
	return ctx.JSON(fiber.Map{
		"device":     c.mapDeviceSummary(device),
		"commands":   wired,
		"nextCursor": nil,
	})
}

// GET /api/device-open/commands/responses?imei=...&deviceUuid=...&limit=20
func (c *DeviceOpenController) GetCommandResponses(ctx *fiber.Ctx) error {
	lookup := c.commandLookup(ctx)
	if lookup == "" {
		return ctx.Status(400).JSON(fiber.Map{"message": "imei or device_uuid is required"})
	}
	if c.commands == nil {
		return ctx.Status(500).JSON(fiber.Map{"message": "commands service unavailable"})
	}
	device, err := c.lookupDevice(lookup)
	if err != nil || device == nil {
		return ctx.Status(404).JSON(fiber.Map{"message": "device not found"})
	}
	deviceRef := strings.TrimSpace(fmt.Sprintf("%v", device["id"]))
	if deviceRef == "" {
		deviceRef = lookup
	}
	limit := clampLimit(ctx.QueryInt("limit", 20), 20, 100)
	items, err := c.commands.ListResponses(deviceRef, limit)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"message": err.Error()})
	}
	wired := make([]wireCommandResponse, 0, len(items))
	for _, item := range items {
		wired = append(wired, toWireCommandResponse(item))
	}
	return ctx.JSON(fiber.Map{
		"device":    c.mapDeviceSummary(device),
		"responses": wired,
	})
}

// GET /api/device-open/commands/status?imei=...&deviceUuid=...&projectId=...
func (c *DeviceOpenController) GetCommandStatus(ctx *fiber.Ctx) error {
	lookup := c.commandLookup(ctx)
	if lookup == "" {
		return ctx.Status(400).JSON(fiber.Map{"message": "imei or device_uuid is required"})
	}
	if c.commands == nil {
		return ctx.Status(500).JSON(fiber.Map{"message": "commands service unavailable"})
	}
	device, err := c.lookupDevice(lookup)
	if err != nil || device == nil {
		return ctx.Status(404).JSON(fiber.Map{"message": "device not found"})
	}
	deviceRef := strings.TrimSpace(fmt.Sprintf("%v", device["id"]))
	if deviceRef == "" {
		deviceRef = lookup
	}
	projectID := queryAlias(ctx, "project_id", "projectId")
	stats, err := c.commands.GetStats(deviceRef, projectID)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"message": err.Error()})
	}
	return ctx.JSON(toWireCommandStats(stats))
}

func (c *DeviceOpenController) commandLookup(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "device_uuid", "deviceUuid", "imei", "device_id", "deviceId")
}

func clampLimit(value, def, max int) int {
	if value <= 0 {
		return def
	}
	if value > max {
		return max
	}
	return value
}

// GET /api/device-open/installations/:device_uuid
func (c *DeviceOpenController) GetInstallation(ctx *fiber.Ctx) error {
	deviceID := pathParamAlias(ctx, "device_uuid", "deviceUuid")
	if deviceID == "" {
		return ctx.Status(400).JSON(fiber.Map{"message": "deviceUuid is required"})
	}
	if c.repo == nil {
		return ctx.Status(500).JSON(fiber.Map{"message": "repo unavailable"})
	}
	inst, err := c.repo.GetInstallationByDevice(deviceID)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"message": err.Error()})
	}
	if inst == nil {
		return ctx.Status(404).JSON(fiber.Map{"message": "installation not found"})
	}
	beneficiaries := []map[string]interface{}{}
	if benID, ok := inst["beneficiary_id"].(string); ok && benID != "" {
		if ben, err := c.repo.GetBeneficiary(benID); err == nil && ben != nil {
			beneficiaries = append(beneficiaries, ben)
		}
	}
	return ctx.JSON(fiber.Map{
		"installation":  inst,
		"beneficiaries": beneficiaries,
	})
}

// POST /api/device-open/errors
// HTTP fallback when MQTT publishing fails.
// Accepts: { open_id?, timestamp, error_code, error_data?, severity?, message?, imei?/device_uuid? }
func (c *DeviceOpenController) PostErrors(ctx *fiber.Ctx) error {
	lookup := strings.TrimSpace(queryAlias(ctx, "device_uuid", "deviceUuid", "imei", "device_id", "deviceId"))
	if lookup == "" {
		var probe struct {
			DeviceID      string `json:"device_id"`
			DeviceIDCamel string `json:"deviceId"`
			DeviceUUID    string `json:"device_uuid"`
			DeviceUUID2   string `json:"deviceUuid"`
			IMEI          string `json:"imei"`
		}
		_ = ctx.BodyParser(&probe)
		lookup = firstNonEmpty(probe.DeviceUUID, probe.DeviceUUID2, probe.IMEI, probe.DeviceID, probe.DeviceIDCamel)
		lookup = strings.TrimSpace(lookup)
	}
	if lookup == "" {
		return ctx.Status(400).JSON(fiber.Map{"message": "imei or device_uuid is required"})
	}
	if c.repo == nil {
		return ctx.Status(500).JSON(fiber.Map{"message": "repo unavailable"})
	}

	var body struct {
		OpenID     string         `json:"open_id"`
		OpenID2    string         `json:"openId"`
		Timestamp  any            `json:"timestamp"`
		ErrorCode  string         `json:"error_code"`
		ErrorCode2 string         `json:"errorCode"`
		ErrorData  map[string]any `json:"error_data"`
		ErrorData2 map[string]any `json:"errorData"`
		Severity   string         `json:"severity"`
		Message    string         `json:"message"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).JSON(fiber.Map{"message": "invalid json payload"})
	}
	openID := strings.TrimSpace(firstNonEmpty(body.OpenID, body.OpenID2))
	errorCode := strings.TrimSpace(firstNonEmpty(body.ErrorCode, body.ErrorCode2))
	if errorCode == "" {
		return ctx.Status(400).JSON(fiber.Map{"message": "error_code is required"})
	}
	if body.Timestamp == nil {
		return ctx.Status(400).JSON(fiber.Map{"message": "timestamp is required"})
	}

	timestampRaw := ""
	occurredAt := time.Now().UTC()
	switch v := body.Timestamp.(type) {
	case float64:
		ms := int64(v)
		timestampRaw = fmt.Sprintf("%d", ms)
		if ms > 0 {
			occurredAt = time.UnixMilli(ms).UTC()
		}
	case int64:
		timestampRaw = fmt.Sprintf("%d", v)
		if v > 0 {
			occurredAt = time.UnixMilli(v).UTC()
		}
	case int:
		ms := int64(v)
		timestampRaw = fmt.Sprintf("%d", ms)
		if ms > 0 {
			occurredAt = time.UnixMilli(ms).UTC()
		}
	case string:
		timestampRaw = strings.TrimSpace(v)
		if timestampRaw == "" {
			return ctx.Status(400).JSON(fiber.Map{"message": "timestamp is required"})
		}
		// Accept RFC3339 or numeric-as-string.
		if t, err := time.Parse(time.RFC3339, timestampRaw); err == nil {
			occurredAt = t.UTC()
		} else if ms, err := parseInt64(timestampRaw); err == nil && ms > 0 {
			occurredAt = time.UnixMilli(ms).UTC()
		}
	default:
		b, _ := json.Marshal(v)
		timestampRaw = strings.Trim(string(b), "\"")
	}
	severity := strings.ToLower(strings.TrimSpace(body.Severity))
	if severity == "" {
		severity = "warning"
	}

	device, err := c.repo.GetDeviceByIDOrIMEI(lookup)
	if err != nil || device == nil {
		return ctx.Status(404).JSON(fiber.Map{"message": "device not found"})
	}
	deviceID, _ := device["id"].(string)
	projectID, _ := device["project_id"].(string)
	if strings.TrimSpace(deviceID) == "" || strings.TrimSpace(projectID) == "" {
		return ctx.Status(500).JSON(fiber.Map{"message": "device metadata incomplete"})
	}

	msg := strings.TrimSpace(body.Message)
	if msg == "" {
		msg = fmt.Sprintf("device_error: %s", errorCode)
	}

	errorData := body.ErrorData
	if errorData == nil {
		errorData = body.ErrorData2
	}

	// Keep the original payload too (for Bell) while also providing stable summary fields.
	var raw map[string]any
	_ = json.Unmarshal(ctx.Body(), &raw)

	data := map[string]any{
		"source":     "device_error_http",
		"open_id":    openID,
		"timestamp":  timestampRaw,
		"error_code": errorCode,
		"error_data": errorData,
		"device": map[string]any{
			"imei": device["imei"],
			"id":   deviceID,
		},
		"first_seen": occurredAt.Format(time.RFC3339),
		"last_seen":  occurredAt.Format(time.RFC3339),
		"count":      1,
		"payload":    raw,
	}

	if c.rules != nil {
		c.rules.EmitDeviceAlert(projectID, deviceID, msg, severity, data)
	} else {
		if err := c.repo.CreateAlertWithData(deviceID, projectID, msg, severity, data); err != nil {
			return ctx.Status(500).JSON(fiber.Map{"message": err.Error()})
		}
	}
	return ctx.Status(202).JSON(fiber.Map{"status": "accepted"})
}

func (c *DeviceOpenController) lookupDevice(identifier string) (map[string]interface{}, error) {
	if c.devices == nil {
		return nil, fmt.Errorf("device service unavailable")
	}
	return c.devices.GetDeviceByIDOrIMEI(identifier)
}

func (c *DeviceOpenController) mapDeviceSummary(device map[string]interface{}) fiber.Map {
	attrs, _ := device["attributes"].(map[string]interface{})
	protocolID := lookupAttrString(attrs, "protocol_version_id", "protocolVersionId", "protocol_id", "protocolId")
	vfdModelID := lookupAttrString(attrs, "vfd_model_id", "vfdModelId")
	return fiber.Map{
		"uuid":                device["id"],
		"imei":                device["imei"],
		"project_id":          device["project_id"],
		"status":              device["status"],
		"state_id":            lookupAttrString(attrs, "state_id", "stateId"),
		"state_authority_id":  lookupAttrString(attrs, "state_authority_id", "authority_id", "authorityId", "stateAuthorityId"),
		"server_vendor_id":    lookupAttrString(attrs, "server_vendor_id", "serverVendorId", "server_vendor_org_id"),
		"protocol_version_id": protocolID,
		"protocol_id":         protocolID,
		"vfd_drive_model_id":  vfdModelID,
		"vfd_model_id":        vfdModelID,
	}
}

func lookupAttrString(attrs map[string]interface{}, keys ...string) string {
	if attrs == nil {
		return ""
	}
	for _, key := range keys {
		if val, ok := attrs[key]; ok {
			switch v := val.(type) {
			case string:
				return v
			case *string:
				if v != nil {
					return *v
				}
			}
		}
	}
	return ""
}

func (c *DeviceOpenController) buildLocalCredential(device map[string]interface{}) (fiber.Map, error) {
	deviceID, _ := device["id"].(string)
	projectID, _ := device["project_id"].(string)
	imei, _ := device["imei"].(string)
	if deviceID == "" {
		return nil, fmt.Errorf("device not found")
	}
	latest, err := c.devices.GetLatestCredentialHistory(deviceID)
	if err != nil || latest == nil {
		return nil, fmt.Errorf("credential history not found")
	}
	bundle, _ := latest["bundle"].(map[string]interface{})
	username, _ := bundle["username"].(string)
	password, _ := bundle["password"].(string)
	clientID, _ := bundle["client_id"].(string)

	endpoints := []fiber.Map{}
	if rawURLs := strings.TrimSpace(os.Getenv("MQTT_PUBLIC_URLS")); rawURLs != "" {
		parts := strings.Split(rawURLs, ",")
		for _, part := range parts {
			s := strings.TrimSpace(part)
			if s == "" {
				continue
			}
			u, err := url.Parse(s)
			if err != nil || u.Scheme == "" {
				continue
			}
			h := u.Hostname()
			p := u.Port()
			if p == "" {
				switch strings.ToLower(u.Scheme) {
				case "mqtts", "ssl", "tls", "mqtt+ssl", "mqtt+tls":
					p = "8883"
				default:
					p = "1883"
				}
			}
			if h == "" {
				continue
			}
			endpoints = append(endpoints, fiber.Map{"protocol": u.Scheme, "host": h, "port": p, "url": fmt.Sprintf("%s://%s:%s", u.Scheme, h, p)})
		}
	}
	if len(endpoints) == 0 {
		proto := strings.TrimSpace(os.Getenv("MQTT_PUBLIC_PROTOCOL"))
		if proto == "" {
			proto = "mqtts"
		}
		host := strings.TrimSpace(os.Getenv("MQTT_PUBLIC_HOST"))
		if host == "" {
			host = strings.TrimSpace(os.Getenv("MQTT_HOST"))
		}
		if host == "" {
			host = "localhost"
		}
		port := strings.TrimSpace(os.Getenv("MQTT_PUBLIC_PORT"))
		if port == "" {
			port = "8883"
		}
		endpoints = append(endpoints, fiber.Map{"protocol": proto, "host": host, "port": port, "url": fmt.Sprintf("%s://%s:%s", proto, host, port)})
	}

	pubTopics, subTopics := c.devices.ResolveTopics(projectID, imei)

	issuedAt := time.Now().UTC().Format(time.RFC3339)
	if createdAt, ok := latest["created_at"].(time.Time); ok {
		issuedAt = createdAt.UTC().Format(time.RFC3339)
	}

	return fiber.Map{
		"client_id":           clientID,
		"username":            username,
		"password":            password,
		"endpoints":           endpoints,
		"publish_topics":      pubTopics,
		"subscribe_topics":    subTopics,
		"mqtt_access_applied": latest["applied"],
		"lifecycle":           latest["lifecycle"],
		"issued_at":           issuedAt,
		"valid_to":            nil,
	}, nil
}

func (c *DeviceOpenController) buildGovernmentCredential(ctx context.Context, device map[string]interface{}) (fiber.Map, error) {
	if c.govtCreds == nil {
		return nil, fmt.Errorf("govt credentials unavailable")
	}
	deviceID, _ := device["id"].(string)
	projectID, _ := device["project_id"].(string)
	imei, _ := device["imei"].(string)
	if deviceID == "" {
		return nil, fmt.Errorf("device not found")
	}
	list, err := c.govtCreds.ListByDevice(ctx, deviceID)
	if err != nil || len(list) == 0 {
		return nil, fmt.Errorf("government credential bundle not found")
	}
	selected := list[0]

	var protoProfile *models.ProtocolProfile
	if c.protocols != nil && selected.ProtocolID != "" {
		if p, err := c.protocols.GetByID(ctx, selected.ProtocolID); err == nil {
			protoProfile = p
		}
	}

	pubTopics := []string{}
	subTopics := []string{}
	endpoints := []fiber.Map{}
	if protoProfile != nil {
		pubTopics = expandTopics(protoProfile.PublishTopics, projectID, imei)
		subTopics = expandTopics(protoProfile.SubscribeTopics, projectID, imei)
		if protoProfile.Host != "" && protoProfile.Port != 0 {
			scheme := protoProfile.Protocol
			if scheme == "" {
				scheme = "mqtt"
			}
			url := fmt.Sprintf("%s://%s:%d", scheme, protoProfile.Host, protoProfile.Port)
			endpoints = []fiber.Map{{"protocol": scheme, "host": protoProfile.Host, "port": protoProfile.Port, "url": url}}
		}
	}

	return fiber.Map{
		"client_id":        selected.ClientID,
		"username":         selected.Username,
		"password":         selected.Password,
		"endpoints":        endpoints,
		"publish_topics":   pubTopics,
		"subscribe_topics": subTopics,
		"lifecycle":        "active",
		"issued_at":        selected.CreatedAt.UTC().Format(time.RFC3339),
		"valid_to":         nil,
		"protocol_id":      selected.ProtocolID,
	}, nil
}

func (c *DeviceOpenController) resolveVFDModels(ctx context.Context, device map[string]interface{}) ([]models.VFDModel, error) {
	if c.repo == nil || c.vfdRepo == nil {
		return []models.VFDModel{}, nil
	}
	deviceID, _ := device["id"].(string)
	projectID, _ := device["project_id"].(string)
	if deviceID == "" {
		return []models.VFDModel{}, nil
	}
	inst, err := c.repo.GetInstallationByDevice(deviceID)
	if err != nil || inst == nil {
		return []models.VFDModel{}, nil
	}
	if pid, ok := inst["project_id"].(string); ok && pid != "" {
		projectID = pid
	}
	var ids []string
	if vfdModelID, ok := inst["vfd_model_id"].(*string); ok && vfdModelID != nil && *vfdModelID != "" {
		ids = append(ids, *vfdModelID)
	}
	if protocolID, ok := inst["protocol_id"].(*string); ok && protocolID != nil && *protocolID != "" {
		assignments, err := c.vfdRepo.ListAssignments(ctx, projectID, *protocolID)
		if err != nil {
			return nil, err
		}
		for _, a := range assignments {
			if a.RevokedAt != nil {
				continue
			}
			ids = append(ids, a.VFDModelID)
		}
	}
	if len(ids) == 0 {
		return []models.VFDModel{}, nil
	}
	return c.vfdRepo.ListModelsByIDs(ctx, projectID, uniqueStrings(ids))
}

func expandTopics(topics []string, projectID, imei string) []string {
	if len(topics) == 0 {
		return topics
	}
	out := make([]string, 0, len(topics))
	for _, t := range topics {
		if strings.TrimSpace(t) == "" {
			continue
		}
		x := t
		x = strings.ReplaceAll(x, "{imei}", imei)
		x = strings.ReplaceAll(x, "{project_id}", projectID)
		x = strings.ReplaceAll(x, "{projectId}", projectID)
		out = append(out, x)
	}
	return out
}

func uniqueStrings(items []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, v := range items {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
