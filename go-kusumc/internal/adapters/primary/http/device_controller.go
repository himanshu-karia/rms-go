package http

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"ingestion-go/internal/adapters/secondary"
	"ingestion-go/internal/core/services"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type DeviceController struct {
	svc        *services.DeviceService
	repo       *secondary.PostgresRepo
	cmds       *services.CommandsService
	importJobs importJobsRepo
}

type importJobsRepo interface {
	ListImportJobs(jobType string) ([]map[string]any, error)
	GetImportJob(jobID string) (map[string]any, error)
	UpdateImportJobStatus(jobID, status string) error
}

type credentialToken struct {
	DeviceID  string
	ExpiresAt time.Time
}

var (
	credentialTokens   = map[string]credentialToken{}
	credentialTokensMu sync.Mutex
)

func NewDeviceController(svc *services.DeviceService, repo *secondary.PostgresRepo) *DeviceController {
	return &DeviceController{svc: svc, repo: repo, importJobs: repo}
}

func NewDeviceControllerWithCommands(svc *services.DeviceService, repo *secondary.PostgresRepo, cmds *services.CommandsService) *DeviceController {
	return &DeviceController{svc: svc, repo: repo, cmds: cmds, importJobs: repo}
}

func toFrontendDevice(d map[string]interface{}) fiber.Map {
	if d == nil {
		return fiber.Map{}
	}

	id, _ := d["id"].(string)
	imei, _ := d["imei"].(string)
	pid, _ := d["project_id"].(string)

	name, _ := d["name"].(string)
	status, _ := d["status"].(string)

	attrs, _ := d["attributes"].(map[string]interface{})
	modelID := ""
	if attrs != nil {
		if v, ok := attrs["model_id"].(string); ok {
			modelID = v
		}
	}

	formatTime := func(value interface{}) string {
		switch v := value.(type) {
		case time.Time:
			return v.UTC().Format(time.RFC3339)
		case *time.Time:
			if v != nil {
				return v.UTC().Format(time.RFC3339)
			}
		case string:
			return v
		}
		return ""
	}

	out := fiber.Map{
		"_id":        id,
		"id":         id,
		"uuid":       id,
		"imei":       imei,
		"project_id": pid,
		"name":       name,
		"status":     status,
		"model_id":   modelID,
		"metadata":   attrs,
	}
	if ts := formatTime(d["last_seen"]); ts != "" {
		out["last_seen"] = ts
	}
	if ts := formatTime(d["connectivity_updated_at"]); ts != "" {
		out["connectivity_updated_at"] = ts
	}
	if v, ok := d["connectivity_status"].(string); ok && v != "" {
		out["connectivity_status"] = v
	}
	if shadow, ok := d["shadow"].(map[string]interface{}); ok {
		out["shadow"] = shadow
	}
	return out
}

// GET /api/devices
func (c *DeviceController) ListDevices(ctx *fiber.Ctx) error {
	page, _ := strconv.Atoi(deviceListPageParam(ctx))
	limitParam := deviceListLimitParam(ctx)
	search := deviceListSearchQuery(ctx)
	projectId := deviceListProjectID(ctx)
	status := strings.TrimSpace(ctx.Query("status", ""))
	includeInactiveParam := deviceListIncludeInactiveParam(ctx)

	issues := []fiber.Map{}
	limit := 50
	if limitParam != "" {
		parsedLimit, err := strconv.Atoi(limitParam)
		if err != nil {
			issues = append(issues, fiber.Map{"field": "limit", "message": "limit must be an integer"})
		} else if parsedLimit < 1 || parsedLimit > 100 {
			issues = append(issues, fiber.Map{"field": "limit", "message": "limit must be between 1 and 100"})
		} else {
			limit = parsedLimit
		}
	}
	includeInactive := false
	if includeInactiveParam != "" {
		parsedInclude, err := strconv.ParseBool(includeInactiveParam)
		if err != nil {
			issues = append(issues, fiber.Map{"field": "include_inactive", "message": "include_inactive must be a boolean"})
		} else {
			includeInactive = parsedInclude
		}
	}
	if status != "" && status != "active" && status != "inactive" {
		issues = append(issues, fiber.Map{"field": "status", "message": "status must be active or inactive"})
	}
	if len(issues) > 0 {
		return ctx.Status(400).JSON(fiber.Map{
			"message": "Invalid device list filters provided",
			"issues":  issues,
		})
	}

	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	rows, total, err := c.svc.ListDevices(projectId, search, status, includeInactive, limit, offset)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	devices := make([]fiber.Map, 0, len(rows))
	for _, d := range rows {
		devices = append(devices, toFrontendDevice(d))
	}

	pages := 1
	if limit > 0 {
		pages = int(math.Ceil(float64(total) / float64(limit)))
		if pages == 0 {
			pages = 1
		}
	}

	return ctx.JSON(fiber.Map{
		"devices": devices,
		"total":   total,
		"page":    page,
		"pages":   pages,
		"pagination": fiber.Map{
			"total":            total,
			"limit":            limit,
			"include_inactive": includeInactive,
			"status":           status,
		},
	})
}

func deviceListProjectID(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "project_id", "projectId")
}

func deviceListIncludeInactiveParam(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "include_inactive", "includeInactive")
}

func deviceListPageParam(ctx *fiber.Ctx) string {
	return queryAliasDefault(ctx, "1", "page", "pageNumber")
}

func deviceListLimitParam(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "limit", "pageSize")
}

func deviceListSearchQuery(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "search", "q")
}

