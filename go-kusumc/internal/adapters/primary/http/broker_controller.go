package http

import (
	"strings"

	"ingestion-go/internal/adapters/secondary"
	"ingestion-go/internal/core/services"

	"github.com/gofiber/fiber/v2"
)

type BrokerController struct {
	emqx    *secondary.EmqxAdapter
	devices *services.DeviceService
	repo    *secondary.PostgresRepo
}

func NewBrokerController(emqx *secondary.EmqxAdapter, devices *services.DeviceService, repo *secondary.PostgresRepo) *BrokerController {
	return &BrokerController{emqx: emqx, devices: devices, repo: repo}
}

// POST /api/broker/sync
func (c *BrokerController) Sync(ctx *fiber.Ctx) error {
	var body struct {
		DeviceUUID      string `json:"device_uuid"`
		DeviceUUIDCamel string `json:"deviceUuid"`
		Reason          string `json:"reason"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).JSON(fiber.Map{"message": "invalid broker resync request"})
	}
	if body.DeviceUUID == "" {
		body.DeviceUUID = body.DeviceUUIDCamel
	}
	deviceID := strings.TrimSpace(body.DeviceUUID)
	if deviceID == "" {
		return ctx.Status(400).JSON(fiber.Map{"message": "device_uuid required"})
	}

	var device map[string]interface{}
	if c.devices != nil {
		if d, err := c.devices.GetDeviceByIDOrIMEI(deviceID); err == nil && d != nil {
			device = d
		}
	}
	if device == nil {
		return ctx.Status(404).JSON(fiber.Map{"message": "device not found"})
	}

	var latestCred map[string]interface{}
	if c.devices != nil {
		if id, ok := device["id"].(string); ok && id != "" {
			if cred, err := c.devices.GetLatestCredentialHistory(id); err == nil {
				latestCred = cred
			}
		}
	}
	var credID string
	if latestCred != nil {
		if id, ok := latestCred["id"].(string); ok {
			credID = id
		}
	}

	if c.devices != nil {
		var targetCred *string
		if credID != "" {
			targetCred = &credID
		}
		if err := c.devices.RetryProvisioning(deviceID, targetCred); err != nil {
			return ctx.Status(400).JSON(fiber.Map{"message": err.Error()})
		}
	}

	if c.emqx != nil {
		if err := c.emqx.SyncBroker(); err != nil {
			return ctx.Status(500).JSON(fiber.Map{"message": err.Error()})
		}
	}

	resyncCount := 0
	if latestCred != nil {
		if attempts, ok := latestCred["attempts"].(int); ok {
			resyncCount = attempts
		}
	}

	projectID, _ := device["project_id"].(string)
	imei, _ := device["imei"].(string)

	return ctx.Status(202).JSON(fiber.Map{
		"device": fiber.Map{
			"id":   device["id"],
			"imei": imei,
		},
		"credential_history_id": credID,
		"previous_job_id":       nil,
		"resync_count":          resyncCount,
		"mqtt_provisioning": fiber.Map{
			"status":     "pending",
			"last_error": latestCred["last_error"],
		},
		"scope": fiber.Map{
			"state_id":     nil,
			"authority_id": nil,
			"project_id":   projectID,
		},
		"reason": body.Reason,
	})
}
