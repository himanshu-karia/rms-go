package http

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type mobileOTPEntry struct {
	Phone     string
	OTP       string
	CreatedAt time.Time
	ExpiresAt time.Time
	Attempts  int
}

var mobileOTPStore = struct {
	sync.Mutex
	items map[string]mobileOTPEntry
}{items: map[string]mobileOTPEntry{}}

type mobileIngestPacket struct {
	IdempotencyKey string         `json:"idempotency_key"`
	CapturedAt     string         `json:"captured_at"`
	TopicSuffix    string         `json:"topic_suffix"`
	RawPayload     map[string]any `json:"raw_payload"`
}

func generateMobileOTP() string {
	const digits = "0123456789"
	out := make([]byte, 6)
	for i := range out {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "123456"
		}
		out[i] = digits[n.Int64()]
	}
	return string(out)
}

func (r *Router) handleMobileRequestOTP(c *fiber.Ctx) error {
	start := time.Now()
	ok := false
	defer func() { recordMobileEndpoint("auth_request_otp", time.Since(start), !ok) }()

	incMobileAuthRequestOtp()
	var req struct {
		Phone             string `json:"phone"`
		DeviceFingerprint string `json:"device_fingerprint"`
		DeviceName        string `json:"device_name"`
		AppVersion        string `json:"app_version"`
	}
	if err := c.BodyParser(&req); err != nil {
		return WriteAPIError(c, fiber.StatusBadRequest, "mobile_invalid_json", "Invalid JSON", nil)
	}
	if strings.TrimSpace(req.Phone) == "" || strings.TrimSpace(req.DeviceFingerprint) == "" || strings.TrimSpace(req.AppVersion) == "" {
		return WriteAPIError(c, fiber.StatusBadRequest, "mobile_invalid_request", "phone, device_fingerprint and app_version are required", nil)
	}

	otpRef := uuid.NewString()
	otp := generateMobileOTP()

	mobileOTPStore.Lock()
	mobileOTPStore.items[otpRef] = mobileOTPEntry{
		Phone:     strings.TrimSpace(req.Phone),
		OTP:       otp,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(5 * time.Minute),
		Attempts:  0,
	}
	mobileOTPStore.Unlock()

	ok = true
	return c.JSON(fiber.Map{
		"status":  "sent",
		"otp_ref": otpRef,
	})
}