// GET /api/devices/:idOrUuid
func (c *DeviceController) GetDevice(ctx *fiber.Ctx) error {
	idOr := ctx.Params("idOrUuid")
	if idOr == "" {
		return ctx.Status(400).JSON(fiber.Map{"error": "id required"})
	}

	d, err := c.svc.GetDeviceByIDOrIMEI(idOr)
	if err != nil {
		return ctx.Status(404).JSON(fiber.Map{"error": "device not found"})
	}

	return ctx.JSON(toFrontendDevice(d))
}

// PUT /api/devices/:idOrUuid
func (c *DeviceController) UpdateDevice(ctx *fiber.Ctx) error {
	idOr := ctx.Params("idOrUuid")
	if idOr == "" {
		return ctx.Status(400).JSON(fiber.Map{"error": "id required"})
	}

	var body map[string]interface{}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).JSON(fiber.Map{"error": "Invalid Body"})
	}

	var namePtr *string
	if v, ok := body["name"].(string); ok {
		namePtr = &v
	}
	var statusPtr *string
	if v, ok := body["status"].(string); ok {
		statusPtr = &v
	}
	var pidPtr *string
	if v, ok := body["project_id"].(string); ok {
		pidPtr = &v
	} else if v, ok := body["projectId"].(string); ok {
		pidPtr = &v
	}

	attrsPatch := map[string]interface{}{}
	if meta, ok := body["metadata"].(map[string]interface{}); ok {
		for k, v := range meta {
			attrsPatch[k] = v
		}
	}
	if v, ok := body["model_id"].(string); ok && v != "" {
		attrsPatch["model_id"] = v
	}
	if len(attrsPatch) == 0 {
		attrsPatch = nil
	}

	updated, err := c.svc.UpdateDeviceByIDOrIMEI(idOr, namePtr, statusPtr, pidPtr, attrsPatch)
	if err != nil {
		return ctx.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.JSON(toFrontendDevice(updated))
}

// DELETE /api/devices/:idOrUuid
func (c *DeviceController) DeleteDevice(ctx *fiber.Ctx) error {
	idOr := ctx.Params("idOrUuid")
	if idOr == "" {
		return ctx.Status(400).JSON(fiber.Map{"message": "device id required"})
	}
	var body struct {
		Reason string `json:"reason"`
	}
	_ = ctx.BodyParser(&body)
	device, err := c.svc.GetDeviceByIDOrIMEI(idOr)
	if err != nil || device == nil {
		return ctx.Status(404).JSON(fiber.Map{"message": "Device not found for provided identifier"})
	}
	if err := c.svc.DeleteDevice(idOr); err != nil {
		return ctx.Status(500).JSON(fiber.Map{"message": err.Error()})
	}
	retiredAt := time.Now().UTC().Format(time.RFC3339)
	return ctx.JSON(fiber.Map{
		"device": fiber.Map{
			"device_id": device["id"],
			"id":        device["id"],
			"uuid":      device["id"],
			"imei":      device["imei"],
		},
		"retired": fiber.Map{
			"at":     retiredAt,
			"reason": strings.TrimSpace(body.Reason),
		},
		"revoked_credentials":            0,
		"revokedCredentials":             0,
		"mqtt_provisioning_jobs_deleted": 0,
		"mqttProvisioningJobsDeleted":    0,
		"government_assignment_count":    0,
		"governmentAssignmentCount":      0,
	})
}

// GET /api/devices/:idOrUuid/status
func (c *DeviceController) GetDeviceStatus(ctx *fiber.Ctx) error {
	deviceRef := ctx.Params("idOrUuid")
	if deviceRef == "" {
		return ctx.Status(400).JSON(fiber.Map{
			"message": "Invalid device identifier provided",
			"issues":  []fiber.Map{{"field": "device_uuid", "message": "device_uuid is required"}},
		})
	}
	device, err := c.svc.GetDeviceByIDOrIMEI(deviceRef)
	if err != nil || device == nil {
		return ctx.Status(404).JSON(fiber.Map{"message": "Device not found for provided identifier"})
	}
	formatTime := func(value interface{}) interface{} {
		switch v := value.(type) {
		case time.Time:
			return v.UTC().Format(time.RFC3339)
		case *time.Time:
			if v != nil {
				return v.UTC().Format(time.RFC3339)
			}
		case string:
			if v != "" {
				return v
			}
		}
		return nil
	}
	lastSeen := formatTime(device["last_seen"])
	connUpdatedAt := formatTime(device["connectivity_updated_at"])
	connStatus := device["connectivity_status"]
	if connStatus == nil {
		connStatus = "unknown"
	}
	devicePayload := fiber.Map{
		"uuid":                               device["id"],
		"imei":                               device["imei"],
		"status":                             device["status"],
		"configuration_status":               nil,
		"last_seen":                          lastSeen,
		"connectivity_status":                connStatus,
		"connectivity_updated_at":            connUpdatedAt,
		"offline_threshold_ms":               nil,
		"offline_notification_channel_count": nil,
		"protocol_version":                   nil,
	}
	return ctx.JSON(fiber.Map{
		"device_id":               device["id"],
		"imei":                    device["imei"],
		"status":                  device["status"],
		"last_seen":               lastSeen,
		"connectivity_status":     connStatus,
		"connectivity_updated_at": connUpdatedAt,
		"device":                  devicePayload,
		"telemetry":               []fiber.Map{},
		"recent_events":           []fiber.Map{},
		"recentEvents":            []fiber.Map{},
		"credentials_history":     []fiber.Map{},
		"credentialsHistory":      []fiber.Map{},
		"active_credentials": fiber.Map{
			"local":      nil,
			"government": nil,
		},
		"activeCredentials": fiber.Map{
			"local":      nil,
			"government": nil,
		},
		"mqtt_provisioning": nil,
		"mqttProvisioning":  nil,
		"thresholds": fiber.Map{
			"effective":    nil,
			"installation": nil,
			"override":     nil,
		},
	})
}

