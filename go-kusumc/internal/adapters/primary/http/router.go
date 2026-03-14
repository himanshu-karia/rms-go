package http

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"ingestion-go/internal/adapters/secondary"
	"ingestion-go/internal/core/services"
	"ingestion-go/internal/models"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

type Router struct {
	auth      *services.AuthService
	project   *services.ProjectService
	bootstrap *services.BootstrapService
	protocols *services.ProtocolService
	govtCreds govtCredsAPI
	bulk      *services.BulkService
	report    *ReportController
	metrics   *MetricsController
	reverify  *services.ReverificationService
	repo      routerRepo
	pg        *secondary.PostgresRepo
	ingest    mobileIngestProcessor
}

type mobileIngestProcessor interface {
	ProcessPacket(topic string, payload []byte, projectID string) error
}

type govtCredsAPI interface {
	Upsert(ctx context.Context, deviceID, protocolID, clientID, username, password string, metadata map[string]any) (models.GovtCredentialBundle, error)
	ListByDevice(ctx context.Context, deviceID string) ([]models.GovtCredentialBundle, error)
	BulkUpsert(ctx context.Context, bundles []models.GovtCredentialBundle) ([]models.GovtCredentialBundle, error)
}

type routerRepo interface {
	GetDeviceByIDOrIMEI(idOrIMEI string) (map[string]interface{}, error)
	CreateImportJob(jobType, projectID string, total, success, errorCount int, errors []map[string]any) (map[string]any, error)
}

func NewRouter(auth *services.AuthService, proj *services.ProjectService, boot *services.BootstrapService, proto *services.ProtocolService, govt *services.GovtCredsService, bulk *services.BulkService, report *ReportController, metrics *MetricsController, reverify *services.ReverificationService, repo *secondary.PostgresRepo, ingest ...mobileIngestProcessor) *Router {
	var proc mobileIngestProcessor
	if len(ingest) > 0 {
		proc = ingest[0]
	}
	return &Router{auth: auth, project: proj, bootstrap: boot, protocols: proto, govtCreds: govt, bulk: bulk, report: report, metrics: metrics, reverify: reverify, repo: repo, pg: repo, ingest: proc}
}

// projectScope reads project_id injected by ApiKey middleware (single-project keys).
func projectScope(c *fiber.Ctx) string {
	if pid, ok := c.Locals("project_id").(string); ok {
		return strings.TrimSpace(pid)
	}
	return ""
}