func (r *Router) handleMobileLatestOTPForInternalTests(c *fiber.Ctx) error {
	if strings.TrimSpace(c.Get("X-Internal-Test")) != "1" {
		return WriteAPIError(c, fiber.StatusForbidden, "mobile_internal_test_forbidden", "internal test header required", nil)
	}
	phone := strings.TrimSpace(c.Query("phone"))
	if phone == "" {
		return WriteAPIError(c, fiber.StatusBadRequest, "mobile_phone_required", "phone query param is required", nil)
	}

	now := time.Now()
	mobileOTPStore.Lock()
	defer mobileOTPStore.Unlock()

	latestRef := ""
	latest := mobileOTPEntry{}
	for ref, entry := range mobileOTPStore.items {
		if entry.Phone != phone {
			continue
		}
		if now.After(entry.ExpiresAt) {
			continue
		}
		if latestRef == "" || entry.CreatedAt.After(latest.CreatedAt) {
			latestRef = ref
			latest = entry
		}
	}

	if latestRef == "" {
		return WriteAPIError(c, fiber.StatusNotFound, "mobile_otp_not_found", "no active otp for phone", nil)
	}

	return c.JSON(fiber.Map{
		"phone":      latest.Phone,
		"otp_ref":    latestRef,
		"otp":        latest.OTP,
		"created_at": latest.CreatedAt.UTC().Format(time.RFC3339),
		"expires_at": latest.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (r *Router) handleMobileVerifyOTP(c *fiber.Ctx) error {
	start := time.Now()
	ok := false
	defer func() { recordMobileEndpoint("auth_verify", time.Since(start), !ok) }()

	incMobileAuthVerify()
	var req struct {
		Phone  string `json:"phone"`
		OTP    string `json:"otp"`
		OTPRef string `json:"otp_ref"`
	}
	if err := c.BodyParser(&req); err != nil {
		return WriteAPIError(c, fiber.StatusBadRequest, "mobile_invalid_json", "Invalid JSON", nil)
	}
	if strings.TrimSpace(req.Phone) == "" || strings.TrimSpace(req.OTP) == "" || strings.TrimSpace(req.OTPRef) == "" {
		return WriteAPIError(c, fiber.StatusBadRequest, "mobile_invalid_request", "phone, otp and otp_ref are required", nil)
	}

	mobileOTPStore.Lock()
	entry, ok := mobileOTPStore.items[req.OTPRef]
	if !ok {
		mobileOTPStore.Unlock()
		return WriteAPIError(c, fiber.StatusUnauthorized, "mobile_otp_invalid", "Invalid OTP reference", nil)
	}
	if time.Now().After(entry.ExpiresAt) {
		delete(mobileOTPStore.items, req.OTPRef)
		mobileOTPStore.Unlock()
		return WriteAPIError(c, fiber.StatusUnauthorized, "mobile_otp_expired", "OTP expired", nil)
	}
	if entry.Phone != strings.TrimSpace(req.Phone) || entry.OTP != strings.TrimSpace(req.OTP) {
		entry.Attempts++
		if entry.Attempts >= 5 {
			delete(mobileOTPStore.items, req.OTPRef)
		} else {
			mobileOTPStore.items[req.OTPRef] = entry
		}
		mobileOTPStore.Unlock()
		return WriteAPIError(c, fiber.StatusUnauthorized, "mobile_otp_invalid", "Invalid OTP", nil)
	}
	delete(mobileOTPStore.items, req.OTPRef)
	mobileOTPStore.Unlock()

	user, err := r.auth.EnsureMobileUser(strings.TrimSpace(req.Phone))
	if err != nil {
		return WriteAPIError(c, fiber.StatusInternalServerError, "mobile_user_issue_failed", err.Error(), nil)
	}

	ip := c.IP()
	ua := c.Get("User-Agent")
	user, tokens, err := r.auth.IssueSessionForUserID(user.ID, &ip, &ua)
	if err != nil {
		return WriteAPIError(c, fiber.StatusInternalServerError, "mobile_session_issue_failed", err.Error(), nil)
	}

	ok = true
	return c.JSON(fiber.Map{
		"access_token":   tokens.AccessToken,
		"refresh_token":  tokens.RefreshToken,
		"token_type":     "Bearer",
		"expires_in_sec": int(time.Until(tokens.AccessExpiresAt).Seconds()),
		"user": fiber.Map{
			"id":           user.ID,
			"phone":        user.Username,
			"display_name": user.DisplayName,
		},
	})
}

func (r *Router) handleMobileRefresh(c *fiber.Ctx) error {
	start := time.Now()
	ok := false
	defer func() { recordMobileEndpoint("auth_refresh", time.Since(start), !ok) }()

	incMobileAuthRefresh()
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.BodyParser(&req); err != nil {
		return WriteAPIError(c, fiber.StatusBadRequest, "mobile_invalid_json", "Invalid JSON", nil)
	}
	if strings.TrimSpace(req.RefreshToken) == "" {
		return WriteAPIError(c, fiber.StatusBadRequest, "mobile_refresh_token_required", "refresh_token required", nil)
	}

	ip := c.IP()
	ua := c.Get("User-Agent")
	_, tokens, err := r.auth.RefreshSession(req.RefreshToken, &ip, &ua)
	if err != nil {
		return WriteAPIError(c, fiber.StatusUnauthorized, "mobile_refresh_failed", err.Error(), nil)
	}

	ok = true
	return c.JSON(fiber.Map{
		"access_token":   tokens.AccessToken,
		"refresh_token":  tokens.RefreshToken,
		"token_type":     "Bearer",
		"expires_in_sec": int(time.Until(tokens.AccessExpiresAt).Seconds()),
	})
}

func (r *Router) handleMobileLogout(c *fiber.Ctx) error {
	start := time.Now()
	defer func() { recordMobileEndpoint("auth_logout", time.Since(start), false) }()

	incMobileAuthLogout()
	if sessionID, ok := c.Locals("session_id").(string); ok && strings.TrimSpace(sessionID) != "" {
		_ = r.auth.LogoutSession(sessionID)
	}
	return c.JSON(fiber.Map{"status": "logged_out"})
}

func (r *Router) handleMobileAssignments(c *fiber.Ctx) error {
	start := time.Now()
	ok := false
	defer func() { recordMobileEndpoint("assignments", time.Since(start), !ok) }()

	incMobileAssignments()
	if r.pg == nil || r.pg.Pool == nil {
		return WriteAPIError(c, fiber.StatusInternalServerError, "mobile_assignments_unavailable", "repository unavailable", nil)
	}

	userID, _ := c.Locals("user_id").(string)
	if strings.TrimSpace(userID) == "" {
		return WriteAPIError(c, fiber.StatusUnauthorized, "mobile_user_missing", "user context missing", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := r.pg.Pool.Query(ctx, `
		SELECT project_id, COALESCE(device_id, ''), COALESCE(metadata->>'role', 'technician')
		FROM mobile_assignments
		WHERE user_id = $1 AND active = true
		ORDER BY id DESC
	`, userID)
	if err != nil {
		return WriteAPIError(c, fiber.StatusInternalServerError, "mobile_assignments_query_failed", err.Error(), nil)
	}
	defer rows.Close()

	items := make([]fiber.Map, 0)
	for rows.Next() {
		var projectID, deviceID, role string
		if err := rows.Scan(&projectID, &deviceID, &role); err != nil {
			return WriteAPIError(c, fiber.StatusInternalServerError, "mobile_assignments_scan_failed", err.Error(), nil)
		}
		item := fiber.Map{
			"project_id": projectID,
			"role":       role,
		}
		if strings.TrimSpace(deviceID) != "" {
			item["device_id"] = deviceID
		}
		items = append(items, item)
	}

	ok = true
	return c.JSON(fiber.Map{"items": items})
}

func (r *Router) handleMobileIngest(c *fiber.Ctx) error {
	start := time.Now()
	ok := false
	defer func() { recordMobileEndpoint("ingest", time.Since(start), !ok) }()

	if r.pg == nil || r.pg.Pool == nil {
		return WriteAPIError(c, fiber.StatusInternalServerError, "mobile_ingest_unavailable", "repository unavailable", nil)
	}
	if strings.TrimSpace(c.Get("Idempotency-Key")) == "" {
		return WriteAPIError(c, fiber.StatusBadRequest, "mobile_idempotency_required", "Idempotency-Key header is required", nil)
	}

	var req struct {
		ProjectID string               `json:"project_id"`
		DeviceID  string               `json:"device_id"`
		Packets   []mobileIngestPacket `json:"packets"`
	}
	if err := c.BodyParser(&req); err != nil {
		return WriteAPIError(c, fiber.StatusBadRequest, "mobile_invalid_json", "Invalid JSON", nil)
	}
	if strings.TrimSpace(req.ProjectID) == "" || strings.TrimSpace(req.DeviceID) == "" || len(req.Packets) == 0 {
		return WriteAPIError(c, fiber.StatusBadRequest, "mobile_invalid_request", "project_id, device_id, and packets are required", nil)
	}

	allowedTopics := map[string]struct{}{
		"heartbeat": {},
		"data":      {},
		"daq":       {},
		"ondemand":  {},
		"errors":    {},
	}

	accepted := 0
	duplicates := 0
	rejected := 0
	results := make([]fiber.Map, 0, len(req.Packets))

	for _, packet := range req.Packets {
		key := strings.TrimSpace(packet.IdempotencyKey)
		topic := strings.TrimSpace(packet.TopicSuffix)
		if key == "" || topic == "" || packet.RawPayload == nil {
			rejected++
			results = append(results, fiber.Map{"idempotency_key": packet.IdempotencyKey, "status": "rejected", "reason": "invalid_packet"})
			continue
		}
		if _, ok := allowedTopics[topic]; !ok {
			rejected++
			results = append(results, fiber.Map{"idempotency_key": packet.IdempotencyKey, "status": "rejected", "reason": "unsupported_topic_suffix"})
			continue
		}

		payloadBytes, err := json.Marshal(packet.RawPayload)
		if err != nil {
			rejected++
			results = append(results, fiber.Map{"idempotency_key": packet.IdempotencyKey, "status": "rejected", "reason": "invalid_payload"})
			continue
		}
		requestHash := sha256.Sum256(payloadBytes)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		var existingPayload []byte
		err = r.pg.Pool.QueryRow(ctx, `
			SELECT response_payload
			FROM mobile_ingest_dedupe
			WHERE project_id = $1 AND device_id = $2 AND idempotency_key = $3
		`, req.ProjectID, req.DeviceID, key).Scan(&existingPayload)
		cancel()
		if err == nil {
			duplicates++
			prior := fiber.Map{}
			if len(existingPayload) > 0 {
				_ = json.Unmarshal(existingPayload, &prior)
			}
			results = append(results, fiber.Map{
				"idempotency_key": key,
				"status":          "duplicate",
				"prior_result":    prior,
			})
			continue
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			rejected++
			results = append(results, fiber.Map{"idempotency_key": key, "status": "rejected", "reason": "dedupe_read_failed"})
			continue
		}

		resultPayload := fiber.Map{"status": "accepted", "topic_suffix": topic}
		statusCode := 200
		if r.ingest != nil {
			if err := r.ingest.ProcessPacket("mobile/"+topic, payloadBytes, req.ProjectID); err != nil {
				resultPayload = fiber.Map{"status": "rejected", "reason": "ingest_failed", "topic_suffix": topic}
				statusCode = 400
			}
		}
		resultBytes, _ := json.Marshal(resultPayload)

		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		var insertedID int64
		err = r.pg.Pool.QueryRow(ctx, `
			INSERT INTO mobile_ingest_dedupe
			(project_id, device_id, idempotency_key, request_hash, status_code, response_payload, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6::jsonb, NOW() + INTERVAL '7 days')
			ON CONFLICT (project_id, device_id, idempotency_key) DO NOTHING
			RETURNING id
		`, req.ProjectID, req.DeviceID, key, hex.EncodeToString(requestHash[:]), statusCode, string(resultBytes)).Scan(&insertedID)
		cancel()
		if err != nil {
			if err == pgx.ErrNoRows {
				duplicates++
				results = append(results, fiber.Map{"idempotency_key": key, "status": "duplicate", "prior_result": resultPayload})
				continue
			}
			rejected++
			results = append(results, fiber.Map{"idempotency_key": key, "status": "rejected", "reason": "dedupe_write_failed"})
			continue
		}

		_ = insertedID
		if statusCode == 200 {
			accepted++
			results = append(results, fiber.Map{"idempotency_key": key, "status": "accepted"})
		} else {
			rejected++
			reason, _ := resultPayload["reason"].(string)
			if strings.TrimSpace(reason) == "" {
				reason = "ingest_failed"
			}
			results = append(results, fiber.Map{"idempotency_key": key, "status": "rejected", "reason": reason})
		}
	}

	addMobileIngestAccepted(accepted)
	addMobileIngestDuplicate(duplicates)
	addMobileIngestRejected(rejected)

	ok = true
	return c.JSON(fiber.Map{
		"accepted":   accepted,
		"duplicates": duplicates,
		"rejected":   rejected,
		"results":    results,
	})
}

func mapMobileCommandStatus(status string) string {
	s := strings.ToLower(strings.TrimSpace(status))
	switch s {
	case "queued":
		return "queued"
	case "published", "sent":
		return "sent"
	case "acked", "ack", "completed", "success", "done":
		return "acked"
	case "timeout", "timed_out":
		return "timed_out"
	case "failed", "error", "rejected":
		return "failed"
	default:
		return "queued"
	}
}

func (r *Router) handleMobileCommandStatus(c *fiber.Ctx) error {
	start := time.Now()
	ok := false
	defer func() { recordMobileEndpoint("command_status", time.Since(start), !ok) }()

	incMobileCommandStatus()
	if r.pg == nil || r.pg.Pool == nil {
		return WriteAPIError(c, fiber.StatusInternalServerError, "mobile_command_status_unavailable", "repository unavailable", nil)
	}

	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		return WriteAPIError(c, fiber.StatusBadRequest, "mobile_command_id_required", "command id is required", nil)
	}

	rec, err := r.pg.GetCommandRequestByCorrelation(id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return WriteAPIError(c, fiber.StatusNotFound, "mobile_command_not_found", "command not found", nil)
		}
		return WriteAPIError(c, fiber.StatusInternalServerError, "mobile_command_status_failed", err.Error(), nil)
	}

	var updatedAt time.Time
	if rec.CompletedAt != nil {
		updatedAt = *rec.CompletedAt
	} else if rec.PublishedAt != nil {
		updatedAt = *rec.PublishedAt
	} else {
		updatedAt = rec.CreatedAt
	}

	ok = true
	return c.JSON(fiber.Map{
		"id":         id,
		"status":     mapMobileCommandStatus(rec.Status),
		"updated_at": updatedAt,
	})
}