// POST /api/devices
func (c *DeviceController) CreateDevice(ctx *fiber.Ctx) error {
	var body struct {
		Name           string                 `json:"name"`
		IMEI           string                 `json:"imei"`
		ProjectID      string                 `json:"project_id"`
		ProjectIDCamel string                 `json:"projectId"`
		ProtocolID     string                 `json:"protocol_id"`
		ContractorID   string                 `json:"contractor_id"`
		SupplierID     string                 `json:"supplier_id"`
		ManufacturerID string                 `json:"manufacturer_id"`
		OrgID          string                 `json:"org_id"`
		Attributes     map[string]interface{} `json:"attributes"`
		Metadata       map[string]interface{} `json:"metadata"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString("Invalid Body")
	}

	if body.ProjectID == "" {
		body.ProjectID = body.ProjectIDCamel
	}
	if body.IMEI == "" || body.ProjectID == "" {
		return ctx.Status(400).SendString("imei and project_id are required")
	}

	if body.Attributes == nil {
		body.Attributes = make(map[string]interface{})
	}
	if body.Metadata != nil {
		for k, v := range body.Metadata {
			if _, exists := body.Attributes[k]; !exists {
				body.Attributes[k] = v
			}
		}
	}

	// Persist identity attributes for downstream bootstrap/envelope
	body.Attributes["protocol_id"] = body.ProtocolID
	body.Attributes["contractor_id"] = body.ContractorID
	body.Attributes["supplier_id"] = body.SupplierID
	body.Attributes["manufacturer_id"] = body.ManufacturerID
	body.Attributes["org_id"] = body.OrgID

	creds, err := c.svc.CreateDevice(body.ProjectID, body.Name, body.IMEI, body.Attributes)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}

	endpoints := []interface{}{}
	if ep, ok := creds["endpoint"]; ok {
		endpoints = append(endpoints, ep)
	}

	response := fiber.Map{
		"device_id":             creds["id"],
		"imei":                  creds["imei"],
		"mqtt_user":             creds["mqtt_user"],
		"mqtt_pass":             creds["mqtt_pass"],
		"client_id":             creds["client_id"],
		"endpoint":              creds["endpoint"],
		"endpoints":             endpoints,
		"publish_topics":        creds["publish_topics"],
		"subscribe_topics":      creds["subscribe_topics"],
		"credential_history_id": creds["credential_history_id"],
		"provisioning_status":   creds["provisioning_status"],
		"project_id":            body.ProjectID,
		"protocol_id":           body.ProtocolID,
		"contractor_id":         body.ContractorID,
		"supplier_id":           body.SupplierID,
		"manufacturer_id":       body.ManufacturerID,
		"org_id":                body.OrgID,
		"topics": fiber.Map{
			"publish":   fmt.Sprintf("%s/heartbeat", body.IMEI),
			"subscribe": fmt.Sprintf("%s/ondemand", body.IMEI),
		},
		"envelope_keys": []string{"packet_type", "project_id", "protocol_id", "contractor_id", "supplier_id", "manufacturer_id", "device_id", "imei", "ts", "msg_id"},
	}

	// Echo attributes for clients (including vendor/org IDs)
	response["attributes"] = body.Attributes

	return ctx.JSON(response)
}

// POST /api/devices/:id/rotate-creds
func (c *DeviceController) RotateCredentials(ctx *fiber.Ctx) error {
	deviceID := ctx.Params("id")
	if deviceID == "" {
		return ctx.Status(400).SendString("device id required")
	}

	creds, err := c.svc.RotateCredentials(deviceID)
	if err != nil {
		return ctx.Status(400).SendString(err.Error())
	}

	return ctx.JSON(creds)
}

// POST /api/devices/:id/credentials/revoke
func (c *DeviceController) RevokeCredentials(ctx *fiber.Ctx) error {
	if c.repo == nil {
		return ctx.Status(500).SendString("repo unavailable")
	}
	deviceRef := strings.TrimSpace(ctx.Params("id"))
	if deviceRef == "" {
		return ctx.Status(400).JSON(fiber.Map{"message": "Invalid credential revocation payload"})
	}
	var body struct {
		Type          string `json:"type"`
		Reason        string `json:"reason"`
		IssuedBy      string `json:"issued_by"`
		IssuedByCamel string `json:"issuedBy"`
	}
	_ = ctx.BodyParser(&body)
	if body.IssuedBy == "" {
		body.IssuedBy = body.IssuedByCamel
	}
	device, err := c.svc.GetDeviceByIDOrIMEI(deviceRef)
	if err != nil || device == nil {
		return ctx.Status(404).JSON(fiber.Map{"message": "Device not found for provided identifier"})
	}
	deviceID, _ := device["id"].(string)
	if deviceID == "" {
		return ctx.Status(404).JSON(fiber.Map{"message": "Device not found for provided identifier"})
	}
	revokedCount, err := c.repo.RevokeCredentialHistoryByDevice(deviceID, body.Reason)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"message": err.Error()})
	}
	return ctx.JSON(fiber.Map{
		"device": fiber.Map{
			"device_id": deviceID,
			"id":        deviceID,
			"uuid":      deviceID,
			"imei":      device["imei"],
		},
		"revoked_count":         revokedCount,
		"revokedCount":          revokedCount,
		"lifecycle_transitions": []fiber.Map{},
		"lifecycleTransitions":  []fiber.Map{},
	})
}

// GET /api/devices/:id/credentials/history
func (c *DeviceController) GetCredentialHistory(ctx *fiber.Ctx) error {
	deviceID := ctx.Params("id")
	if deviceID == "" {
		return ctx.Status(400).SendString("device id required")
	}
	items, err := c.svc.ListCredentialHistory(deviceID)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.JSON(fiber.Map{"items": items})
}

// POST /api/devices/:id/mqtt-provisioning/retry
func (c *DeviceController) RetryProvisioning(ctx *fiber.Ctx) error {
	deviceID := ctx.Params("id")
	if deviceID == "" {
		return ctx.Status(400).SendString("device id required")
	}
	var body struct {
		CredentialHistoryID *string `json:"credential_history_id"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}
	if err := c.svc.RetryProvisioning(deviceID, body.CredentialHistoryID); err != nil {
		return ctx.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.JSON(fiber.Map{"status": "queued"})
}

// POST /api/devices/:idOrUuid/configuration
func (c *DeviceController) QueueDeviceConfiguration(ctx *fiber.Ctx) error {
	if c.repo == nil {
		return ctx.Status(500).SendString("config repo unavailable")
	}
	if c.cmds == nil {
		return ctx.Status(500).JSON(fiber.Map{"error": "commands service unavailable"})
	}
	deviceRef := ctx.Params("idOrUuid")
	if deviceRef == "" {
		return ctx.Status(400).SendString("device id required")
	}
	device, err := c.svc.GetDeviceByIDOrIMEI(deviceRef)
	if err != nil || device == nil {
		return ctx.Status(404).JSON(fiber.Map{"error": "device not found"})
	}
	deviceID, _ := device["id"].(string)
	projectID, _ := device["project_id"].(string)
	if deviceID == "" {
		return ctx.Status(404).JSON(fiber.Map{"error": "device not found"})
	}
	var body map[string]any
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).JSON(fiber.Map{"error": "invalid body"})
	}
	item, err := c.repo.CreateDeviceConfiguration(deviceID, body)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	configID, _ := item["id"].(string)
	if strings.TrimSpace(configID) == "" {
		return ctx.Status(500).JSON(fiber.Map{"error": "configuration id unavailable"})
	}
	cmdID, err := c.repo.GetCoreCommandIDByName("apply_device_configuration")
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	if strings.TrimSpace(cmdID) == "" {
		return ctx.Status(500).JSON(fiber.Map{"error": "apply_device_configuration command missing"})
	}

	configPayload, _ := item["config"].(map[string]any)
	commandPayload := map[string]any{
		"config_id": configID,
		"config":    configPayload,
	}

	if _, err := c.cmds.SendCommandWithCorrelation(deviceID, cmdID, projectID, configID, commandPayload); err != nil {
		_ = c.repo.FinalizeDeviceConfiguration(configID, "failed", map[string]any{
			"message": err.Error(),
			"stage":   "publish",
		})
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(201).JSON(deviceConfigurationToWireRecord(item))
}

