package http

import (
	"strings"
	"time"

	"ingestion-go/internal/adapters/secondary"
	"ingestion-go/internal/core/services"
	"ingestion-go/internal/models"

	"github.com/gofiber/fiber/v2"
)

type TelemetryThresholdsController struct {
	devices *services.DeviceService
	dna     *services.DnaSpecService
	repo    *secondary.PostgresRepo
}

type telemetryThresholdEntry struct {
	Parameter     string   `json:"parameter"`
	Min           *float64 `json:"min,omitempty"`
	Max           *float64 `json:"max,omitempty"`
	WarnLow       *float64 `json:"warn_low,omitempty"`
	WarnHigh      *float64 `json:"warn_high,omitempty"`
	AlertLow      *float64 `json:"alert_low,omitempty"`
	AlertHigh     *float64 `json:"alert_high,omitempty"`
	Target        *float64 `json:"target,omitempty"`
	Unit          *string  `json:"unit,omitempty"`
	DecimalPlaces *int     `json:"decimal_places,omitempty"`
	Source        string   `json:"source"`
}

type telemetryThresholdLayer struct {
	Entries   []telemetryThresholdEntry `json:"entries"`
	Template  *string                   `json:"template_id"`
	UpdatedAt *string                   `json:"updated_at"`
	UpdatedBy *telemetryThresholdActor  `json:"updated_by"`
	Metadata  map[string]interface{}    `json:"metadata"`
	Reason    *string                   `json:"reason"`
}

type telemetryThresholdActor struct {
	ID          string  `json:"id"`
	DisplayName *string `json:"display_name"`
}

type telemetryThresholdResponse struct {
	DeviceUUID string `json:"device_uuid"`
	Thresholds struct {
		Effective    []telemetryThresholdEntry `json:"effective"`
		Installation *telemetryThresholdLayer  `json:"installation"`
		Override     *telemetryThresholdLayer  `json:"override"`
	} `json:"thresholds"`
}

type telemetryThresholdUpsertPayload struct {
	Scope      string `json:"scope"`
	Thresholds []struct {
		Parameter          string   `json:"parameter"`
		Min                *float64 `json:"min,omitempty"`
		Max                *float64 `json:"max,omitempty"`
		WarnLow            *float64 `json:"warn_low,omitempty"`
		WarnLowCamel       *float64 `json:"warnLow,omitempty"`
		WarnHigh           *float64 `json:"warn_high,omitempty"`
		WarnHighCamel      *float64 `json:"warnHigh,omitempty"`
		AlertLow           *float64 `json:"alert_low,omitempty"`
		AlertLowCamel      *float64 `json:"alertLow,omitempty"`
		AlertHigh          *float64 `json:"alert_high,omitempty"`
		AlertHighCamel     *float64 `json:"alertHigh,omitempty"`
		Target             *float64 `json:"target,omitempty"`
		Unit               *string  `json:"unit,omitempty"`
		DecimalPlaces      *int     `json:"decimal_places,omitempty"`
		DecimalPlacesCamel *int     `json:"decimalPlaces,omitempty"`
	} `json:"thresholds"`
	Reason          *string                `json:"reason"`
	TemplateID      *string                `json:"template_id"`
	TemplateIDCamel *string                `json:"templateId"`
	Metadata        map[string]interface{} `json:"metadata"`
}

type telemetryThresholdDeletePayload struct {
	Scope  string  `json:"scope"`
	Reason *string `json:"reason"`
}

func NewTelemetryThresholdsController(deviceSvc *services.DeviceService, dnaSvc *services.DnaSpecService, repo *secondary.PostgresRepo) *TelemetryThresholdsController {
	return &TelemetryThresholdsController{devices: deviceSvc, dna: dnaSvc, repo: repo}
}

