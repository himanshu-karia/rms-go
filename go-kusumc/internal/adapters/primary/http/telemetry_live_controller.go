package http

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"ingestion-go/internal/core/services"

	"github.com/gofiber/fiber/v2"
)

type TelemetryLiveController struct {
	svc      *services.DeviceService
	state    liveStateStore
	ticketMu sync.Mutex
	tickets  map[string]liveTicket
}

type liveTicket struct {
	DeviceID  string
	ExpiresAt time.Time
}

type liveTicketStore interface {
	SetRaw(key string, val string, ttl time.Duration) error
	GetRaw(key string) (string, bool, error)
	Delete(key string) error
}

type liveStateStore interface {
	GetPackets(deviceId string) ([]interface{}, error)
}

func NewTelemetryLiveController(deviceSvc *services.DeviceService, state liveStateStore) *TelemetryLiveController {
	return &TelemetryLiveController{svc: deviceSvc, state: state, tickets: map[string]liveTicket{}}
}

// GET /api/telemetry/devices/:device_uuid/live
func (c *TelemetryLiveController) StreamDevice(ctx *fiber.Ctx) error {
	deviceID := pathParamAlias(ctx, "device_uuid", "deviceUuid")
	if deviceID == "" {
		return ctx.Status(400).SendString("device id required")
	}

	token := ctx.Query("token")
	if token == "" {
		token = ctx.Query("live_token")
	}
	if strings.TrimSpace(token) == "" || !c.consumeTicket(strings.TrimSpace(token), deviceID) {
		return ctx.Status(401).SendString("invalid or expired live token")
	}

	imei := deviceID
	if c.svc != nil {
		if dev, err := c.svc.GetDeviceByIDOrIMEI(deviceID); err == nil && dev != nil {
			if v, ok := dev["imei"].(string); ok && v != "" {
				imei = v
			}
		}
	}

	ctx.Set("Content-Type", "text/event-stream")
	ctx.Set("Cache-Control", "no-cache")
	ctx.Set("Connection", "keep-alive")
	ctx.Set("X-Accel-Buffering", "no")

	ctx.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		var lastSent string
		for {
			select {
			case <-ctx.Context().Done():
				return
			case <-ticker.C:
				if c.state == nil {
					continue
				}
				packets, err := c.state.GetPackets(imei)
				if err != nil || len(packets) == 0 {
					_, _ = w.WriteString("event: heartbeat\ndata: {}\n\n")
					_ = w.Flush()
					continue
				}
				payload := packets[0]
				blob, err := json.Marshal(payload)
				if err != nil {
					continue
				}
				if string(blob) == lastSent {
					continue
				}
				lastSent = string(blob)
				_, _ = w.WriteString(fmt.Sprintf("event: telemetry\ndata: %s\n\n", string(blob)))
				_ = w.Flush()
			}
		}
	})

	return nil
}

// POST /api/telemetry/devices/:device_uuid/live-token
func (c *TelemetryLiveController) IssueLiveToken(ctx *fiber.Ctx) error {
	deviceID := pathParamAlias(ctx, "device_uuid", "deviceUuid")
	if deviceID == "" {
		return ctx.Status(400).SendString("device id required")
	}
	token := issueLiveTokenValue()
	expiresAtTime := time.Now().Add(30 * time.Minute).UTC()
	expiresAt := expiresAtTime.Format(time.RFC3339)
	ticket := liveTicket{DeviceID: deviceID, ExpiresAt: expiresAtTime}

	if !c.storeTicketInRedis(token, ticket) {
		c.ticketMu.Lock()
		c.tickets[token] = ticket
		c.ticketMu.Unlock()
	}

	return ctx.JSON(fiber.Map{"token": token, "expires_at": expiresAt})
}

func (c *TelemetryLiveController) consumeTicket(token, deviceID string) bool {
	if c.consumeTicketFromRedis(token, deviceID) {
		return true
	}

	now := time.Now().UTC()

	c.ticketMu.Lock()
	defer c.ticketMu.Unlock()

	meta, ok := c.tickets[token]
	if !ok {
		return false
	}
	if now.After(meta.ExpiresAt) {
		delete(c.tickets, token)
		return false
	}
	if strings.TrimSpace(meta.DeviceID) != strings.TrimSpace(deviceID) {
		return false
	}
	delete(c.tickets, token)
	return true
}

func (c *TelemetryLiveController) storeTicketInRedis(token string, ticket liveTicket) bool {
	store, ok := c.state.(liveTicketStore)
	if !ok {
		return false
	}
	blob, err := json.Marshal(ticket)
	if err != nil {
		return false
	}
	key := fmt.Sprintf("telemetry:live:ticket:%s", token)
	ttl := time.Until(ticket.ExpiresAt)
	if ttl <= 0 {
		return false
	}
	if err := store.SetRaw(key, string(blob), ttl); err != nil {
		return false
	}
	return true
}

func (c *TelemetryLiveController) consumeTicketFromRedis(token, deviceID string) bool {
	store, ok := c.state.(liveTicketStore)
	if !ok {
		return false
	}
	key := fmt.Sprintf("telemetry:live:ticket:%s", token)
	raw, found, err := store.GetRaw(key)
	if err != nil || !found || strings.TrimSpace(raw) == "" {
		return false
	}
	var meta liveTicket
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		_ = store.Delete(key)
		return false
	}
	if time.Now().UTC().After(meta.ExpiresAt) {
		_ = store.Delete(key)
		return false
	}
	if strings.TrimSpace(meta.DeviceID) != strings.TrimSpace(deviceID) {
		return false
	}
	if err := store.Delete(key); err != nil {
		return false
	}
	return true
}

func issueLiveTokenValue() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("live_%d", time.Now().UnixNano())
	}
	return "live_" + hex.EncodeToString(buf)
}