// RegisterRoutes wires public auth endpoints on public, and protected CRUD/device endpoints on protected.
func (r *Router) RegisterRoutes(public fiber.Router, protected fiber.Router) {
	bootMax := 10
	if raw := strings.TrimSpace(os.Getenv("BOOTSTRAP_RATE_LIMIT_MAX")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			bootMax = v
		}
	}
	bootWindow := 30 * time.Second
	if raw := strings.TrimSpace(os.Getenv("BOOTSTRAP_RATE_LIMIT_WINDOW_SEC")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			bootWindow = time.Duration(v) * time.Second
		}
	}
	bootLimiter := limiter.New(limiter.Config{Max: bootMax, Expiration: bootWindow})

	// Auth (public)
	public.Post("/auth/login", r.handleLogin)
	public.Post("/auth/google", r.handleGoogleLogin)
	public.Post("/auth/register", r.handleRegister)
	public.Post("/auth/refresh", r.handleRefresh)
	public.Post("/auth/logout", r.handleLogout)
	public.Post("/auth/password-reset", r.handlePasswordReset)
	public.Get("/auth/session", r.handleSession)
	public.Get("/auth/me", r.handleSession)
	public.Post("/v1/auth/login", r.handleLogin)
	public.Post("/v1/auth/google", r.handleGoogleLogin)
	public.Post("/v1/auth/register", r.handleRegister)
	public.Post("/v1/auth/refresh", r.handleRefresh)
	public.Post("/v1/auth/logout", r.handleLogout)
	public.Post("/v1/auth/password-reset", r.handlePasswordReset)
	public.Get("/v1/auth/session", r.handleSession)

	// Mobile auth/session APIs (additive namespace)
	mobilePublic := public.Group("/mobile", RequestIDMiddleware())
	mobilePublic.Post("/auth/request-otp", r.handleMobileRequestOTP)
	mobilePublic.Get("/auth/dev-otp/latest", r.handleMobileLatestOTPForInternalTests)
	mobilePublic.Post("/auth/verify", r.handleMobileVerifyOTP)
	mobilePublic.Post("/auth/refresh", r.handleMobileRefresh)

	mobileProtected := public.Group("/mobile", RequestIDMiddleware(), AuthMiddleware(r.auth), AuditMiddleware(r.pg))
	mobileProtected.Post("/auth/logout", r.handleMobileLogout)
	mobileProtected.Get("/me/assignments", r.handleMobileAssignments)
	mobileProtected.Post("/ingest", r.handleMobileIngest)
	mobileProtected.Get("/commands/:id/status", r.handleMobileCommandStatus)

	// Projects
	protected.Post("/projects", RequireCapability([]string{"hierarchy:manage"}, false), r.handleCreateProject)
	protected.Put("/projects/:id", RequireCapability([]string{"hierarchy:manage"}, false), r.handleUpdateProject)
	protected.Get("/projects", RequireCapability([]string{"hierarchy:manage"}, false), r.handleListProjects)
	protected.Get("/projects/:id", RequireCapability([]string{"hierarchy:manage"}, false), r.handleGetProject)
	protected.Delete("/projects/:id", RequireCapability([]string{"hierarchy:manage"}, false), r.handleDeleteProject)
	protected.Post("/admin/projects", RequireCapability([]string{"hierarchy:manage"}, false), r.handleCreateProject)
	protected.Put("/admin/projects/:id", RequireCapability([]string{"hierarchy:manage"}, false), r.handleUpdateProject)
	protected.Patch("/admin/projects/:id", RequireCapability([]string{"hierarchy:manage"}, false), r.handleUpdateProject)
	protected.Get("/admin/projects", RequireCapability([]string{"hierarchy:manage"}, false), r.handleListProjects)
	protected.Get("/admin/projects/:id", RequireCapability([]string{"hierarchy:manage"}, false), r.handleGetProject)
	protected.Delete("/admin/projects/:id", RequireCapability([]string{"hierarchy:manage"}, false), r.handleDeleteProject)
	protected.Get("/lookup/projects", RequireCapability([]string{"hierarchy:manage"}, false), r.handleListProjects)

	// Protocol profiles
	protected.Post("/projects/:id/protocols", RequireCapability([]string{"catalog:protocols"}, false), r.handleCreateProtocol)
	protected.Get("/projects/:id/protocols", RequireCapability([]string{"catalog:protocols"}, false), r.handleListProtocols)
	protected.Delete("/protocols/:id", RequireCapability([]string{"catalog:protocols"}, false), r.handleDeleteProtocol)

	// Govt credentials (per device)
	protected.Post("/devices/:id/govt-creds", RequireCapability([]string{"devices:credentials"}, false), r.handleUpsertGovtCreds)
	protected.Get("/devices/:id/govt-creds", RequireCapability([]string{"devices:credentials"}, false), r.handleListGovtCreds)
	protected.Post("/devices/govt-creds/bulk", RequireCapability([]string{"devices:credentials"}, false), r.handleBulkUpsertGovtCreds)
	protected.Post("/devices/government-credentials/import", RequireCapability([]string{"devices:credentials"}, false), r.handleBulkUpsertGovtCreds)
	protected.Post("/v1/devices/government-credentials/import", RequireCapability([]string{"devices:credentials"}, false), r.handleBulkUpsertGovtCreds)
	protected.Put("/devices/:id/government-credentials", RequireCapability([]string{"devices:credentials"}, false), r.handleUpsertGovtCreds)
	protected.Put("/v1/devices/:id/government-credentials", RequireCapability([]string{"devices:credentials"}, false), r.handleUpsertGovtCreds)
	protected.Get("/devices/:id/government-credentials/history", RequireCapability([]string{"devices:credentials"}, false), r.handleListGovtCredsHistory)
	protected.Get("/v1/devices/:id/government-credentials/history", RequireCapability([]string{"devices:credentials"}, false), r.handleListGovtCredsHistory)
	protected.Post("/devices/import", RequireCapability([]string{"devices:bulk_import"}, false), r.handleImportDevices)

	// Device bootstrap (guarded via ApiKey middleware)
	protected.Get("/bootstrap", bootLimiter, r.handleBootstrap)

	// Reports
	protected.Get("/reports/:id", RequireCapability([]string{"reports:manage"}, false), r.report.GenerateReport)

	// Reverification (dev-only trigger)
	protected.Post("/reverify/:projectId", RequireCapability([]string{"diagnostics:commands"}, false), r.handleReverifyProject)
}