// GET /api/devices/:idOrUuid/configuration/pending
func (c *DeviceController) GetPendingConfiguration(ctx *fiber.Ctx) error {
	if c.repo == nil {
		return ctx.Status(500).SendString("config repo unavailable")
	}
	deviceRef := ctx.Params("idOrUuid")
	if deviceRef == "" {
		return ctx.Status(400).SendString("device id required")
	}
	device, err := c.svc.GetDeviceByIDOrIMEI(deviceRef)
	if err != nil || device == nil {
		return ctx.Status(404).JSON(fiber.Map{"error": "device not found"})
	}
	deviceID, _ := device["id"].(string)
	if deviceID == "" {
		return ctx.Status(404).JSON(fiber.Map{"error": "device not found"})
	}
	item, err := c.repo.GetPendingDeviceConfiguration(deviceID)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	if item == nil {
		return ctx.SendStatus(204)
	}
	return ctx.JSON(deviceConfigurationToWireRecord(item))
}

// POST /api/devices/:idOrUuid/configuration/ack
func (c *DeviceController) AcknowledgeConfiguration(ctx *fiber.Ctx) error {
	if c.repo == nil {
		return ctx.Status(500).SendString("config repo unavailable")
	}
	deviceRef := ctx.Params("idOrUuid")
	if deviceRef == "" {
		return ctx.Status(400).SendString("device id required")
	}
	device, err := c.svc.GetDeviceByIDOrIMEI(deviceRef)
	if err != nil || device == nil {
		return ctx.Status(404).JSON(fiber.Map{"error": "device not found"})
	}
	deviceID, _ := device["id"].(string)
	if deviceID == "" {
		return ctx.Status(404).JSON(fiber.Map{"error": "device not found"})
	}
	var body struct {
		// Legacy/manual payload form
		ConfigID      string                 `json:"config_id"`
		ConfigIDCamel string                 `json:"configId"`
		Ack           map[string]interface{} `json:"ack"`

		// Old UI payload form
		Status     string                 `json:"status"`
		MsgID      string                 `json:"msgid"`
		ReceivedAt string                 `json:"received_at"`
		Payload    map[string]interface{} `json:"payload"`
	}
	_ = ctx.BodyParser(&body)

	configID := strings.TrimSpace(firstNonEmpty(body.MsgID, body.ConfigID, body.ConfigIDCamel))
	if configID == "" {
		pending, err := c.repo.GetPendingDeviceConfiguration(deviceID)
		if err != nil {
			return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		if pending == nil {
			return ctx.SendStatus(204)
		}
		if v, ok := pending["id"].(string); ok {
			configID = v
		}
	}
	if configID == "" {
		return ctx.Status(404).JSON(fiber.Map{"error": "configuration not found"})
	}

	ackPayload := map[string]any{}
	if body.Payload != nil {
		ackPayload = body.Payload
	} else if body.Ack != nil {
		ackPayload = body.Ack
	}
	if strings.TrimSpace(body.ReceivedAt) != "" {
		ackPayload["received_at"] = strings.TrimSpace(body.ReceivedAt)
	}
	if strings.TrimSpace(body.Status) != "" {
		ackPayload["status"] = strings.TrimSpace(body.Status)
	}

	finalStatus := strings.ToLower(strings.TrimSpace(body.Status))
	if finalStatus == "" {
		finalStatus = "acknowledged"
	}
	if finalStatus != "acknowledged" && finalStatus != "failed" {
		finalStatus = "acknowledged"
	}

	if err := c.repo.FinalizeDeviceConfiguration(configID, finalStatus, ackPayload); err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	item, err := c.repo.GetDeviceConfigurationByID(configID)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	if item == nil {
		return ctx.Status(404).JSON(fiber.Map{"error": "configuration not found"})
	}
	return ctx.JSON(deviceConfigurationToWireRecord(item))
}

func deviceConfigurationToWireRecord(item map[string]any) fiber.Map {
	formatTime := func(value any) string {
		switch v := value.(type) {
		case time.Time:
			return v.UTC().Format(time.RFC3339)
		case *time.Time:
			if v != nil {
				return v.UTC().Format(time.RFC3339)
			}
		case string:
			return v
		}
		return ""
	}

	id, _ := item["id"].(string)
	status, _ := item["status"].(string)
	cfg, _ := item["config"].(map[string]any)
	ack, _ := item["ack_payload"].(map[string]any)

	ackAt := formatTime(item["acknowledged_at"])
	if ackAt == "" {
		ackAt = ""
	}

	out := fiber.Map{
		"id":                      id,
		"status":                  strings.ToLower(strings.TrimSpace(status)),
		"transport":               "mqtt",
		"msgid":                   id,
		"requested_at":            formatTime(item["created_at"]),
		"acknowledged_at":         ackAt,
		"configuration":           cfg,
		"acknowledgement_payload": ack,
	}
	if ack == nil {
		out["acknowledgement_payload"] = nil
	}
	if ackAt == "" {
		out["acknowledged_at"] = nil
	}
	return out
}

// POST /api/devices/configuration/import
func (c *DeviceController) ImportDeviceConfigurations(ctx *fiber.Ctx) error {
	if c.repo == nil {
		return ctx.Status(500).SendString("config repo unavailable")
	}
	projectID := deviceConfigImportProjectQuery(ctx)
	reader := csv.NewReader(strings.NewReader(string(extractCSVImportBody(ctx.Body()))))
	head, err := reader.Read()
	if err != nil {
		return ctx.Status(400).JSON(fiber.Map{"error": "invalid csv"})
	}
	colIdx := map[string]int{}
	for i, h := range head {
		colIdx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	get := func(row []string, key string) string {
		if idx, ok := colIdx[key]; ok && idx < len(row) {
			return strings.TrimSpace(row[idx])
		}
		return ""
	}
	rowNum := 1
	success := 0
	errors := []map[string]any{}
	for {
		row, err := reader.Read()
		if err != nil {
			break
		}
		rowNum++
		deviceRef := get(row, "device_id")
		if deviceRef == "" {
			deviceRef = get(row, "device_uuid")
		}
		if deviceRef == "" {
			deviceRef = get(row, "imei")
		}
		if deviceRef == "" {
			errors = append(errors, map[string]any{"row": rowNum, "error": "device_id or imei required"})
			continue
		}
		device, err := c.svc.GetDeviceByIDOrIMEI(deviceRef)
		if err != nil || device == nil {
			errors = append(errors, map[string]any{"row": rowNum, "error": "device not found"})
			continue
		}
		deviceID, _ := device["id"].(string)
		if deviceID == "" {
			errors = append(errors, map[string]any{"row": rowNum, "error": "device not found"})
			continue
		}
		config := map[string]any{}
		for key, idx := range colIdx {
			if idx >= len(row) {
				continue
			}
			if key == "device_id" || key == "device_uuid" || key == "imei" {
				continue
			}
			value := strings.TrimSpace(row[idx])
			if value == "" {
				continue
			}
			parsed, ok := parseConfigValue(value)
			if ok {
				config[key] = parsed
			} else {
				config[key] = value
			}
		}
		if len(config) == 0 {
			errors = append(errors, map[string]any{"row": rowNum, "error": "no config values provided"})
			continue
		}
		if _, err := c.repo.CreateDeviceConfiguration(deviceID, config); err != nil {
			errors = append(errors, map[string]any{"row": rowNum, "error": err.Error()})
			continue
		}
		success++
	}
	job, _ := c.repo.CreateImportJob("device_configuration_import", projectID, success+len(errors), success, len(errors), errors)
	resp := fiber.Map{
		"success_count": success,
		"error_count":   len(errors),
		"errors":        errors,
	}
	if job != nil {
		if id, ok := job["id"]; ok {
			resp["job_id"] = id
		}
	}
	return ctx.JSON(resp)
}

func (c *DeviceController) ListImportJobs(ctx *fiber.Ctx) error {
	if c.importJobs == nil {
		return ctx.Status(500).SendString("import repo unavailable")
	}
	jobType := importJobTypeFromQuery(ctx)
	items, err := c.importJobs.ListImportJobs(jobType)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(fiber.Map{"jobs": items})
}

func (c *DeviceController) ListGovtImportJobs(ctx *fiber.Ctx) error {
	if c.importJobs == nil {
		return ctx.Status(500).SendString("import repo unavailable")
	}
	jobType := importJobTypeFromQuery(ctx)
	if jobType == "" {
		jobType = "government_credentials_import"
	}
	items, err := c.importJobs.ListImportJobs(jobType)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(fiber.Map{"jobs": items})
}

func (c *DeviceController) GetImportJob(ctx *fiber.Ctx) error {
	if c.importJobs == nil {
		return ctx.Status(500).SendString("import repo unavailable")
	}
	jobID := strings.TrimSpace(ctx.Params("jobId"))
	if jobID == "" {
		return ctx.Status(400).SendString("job_id required")
	}
	job, err := c.importJobs.GetImportJob(jobID)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	if job == nil {
		return ctx.SendStatus(404)
	}
	return ctx.JSON(job)
}

func (c *DeviceController) GetImportJobErrorsCSV(ctx *fiber.Ctx) error {
	if c.importJobs == nil {
		return ctx.Status(500).SendString("import repo unavailable")
	}
	jobID := strings.TrimSpace(ctx.Params("jobId"))
	if jobID == "" {
		return ctx.Status(400).SendString("job_id required")
	}
	job, err := c.importJobs.GetImportJob(jobID)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	if job == nil {
		return ctx.SendStatus(404)
	}
	errors, _ := job["errors"].([]map[string]any)
	if errors == nil {
		errors = []map[string]any{}
	}
	var sb strings.Builder
	w := csv.NewWriter(&sb)
	_ = w.Write([]string{"row", "error"})
	for _, e := range errors {
		row := ""
		if v, ok := e["row"]; ok {
			row = fmt.Sprintf("%v", v)
		}
		errMsg := ""
		if v, ok := e["error"]; ok {
			errMsg = fmt.Sprintf("%v", v)
		}
		_ = w.Write([]string{row, errMsg})
	}
	w.Flush()
	ctx.Set("Content-Type", "text/csv")
	return ctx.SendString(sb.String())
}

func (c *DeviceController) RetryImportJob(ctx *fiber.Ctx) error {
	if c.importJobs == nil {
		return ctx.Status(500).SendString("import repo unavailable")
	}
	jobID := strings.TrimSpace(ctx.Params("jobId"))
	if jobID == "" {
		return ctx.Status(400).SendString("job_id required")
	}
	if err := c.importJobs.UpdateImportJobStatus(jobID, "retry_requested"); err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(fiber.Map{"status": "retry_requested"})
}

func importJobTypeFromQuery(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "type", "jobType", "job_type", "importType", "import_type")
}

func deviceConfigImportProjectQuery(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "project_id", "projectId")
}

