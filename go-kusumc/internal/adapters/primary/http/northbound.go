package http

import (
	"encoding/json"
	"ingestion-go/internal/core/services"
	"time"

	"github.com/gofiber/fiber/v2"
)

type NorthboundController struct {
	ingest *services.IngestionService
	config *services.ConfigService
}

func NewNorthboundController(ingest *services.IngestionService, config *services.ConfigService) *NorthboundController {
	return &NorthboundController{ingest: ingest, config: config}
}

func (c *NorthboundController) HandleChirpStack(ctx *fiber.Ctx) error {
	// [FEATURE FLAG] Check if enabled
	if !c.config.EnableChirpstack {
		return ctx.Status(404).SendString("ChirpStack Integration Disabled")
	}

	// [SEC] 0. API KEY AUTH
	apiKey := ctx.Get("X-API-KEY")
	// In prod, check against vault. For V1, simple check.
	// We allow if key is present and > 5 chars (Dummy check) or Env Var
	if len(apiKey) < 5 {
		return ctx.Status(401).SendString("Unauthorized")
	}

	// 1. Validate
	if ctx.Query("event") != "up" {
		return ctx.SendStatus(200)
	} // Ignore non-uplink

	var req struct {
		DeviceInfo struct {
			DevEUI string `json:"devEui"`
		} `json:"deviceInfo"`
		Data   string `json:"data"` // Base64
		RxInfo []struct {
			RSSI int `json:"rssi"`
		} `json:"rxInfo"`
	}

	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(400).SendString("Bad JSON")
	}

	// 2. Normalize
	// No decoder here (Simple Pass-through) because Go Engine expects JSON data map?
	// We put raw base64 into a special 'raw' field or decode if we knew the format.
	// Node.js implementation passed it as `data: { _raw: ... }`.

	payload := map[string]interface{}{
		"imei":      req.DeviceInfo.DevEUI,
		"type":      "lora",
		"timestamp": time.Now().UnixMilli(),
		"payload":   map[string]interface{}{"_raw": req.Data},
		"metadata": map[string]interface{}{
			"rssi": req.RxInfo[0].RSSI,
		},
	}

	// 3. Ingest
	bodyBytes, _ := json.Marshal(payload) // Helper needed or standard json
	err := c.ingest.ProcessPacket("http/northbound", bodyBytes, "")
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}

	return ctx.JSON(fiber.Map{"status": "accepted"})
}
