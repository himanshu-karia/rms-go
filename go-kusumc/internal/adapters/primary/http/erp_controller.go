package http

import (
	"ingestion-go/internal/core/services"

	"github.com/gofiber/fiber/v2"
)

type ERPController struct {
	maint *services.MaintenanceService
	inv   *services.InventoryService
	logi  *services.LogisticsService
	traf  *services.TrafficService
}

func NewERPController(
	m *services.MaintenanceService,
	i *services.InventoryService,
	l *services.LogisticsService,
	t *services.TrafficService,
) *ERPController {
	return &ERPController{
		maint: m,
		inv:   i,
		logi:  l,
		traf:  t,
	}
}

// Maintenance
func (c *ERPController) GetWorkOrders(ctx *fiber.Ctx) error {
	data, err := c.maint.GetWorkOrders()
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(normalizeToSnakeKeys(data))
}

// Inventory
func (c *ERPController) GetProducts(ctx *fiber.Ctx) error {
	data, err := c.inv.GetProducts()
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(normalizeToSnakeKeys(data))
}

func (c *ERPController) GetStockLevels(ctx *fiber.Ctx) error {
	locID := ctx.Params("locId")
	if locID == "" {
		locID = ctx.Params("loc_id")
	}
	if locID == "" {
		return ctx.Status(400).SendString("location_id required")
	}
	data, err := c.inv.GetStockLevels(locID)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(normalizeToSnakeKeys(data))
}

// Logistics
func (c *ERPController) GetTrips(ctx *fiber.Ctx) error {
	data, err := c.logi.GetTrips()
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(normalizeToSnakeKeys(data))
}

func (c *ERPController) GetAssetTimeline(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	data, err := c.logi.GetAssetTimeline(id)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(normalizeToSnakeKeys(data))
}

func (c *ERPController) GetGeofences(ctx *fiber.Ctx) error {
	data, err := c.logi.GetGeofences()
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(normalizeToSnakeKeys(data))
}

// Traffic
func (c *ERPController) GetCameras(ctx *fiber.Ctx) error {
	data, err := c.traf.GetCameras()
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(normalizeToSnakeKeys(data))
}

func (c *ERPController) GetTrafficMetrics(ctx *fiber.Ctx) error {
	deviceID := ctx.Params("deviceId")
	if deviceID == "" {
		deviceID = ctx.Params("device_id")
	}
	if deviceID == "" {
		return ctx.Status(400).SendString("device_id required")
	}
	data, err := c.traf.GetMetrics(deviceID, 50)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(normalizeToSnakeKeys(data))
}

// --- WRITE HANDLERS ---

func (c *ERPController) CreateWorkOrder(ctx *fiber.Ctx) error {
	var body map[string]interface{}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString(err.Error())
	}
	if err := c.maint.CreateWorkOrder(normalizeToSnakeKeys(body).(map[string]interface{})); err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.SendStatus(201)
}

func (c *ERPController) ResolveWorkOrder(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	var body struct {
		ResolutionNotes      string `json:"resolution_notes"`
		ResolutionNotesCamel string `json:"resolutionNotes"`
		Notes                string `json:"notes"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString(err.Error())
	}
	notes := body.ResolutionNotes
	if notes == "" {
		notes = body.ResolutionNotesCamel
	}
	if notes == "" {
		notes = body.Notes
	}
	if notes == "" {
		return ctx.Status(400).SendString("resolution_notes required")
	}
	if err := c.maint.ResolveWorkOrder(id, notes); err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.SendStatus(200)
}

func (c *ERPController) CreateProduct(ctx *fiber.Ctx) error {
	var body map[string]interface{}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString(err.Error())
	}
	if err := c.inv.CreateProduct(normalizeToSnakeKeys(body).(map[string]interface{})); err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.SendStatus(201)
}

func (c *ERPController) CreateTrip(ctx *fiber.Ctx) error {
	var body map[string]interface{}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString(err.Error())
	}
	if err := c.logi.CreateTrip(normalizeToSnakeKeys(body).(map[string]interface{})); err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.SendStatus(201)
}

func (c *ERPController) CreateTrafficMetric(ctx *fiber.Ctx) error {
	var body struct {
		DeviceID      string `json:"device_id"`
		DeviceIDCamel string `json:"deviceId"`
		Data          struct {
			Counts map[string]int `json:"counts"`
			Speed  float64        `json:"speed"`
		} `json:"data"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString(err.Error())
	}
	if body.DeviceID == "" {
		body.DeviceID = body.DeviceIDCamel
	}
	if body.DeviceID == "" {
		return ctx.Status(400).SendString("device_id required")
	}
	vehicleCount := 0
	breakdown := map[string]interface{}{}
	for key, value := range body.Data.Counts {
		vehicleCount += value
		breakdown[key] = value
	}
	congestion := "LOW"
	if vehicleCount >= 30 {
		congestion = "HIGH"
	} else if vehicleCount >= 15 {
		congestion = "MODERATE"
	}
	if err := c.traf.CreateMetric(body.DeviceID, breakdown, body.Data.Speed, congestion, vehicleCount); err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.SendStatus(201)
}