func parseConfigValue(value string) (any, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, false
	}
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		var out any
		if err := json.Unmarshal([]byte(trimmed), &out); err == nil {
			return out, true
		}
	}
	return trimmed, false
}

// POST /api/devices/:id/credentials/download-token
func (c *DeviceController) IssueCredentialDownloadToken(ctx *fiber.Ctx) error {
	deviceID := ctx.Params("id")
	if deviceID == "" {
		return ctx.Status(400).SendString("device id required")
	}
	var body struct {
		ExpiresInSeconds      int `json:"expires_in_seconds"`
		ExpiresInSecondsCamel int `json:"expiresInSeconds"`
	}
	_ = ctx.BodyParser(&body)

	expiresIn := 10 * time.Minute
	expiresInSeconds := body.ExpiresInSeconds
	if expiresInSeconds <= 0 {
		expiresInSeconds = body.ExpiresInSecondsCamel
	}
	if expiresInSeconds > 0 {
		expiresIn = time.Duration(expiresInSeconds) * time.Second
	}

	token := uuid.NewString()
	expiresAt := time.Now().Add(expiresIn).UTC()
	credentialTokensMu.Lock()
	credentialTokens[token] = credentialToken{DeviceID: deviceID, ExpiresAt: expiresAt}
	credentialTokensMu.Unlock()

	return ctx.JSON(fiber.Map{
		"token":      token,
		"expires_at": expiresAt.Format(time.RFC3339),
		"expiresAt":  expiresAt.Format(time.RFC3339),
	})
}