// --- Handlers ---

func (r *Router) handleLogin(c *fiber.Ctx) error {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return WriteAPIError(c, fiber.StatusBadRequest, "auth_invalid_json", "Invalid JSON", nil)
	}

	ip := c.IP()
	ua := c.Get("User-Agent")
	user, tokens, err := r.auth.LoginWithSession(req.Username, req.Password, &ip, &ua)
	if err != nil {
		return WriteAPIError(c, fiber.StatusUnauthorized, "auth_login_failed", err.Error(), nil)
	}
	return c.JSON(fiber.Map{
		"id":      user.ID,
		"name":    user.Username,
		"email":   user.Username,
		"role":    user.Role,
		"token":   tokens.AccessToken,
		"session": fiber.Map{"id": tokens.SessionID},
		"refresh": fiber.Map{"token": tokens.RefreshToken, "expires_at": tokens.RefreshExpiresAt},
		"access":  fiber.Map{"expires_at": tokens.AccessExpiresAt},
	})
}

// Reverify suspicious telemetry for a project (developer-use).
func (r *Router) handleReverifyProject(c *fiber.Ctx) error {
	projectID := c.Params("projectId")
	if projectID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "project_id required"})
	}
	if r.reverify == nil {
		return c.Status(500).JSON(fiber.Map{"error": "reverification service unavailable"})
	}
	if token := os.Getenv("REVERIFY_TOKEN"); token != "" {
		reqTok := c.Get("X-Dev-Token")
		if reqTok == "" || reqTok != token {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}
	}
	go r.reverify.ReverifyProject(projectID)
	return c.JSON(fiber.Map{"status": "started", "project_id": projectID})
}

func (r *Router) handleGoogleLogin(c *fiber.Ctx) error {
	var req struct {
		Token string `json:"token"`
	}
	if err := c.BodyParser(&req); err != nil {
		return WriteAPIError(c, fiber.StatusBadRequest, "auth_invalid_json", "Invalid JSON", nil)
	}

	token, err := r.auth.LoginWithGoogle(c.Context(), req.Token)
	if err != nil {
		return WriteAPIError(c, fiber.StatusUnauthorized, "auth_google_login_failed", err.Error(), nil)
	}
	return c.JSON(fiber.Map{"token": token})
}

func (r *Router) handleRefresh(c *fiber.Ctx) error {
	var req struct {
		RefreshToken      string `json:"refresh_token"`
		RefreshTokenCamel string `json:"refreshToken"`
	}
	if err := c.BodyParser(&req); err != nil {
		return WriteAPIError(c, fiber.StatusBadRequest, "auth_invalid_json", "Invalid JSON", nil)
	}
	if strings.TrimSpace(req.RefreshToken) == "" {
		req.RefreshToken = req.RefreshTokenCamel
	}
	ip := c.IP()
	ua := c.Get("User-Agent")
	user, tokens, err := r.auth.RefreshSession(req.RefreshToken, &ip, &ua)
	if err != nil {
		return WriteAPIError(c, fiber.StatusUnauthorized, "auth_refresh_failed", err.Error(), nil)
	}
	return c.JSON(fiber.Map{
		"id":      user.ID,
		"name":    user.Username,
		"email":   user.Username,
		"role":    user.Role,
		"token":   tokens.AccessToken,
		"session": fiber.Map{"id": tokens.SessionID},
		"refresh": fiber.Map{"token": tokens.RefreshToken, "expires_at": tokens.RefreshExpiresAt},
		"access":  fiber.Map{"expires_at": tokens.AccessExpiresAt},
	})
}

func (r *Router) handleLogout(c *fiber.Ctx) error {
	claims, err := r.auth.ValidateToken(extractBearer(c))
	if err != nil {
		return c.Status(200).JSON(fiber.Map{"success": true})
	}
	if sessionID, ok := claims["session_id"].(string); ok {
		_ = r.auth.LogoutSession(sessionID)
	}
	return c.Status(200).JSON(fiber.Map{"success": true})
}

