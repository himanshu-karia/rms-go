package http

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"ingestion-go/internal/core/services"
	"ingestion-go/internal/models"
)

// DnaSpecController handles sensors and thresholds endpoints for Project DNA tables.
type DnaSpecController struct {
	svc *services.DnaSpecService
}

func NewDnaSpecController(svc *services.DnaSpecService) *DnaSpecController {
	return &DnaSpecController{svc: svc}
}

func (h *DnaSpecController) ListSensors(c *fiber.Ctx) error {
	projectID := c.Params("projectId")
	sensors, err := h.svc.ListSensors(ctx(c), projectID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"sensors": sensors})
}

func (h *DnaSpecController) UpsertSensors(c *fiber.Ctx) error {
	projectID := c.Params("projectId")
	var body struct {
		Sensors []models.DnaSensor `json:"sensors"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid payload"})
	}
	if err := h.svc.UpsertSensors(ctx(c), projectID, body.Sensors); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(200)
}

func (h *DnaSpecController) GetThresholds(c *fiber.Ctx) error {
	projectID := c.Params("projectId")
	device := c.Query("device")
	var devicePtr *string
	if device != "" {
		devicePtr = &device
	}
	thresholds, source, err := h.svc.GetThresholds(ctx(c), projectID, devicePtr)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"thresholds": thresholds, "source": source})
}

// ListThresholdDevices returns known device IDs with overrides for suggestions.
func (h *DnaSpecController) ListThresholdDevices(c *fiber.Ctx) error {
	projectID := c.Params("projectId")
	ids, err := h.svc.ListThresholdDevices(ctx(c), projectID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"devices": ids})
}

func (h *DnaSpecController) UpsertThresholds(c *fiber.Ctx) error {
	projectID := c.Params("projectId")
	var body struct {
		Thresholds []models.DnaThreshold `json:"thresholds"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid payload"})
	}
	if err := h.svc.UpsertThresholds(ctx(c), projectID, body.Thresholds, "project", nil); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(200)
}

func (h *DnaSpecController) UpsertDeviceThresholds(c *fiber.Ctx) error {
	projectID := c.Params("projectId")
	deviceID := c.Params("deviceId")
	if deviceID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "deviceId required"})
	}
	var body struct {
		Thresholds []models.DnaThreshold `json:"thresholds"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid payload"})
	}
	if err := h.svc.UpsertThresholds(ctx(c), projectID, body.Thresholds, "device", &deviceID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(200)
}

// ExportSensorsCSV returns sensors as CSV for audits.
func (h *DnaSpecController) ExportSensorsCSV(c *fiber.Ctx) error {
	projectID := c.Params("projectId")
	data, err := h.svc.ExportSensorsCSV(ctx(c), projectID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	c.Set(fiber.HeaderContentType, "text/csv")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%s-sensors.csv", projectID))
	return c.Status(http.StatusOK).Send(data)
}

// ImportSensorsCSV ingests sensors from CSV and upserts.
func (h *DnaSpecController) ImportSensorsCSV(c *fiber.Ctx) error {
	projectID := c.Params("projectId")
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "file required"})
	}

	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "cannot read file"})
	}
	defer file.Close()

	count, err := h.svc.ImportSensorsCSV(ctx(c), projectID, file)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"imported": count})
}

// CreateSensorVersion stores a CSV snapshot as a draft.
func (h *DnaSpecController) CreateSensorVersion(c *fiber.Ctx) error {
	projectID := c.Params("projectId")
	label := c.FormValue("label")
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "file required"})
	}

	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "cannot read file"})
	}
	defer file.Close()

	versionID, count, err := h.svc.CreateSensorVersionFromCSV(ctx(c), projectID, label, file, userIdentifier(c))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"version_id": versionID, "imported": count})
}

// PublishSensorVersion applies a stored version and syncs caches.
func (h *DnaSpecController) PublishSensorVersion(c *fiber.Ctx) error {
	projectID := c.Params("projectId")
	versionStr := c.Params("versionId")
	versionID, err := strconv.ParseInt(versionStr, 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid version id"})
	}
	count, err := h.svc.PublishSensorVersion(ctx(c), projectID, versionID, userIdentifier(c))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"applied": count, "version_id": versionID})
}

// RollbackSensorVersion reapplies a previous version.
func (h *DnaSpecController) RollbackSensorVersion(c *fiber.Ctx) error {
	projectID := c.Params("projectId")
	versionStr := c.Params("versionId")
	versionID, err := strconv.ParseInt(versionStr, 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid version id"})
	}
	count, err := h.svc.RollbackSensorVersion(ctx(c), projectID, versionID, userIdentifier(c))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"applied": count, "version_id": versionID})
}

// DownloadSensorVersionCSV streams the stored CSV for a version.
func (h *DnaSpecController) DownloadSensorVersionCSV(c *fiber.Ctx) error {
	projectID := c.Params("projectId")
	versionStr := c.Params("versionId")
	versionID, err := strconv.ParseInt(versionStr, 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid version id"})
	}
	data, versionMeta, err := h.svc.GetSensorVersionCSV(ctx(c), projectID, versionID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	filename := fmt.Sprintf("%s-sensors-v%d.csv", projectID, versionID)
	if versionMeta != nil && versionMeta.Label != "" {
		filename = fmt.Sprintf("%s-%s.csv", projectID, versionMeta.Label)
	}
	c.Set(fiber.HeaderContentType, "text/csv")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf("attachment; filename=\"%s\"", filename))
	return c.Status(http.StatusOK).Send(data)
}

// ListSensorVersions returns metadata for recent versions.
func (h *DnaSpecController) ListSensorVersions(c *fiber.Ctx) error {
	projectID := c.Params("projectId")
	versions, err := h.svc.ListSensorVersions(ctx(c), projectID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"versions": versions})
}

func ctx(c *fiber.Ctx) context.Context {
	if c != nil && c.Context() != nil {
		return c.Context()
	}
	return context.Background()
}

// userIdentifier returns the actor identifier if present in request context.
func userIdentifier(c *fiber.Ctx) *string {
	if c == nil {
		return nil
	}
	if v, ok := c.Locals("user_id").(string); ok && v != "" {
		return &v
	}
	if v, ok := c.Locals("user_email").(string); ok && v != "" {
		return &v
	}
	return nil
}