// POST /api/devices/credentials/claim
func (c *DeviceController) ClaimCredentials(ctx *fiber.Ctx) error {
	var body struct {
		DeviceID      string `json:"device_id"`
		DeviceIDCamel string `json:"deviceId"`
		Device        string `json:"device"`
		IMEI          string `json:"imei"`
		Secret        string `json:"secret"`
		Audience      string `json:"audience"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).JSON(fiber.Map{
			"message": "Invalid secure credential claim request",
			"issues":  []fiber.Map{{"field": "body", "message": "invalid json payload"}},
		})
	}
	lookup := strings.TrimSpace(body.DeviceID)
	if lookup == "" {
		lookup = strings.TrimSpace(body.DeviceIDCamel)
	}
	if lookup == "" {
		lookup = strings.TrimSpace(body.Device)
	}
	if lookup == "" {
		lookup = strings.TrimSpace(body.IMEI)
	}
	issues := []fiber.Map{}
	if lookup == "" || len(lookup) < 8 {
		issues = append(issues, fiber.Map{"field": "imei", "message": "IMEI must be at least 8 characters long"})
	}
	if strings.TrimSpace(body.Secret) == "" {
		issues = append(issues, fiber.Map{"field": "secret", "message": "Bootstrap secret is required"})
	}
	if len(issues) > 0 {
		return ctx.Status(400).JSON(fiber.Map{
			"message": "Invalid secure credential claim request",
			"issues":  issues,
		})
	}

	resp, err := c.buildCredentialResponse(lookup)
	if err != nil {
		return ctx.Status(404).JSON(fiber.Map{"message": err.Error()})
	}
	device, _ := c.svc.GetDeviceByIDOrIMEI(lookup)
	devicePayload := fiber.Map{"device_id": lookup, "id": lookup, "uuid": lookup, "imei": lookup}
	if device != nil {
		devicePayload["device_id"] = device["id"]
		devicePayload["id"] = device["id"]
		devicePayload["uuid"] = device["id"]
		devicePayload["imei"] = device["imei"]
	}
	credentialPayload := fiber.Map{
		"client_id":           resp["client_id"],
		"clientId":            resp["clientId"],
		"username":            resp["username"],
		"password":            resp["password"],
		"endpoints":           resp["endpoints"],
		"publish_topics":      resp["publish_topics"],
		"publishTopics":       resp["publishTopics"],
		"subscribe_topics":    resp["subscribe_topics"],
		"subscribeTopics":     resp["subscribeTopics"],
		"mqtt_access_applied": resp["mqtt_access_applied"],
		"mqttAccessApplied":   resp["mqttAccessApplied"],
		"protocol_selector":   nil,
		"protocolSelector":    nil,
		"lifecycle":           resp["lifecycle"],
		"issued_at":           resp["issued_at"],
		"issuedAt":            resp["issuedAt"],
		"valid_to":            resp["valid_to"],
		"validTo":             resp["validTo"],
	}
	issuedAt := time.Now().UTC().Format(time.RFC3339)
	tokenExpiry := time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339)
	ctx.Set("Cache-Control", "no-store")
	return ctx.JSON(fiber.Map{
		"device":     devicePayload,
		"credential": credentialPayload,
		"claim": fiber.Map{
			"type":                "secure",
			"claimed_at":          issuedAt,
			"claimedAt":           issuedAt,
			"previous_claim_at":   nil,
			"previousClaimAt":     nil,
			"total_secure_claims": 1,
			"totalSecureClaims":   1,
		},
		"token": fiber.Map{
			"value":      uuid.NewString(),
			"expires_at": tokenExpiry,
			"expiresAt":  tokenExpiry,
		},
	})
}

// POST /api/devices/credentials/plain-claim
func (c *DeviceController) PlainClaimCredentials(ctx *fiber.Ctx) error {
	var body struct {
		IMEI string `json:"imei"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).JSON(fiber.Map{
			"message": "Invalid plain credential claim request",
			"issues":  []fiber.Map{{"field": "body", "message": "invalid json payload"}},
		})
	}
	lookup := strings.TrimSpace(body.IMEI)
	if lookup == "" || len(lookup) < 8 {
		return ctx.Status(400).JSON(fiber.Map{
			"message": "Invalid plain credential claim request",
			"issues":  []fiber.Map{{"field": "imei", "message": "IMEI must be at least 8 characters long"}},
		})
	}
	resp, err := c.buildCredentialResponse(lookup)
	if err != nil {
		return ctx.Status(404).JSON(fiber.Map{"message": err.Error()})
	}
	devicePayload := fiber.Map{"device_id": lookup, "id": lookup, "uuid": lookup, "imei": lookup}
	if device, _ := c.svc.GetDeviceByIDOrIMEI(lookup); device != nil {
		devicePayload["device_id"] = device["id"]
		devicePayload["id"] = device["id"]
		devicePayload["uuid"] = device["id"]
		devicePayload["imei"] = device["imei"]
	}
	credentialPayload := fiber.Map{
		"client_id":           resp["client_id"],
		"clientId":            resp["clientId"],
		"username":            resp["username"],
		"password":            resp["password"],
		"endpoints":           resp["endpoints"],
		"publish_topics":      resp["publish_topics"],
		"publishTopics":       resp["publishTopics"],
		"subscribe_topics":    resp["subscribe_topics"],
		"subscribeTopics":     resp["subscribeTopics"],
		"mqtt_access_applied": resp["mqtt_access_applied"],
		"mqttAccessApplied":   resp["mqttAccessApplied"],
		"protocol_selector":   nil,
		"protocolSelector":    nil,
		"lifecycle":           resp["lifecycle"],
		"issued_at":           resp["issued_at"],
		"issuedAt":            resp["issuedAt"],
		"valid_to":            resp["valid_to"],
		"validTo":             resp["validTo"],
	}
	ctx.Set("Cache-Control", "no-store")
	return ctx.JSON(fiber.Map{
		"device":     devicePayload,
		"credential": credentialPayload,
		"claim": fiber.Map{
			"type":               "plain",
			"claimed_at":         time.Now().UTC().Format(time.RFC3339),
			"claimedAt":          time.Now().UTC().Format(time.RFC3339),
			"previous_claim_at":  nil,
			"previousClaimAt":    nil,
			"total_plain_claims": 1,
			"totalPlainClaims":   1,
		},
	})
}