func (r *Router) handleSession(c *fiber.Ctx) error {
	claims, err := r.auth.ValidateToken(extractBearer(c))
	if err != nil {
		return WriteAPIError(c, fiber.StatusUnauthorized, "auth_invalid_token", "invalid token", nil)
	}
	userID, _ := claims["id"].(string)
	user, err := r.auth.GetUserByID(userID)
	if err != nil || user == nil {
		return WriteAPIError(c, fiber.StatusNotFound, "auth_user_not_found", "user not found", nil)
	}
	return c.JSON(fiber.Map{
		"id":    user.ID,
		"name":  user.Username,
		"email": user.Username,
		"role":  user.Role,
	})
}

func (r *Router) handlePasswordReset(c *fiber.Ctx) error {
	var req struct {
		Username             string `json:"username"`
		CurrentPassword      string `json:"current_password"`
		CurrentPasswordCamel string `json:"currentPassword"`
		NewPassword          string `json:"new_password"`
		NewPasswordCamel     string `json:"newPassword"`
	}
	if err := c.BodyParser(&req); err != nil {
		return WriteAPIError(c, fiber.StatusBadRequest, "auth_invalid_json", "Invalid JSON", nil)
	}
	if strings.TrimSpace(req.CurrentPassword) == "" {
		req.CurrentPassword = req.CurrentPasswordCamel
	}
	if strings.TrimSpace(req.NewPassword) == "" {
		req.NewPassword = req.NewPasswordCamel
	}
	if err := r.auth.ResetPassword(req.Username, req.CurrentPassword, req.NewPassword); err != nil {
		return WriteAPIError(c, fiber.StatusUnauthorized, "auth_password_reset_failed", err.Error(), nil)
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

func extractBearer(c *fiber.Ctx) string {
	authHeader := c.Get("Authorization")
	parts := strings.Split(authHeader, " ")
	if len(parts) == 2 && parts[0] == "Bearer" {
		return parts[1]
	}
	return ""
}

func (r *Router) handleRegister(c *fiber.Ctx) error {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := c.BodyParser(&req); err != nil {
		return WriteAPIError(c, fiber.StatusBadRequest, "auth_invalid_json", "Bad Request", nil)
	}
	ip := c.IP()
	ua := c.Get("User-Agent")
	user, tokens, err := r.auth.RegisterWithSession(req.Username, req.Password, req.Role, &ip, &ua)
	if err != nil {
		return WriteAPIError(c, fiber.StatusInternalServerError, "auth_register_failed", err.Error(), nil)
	}
	return c.Status(201).JSON(fiber.Map{
		"id":      user.ID,
		"name":    user.Username,
		"email":   user.Username,
		"role":    user.Role,
		"token":   tokens.AccessToken,
		"session": fiber.Map{"id": tokens.SessionID},
		"refresh": fiber.Map{"token": tokens.RefreshToken, "expires_at": tokens.RefreshExpiresAt},
		"access":  fiber.Map{"expires_at": tokens.AccessExpiresAt},
	})
}

func (r *Router) handleCreateProject(c *fiber.Ctx) error {
	if scoped := projectScope(c); scoped != "" {
		// Single-project ApiKey cannot create arbitrary projects
		return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
	}

	var req struct {
		ID         string      `json:"id"`
		Name       string      `json:"name"`
		Type       string      `json:"type"`
		Location   string      `json:"location"`
		Config     interface{} `json:"config"`
		OwnerOrgID string      `json:"owner_org_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).SendString("Bad Request")
	}

	// Basic validation
	id := strings.TrimSpace(req.ID)
	name := strings.TrimSpace(req.Name)
	projType := strings.TrimSpace(req.Type)
	location := strings.TrimSpace(req.Location)
	ownerOrgID := strings.TrimSpace(req.OwnerOrgID)
	if id == "" || name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "id and name are required"})
	}

	// Fold extra fields into config for backward compatibility
	cfg := req.Config
	if cfg == nil {
		cfg = map[string]interface{}{}
	}
	if m, ok := cfg.(map[string]interface{}); ok {
		if ownerOrgID != "" {
			m["owner_org_id"] = ownerOrgID
		}
		if projType != "" {
			m["type"] = projType
		}
		if location != "" {
			m["location"] = location
		}
		cfg = m
	}

	err := r.project.CreateProject(id, name, projType, location, cfg)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.Status(201).JSON(fiber.Map{
		"id":           id,
		"name":         name,
		"type":         projType,
		"location":     location,
		"config":       cfg,
		"owner_org_id": ownerOrgID,
	})
}

func (r *Router) handleUpdateProject(c *fiber.Ctx) error {
	id := c.Params("id")
	if scoped := projectScope(c); scoped != "" && scoped != id {
		return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
	}
	var req struct {
		Name   string      `json:"name"`
		Config interface{} `json:"config"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).SendString("Bad Request")
	}

	err := r.project.UpdateProject(id, req.Config)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.SendString("Updated")
}

func (r *Router) handleGetProject(c *fiber.Ctx) error {
	id := c.Params("id")
	if scoped := projectScope(c); scoped != "" && scoped != id {
		return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
	}
	p, err := r.project.GetProject(id)
	if err != nil {
		return c.Status(404).SendString("Not Found")
	}
	return c.JSON(p)
}

func (r *Router) handleListProjects(c *fiber.Ctx) error {
	if strings.Contains(c.Path(), "/lookup/") {
		stateID := strings.TrimSpace(c.Query("stateId"))
		authorityID := strings.TrimSpace(c.Query("stateAuthorityId"))
		if stateID == "" {
			return validationError(c, "Invalid query parameters", "stateId", "stateId is required")
		}
		if authorityID == "" {
			return validationError(c, "Invalid query parameters", "stateAuthorityId", "stateAuthorityId is required")
		}
	}
	if scoped := projectScope(c); scoped != "" {
		p, err := r.project.GetProject(scoped)
		if err != nil {
			return c.Status(404).SendString("Not Found")
		}
		return c.JSON([]interface{}{p})
	}
	projects, err := r.project.ListProjects()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(projects)
}

func (r *Router) handleDeleteProject(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "project id required"})
	}
	if scoped := projectScope(c); scoped != "" && scoped != id {
		return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
	}
	if err := r.project.DeleteProject(id); err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.SendStatus(204)
}