// GET /api/telemetry/thresholds/:device_uuid
func (h *TelemetryThresholdsController) GetDeviceThresholds(c *fiber.Ctx) error {
	deviceID := pathParamAlias(c, "device_uuid", "deviceUuid")
	if strings.TrimSpace(deviceID) == "" {
		return thresholdValidationError(c, "Invalid device identifier provided", "device_uuid", "device_uuid is required")
	}

	device, err := h.devices.GetDeviceByIDOrIMEI(deviceID)
	if err != nil || device == nil {
		return c.Status(404).JSON(fiber.Map{"message": "Device not found for provided identifier"})
	}
	projectID, _ := device["project_id"].(string)
	if projectID == "" {
		return c.Status(500).JSON(fiber.Map{"message": "device missing project_id"})
	}
	deviceUUID, _ := device["id"].(string)
	if deviceUUID == "" {
		deviceUUID = deviceID
	}

	defaults, err := h.dna.ListThresholds(ctx(c), projectID, "project", nil)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"message": err.Error()})
	}
	overrides, err := h.dna.ListThresholds(ctx(c), projectID, "device", &deviceUUID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"message": err.Error()})
	}
	merged, _, err := h.dna.GetThresholds(ctx(c), projectID, &deviceUUID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"message": err.Error()})
	}

	response := telemetryThresholdResponse{DeviceUUID: deviceUUID}
	response.Thresholds.Effective = mapEntries(merged, "effective")
	if len(defaults) > 0 {
		response.Thresholds.Installation = &telemetryThresholdLayer{
			Entries:   mapEntries(defaults, "installation"),
			Template:  firstTemplateID(defaults),
			UpdatedAt: latestTimestamp(defaults),
			UpdatedBy: firstUpdatedBy(defaults),
			Metadata:  firstMetadata(defaults),
			Reason:    firstReason(defaults),
		}
	}
	if len(overrides) > 0 {
		response.Thresholds.Override = &telemetryThresholdLayer{
			Entries:   mapEntries(overrides, "override"),
			UpdatedAt: latestTimestamp(overrides),
			UpdatedBy: firstUpdatedBy(overrides),
			Reason:    firstReason(overrides),
			Template:  firstTemplateID(overrides),
			Metadata:  firstMetadata(overrides),
		}
	}

	return c.JSON(response)
}

// PUT /api/telemetry/thresholds/:device_uuid
func (h *TelemetryThresholdsController) UpsertDeviceThresholds(c *fiber.Ctx) error {
	deviceID := pathParamAlias(c, "device_uuid", "deviceUuid")
	if strings.TrimSpace(deviceID) == "" {
		return c.Status(400).JSON(fiber.Map{"message": "device id required"})
	}

	var body telemetryThresholdUpsertPayload
	if err := c.BodyParser(&body); err != nil {
		return thresholdValidationError(c, "Invalid threshold payload provided", "body", "invalid threshold payload provided")
	}
	if body.TemplateID == nil {
		body.TemplateID = body.TemplateIDCamel
	}
	if len(body.Thresholds) == 0 {
		return thresholdValidationError(c, "Invalid threshold payload provided", "thresholds", "Provide at least one threshold override")
	}

	device, err := h.devices.GetDeviceByIDOrIMEI(deviceID)
	if err != nil || device == nil {
		return c.Status(404).JSON(fiber.Map{"message": "Device not found for provided identifier"})
	}
	projectID, _ := device["project_id"].(string)
	if projectID == "" {
		return c.Status(500).JSON(fiber.Map{"message": "device missing project_id"})
	}
	deviceUUID, _ := device["id"].(string)
	if deviceUUID == "" {
		deviceUUID = deviceID
	}

	scope := strings.ToLower(strings.TrimSpace(body.Scope))
	if scope == "" {
		scope = "installation"
	}
	storageScope := "project"
	var devicePtr *string
	if scope == "override" {
		storageScope = "device"
		devicePtr = &deviceUUID
	}
	origin := scope

	thresholds := make([]models.DnaThreshold, 0, len(body.Thresholds))
	for _, entry := range body.Thresholds {
		param := strings.TrimSpace(entry.Parameter)
		if param == "" {
			return thresholdValidationError(c, "Invalid threshold payload provided", "parameter", "Parameter key is required")
		}
		warnLow := entry.WarnLow
		if warnLow == nil {
			warnLow = entry.WarnLowCamel
		}
		warnHigh := entry.WarnHigh
		if warnHigh == nil {
			warnHigh = entry.WarnHighCamel
		}
		alertLow := entry.AlertLow
		if alertLow == nil {
			alertLow = entry.AlertLowCamel
		}
		alertHigh := entry.AlertHigh
		if alertHigh == nil {
			alertHigh = entry.AlertHighCamel
		}
		decimalPlaces := entry.DecimalPlaces
		if decimalPlaces == nil {
			decimalPlaces = entry.DecimalPlacesCamel
		}
		thresholds = append(thresholds, models.DnaThreshold{
			Param:         param,
			MinValue:      entry.Min,
			MaxValue:      entry.Max,
			Target:        entry.Target,
			Unit:          entry.Unit,
			DecimalPlaces: decimalPlaces,
			TemplateID:    body.TemplateID,
			Metadata:      body.Metadata,
			Reason:        body.Reason,
			UpdatedBy:     stringPtr(c.Locals("user_id")),
			WarnLow:       warnLow,
			WarnHigh:      warnHigh,
			AlertLow:      alertLow,
			AlertHigh:     alertHigh,
			Origin:        &origin,
		})
	}

	if err := h.dna.UpsertThresholds(ctx(c), projectID, thresholds, storageScope, devicePtr); err != nil {
		return c.Status(500).JSON(fiber.Map{"message": err.Error()})
	}
	h.logAudit(c, "telemetry.thresholds.upserted", map[string]interface{}{
		"device_uuid":    deviceUUID,
		"scope":          scope,
		"parameterCount": len(thresholds),
		"template_id":    body.TemplateID,
		"reason":         body.Reason,
	})

	return h.GetDeviceThresholds(c)
}