// GET /api/devices/credentials/download?token=...
func (c *DeviceController) DownloadCredentials(ctx *fiber.Ctx) error {
	token := strings.TrimSpace(ctx.Query("token"))
	if token == "" {
		return ctx.Status(400).JSON(fiber.Map{
			"message": "Invalid credential download request",
			"issues":  []fiber.Map{{"field": "token", "message": "Credential download token is required"}},
		})
	}
	credentialTokensMu.Lock()
	entry, ok := credentialTokens[token]
	if ok {
		delete(credentialTokens, token)
	}
	credentialTokensMu.Unlock()
	if !ok {
		return ctx.Status(404).JSON(fiber.Map{"message": "Token expired"})
	}
	if time.Now().After(entry.ExpiresAt) {
		return ctx.Status(404).JSON(fiber.Map{"message": "Token expired"})
	}

	resp, err := c.buildCredentialResponse(entry.DeviceID)
	if err != nil {
		return ctx.Status(404).JSON(fiber.Map{"error": err.Error()})
	}
	credentialPayload := fiber.Map{
		"client_id":         resp["client_id"],
		"clientId":          resp["clientId"],
		"username":          resp["username"],
		"password":          resp["password"],
		"endpoints":         resp["endpoints"],
		"topics":            fiber.Map{"publish": resp["publish_topics"], "subscribe": resp["subscribe_topics"]},
		"issued_at":         resp["issued_at"],
		"issuedAt":          resp["issuedAt"],
		"valid_to":          resp["valid_to"],
		"validTo":           resp["validTo"],
		"protocol_selector": nil,
		"protocolSelector":  nil,
		"lifecycle":         resp["lifecycle"],
	}
	resp["device_uuid"] = resp["device_id"]
	resp["deviceUuid"] = resp["deviceId"]
	resp["type"] = "local"
	resp["credential"] = credentialPayload
	ctx.Set("Cache-Control", "no-store")
	return ctx.JSON(resp)
}