// --- Protocols ---

func (r *Router) handleCreateProtocol(c *fiber.Ctx) error {
	projectID := c.Params("id")
	var req struct {
		Kind            string         `json:"kind"`
		Protocol        string         `json:"protocol"`
		Host            string         `json:"host"`
		Port            int            `json:"port"`
		PublishTopics   []string       `json:"publish_topics"`
		SubscribeTopics []string       `json:"subscribe_topics"`
		ServerVendorOrg string         `json:"server_vendor_org_id"`
		Metadata        map[string]any `json:"metadata"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).SendString("Bad Request")
	}
	p, err := r.protocols.Create(c.Context(), projectID, req.Kind, req.Protocol, req.Host, req.Port, req.PublishTopics, req.SubscribeTopics, req.ServerVendorOrg, req.Metadata)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.Status(201).JSON(p)
}

func (r *Router) handleListProtocols(c *fiber.Ctx) error {
	projectID := c.Params("id")
	list, err := r.protocols.ListByProject(c.Context(), projectID)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(list)
}

func (r *Router) handleDeleteProtocol(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := r.protocols.Delete(c.Context(), id); err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.SendStatus(204)
}

// --- Govt credentials ---

func markGovtCredsLegacyPath(c *fiber.Ctx) {
	path := strings.ToLower(c.Path())
	if strings.Contains(path, "/govt-creds") {
		c.Set("Deprecation", "true")
		c.Set("Sunset", "2026-12-31")
		c.Set("Link", `</api/devices/{id}/government-credentials>; rel="successor-version"`)
	}
}

func (r *Router) handleUpsertGovtCreds(c *fiber.Ctx) error {
	markGovtCredsLegacyPath(c)
	deviceID := c.Params("id")
	var req struct {
		ProtocolID string         `json:"protocol_id"`
		ClientID   string         `json:"client_id"`
		Username   string         `json:"username"`
		Password   string         `json:"password"`
		Metadata   map[string]any `json:"metadata"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).SendString("Bad Request")
	}
	res, err := r.govtCreds.Upsert(c.Context(), deviceID, req.ProtocolID, req.ClientID, req.Username, req.Password, req.Metadata)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(res)
}

func (r *Router) handleListGovtCreds(c *fiber.Ctx) error {
	markGovtCredsLegacyPath(c)
	deviceID := c.Params("id")
	res, err := r.govtCreds.ListByDevice(c.Context(), deviceID)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(res)
}