// DELETE /api/telemetry/thresholds/:device_uuid
func (h *TelemetryThresholdsController) DeleteDeviceThresholds(c *fiber.Ctx) error {
	deviceID := pathParamAlias(c, "device_uuid", "deviceUuid")
	if strings.TrimSpace(deviceID) == "" {
		return c.Status(400).JSON(fiber.Map{"message": "device id required"})
	}

	var body telemetryThresholdDeletePayload
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&body); err != nil {
			return thresholdValidationError(c, "Invalid deletion payload provided", "body", "invalid deletion payload provided")
		}
	}

	device, err := h.devices.GetDeviceByIDOrIMEI(deviceID)
	if err != nil || device == nil {
		return c.Status(404).JSON(fiber.Map{"message": "Device not found for provided identifier"})
	}
	projectID, _ := device["project_id"].(string)
	if projectID == "" {
		return c.Status(500).JSON(fiber.Map{"message": "device missing project_id"})
	}
	deviceUUID, _ := device["id"].(string)
	if deviceUUID == "" {
		deviceUUID = deviceID
	}

	scope := strings.ToLower(strings.TrimSpace(body.Scope))
	if scope == "" {
		scope = "override"
	}
	storageScope := "project"
	var devicePtr *string
	if scope == "override" {
		storageScope = "device"
		devicePtr = &deviceUUID
	}

	deleted, err := h.dna.DeleteThresholds(ctx(c), projectID, storageScope, devicePtr)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"message": err.Error()})
	}
	if deleted == 0 {
		return c.Status(404).JSON(fiber.Map{"message": "No thresholds found for device"})
	}
	h.logAudit(c, "telemetry.thresholds.deleted", map[string]interface{}{
		"device_uuid": deviceUUID,
		"scope":       scope,
		"reason":      body.Reason,
	})

	return c.SendStatus(204)
}

func mapEntries(thresholds []models.DnaThreshold, source string) []telemetryThresholdEntry {
	entries := make([]telemetryThresholdEntry, 0, len(thresholds))
	for _, t := range thresholds {
		entries = append(entries, telemetryThresholdEntry{
			Parameter:     t.Param,
			Min:           t.MinValue,
			Max:           t.MaxValue,
			WarnLow:       t.WarnLow,
			WarnHigh:      t.WarnHigh,
			AlertLow:      t.AlertLow,
			AlertHigh:     t.AlertHigh,
			Target:        t.Target,
			Unit:          t.Unit,
			DecimalPlaces: t.DecimalPlaces,
			Source:        source,
		})
	}
	return entries
}

func firstTemplateID(items []models.DnaThreshold) *string {
	for _, t := range items {
		if t.TemplateID != nil && strings.TrimSpace(*t.TemplateID) != "" {
			return t.TemplateID
		}
	}
	return nil
}

func firstMetadata(items []models.DnaThreshold) map[string]interface{} {
	for _, t := range items {
		if t.Metadata != nil {
			return t.Metadata
		}
	}
	return nil
}

func firstReason(items []models.DnaThreshold) *string {
	for _, t := range items {
		if t.Reason != nil && strings.TrimSpace(*t.Reason) != "" {
			return t.Reason
		}
	}
	return nil
}

func firstUpdatedBy(items []models.DnaThreshold) *telemetryThresholdActor {
	for _, t := range items {
		if t.UpdatedBy != nil && strings.TrimSpace(*t.UpdatedBy) != "" {
			return &telemetryThresholdActor{ID: *t.UpdatedBy}
		}
	}
	return nil
}

func stringPtr(value interface{}) *string {
	if raw, ok := value.(string); ok {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return nil
		}
		return &trimmed
	}
	return nil
}

func thresholdValidationError(c *fiber.Ctx, message, field, detail string) error {
	return c.Status(400).JSON(fiber.Map{
		"message": message,
		"issues": []fiber.Map{
			{
				"path":    []string{field},
				"message": detail,
			},
		},
	})
}

func (h *TelemetryThresholdsController) logAudit(c *fiber.Ctx, action string, metadata map[string]interface{}) {
	if h == nil || h.repo == nil {
		return
	}
	userID := "system"
	if raw, ok := c.Locals("user_id").(string); ok && strings.TrimSpace(raw) != "" {
		userID = raw
	}
	_ = h.repo.LogAudit(userID, action, c.Path(), c.IP(), "success", metadata)
}

func latestTimestamp(thresholds []models.DnaThreshold) *string {
	var latest time.Time
	for _, t := range thresholds {
		if t.UpdatedAt != nil && t.UpdatedAt.After(latest) {
			latest = *t.UpdatedAt
		}
	}
	if latest.IsZero() {
		return nil
	}
	formatted := latest.UTC().Format(time.RFC3339)
	return &formatted
}
