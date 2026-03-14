package http

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
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

type TelemetryMirrorController struct {
	ingest *services.IngestionService
	config *services.ConfigService
	repo   *secondary.PostgresRepo
	rates  *mirrorRateLimiter
}

type mirrorRateLimiter struct {
	mu           sync.Mutex
	entries      map[string]rateEntry
	maxPerMinute int
}

type rateEntry struct {
	count       int
	windowStart time.Time
}

var allowedMirrorTopics = map[string]bool{
	"heartbeat": true,
	"data":      true,
	"daq":       true,
	"ondemand":  true,
	"errors":    true,
}

func NewTelemetryMirrorController(ingest *services.IngestionService, config *services.ConfigService, repo *secondary.PostgresRepo) *TelemetryMirrorController {
	max := 60
	if configured := strings.TrimSpace(os.Getenv("HTTPS_TELEMETRY_RATE_LIMIT_PER_MINUTE")); configured != "" {
		if v, err := parsePositiveInt(configured); err == nil && v > 0 {
			max = v
		}
	}
	return &TelemetryMirrorController{
		ingest: ingest,
		config: config,
		repo:   repo,
		rates:  &mirrorRateLimiter{entries: make(map[string]rateEntry), maxPerMinute: max},
	}
}

func (c *TelemetryMirrorController) Ingest(ctx *fiber.Ctx) error {
	if !c.config.EnableHttpsTelemetryMirror {
		return ctx.Status(501).JSON(fiber.Map{
			"message":    "HTTPS telemetry mirror is not enabled",
			"next_steps": "Set ENABLE_HTTPS_TELEMETRY_MIRROR=true to activate the handler.",
		})
	}
	topicSuffix := pathParamAlias(ctx, "topic_suffix", "topicSuffix")
	if topicSuffix == "" || !allowedMirrorTopics[topicSuffix] {
		return ctx.Status(400).JSON(fiber.Map{
			"message": "Unsupported topic suffix for HTTPS ingestion mirror",
			"details": fiber.Map{"topic_suffix": topicSuffix},
		})
	}

	imei := strings.TrimSpace(ctx.Get("X-RMS-IMEI"))
	if imei == "" {
		return ctx.Status(400).JSON(fiber.Map{"message": "Missing required X-RMS-IMEI header"})
	}

	clientID := strings.TrimSpace(ctx.Get("X-RMS-ClientId"))
	if clientID == "" {
		return ctx.Status(400).JSON(fiber.Map{"message": "Missing required X-RMS-ClientId header"})
	}

	var payload map[string]interface{}
	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(400).JSON(fiber.Map{"message": "Telemetry payload must be a JSON object"})
	}

	credentials, ok := parseBasicAuth(ctx.Get("Authorization"))
	if !ok {
		return unauthorized(ctx, "Basic authentication is required for HTTPS ingestion")
	}

	credRecord, err := c.repo.GetCredentialHistoryByUsername(credentials.username)
	if err != nil || credRecord == nil {
		return unauthorized(ctx, "Invalid device credentials for HTTPS ingestion")
	}

	bundle, _ := credRecord["bundle"].(map[string]interface{})
	if bundle == nil {
		return unauthorized(ctx, "Invalid device credentials for HTTPS ingestion")
	}

	bundlePass, _ := bundle["password"].(string)
	if bundlePass == "" || bundlePass != credentials.password {
		return unauthorized(ctx, "Invalid device credentials for HTTPS ingestion")
	}

	bundleClientID, _ := bundle["client_id"].(string)
	if bundleClientID != "" && bundleClientID != clientID {
		return unauthorized(ctx, "Client identifier does not match credential bundle")
	}

	deviceIMEI, _ := credRecord["imei"].(string)
	if deviceIMEI != "" && deviceIMEI != imei {
		return unauthorized(ctx, "IMEI does not match credential bundle")
	}

	if status, ok := credRecord["status"].(string); ok && status != "" && status != "active" {
		return unauthorized(ctx, "Device is not active for HTTPS ingestion")
	}

	if !c.rates.allow(fmt.Sprintf("%s:%s", imei, credentials.username)) {
		retry := c.rates.retryAfter(fmt.Sprintf("%s:%s", imei, credentials.username))
		return ctx.Status(429).JSON(fiber.Map{
			"message":             "HTTPS telemetry rate limit exceeded",
			"retry_after_seconds": retry,
		})
	}

	msgid := strings.TrimSpace(ctx.Get("X-RMS-MsgId"))
	telemetryID := msgid
	if telemetryID == "" {
		telemetryID = uuid.NewString()
	}

	envelope := map[string]interface{}{}
	for k, v := range payload {
		envelope[k] = v
	}
	envelope["imei"] = imei
	if msgid != "" {
		envelope["msgid"] = msgid
	}
	if _, ok := envelope["packet_type"].(string); !ok {
		envelope["packet_type"] = topicSuffix
	}

	blob, err := json.Marshal(envelope)
	if err != nil {
		return ctx.Status(400).JSON(fiber.Map{"message": "Telemetry payload must be a JSON object"})
	}

	if err := c.ingest.ProcessPacket("https/mirror/"+topicSuffix, blob, ""); err != nil {
		return ctx.Status(500).JSON(fiber.Map{"message": err.Error()})
	}

	return ctx.Status(202).JSON(fiber.Map{"status": "accepted", "telemetry_id": telemetryID})
}

func (r *mirrorRateLimiter) allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	entry, exists := r.entries[key]
	if !exists || now.Sub(entry.windowStart) >= time.Minute {
		r.entries[key] = rateEntry{count: 1, windowStart: now}
		return true
	}

	if entry.count >= r.maxPerMinute {
		return false
	}

	entry.count++
	r.entries[key] = entry
	return true
}

func (r *mirrorRateLimiter) retryAfter(key string) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.entries[key]
	if !ok {
		return 0
	}
	remaining := time.Minute - time.Since(entry.windowStart)
	if remaining < 0 {
		return 0
	}
	return int(remaining.Seconds())
}

type basicCredentials struct {
	username string
	password string
}

func parseBasicAuth(header string) (basicCredentials, bool) {
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "basic" {
		return basicCredentials{}, false
	}

	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return basicCredentials{}, false
	}

	value := string(decoded)
	sep := strings.Index(value, ":")
	if sep <= 0 {
		return basicCredentials{}, false
	}

	username := value[:sep]
	password := value[sep+1:]
	if username == "" || password == "" {
		return basicCredentials{}, false
	}

	return basicCredentials{username: username, password: password}, true
}

func unauthorized(ctx *fiber.Ctx, message string) error {
	ctx.Set("WWW-Authenticate", "Basic realm=\"RMS HTTPS Mirror\"")
	return ctx.Status(401).JSON(fiber.Map{"message": message})
}

func parsePositiveInt(raw string) (int, error) {
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("invalid int")
	}
	return value, nil
}