func (r *Router) handleListGovtCredsHistory(c *fiber.Ctx) error {
	deviceID := c.Params("id")
	if deviceID == "" {
		return c.Status(400).JSON(fiber.Map{"message": "Invalid device identifier provided"})
	}
	res, err := r.govtCreds.ListByDevice(c.Context(), deviceID)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	device, _ := r.repo.GetDeviceByIDOrIMEI(deviceID)
	devicePayload := fiber.Map{"uuid": deviceID, "imei": nil}
	if device != nil {
		devicePayload["uuid"] = device["id"]
		devicePayload["imei"] = device["imei"]
	}
	return c.JSON(fiber.Map{
		"device":      devicePayload,
		"assignments": res,
		"nextCursor":  nil,
	})
}

func (r *Router) handleBulkUpsertGovtCreds(c *fiber.Ctx) error {
	markGovtCredsLegacyPath(c)
	if r.govtCreds == nil {
		return c.Status(500).SendString("govt credentials service unavailable")
	}

	trimmed := strings.TrimSpace(string(c.Body()))
	if trimmed == "" {
		return c.Status(400).SendString("payload cannot be empty")
	}

	if strings.HasPrefix(trimmed, "[") {
		var req []struct {
			DeviceID   string         `json:"device_id"`
			ProtocolID string         `json:"protocol_id"`
			ClientID   string         `json:"client_id"`
			Username   string         `json:"username"`
			Password   string         `json:"password"`
			Metadata   map[string]any `json:"metadata"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).SendString("Bad Request")
		}
		if len(req) == 0 {
			return c.Status(400).SendString("payload cannot be empty")
		}
		bundles := make([]models.GovtCredentialBundle, 0, len(req))
		for _, rbody := range req {
			if rbody.DeviceID == "" || rbody.ProtocolID == "" {
				return c.Status(400).SendString("device_id and protocol_id are required")
			}
			bundles = append(bundles, models.GovtCredentialBundle{
				DeviceID:   rbody.DeviceID,
				ProtocolID: rbody.ProtocolID,
				ClientID:   rbody.ClientID,
				Username:   rbody.Username,
				Password:   rbody.Password,
				Metadata:   rbody.Metadata,
			})
		}
		res, err := r.govtCreds.BulkUpsert(c.Context(), bundles)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}
		if jobID := r.createGovtImportJob(c, len(res), len(res), 0, []map[string]any{}); jobID != "" {
			c.Set("X-Import-Job-Id", jobID)
		}
		return c.Status(201).JSON(res)
	}

	csvBody := extractCSVImportBody(c.Body())
	bundles, rowErrors := r.parseGovtCredBundlesFromCSV(csvBody)
	if len(bundles) == 0 {
		resp := fiber.Map{
			"success_count": 0,
			"error_count":   len(rowErrors),
			"errors":        rowErrors,
		}
		if len(rowErrors) == 0 {
			resp["errors"] = []string{"payload cannot be empty"}
			resp["error_count"] = 1
			rowErrors = []string{"payload cannot be empty"}
		}
		if r.repo != nil {
			jobErrors := make([]map[string]any, 0, len(rowErrors))
			for _, msg := range rowErrors {
				jobErrors = append(jobErrors, map[string]any{"error": msg})
			}
			if jobID := r.createGovtImportJob(c, len(rowErrors), 0, len(rowErrors), jobErrors); jobID != "" {
				resp["job_id"] = jobID
				c.Set("X-Import-Job-Id", jobID)
			}
		}
		return c.Status(400).JSON(resp)
	}

	_, err := r.govtCreds.BulkUpsert(c.Context(), bundles)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}

	status := 200
	if len(rowErrors) > 0 {
		status = 207
	}
	resp := fiber.Map{
		"success_count": len(bundles),
		"error_count":   len(rowErrors),
		"errors":        rowErrors,
	}

	if r.repo != nil {
		jobErrors := make([]map[string]any, 0, len(rowErrors))
		for _, msg := range rowErrors {
			jobErrors = append(jobErrors, map[string]any{"error": msg})
		}
		if jobID := r.createGovtImportJob(c, len(bundles)+len(rowErrors), len(bundles), len(rowErrors), jobErrors); jobID != "" {
			resp["job_id"] = jobID
			c.Set("X-Import-Job-Id", jobID)
		}
	}

	return c.Status(status).JSON(resp)
}

func (r *Router) createGovtImportJob(c *fiber.Ctx, total, success, errorCount int, errors []map[string]any) string {
	if r.repo == nil {
		return ""
	}
	job, err := r.repo.CreateImportJob("government_credentials_import", projectScope(c), total, success, errorCount, errors)
	if err != nil || job == nil {
		return ""
	}
	return importJobID(job)
}

func importJobID(job map[string]any) string {
	if job == nil {
		return ""
	}
	id, ok := job["id"]
	if !ok {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", id))
}

func (r *Router) parseGovtCredBundlesFromCSV(csvBody []byte) ([]models.GovtCredentialBundle, []string) {
	reader := csv.NewReader(strings.NewReader(string(csvBody)))
	head, err := reader.Read()
	if err != nil {
		return nil, []string{"invalid csv"}
	}

	colIdx := map[string]int{}
	for i, h := range head {
		colIdx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	get := func(row []string, keys ...string) string {
		for _, key := range keys {
			if idx, ok := colIdx[key]; ok && idx < len(row) {
				if value := strings.TrimSpace(row[idx]); value != "" {
					return value
				}
			}
		}
		return ""
	}

	bundles := make([]models.GovtCredentialBundle, 0)
	errors := make([]string, 0)
	rowNum := 1

	for {
		row, rowErr := reader.Read()
		if rowErr != nil {
			break
		}
		rowNum++

		deviceRef := get(row, "device_id", "device_uuid", "imei")
		protocolID := get(row, "protocol_id", "govt_protocol_id")
		clientID := get(row, "client_id", "govt_client_id")
		username := get(row, "username", "govt_username")
		password := get(row, "password", "govt_password")

		if deviceRef == "" {
			errors = append(errors, fmt.Sprintf("row %d: device_id/device_uuid/imei required", rowNum))
			continue
		}
		if protocolID == "" {
			errors = append(errors, fmt.Sprintf("row %d (%s): protocol_id required", rowNum, deviceRef))
			continue
		}

		resolvedDeviceID := deviceRef
		if r.repo != nil {
			device, err := r.repo.GetDeviceByIDOrIMEI(deviceRef)
			if err != nil || device == nil {
				errors = append(errors, fmt.Sprintf("row %d (%s): device not found", rowNum, deviceRef))
				continue
			}
			if deviceID, ok := device["id"].(string); ok && strings.TrimSpace(deviceID) != "" {
				resolvedDeviceID = deviceID
			}
		}

		bundles = append(bundles, models.GovtCredentialBundle{
			DeviceID:   resolvedDeviceID,
			ProtocolID: protocolID,
			ClientID:   clientID,
			Username:   username,
			Password:   password,
		})
	}

	return bundles, errors
}

func (r *Router) handleBootstrap(c *fiber.Ctx) error {
	if method := c.Locals("auth_method"); method != "api_key" {
		return c.Status(401).SendString("api key required")
	}

	imei := c.Query("imei")
	if imei == "" {
		return c.Status(400).SendString("Missing IMEI")
	}

	cfg, err := r.bootstrap.GetDeviceConfig(imei)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}

	projectID := ""
	if pid := c.Locals("project_id"); pid != nil {
		if v, ok := pid.(string); ok {
			projectID = v
		}
	}

	if projectID != "" && r.auth != nil {
		if token, err := r.auth.IssueDeviceToken(projectID, imei, time.Hour); err == nil {
			c.Set("X-Device-JWT", token)
		}
	}
	return c.JSON(cfg)
}

func (r *Router) handleImportDevices(c *fiber.Ctx) error {
	if r.bulk == nil {
		return c.Status(500).SendString("bulk service unavailable")
	}
	projectID := strings.TrimSpace(c.Query("project_id"))
	if projectID == "" {
		projectID = strings.TrimSpace(c.Query("projectId"))
	}
	reader := bytes.NewReader(extractCSVImportBody(c.Body()))
	success, errs := r.bulk.ImportDevices(reader, projectID)
	resp := fiber.Map{
		"success_count": success,
		"error_count":   len(errs),
	}
	if len(errs) > 0 {
		messages := make([]string, 0, len(errs))
		for _, e := range errs {
			messages = append(messages, e.Error())
		}
		resp["errors"] = messages
	}
	status := 200
	if len(errs) > 0 {
		status = 207 // Multi-Status style for partial success
	}
	return c.Status(status).JSON(resp)
}

func extractCSVImportBody(body []byte) []byte {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" || !strings.HasPrefix(trimmed, "{") {
		return body
	}

	var legacy struct {
		CSV string `json:"csv"`
	}
	if err := json.Unmarshal(body, &legacy); err != nil {
		return body
	}
	if strings.TrimSpace(legacy.CSV) == "" {
		return body
	}
	return []byte(legacy.CSV)
}