// GET /api/devices/lookup?imei=...&device_uuid=...
func (c *DeviceController) LookupDevice(ctx *fiber.Ctx) error {
	imei := strings.TrimSpace(ctx.Query("imei"))
	deviceUUID := strings.TrimSpace(ctx.Query("device_uuid"))
	if deviceUUID == "" {
		deviceUUID = strings.TrimSpace(ctx.Query("deviceUuid"))
	}
	lookup := deviceUUID
	if lookup == "" {
		lookup = imei
	}
	issues := []fiber.Map{}
	if deviceUUID == "" && imei == "" {
		issues = append(issues, fiber.Map{"field": "device_uuid", "message": "Provide device_uuid or imei when looking up a device"})
	}
	if deviceUUID != "" {
		if _, err := uuid.Parse(deviceUUID); err != nil {
			issues = append(issues, fiber.Map{"field": "device_uuid", "message": "device_uuid must be a valid UUID"})
		}
	}
	if imei != "" && len(imei) < 8 {
		issues = append(issues, fiber.Map{"field": "imei", "message": "imei must be at least 8 characters"})
	}
	if len(issues) > 0 {
		return ctx.Status(400).JSON(fiber.Map{
			"message": "Invalid device lookup parameters",
			"issues":  issues,
		})
	}
	device, err := c.svc.GetDeviceByIDOrIMEI(lookup)
	if err != nil || device == nil {
		return ctx.Status(404).JSON(fiber.Map{"message": "Device not found"})
	}
	formatTime := func(value interface{}) interface{} {
		switch v := value.(type) {
		case time.Time:
			return v.UTC().Format(time.RFC3339)
		case *time.Time:
			if v != nil {
				return v.UTC().Format(time.RFC3339)
			}
		case string:
			if v != "" {
				return v
			}
		}
		return nil
	}
	lastSeen := formatTime(device["last_seen"])
	connUpdatedAt := formatTime(device["connectivity_updated_at"])
	resp := fiber.Map{
		"uuid":                               device["id"],
		"imei":                               device["imei"],
		"status":                             device["status"],
		"configuration_status":               nil,
		"connectivity_status":                device["connectivity_status"],
		"connectivity_updated_at":            connUpdatedAt,
		"last_seen":                          lastSeen,
		"protocol_version":                   nil,
		"offline_threshold_ms":               nil,
		"offline_notification_channel_count": nil,
	}
	return ctx.JSON(fiber.Map{"device": resp})
}

// GET /api/devices/:id/beneficiary
func (c *DeviceController) GetDeviceBeneficiary(ctx *fiber.Ctx) error {
	deviceID := strings.TrimSpace(ctx.Params("idOrUuid"))
	if deviceID == "" {
		deviceID = strings.TrimSpace(ctx.Params("id"))
	}
	if deviceID == "" {
		return ctx.Status(400).JSON(fiber.Map{"message": "device id required"})
	}
	device, err := c.svc.GetDeviceByIDOrIMEI(deviceID)
	if err != nil || device == nil {
		return ctx.Status(404).JSON(fiber.Map{"message": "Device not found"})
	}
	id, _ := device["id"].(string)
	if id == "" {
		return ctx.Status(404).JSON(fiber.Map{"message": "Device not found"})
	}
	inst, err := c.repo.GetInstallationByDevice(id)
	if err != nil || inst == nil {
		return ctx.JSON(fiber.Map{"beneficiary": nil})
	}
	benID, _ := inst["beneficiary_id"].(string)
	if benID == "" {
		return ctx.JSON(fiber.Map{"beneficiary": nil})
	}
	ben, err := c.repo.GetBeneficiary(benID)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"message": err.Error()})
	}
	return ctx.JSON(fiber.Map{"beneficiary": ben})
}

func (c *DeviceController) buildCredentialResponse(deviceRef string) (fiber.Map, error) {
	device, err := c.svc.GetDeviceByIDOrIMEI(deviceRef)
	if err != nil || device == nil {
		return nil, fmt.Errorf("device not found")
	}
	deviceID, _ := device["id"].(string)
	projectID, _ := device["project_id"].(string)
	imei, _ := device["imei"].(string)

	latest, err := c.svc.GetLatestCredentialHistory(deviceID)
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

	pubTopics, subTopics := c.svc.ResolveTopics(projectID, imei)

	issuedAt := time.Now().UTC().Format(time.RFC3339)
	if createdAt, ok := latest["created_at"].(time.Time); ok {
		issuedAt = createdAt.UTC().Format(time.RFC3339)
	}

	return fiber.Map{
		"device_id":           deviceID,
		"deviceId":            deviceID,
		"imei":                imei,
		"client_id":           clientID,
		"clientId":            clientID,
		"username":            username,
		"password":            password,
		"endpoints":           endpoints,
		"publish_topics":      pubTopics,
		"publishTopics":       pubTopics,
		"subscribe_topics":    subTopics,
		"subscribeTopics":     subTopics,
		"mqtt_access_applied": latest["applied"],
		"mqttAccessApplied":   latest["applied"],
		"lifecycle":           latest["lifecycle"],
		"issued_at":           issuedAt,
		"issuedAt":            issuedAt,
		"valid_to":            nil,
		"validTo":             nil,
	}, nil
}
