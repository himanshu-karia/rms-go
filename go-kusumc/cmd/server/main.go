package main

import (
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"ingestion-go/internal/adapters/primary"
	api "ingestion-go/internal/adapters/primary/http"
	"ingestion-go/internal/adapters/secondary"
	"ingestion-go/internal/adapters/secondary/storage"
	"ingestion-go/internal/core/services"
	"ingestion-go/internal/core/workers"
	"ingestion-go/internal/pkg/logger"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/google/uuid"
)

func main() {
	log.Println("⚡ Starting Unified IoT Engine V1.0 (Go)...")

	// 1. Config
	redisUrl := os.Getenv("REDIS_URL")
	pgUrl := os.Getenv("TIMESCALE_URI")

	// 2. Adapters (Secondary)
	redisStore := secondary.NewRedisStore(redisUrl)
	pgRepo, err := secondary.NewPostgresRepo(pgUrl)
	if err != nil {
		log.Fatal("Failed to connect to Postgres:", err)
	}

	// 3. Core Repositories (INITIALIZED FIRST)
	userRepo := secondary.NewPostgresUserRepo(pgRepo.Pool)
	projRepo := secondary.NewPostgresProjectRepo(pgRepo.Pool)

	// 4. Infrastructure Services
	// [ROUND 11] Critical Logic Injection: Rules + MQTT (Needs projRepo, redisStore)
	// [ROUND 17] Device Service Wiring (Full Stack Verification)
	// EmqxAdapter reads from ENV: EMQX_API_URL, EMQX_APP_ID, EMQX_APP_SECRET
	emqxAdapter := secondary.NewEmqxAdapter()
	dnaRepo := secondary.NewPostgresDNARepo(pgRepo.Pool)
	deviceService := services.NewDeviceService(pgRepo, redisStore, emqxAdapter, dnaRepo)

	// Commands service is needed both for /api/commands/* and for device configuration apply over MQTT.
	cmdService := services.NewCommandsService(pgRepo, pgRepo)

	deviceController := api.NewDeviceControllerWithCommands(deviceService, pgRepo, cmdService)
	brokerController := api.NewBrokerController(emqxAdapter, deviceService, pgRepo)

	// 4. Infrastructure Services
	// [ROUND 11] Critical Logic Injection: Rules + MQTT (Needs projRepo, redisStore)
	bundleEnabled := strings.EqualFold(os.Getenv("CONFIG_BUNDLE_ENABLED"), "true") || strings.EqualFold(os.Getenv("FEATURE_CONFIG_BUNDLE"), "true")
	if bundleEnabled {
		log.Println("[ConfigSync] Bundle publishing enabled")
	}
	configSyncService := services.NewConfigSyncService(projRepo, redisStore, dnaRepo, pgRepo, pgRepo, bundleEnabled)
	configSyncService.SyncAll()

	rulesService := services.NewRulesService(pgRepo, redisStore, configSyncService)
	// rulesController initialized late

	transformer := services.NewGovaluateTransformer()
	// [FIX] Inject DeviceService + RulesService for alert triggering
	ingestService := services.NewIngestionService(redisStore, pgRepo, pgRepo, transformer, deviceService, rulesService)

	// Core Services
	authSecret := strings.TrimSpace(os.Getenv("AUTH_SECRET"))
	if authSecret == "" {
		log.Fatal("AUTH_SECRET is required and must be set")
	}
	authService := services.NewAuthService(userRepo, authSecret)
	projService := services.NewProjectService(projRepo, redisStore)
	protoRepo := secondary.NewPostgresProtocolRepo(pgRepo.Pool)
	govtRepo := secondary.NewPostgresGovtCredsRepo(pgRepo.Pool)
	vfdRepo := secondary.NewPostgresVFDRepo(pgRepo.Pool)
	protocolService := services.NewProtocolService(protoRepo)
	govtCredsService := services.NewGovtCredsService(govtRepo)
	bootService := services.NewBootstrapService(pgRepo, protocolService, govtCredsService, vfdRepo, dnaRepo)
	bulkService := services.NewBulkService(deviceService, govtCredsService)

	// [PHASE 13] Seeder
	seederService := services.NewSeederService(pgRepo, userRepo, authService)
	seederService.Seed()
	configSyncService.SyncAll()

	// 5. Background Workers
	scheduler := services.NewSchedulerService(pgRepo)
	scheduler.Start()

	offlineMonitor := services.NewOfflineMonitorService(pgRepo, protocolService)
	offlineMonitor.Start()

	notificationProcessor := services.NewNotificationProcessorService(pgRepo)
	notificationProcessor.Start()

	analytics := services.NewAnalyticsWorker(pgRepo)
	analytics.Start()

	mqttWorker := workers.NewMqttWorker(pgRepo, protocolService)
	mqttWorker.Start()
	deadLetterReplayWorker := workers.NewDeadLetterReplayWorker(redisStore, ingestService)
	deadLetterReplayWorker.Start()

	// Reverification (optional state store used for pushing hot cache; nil-safe in service)
	reverifyService := services.NewReverificationService(pgRepo, redisStore)
	reverifyProjectsEnv := strings.TrimSpace(os.Getenv("REVERIFY_PROJECTS"))
	reverifyIntervalMin := 0
	if v := strings.TrimSpace(os.Getenv("REVERIFY_INTERVAL_MINUTES")); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			reverifyIntervalMin = parsed
		}
	}
	if reverifyProjectsEnv != "" && reverifyIntervalMin > 0 {
		projects := []string{}
		for _, p := range strings.Split(reverifyProjectsEnv, ",") {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				projects = append(projects, trimmed)
			}
		}
		if len(projects) > 0 {
			interval := time.Duration(reverifyIntervalMin) * time.Minute
			log.Printf("[Reverify] scheduler enabled: %d min interval for %d project(s)", reverifyIntervalMin, len(projects))
			go func() {
				ticker := time.NewTicker(interval)
				defer ticker.Stop()
				for {
					for _, pid := range projects {
						log.Printf("[Reverify] scheduled run for project %s", pid)
						reverifyService.ReverifyProject(pid)
					}
					<-ticker.C
				}
			}()
		}
	}

	// cmdController initialized late

	// 6. Start MQTT (Async) - Explicit Setup
	mqttHandler := primary.NewMqttHandler(ingestService)
	mqttClient := mqttHandler.SetupClient() // explicit setup
	cmdService.SetMqttClient(mqttClient)
	cmdService.SetHTTPClient(nil, strings.TrimSpace(os.Getenv("COMMAND_HTTP_ENDPOINT")))

	// Command retry worker (republishes queued/published commands)
	cmdRetryWorker := workers.NewCommandRetryWorker(pgRepo, cmdService)
	cmdRetryWorker.Start()

	rulesService.SetMqttClient(mqttClient) // Link Rules -> MQTT
	deviceService.SetMqttClient(mqttClient)

	go mqttHandler.Start() // Will reuse the client

	// 7. Primary Adapters (HTTP)
	app := fiber.New()

	// [SEC] Rate Limiting
	app.Use(limiter.New(limiter.Config{
		Max:        100,
		Expiration: 1 * time.Second,
	}))

	// [SEC] CORS — restrict to frontend origins; device/webhook paths skip CORS
	allowedOrigins := strings.TrimSpace(os.Getenv("FRONTEND_ORIGINS"))
	if allowedOrigins == "" {
		allowedOrigins = strings.TrimSpace(os.Getenv("VITE_URL"))
	}
	if allowedOrigins == "" {
		allowedOrigins = "http://localhost:5173" // dev default; override in prod
	}
	corsMiddleware := cors.New(cors.Config{
		AllowOrigins: allowedOrigins,
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, x-api-key",
		AllowMethods: "GET, POST, HEAD, PUT, DELETE, PATCH",
	})

	// Apply CORS selectively: skip device ingress/bootstrap/northbound
	app.Use(func(c *fiber.Ctx) error {
		path := c.Path()
		if strings.HasPrefix(path, "/api/ingest") || strings.HasPrefix(path, "/api/bootstrap") || strings.HasPrefix(path, "/api/northbound/") {
			return c.Next()
		}
		return corsMiddleware(c)
	})

	// Optional: strict snake_case enforcement for query params and JSON bodies.
	// Enabled via env STRICT_SNAKE_WIRE=true.
	app.Use("/api", api.StrictSnakeWireMiddleware())

	// Services
	configService := services.NewConfigService(pgRepo)

	// Controllers
	dnaService := services.NewDNAService(pgRepo, dnaRepo, configSyncService)
	dnaController := api.NewDNAController(dnaService)
	configController := api.NewConfigController(configService)
	nbController := api.NewNorthboundController(ingestService, configService)

	// Webhook/Northbound routes (guard with secret/IP allowlist)
	chirpstackSecret := strings.TrimSpace(os.Getenv("CHIRPSTACK_WEBHOOK_SECRET"))
	chirpstackIPAllowlist := strings.Split(strings.TrimSpace(os.Getenv("CHIRPSTACK_IP_ALLOWLIST")), ",")
	app.Post("/api/northbound/chirpstack", func(c *fiber.Ctx) error {
		if chirpstackSecret != "" {
			if c.Get("X-Webhook-Secret") != chirpstackSecret {
				return c.Status(403).SendString("forbidden")
			}
		}
		if len(chirpstackIPAllowlist) > 0 && strings.TrimSpace(chirpstackIPAllowlist[0]) != "" {
			clientIP := c.IP()
			allowed := false
			for _, ip := range chirpstackIPAllowlist {
				if strings.TrimSpace(ip) == clientIP {
					allowed = true
					break
				}
			}
			if !allowed {
				return c.Status(403).SendString("forbidden")
			}
		}
		return nbController.HandleChirpStack(c)
	})

	// [PHASE 5] ERP Services
	maintService := services.NewMaintenanceService(pgRepo)
	invService := services.NewInventoryService(pgRepo)
	logiService := services.NewLogisticsService(pgRepo)
	trafService := services.NewTrafficService(pgRepo)
	erpController := api.NewERPController(maintService, invService, logiService, trafService)

	// [PHASE 9] Vertical Domain Services
	verticalService := services.NewVerticalService(pgRepo, vfdRepo)
	verticalController := api.NewVerticalController(verticalService)

	// [PHASE 14] Shadow Service
	shadowService := services.NewShadowService(pgRepo)
	go shadowService.Start()

	// [ROUND 1] OTA Service
	otaService := services.NewOtaService(pgRepo)
	otaController := api.NewOtaController(otaService)

	// [ROUND 1] Archiver Service
	var storageProvider storage.StorageProvider
	storageType := os.Getenv("STORAGE_TYPE")

	if storageType == "S3" {
		storageProvider = storage.NewS3Storage()
		log.Println("📦 Storage Provider: S3 (AWS)")
	} else if storageType == "GCS" {
		var err error
		storageProvider, err = storage.NewGCSStorage()
		if err != nil {
			log.Fatalf("Failed to init GCS: %v", err)
		}
		log.Println("📦 Storage Provider: GCS (Google)")
	} else {
		storageProvider = storage.NewLocalStorage("./cold-storage")
		log.Println("📦 Storage Provider: Local Filesystem")
	}

	archiverService := services.NewArchiverService(pgRepo, projRepo, storageProvider)
	go archiverService.Start()

	// [ROUND 2] Gap Controllers
	cmdController := api.NewCommandsController(cmdService)
	cmdCatalogController := api.NewCommandCatalogController(cmdService)
	meshNodesService := services.NewMeshNodesService(pgRepo)
	meshNodesController := api.NewMeshNodesController(meshNodesService)

	rulesController := api.NewRulesController(rulesService) // Uses initialized rulesService

	// [ROUND 17] Device Service was moved to top

	// [PHASE 10] The Final Mile
	analyticsService := services.NewAnalyticsService(pgRepo, pgRepo, archiverService)
	analyticsController := api.NewAnalyticsController(analyticsService)

	adminService := services.NewAdminService(pgRepo, userRepo)
	adminController := api.NewAdminController(adminService, protocolService)
	auditController := api.NewAuditController(pgRepo)
	orgService := services.NewOrgService(pgRepo)
	orgController := api.NewOrgController(orgService)
	apiKeyService := services.NewApiKeyService(pgRepo)
	apiKeyController := api.NewApiKeyController(apiKeyService)
	userGroupService := services.NewUserGroupService(pgRepo, userRepo)
	userGroupsController := api.NewUserGroupsController(userGroupService)

	simService := services.NewSimulatorService(pgRepo)
	simController := api.NewSimulatorController(simService)
	mirrorController := api.NewTelemetryMirrorController(ingestService, configService, pgRepo)
	httpsTelemetryController := api.NewTelemetryHttpsController(ingestService, configService, pgRepo)
	deviceOpenController := api.NewDeviceOpenController(deviceService, govtCredsService, protocolService, cmdService, rulesService, pgRepo, vfdRepo)

	// Project DNA (canonical sensors/thresholds)
	dnaSpecService := services.NewDnaSpecService(pgRepo, configSyncService)
	dnaSpecController := api.NewDnaSpecController(dnaSpecService)
	telemetryLiveController := api.NewTelemetryLiveController(deviceService, redisStore)
	telemetryThresholdsController := api.NewTelemetryThresholdsController(deviceService, dnaSpecService, pgRepo)

	// [ROUND 6] Report Service
	reportService := services.NewReportService(pgRepo)
	reportController := api.NewReportController(reportService)

	// [SEC] Middleware
	authMiddleware := api.AuthMiddleware(authService)
	auditMiddleware := api.AuditMiddleware(pgRepo)

	// [ROUND 10] Metrics Controller
	metricsController := api.NewMetricsController(reverifyService)
	diagnosticsController := api.NewDeadLetterDiagnosticsController(deadLetterReplayWorker)

	// --- 1. Public & Auth Routes ---
	httpRouter := api.NewRouter(authService, projService, bootService, protocolService, govtCredsService, bulkService, reportController, metricsController, reverifyService, pgRepo, ingestService)

	// --- 2. Southbound (Device) APIs ---
	devOpenLimiter := limiter.New(limiter.Config{Max: 20, Expiration: 30 * time.Second})
	devOpen := app.Group("/api/device-open", devOpenLimiter)
	devOpen.Get("/bootstrap", func(c *fiber.Ctx) error {
		return c.Redirect("/api/bootstrap" + "?" + string(c.Request().URI().QueryString()))
	})
	devOpen.Get("/credentials/local", deviceOpenController.GetLocalCredentials)
	devOpen.Get("/credentials/government", deviceOpenController.GetGovernmentCredentials)
	devOpen.Get("/vfd", deviceOpenController.GetVFDModels)
	devOpen.Get("/commands/history", deviceOpenController.GetCommandHistory)
	devOpen.Get("/commands/responses", deviceOpenController.GetCommandResponses)
	devOpen.Get("/commands/status", deviceOpenController.GetCommandStatus)
	devOpen.Get("/nodes", deviceOpenController.GetMeshNodes)
	devOpen.Get("/installations/:device_uuid", deviceOpenController.GetInstallation)
	devOpen.Post("/errors", deviceOpenController.PostErrors)

	devOpenLegacy := app.Group("/api/devices/open", devOpenLimiter)
	devOpenLegacy.Get("/bootstrap", func(c *fiber.Ctx) error {
		return c.Redirect("/api/bootstrap" + "?" + string(c.Request().URI().QueryString()))
	})
	devOpenLegacy.Get("/credentials/local", deviceOpenController.GetLocalCredentials)
	devOpenLegacy.Get("/credentials/government", deviceOpenController.GetGovernmentCredentials)
	devOpenLegacy.Get("/vfd", deviceOpenController.GetVFDModels)
	devOpenLegacy.Get("/commands/history", deviceOpenController.GetCommandHistory)
	devOpenLegacy.Get("/commands/responses", deviceOpenController.GetCommandResponses)
	devOpenLegacy.Get("/commands/status", deviceOpenController.GetCommandStatus)
	devOpenLegacy.Get("/nodes", deviceOpenController.GetMeshNodes)
	devOpenLegacy.Get("/installations/:device_uuid", deviceOpenController.GetInstallation)
	devOpenLegacy.Post("/errors", deviceOpenController.PostErrors)

	devOpenV1 := app.Group("/api/v1/device-open", devOpenLimiter)
	devOpenV1.Get("/bootstrap", func(c *fiber.Ctx) error {
		return c.Redirect("/api/bootstrap" + "?" + string(c.Request().URI().QueryString()))
	})
	devOpenV1.Get("/credentials/local", deviceOpenController.GetLocalCredentials)
	devOpenV1.Get("/credentials/government", deviceOpenController.GetGovernmentCredentials)
	devOpenV1.Get("/vfd", deviceOpenController.GetVFDModels)
	devOpenV1.Get("/commands/history", deviceOpenController.GetCommandHistory)
	devOpenV1.Get("/commands/responses", deviceOpenController.GetCommandResponses)
	devOpenV1.Get("/commands/status", deviceOpenController.GetCommandStatus)
	devOpenV1.Get("/nodes", deviceOpenController.GetMeshNodes)
	devOpenV1.Get("/installations/:device_uuid", deviceOpenController.GetInstallation)
	devOpenV1.Post("/errors", deviceOpenController.PostErrors)

	devOpenLegacyV1 := app.Group("/api/v1/devices/open", devOpenLimiter)
	devOpenLegacyV1.Get("/bootstrap", func(c *fiber.Ctx) error {
		return c.Redirect("/api/bootstrap" + "?" + string(c.Request().URI().QueryString()))
	})
	devOpenLegacyV1.Get("/credentials/local", deviceOpenController.GetLocalCredentials)
	devOpenLegacyV1.Get("/credentials/government", deviceOpenController.GetGovernmentCredentials)
	devOpenLegacyV1.Get("/vfd", deviceOpenController.GetVFDModels)
	devOpenLegacyV1.Get("/commands/history", deviceOpenController.GetCommandHistory)
	devOpenLegacyV1.Get("/commands/responses", deviceOpenController.GetCommandResponses)
	devOpenLegacyV1.Get("/commands/status", deviceOpenController.GetCommandStatus)
	devOpenLegacyV1.Get("/nodes", deviceOpenController.GetMeshNodes)
	devOpenLegacyV1.Get("/installations/:device_uuid", deviceOpenController.GetInstallation)
	devOpenLegacyV1.Post("/errors", deviceOpenController.PostErrors)

	// Force guarded ingest by default; override only if explicitly needed for legacy bring-up.
	ingestOpen := false
	ingestIPAllowlist := strings.Split(strings.TrimSpace(os.Getenv("INGEST_IP_ALLOWLIST")), ",")
	ingestLimiter := limiter.New(limiter.Config{Max: 100, Expiration: time.Second})

	app.Post("/api/ingest", ingestLimiter, api.ApiKeyMiddleware(pgRepo), func(c *fiber.Ctx) error {
		if len(ingestIPAllowlist) > 0 && strings.TrimSpace(ingestIPAllowlist[0]) != "" {
			clientIP := c.IP()
			allowed := false
			for _, ip := range ingestIPAllowlist {
				if strings.TrimSpace(ip) == clientIP {
					allowed = true
					break
				}
			}
			if !allowed {
				return c.Status(403).SendString("forbidden")
			}
		}

		// Guarded path: ApiKey required unless ingestOpen is explicitly enabled.
		if !ingestOpen {
			if method := c.Locals("auth_method"); method != "api_key" {
				return c.Status(401).SendString("api key required")
			}
		}

		projectID, _ := c.Locals("project_id").(string)
		if projectID == "" {
			return c.Status(400).SendString("project_id required from api key")
		}

		err := ingestService.ProcessPacket("http/debug", c.Body(), projectID)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}
		return c.Send(nil)
	})

	app.Post("/api/telemetry/mirror/:topic_suffix", mirrorController.Ingest)
	app.Post("/api/v1/telemetry/mirror/:topic_suffix", mirrorController.Ingest)
	app.Post("/api/telemetry/:topic_suffix", httpsTelemetryController.Ingest)
	app.Post("/api/v1/telemetry/:topic_suffix", httpsTelemetryController.Ingest)

	app.Get("/api/builder/simulator/:projectId", simController.GetScript)

	// --- 3. Northbound (UI & Integrations) Protected APIs ---
	// Hybrid Auth: Allows JWT or API Key
	// Order: ApiKeyMiddleware -> AuthMiddleware
	// AuthMiddleware detects if "auth_method"=="api_key" and skips verification.
	public := app.Group("/api")
	protected := app.Group("/api", api.ApiKeyMiddleware(pgRepo), authMiddleware, auditMiddleware)
	app.Get("/health", func(c *fiber.Ctx) error {
		mqttSummary, _ := pgRepo.GetMqttProvisioningSummary()
		credSummary, _ := pgRepo.GetCredentialHealthSummary()
		workerRuntime := mqttWorker.RuntimeStats()
		return c.JSON(fiber.Map{
			"status":            "ok",
			"mqtt_provisioning": mqttSummary,
			"mqttProvisioning":  mqttSummary,
			"mqtt_worker":       workerRuntime,
			"mqttWorker":        workerRuntime,
			"credentials":       credSummary,
		})
	})
	app.Get("/api/health", func(c *fiber.Ctx) error {
		mqttSummary, _ := pgRepo.GetMqttProvisioningSummary()
		credSummary, _ := pgRepo.GetCredentialHealthSummary()
		workerRuntime := mqttWorker.RuntimeStats()
		return c.JSON(fiber.Map{
			"status":            "ok",
			"mqtt_provisioning": mqttSummary,
			"mqttProvisioning":  mqttSummary,
			"mqtt_worker":       workerRuntime,
			"mqttWorker":        workerRuntime,
			"credentials":       credSummary,
		})
	})
	app.Get("/api/v1/health", func(c *fiber.Ctx) error {
		mqttSummary, _ := pgRepo.GetMqttProvisioningSummary()
		credSummary, _ := pgRepo.GetCredentialHealthSummary()
		workerRuntime := mqttWorker.RuntimeStats()
		return c.JSON(fiber.Map{
			"status":            "ok",
			"mqtt_provisioning": mqttSummary,
			"mqttProvisioning":  mqttSummary,
			"mqtt_worker":       workerRuntime,
			"mqttWorker":        workerRuntime,
			"credentials":       credSummary,
		})
	})

	geoEstimate := func(ip *string) string {
		if ip == nil || strings.TrimSpace(*ip) == "" {
			return "unknown"
		}
		v := strings.TrimSpace(*ip)
		if strings.HasPrefix(v, "10.") || strings.HasPrefix(v, "192.168.") || strings.HasPrefix(v, "172.") || v == "127.0.0.1" || v == "::1" {
			return "private-network"
		}
		return "public-network"
	}

	mapSessionView := func(session secondary.UserSessionRecord) fiber.Map {
		now := time.Now().UTC()
		lastUsed := session.LastUsedAt
		if lastUsed == nil {
			lastUsed = &session.CreatedAt
		}
		durationSeconds := int(now.Sub(session.CreatedAt).Seconds())
		if durationSeconds < 0 {
			durationSeconds = 0
		}
		var revokedAt *time.Time
		if session.RevokedAt != nil {
			revokedAt = session.RevokedAt
		}
		return fiber.Map{
			"session_id":       session.ID,
			"user_id":          session.UserID,
			"started_at":       session.CreatedAt,
			"last_used_at":     lastUsed,
			"expires_at":       session.ExpiresAt,
			"revoked_at":       revokedAt,
			"revoked":          session.RevokedAt != nil,
			"ip_address":       session.IPAddress,
			"geo_estimate":     geoEstimate(session.IPAddress),
			"duration_seconds": durationSeconds,
			"user_agent":       session.UserAgent,
		}
	}

	type profileOTPEntry struct {
		UserID    string
		Channel   string
		Target    string
		Purpose   string
		Code      string
		ExpiresAt time.Time
	}
	profileOTPStore := map[string]profileOTPEntry{}
	var profileOTPMu sync.Mutex
	app.Get("/metrics", metricsController.GetMetrics)
	httpRouter.RegisterRoutes(public, protected)
	public.Post("/devices/credentials/claim", deviceController.ClaimCredentials)
	public.Post("/devices/credentials/plain-claim", deviceController.PlainClaimCredentials)
	public.Get("/devices/credentials/download", deviceController.DownloadCredentials)

	// Project DNA routes (protected)
	dnaGroup := protected.Group("/project-dna")
	dnaGroup.Get("/:projectId/sensors", dnaSpecController.ListSensors)
	dnaGroup.Put("/:projectId/sensors", dnaSpecController.UpsertSensors)
	dnaGroup.Get("/:projectId/sensors/export", dnaSpecController.ExportSensorsCSV)
	dnaGroup.Post("/:projectId/sensors/import", dnaSpecController.ImportSensorsCSV)
	dnaGroup.Get("/:projectId/sensors/versions", dnaSpecController.ListSensorVersions)
	dnaGroup.Post("/:projectId/sensors/versions", dnaSpecController.CreateSensorVersion)
	dnaGroup.Post("/:projectId/sensors/versions/:versionId/publish", dnaSpecController.PublishSensorVersion)
	dnaGroup.Get("/:projectId/sensors/versions/:versionId/csv", dnaSpecController.DownloadSensorVersionCSV)
	dnaGroup.Post("/:projectId/sensors/versions/:versionId/rollback", dnaSpecController.RollbackSensorVersion)
	dnaGroup.Get("/:projectId/thresholds", dnaSpecController.GetThresholds)
	dnaGroup.Put("/:projectId/thresholds", dnaSpecController.UpsertThresholds)
	dnaGroup.Put("/:projectId/thresholds/:deviceId", dnaSpecController.UpsertDeviceThresholds)
	dnaGroup.Get("/:projectId/thresholds/devices", dnaSpecController.ListThresholdDevices)

	// [ROUND 1] OTA Routes
	otaRoutes := protected.Group("/ota")
	otaRoutes.Post("/upload", otaController.UploadFirmware)
	otaRoutes.Post("/campaign", otaController.StartCampaign)

	// [ROUND 2] Commands & Rules Routes
	protected.Get("/commands/catalog", api.RequireCapability([]string{"devices:commands"}, false), cmdController.GetCatalog)
	protected.Get("/commands/catalog-admin", api.RequireCapability([]string{"devices:commands"}, false), cmdCatalogController.List)
	protected.Get("/commands", api.RequireCapability([]string{"devices:commands"}, false), cmdController.ListCommands)
	protected.Get("/commands/status", api.RequireCapability([]string{"devices:commands"}, false), cmdController.GetStatus)
	protected.Get("/commands/responses", api.RequireCapability([]string{"devices:commands"}, false), cmdController.ListResponses)
	protected.Post("/commands/send", api.RequireCapability([]string{"devices:commands"}, false), cmdController.SendCommand)
	protected.Post("/commands/:correlationId/retry", api.RequireCapability([]string{"devices:commands"}, false), cmdController.RetryCommand)
	protected.Post("/commands/catalog", api.RequireCapability([]string{"devices:commands"}, false), cmdCatalogController.Upsert)
	protected.Delete("/commands/catalog/:id", api.RequireCapability([]string{"devices:commands"}, false), cmdCatalogController.Delete)
	protected.Post("/devices/:device_uuid/commands", api.RequireCapability([]string{"devices:commands"}, false), cmdController.IssueDeviceCommand)
	protected.Post("/v1/devices/:device_uuid/commands", api.RequireCapability([]string{"devices:commands"}, false), cmdController.IssueDeviceCommand)
	protected.Post("/devices/:device_uuid/commands/set-vfd", api.RequireCapability([]string{"devices:commands"}, false), func(c *fiber.Ctx) error {
		return cmdController.IssueSimpleDeviceCommand(c, "set_vfd")
	})
	protected.Post("/v1/devices/:device_uuid/commands/set-vfd", api.RequireCapability([]string{"devices:commands"}, false), func(c *fiber.Ctx) error {
		return cmdController.IssueSimpleDeviceCommand(c, "set_vfd")
	})
	protected.Post("/devices/:device_uuid/commands/get-vfd", api.RequireCapability([]string{"devices:commands"}, false), func(c *fiber.Ctx) error {
		return cmdController.IssueSimpleDeviceCommand(c, "get_vfd")
	})
	protected.Post("/v1/devices/:device_uuid/commands/get-vfd", api.RequireCapability([]string{"devices:commands"}, false), func(c *fiber.Ctx) error {
		return cmdController.IssueSimpleDeviceCommand(c, "get_vfd")
	})
	protected.Post("/devices/:device_uuid/commands/set-beneficiary", api.RequireCapability([]string{"devices:commands"}, false), func(c *fiber.Ctx) error {
		return cmdController.IssueSimpleDeviceCommand(c, "set_beneficiary")
	})
	protected.Post("/v1/devices/:device_uuid/commands/set-beneficiary", api.RequireCapability([]string{"devices:commands"}, false), func(c *fiber.Ctx) error {
		return cmdController.IssueSimpleDeviceCommand(c, "set_beneficiary")
	})
	protected.Post("/devices/:device_uuid/commands/get-beneficiary", api.RequireCapability([]string{"devices:commands"}, false), func(c *fiber.Ctx) error {
		return cmdController.IssueSimpleDeviceCommand(c, "get_beneficiary")
	})
	protected.Post("/v1/devices/:device_uuid/commands/get-beneficiary", api.RequireCapability([]string{"devices:commands"}, false), func(c *fiber.Ctx) error {
		return cmdController.IssueSimpleDeviceCommand(c, "get_beneficiary")
	})
	protected.Post("/devices/:device_uuid/commands/set-installation", api.RequireCapability([]string{"devices:commands"}, false), func(c *fiber.Ctx) error {
		return cmdController.IssueSimpleDeviceCommand(c, "set_installation")
	})
	protected.Post("/v1/devices/:device_uuid/commands/set-installation", api.RequireCapability([]string{"devices:commands"}, false), func(c *fiber.Ctx) error {
		return cmdController.IssueSimpleDeviceCommand(c, "set_installation")
	})
	protected.Post("/devices/:device_uuid/commands/get-installation", api.RequireCapability([]string{"devices:commands"}, false), func(c *fiber.Ctx) error {
		return cmdController.IssueSimpleDeviceCommand(c, "get_installation")
	})
	protected.Post("/v1/devices/:device_uuid/commands/get-installation", api.RequireCapability([]string{"devices:commands"}, false), func(c *fiber.Ctx) error {
		return cmdController.IssueSimpleDeviceCommand(c, "get_installation")
	})
	protected.Post("/devices/:device_uuid/commands/ack", api.RequireCapability([]string{"devices:commands"}, false), cmdController.AckDeviceCommand)
	protected.Post("/v1/devices/:device_uuid/commands/ack", api.RequireCapability([]string{"devices:commands"}, false), cmdController.AckDeviceCommand)
	protected.Get("/devices/:device_uuid/commands/history", api.RequireCapability([]string{"devices:commands"}, false), cmdController.GetDeviceCommandHistory)
	protected.Get("/v1/devices/:device_uuid/commands/history", api.RequireCapability([]string{"devices:commands"}, false), cmdController.GetDeviceCommandHistory)

	// Mesh nodes (gateway forwarding) management
	protected.Get("/devices/:device_uuid/nodes", api.RequireCapability([]string{"devices:commands"}, false), meshNodesController.ListForGateway)
	protected.Post("/devices/:device_uuid/nodes/attach", api.RequireCapability([]string{"devices:commands"}, false), meshNodesController.Attach)
	protected.Post("/devices/:device_uuid/nodes/detach", api.RequireCapability([]string{"devices:commands"}, false), meshNodesController.Detach)
	protected.Post("/devices/:device_uuid/nodes/discovery", api.RequireCapability([]string{"devices:commands"}, false), meshNodesController.Discovery)

	protected.Get("/rules", api.RequireCapability([]string{"alerts:manage"}, false), rulesController.GetRules)
	protected.Post("/rules", api.RequireCapability([]string{"alerts:manage"}, false), rulesController.CreateRule)
	protected.Delete("/rules/:id", api.RequireCapability([]string{"alerts:manage"}, false), rulesController.DeleteRule)

	// [ROUND 17] Device Management
	protected.Get("/devices", api.RequireCapability([]string{"devices:read"}, false), deviceController.ListDevices)
	protected.Get("/devices/", api.RequireCapability([]string{"devices:read"}, false), deviceController.ListDevices)
	protected.Get("/devices/local", api.RequireCapability([]string{"devices:read"}, false), deviceController.ListDevices)
	protected.Get("/devices/local/", api.RequireCapability([]string{"devices:read"}, false), deviceController.ListDevices)
	protected.Get("/devices/government", api.RequireCapability([]string{"devices:read"}, false), deviceController.ListDevices)
	protected.Get("/devices/government/", api.RequireCapability([]string{"devices:read"}, false), deviceController.ListDevices)
	protected.Get("/v1/devices", api.RequireCapability([]string{"devices:read"}, false), deviceController.ListDevices)
	protected.Get("/devices/:idOrUuid", api.RequireCapability([]string{"devices:read"}, false), deviceController.GetDevice)
	protected.Get("/v1/devices/:idOrUuid", api.RequireCapability([]string{"devices:read"}, false), deviceController.GetDevice)
	protected.Get("/devices/:idOrUuid/status", api.RequireCapability([]string{"devices:read"}, false), deviceController.GetDeviceStatus)
	protected.Get("/v1/devices/:idOrUuid/status", api.RequireCapability([]string{"devices:read"}, false), deviceController.GetDeviceStatus)
	protected.Get("/devices/lookup", api.RequireCapability([]string{"devices:read"}, false), deviceController.LookupDevice)
	protected.Get("/v1/devices/lookup", api.RequireCapability([]string{"devices:read"}, false), deviceController.LookupDevice)
	protected.Get("/devices/:idOrUuid/beneficiary", api.RequireCapability([]string{"devices:read"}, false), deviceController.GetDeviceBeneficiary)
	protected.Get("/v1/devices/:idOrUuid/beneficiary", api.RequireCapability([]string{"devices:read"}, false), deviceController.GetDeviceBeneficiary)
	protected.Put("/devices/:idOrUuid", api.RequireCapability([]string{"devices:write"}, false), deviceController.UpdateDevice)
	protected.Put("/v1/devices/:idOrUuid", api.RequireCapability([]string{"devices:write"}, false), deviceController.UpdateDevice)
	protected.Post("/devices", api.RequireCapability([]string{"devices:write"}, false), deviceController.CreateDevice)
	protected.Post("/devices/", api.RequireCapability([]string{"devices:write"}, false), deviceController.CreateDevice)
	protected.Post("/v1/devices", api.RequireCapability([]string{"devices:write"}, false), deviceController.CreateDevice)
	protected.Delete("/devices/:idOrUuid", api.RequireCapability([]string{"devices:write"}, false), deviceController.DeleteDevice)
	protected.Delete("/v1/devices/:idOrUuid", api.RequireCapability([]string{"devices:write"}, false), deviceController.DeleteDevice)
	protected.Post("/devices/:id/rotate-creds", api.RequireCapability([]string{"devices:credentials"}, false), deviceController.RotateCredentials)
	protected.Post("/v1/devices/:id/rotate-creds", api.RequireCapability([]string{"devices:credentials"}, false), deviceController.RotateCredentials)
	protected.Post("/devices/:id/credentials/rotate", api.RequireCapability([]string{"devices:credentials"}, false), deviceController.RotateCredentials)
	protected.Post("/v1/devices/:id/credentials/rotate", api.RequireCapability([]string{"devices:credentials"}, false), deviceController.RotateCredentials)
	protected.Post("/devices/:id/credentials/revoke", api.RequireCapability([]string{"devices:credentials"}, false), deviceController.RevokeCredentials)
	protected.Post("/v1/devices/:id/credentials/revoke", api.RequireCapability([]string{"devices:credentials"}, false), deviceController.RevokeCredentials)
	protected.Get("/devices/:id/credentials/history", api.RequireCapability([]string{"devices:credentials"}, false), deviceController.GetCredentialHistory)
	protected.Get("/v1/devices/:id/credentials/history", api.RequireCapability([]string{"devices:credentials"}, false), deviceController.GetCredentialHistory)
	protected.Post("/devices/:id/mqtt-provisioning/retry", api.RequireCapability([]string{"devices:credentials"}, false), deviceController.RetryProvisioning)
	protected.Post("/v1/devices/:id/mqtt-provisioning/retry", api.RequireCapability([]string{"devices:credentials"}, false), deviceController.RetryProvisioning)
	protected.Post("/devices/:id/credentials/download-token", api.RequireCapability([]string{"devices:credentials"}, false), deviceController.IssueCredentialDownloadToken)
	protected.Post("/v1/devices/:id/credentials/download-token", api.RequireCapability([]string{"devices:credentials"}, false), deviceController.IssueCredentialDownloadToken)
	protected.Post("/devices/:idOrUuid/configuration", api.RequireCapability([]string{"devices:write"}, false), deviceController.QueueDeviceConfiguration)
	protected.Post("/v1/devices/:idOrUuid/configuration", api.RequireCapability([]string{"devices:write"}, false), deviceController.QueueDeviceConfiguration)
	protected.Get("/devices/:idOrUuid/configuration/pending", api.RequireCapability([]string{"devices:read"}, false), deviceController.GetPendingConfiguration)
	protected.Get("/v1/devices/:idOrUuid/configuration/pending", api.RequireCapability([]string{"devices:read"}, false), deviceController.GetPendingConfiguration)
	protected.Post("/devices/:idOrUuid/configuration/ack", api.RequireCapability([]string{"devices:write"}, false), deviceController.AcknowledgeConfiguration)
	protected.Post("/v1/devices/:idOrUuid/configuration/ack", api.RequireCapability([]string{"devices:write"}, false), deviceController.AcknowledgeConfiguration)
	protected.Post("/devices/configuration/import", api.RequireCapability([]string{"devices:bulk_import"}, false), deviceController.ImportDeviceConfigurations)
	protected.Post("/v1/devices/configuration/import", api.RequireCapability([]string{"devices:bulk_import"}, false), deviceController.ImportDeviceConfigurations)
	protected.Get("/devices/import/jobs", api.RequireCapability([]string{"devices:bulk_import"}, false), deviceController.ListImportJobs)
	protected.Get("/v1/devices/import/jobs", api.RequireCapability([]string{"devices:bulk_import"}, false), deviceController.ListImportJobs)
	protected.Get("/devices/government-credentials/import/jobs", api.RequireCapability([]string{"devices:bulk_import"}, false), deviceController.ListGovtImportJobs)
	protected.Get("/v1/devices/government-credentials/import/jobs", api.RequireCapability([]string{"devices:bulk_import"}, false), deviceController.ListGovtImportJobs)
	protected.Get("/devices/government-credentials/import/jobs/:jobId", api.RequireCapability([]string{"devices:bulk_import"}, false), deviceController.GetImportJob)
	protected.Get("/v1/devices/government-credentials/import/jobs/:jobId", api.RequireCapability([]string{"devices:bulk_import"}, false), deviceController.GetImportJob)
	protected.Get("/devices/government-credentials/import/jobs/:jobId/errors.csv", api.RequireCapability([]string{"devices:bulk_import"}, false), deviceController.GetImportJobErrorsCSV)
	protected.Get("/v1/devices/government-credentials/import/jobs/:jobId/errors.csv", api.RequireCapability([]string{"devices:bulk_import"}, false), deviceController.GetImportJobErrorsCSV)
	protected.Post("/devices/government-credentials/import/jobs/:jobId/retry", api.RequireCapability([]string{"devices:bulk_import"}, false), deviceController.RetryImportJob)
	protected.Post("/v1/devices/government-credentials/import/jobs/:jobId/retry", api.RequireCapability([]string{"devices:bulk_import"}, false), deviceController.RetryImportJob)
	protected.Get("/devices/import/jobs/:jobId", api.RequireCapability([]string{"devices:bulk_import"}, false), deviceController.GetImportJob)
	protected.Get("/v1/devices/import/jobs/:jobId", api.RequireCapability([]string{"devices:bulk_import"}, false), deviceController.GetImportJob)
	protected.Get("/devices/import/jobs/:jobId/errors.csv", api.RequireCapability([]string{"devices:bulk_import"}, false), deviceController.GetImportJobErrorsCSV)
	protected.Get("/v1/devices/import/jobs/:jobId/errors.csv", api.RequireCapability([]string{"devices:bulk_import"}, false), deviceController.GetImportJobErrorsCSV)
	protected.Post("/devices/import/jobs/:jobId/retry", api.RequireCapability([]string{"devices:bulk_import"}, false), deviceController.RetryImportJob)
	protected.Post("/v1/devices/import/jobs/:jobId/retry", api.RequireCapability([]string{"devices:bulk_import"}, false), deviceController.RetryImportJob)

	// [PHASE 6] ERP Routes
	protected.Get("/maintenance/work-orders", erpController.GetWorkOrders)
	protected.Post("/maintenance/work-orders", erpController.CreateWorkOrder)
	protected.Put("/maintenance/work-orders/:id/resolve", erpController.ResolveWorkOrder)

	protected.Get("/inventory/products", erpController.GetProducts)
	protected.Post("/inventory/products", erpController.CreateProduct)
	protected.Get("/inventory/stock/:locId", erpController.GetStockLevels)

	protected.Get("/logistics/trips", erpController.GetTrips)
	protected.Post("/logistics/trips", erpController.CreateTrip)
	protected.Get("/logistics/assets/:id/timeline", erpController.GetAssetTimeline)

	protected.Get("/logistics/geofences", erpController.GetGeofences)
	protected.Get("/traffic/cameras", erpController.GetCameras)
	protected.Get("/traffic/metrics/:deviceId", erpController.GetTrafficMetrics)
	protected.Post("/traffic/metrics", erpController.CreateTrafficMetric)

	// [PHASE 8] Config Routes
	protected.Post("/config/automation", api.RequireCapability([]string{"alerts:manage"}, false), configController.SaveAutomationFlow)
	protected.Get("/config/automation/:projectId", api.RequireCapability([]string{"alerts:manage"}, false), configController.GetAutomationFlow)
	protected.Post("/config/profiles", configController.CreateDeviceProfile)
	protected.Get("/config/profiles", configController.GetDeviceProfiles)

	protected.Get("/dna", dnaController.List)
	protected.Get("/dna/:projectId", dnaController.Get)
	protected.Put("/dna/:projectId", dnaController.Upsert)

	// [PHASE 9] Vertical Routes
	protected.Post("/beneficiaries", api.RequireCapability([]string{"beneficiaries:manage"}, false), verticalController.CreateBeneficiary)
	protected.Get("/beneficiaries", api.RequireCapability([]string{"beneficiaries:manage"}, false), verticalController.GetBeneficiaries)
	protected.Get("/installations/beneficiaries", api.RequireCapability([]string{"beneficiaries:manage"}, false), verticalController.GetBeneficiaries)
	protected.Get("/beneficiaries/:beneficiaryUuid", api.RequireCapability([]string{"beneficiaries:manage"}, false), verticalController.GetBeneficiary)
	protected.Get("/installations/beneficiaries/:beneficiaryUuid", api.RequireCapability([]string{"beneficiaries:manage"}, false), verticalController.GetBeneficiary)
	protected.Patch("/beneficiaries/:beneficiaryUuid", api.RequireCapability([]string{"beneficiaries:manage"}, false), verticalController.UpdateBeneficiary)
	protected.Patch("/installations/beneficiaries/:beneficiaryUuid", api.RequireCapability([]string{"beneficiaries:manage"}, false), verticalController.UpdateBeneficiary)
	protected.Put("/beneficiaries/:id", api.RequireCapability([]string{"beneficiaries:manage"}, false), verticalController.UpdateBeneficiary)
	protected.Post("/beneficiaries/:beneficiaryUuid/archive", api.RequireCapability([]string{"beneficiaries:manage"}, false), verticalController.ArchiveBeneficiary)
	protected.Delete("/beneficiaries/:beneficiaryUuid", api.RequireCapability([]string{"beneficiaries:manage"}, false), verticalController.ArchiveBeneficiary)
	protected.Delete("/installations/beneficiaries/:beneficiaryUuid", api.RequireCapability([]string{"beneficiaries:manage"}, false), verticalController.ArchiveBeneficiary)
	protected.Post("/installations/beneficiaries", api.RequireCapability([]string{"beneficiaries:manage"}, false), verticalController.CreateBeneficiary)

	protected.Post("/installations", api.RequireCapability([]string{"installations:manage"}, false), verticalController.CreateInstallation)
	protected.Get("/installations", api.RequireCapability([]string{"installations:manage"}, false), verticalController.GetInstallations)
	protected.Get("/installations/:installationUuid", api.RequireCapability([]string{"installations:manage"}, false), verticalController.GetInstallation)
	protected.Patch("/installations/:installationUuid", api.RequireCapability([]string{"installations:manage"}, false), verticalController.UpdateInstallation)
	protected.Put("/installations/:id", api.RequireCapability([]string{"installations:manage"}, false), verticalController.UpdateInstallation)
	protected.Get("/installations/:installationUuid/beneficiaries", api.RequireCapability([]string{"installations:manage", "beneficiaries:manage"}, true), verticalController.GetInstallationBeneficiaries)
	protected.Post("/installations/:installationUuid/beneficiaries", api.RequireCapability([]string{"installations:manage", "beneficiaries:manage"}, true), verticalController.AddInstallationBeneficiary)
	protected.Delete("/installations/:installationUuid/beneficiaries/:beneficiaryUuid", api.RequireCapability([]string{"installations:manage", "beneficiaries:manage"}, true), verticalController.RemoveInstallationBeneficiary)

	protected.Post("/projects/:projectId/vfd/manufacturers", verticalController.CreateVFDManufacturer)
	protected.Get("/projects/:projectId/vfd/manufacturers", verticalController.GetVFDManufacturers)
	protected.Post("/projects/:projectId/vfd/models", verticalController.CreateVFDModel)
	protected.Get("/projects/:projectId/vfd/models", verticalController.GetVFDModels)
	protected.Patch("/projects/:projectId/vfd/models/:modelId", verticalController.UpdateVFDModel)
	protected.Patch("/vfd-models/:vfdModelId", verticalController.UpdateVFDModel)
	protected.Get("/vfd-models", func(c *fiber.Ctx) error {
		projectID := strings.TrimSpace(c.Query("project_id"))
		if projectID == "" {
			projectID = strings.TrimSpace(c.Query("projectId"))
		}
		if projectID == "" {
			return c.Status(400).SendString("project_id required")
		}
		target := "/api/projects/" + projectID + "/vfd/models"
		if qs := string(c.Request().URI().QueryString()); qs != "" {
			target += "?" + qs
		}
		return c.Redirect(target, 307)
	})
	protected.Get("/vfd-models/", func(c *fiber.Ctx) error {
		projectID := strings.TrimSpace(c.Query("project_id"))
		if projectID == "" {
			projectID = strings.TrimSpace(c.Query("projectId"))
		}
		if projectID == "" {
			return c.Status(400).SendString("project_id required")
		}
		target := "/api/projects/" + projectID + "/vfd/models"
		if qs := string(c.Request().URI().QueryString()); qs != "" {
			target += "?" + qs
		}
		return c.Redirect(target, 307)
	})
	protected.Post("/vfd-models", func(c *fiber.Ctx) error {
		projectID := strings.TrimSpace(c.Query("project_id"))
		if projectID == "" {
			projectID = strings.TrimSpace(c.Query("projectId"))
		}
		if projectID == "" {
			return c.Status(400).SendString("project_id required")
		}
		target := "/api/projects/" + projectID + "/vfd/models"
		if qs := string(c.Request().URI().QueryString()); qs != "" {
			target += "?" + qs
		}
		return c.Redirect(target, 307)
	})
	protected.Post("/vfd-models/", func(c *fiber.Ctx) error {
		projectID := strings.TrimSpace(c.Query("project_id"))
		if projectID == "" {
			projectID = strings.TrimSpace(c.Query("projectId"))
		}
		if projectID == "" {
			return c.Status(400).SendString("project_id required")
		}
		target := "/api/projects/" + projectID + "/vfd/models"
		if qs := string(c.Request().URI().QueryString()); qs != "" {
			target += "?" + qs
		}
		return c.Redirect(target, 307)
	})
	protected.Post("/projects/:projectId/vfd/models/:modelId/import", verticalController.ImportVFDModelArtifacts)
	protected.Get("/vfd-models/export.csv", api.RequireCapability([]string{"catalog:drives"}, false), verticalController.ExportVFDModelsCSV)
	protected.Post("/vfd-models/import", api.RequireCapability([]string{"catalog:drives"}, false), verticalController.ImportVFDModels)
	protected.Get("/vfd-models/command-dictionaries/import/jobs", api.RequireCapability([]string{"catalog:drives"}, false), verticalController.ListVFDImportJobs)
	protected.Post("/vfd-models/command-dictionaries/import", api.RequireCapability([]string{"catalog:drives"}, false), verticalController.ImportVFDCommandDictionary)
	protected.Get("/v1/vfd-models/export.csv", api.RequireCapability([]string{"catalog:drives"}, false), verticalController.ExportVFDModelsCSV)
	protected.Post("/v1/vfd-models/import", api.RequireCapability([]string{"catalog:drives"}, false), verticalController.ImportVFDModels)
	protected.Get("/v1/vfd-models/command-dictionaries/import/jobs", api.RequireCapability([]string{"catalog:drives"}, false), verticalController.ListVFDImportJobs)
	protected.Post("/v1/vfd-models/command-dictionaries/import", api.RequireCapability([]string{"catalog:drives"}, false), verticalController.ImportVFDCommandDictionary)
	protected.Post("/projects/:projectId/protocols/:protocolId/vfd-assignments", verticalController.CreateProtocolVFDAssignment)
	protected.Get("/projects/:projectId/protocols/:protocolId/vfd-assignments", verticalController.GetProtocolVFDAssignments)
	protected.Get("/vfd-models/assignments", func(c *fiber.Ctx) error {
		projectID := strings.TrimSpace(c.Query("project_id"))
		if projectID == "" {
			projectID = strings.TrimSpace(c.Query("projectId"))
		}
		protocolID := strings.TrimSpace(c.Query("protocol_id"))
		if protocolID == "" {
			protocolID = strings.TrimSpace(c.Query("protocolId"))
		}
		if projectID == "" || protocolID == "" {
			return c.Status(400).SendString("project_id and protocol_id required")
		}
		target := "/api/projects/" + projectID + "/protocols/" + protocolID + "/vfd-assignments"
		if qs := string(c.Request().URI().QueryString()); qs != "" {
			target += "?" + qs
		}
		return c.Redirect(target, 307)
	})
	protected.Post("/vfd-models/assignments", func(c *fiber.Ctx) error {
		projectID := strings.TrimSpace(c.Query("project_id"))
		if projectID == "" {
			projectID = strings.TrimSpace(c.Query("projectId"))
		}
		protocolID := strings.TrimSpace(c.Query("protocol_id"))
		if protocolID == "" {
			protocolID = strings.TrimSpace(c.Query("protocolId"))
		}
		if projectID == "" || protocolID == "" {
			return c.Status(400).SendString("project_id and protocol_id required")
		}
		target := "/api/projects/" + projectID + "/protocols/" + protocolID + "/vfd-assignments"
		if qs := string(c.Request().URI().QueryString()); qs != "" {
			target += "?" + qs
		}
		return c.Redirect(target, 307)
	})
	protected.Post("/vfd-models/assignments/:assignmentId/revoke", func(c *fiber.Ctx) error {
		projectID := strings.TrimSpace(c.Query("project_id"))
		if projectID == "" {
			projectID = strings.TrimSpace(c.Query("projectId"))
		}
		protocolID := strings.TrimSpace(c.Query("protocol_id"))
		if protocolID == "" {
			protocolID = strings.TrimSpace(c.Query("protocolId"))
		}
		assignmentID := c.Params("assignmentId")
		if projectID == "" || protocolID == "" || assignmentID == "" {
			return c.Status(400).SendString("project_id, protocol_id, and assignment_id required")
		}
		target := "/api/projects/" + projectID + "/protocols/" + protocolID + "/vfd-assignments/" + assignmentID + "/revoke"
		if qs := string(c.Request().URI().QueryString()); qs != "" {
			target += "?" + qs
		}
		return c.Redirect(target, 307)
	})
	protected.Post("/projects/:projectId/protocols/:protocolId/vfd-assignments/:id/revoke", verticalController.RevokeProtocolVFDAssignment)

	protected.Post("/patients", verticalController.CreatePatient)
	protected.Get("/patients", verticalController.GetPatients)
	protected.Post("/healthcare/patients", verticalController.CreatePatient)
	protected.Get("/healthcare/patients", verticalController.GetPatients)
	protected.Post("/healthcare/sessions/start", verticalController.StartMedicalSession)
	protected.Post("/healthcare/sessions/:sessionId/end", verticalController.EndMedicalSession)

	protected.Post("/agriculture/advice", verticalController.GenerateSoilAdvice)
	protected.Post("/agriculture/rules", rulesController.CreateRule)

	protected.Post("/gis/layers", verticalController.CreateGISLayer)
	protected.Get("/gis/layers", verticalController.GetGISLayers)

	// [PHASE 10] Final Gap Routes
	protected.Get("/telemetry/history", api.RequireCapability([]string{"telemetry:read"}, false), analyticsController.GetHistory)
	protected.Get("/telemetry/history/project", api.RequireCapability([]string{"telemetry:read"}, false), analyticsController.GetProjectHistory)
	protected.Get("/telemetry", api.RequireCapability([]string{"telemetry:read"}, false), analyticsController.GetHistory)
	protected.Post("/telemetry/ingest", api.RequireCapability([]string{"telemetry:export"}, false), func(c *fiber.Ctx) error {
		var body struct {
			IMEI             string                 `json:"imei"`
			TopicSuffix      string                 `json:"topic_suffix"`
			TopicSuffixCamel string                 `json:"topicSuffix"`
			Payload          map[string]interface{} `json:"payload"`
			MsgID            string                 `json:"msgid"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"message": "Invalid telemetry ingest payload"})
		}
		if strings.TrimSpace(body.TopicSuffix) == "" {
			body.TopicSuffix = body.TopicSuffixCamel
		}
		if strings.TrimSpace(body.IMEI) == "" || strings.TrimSpace(body.TopicSuffix) == "" {
			return c.Status(400).JSON(fiber.Map{"message": "Invalid telemetry ingest payload"})
		}
		envelope := map[string]interface{}{}
		for k, v := range body.Payload {
			envelope[k] = v
		}
		envelope["imei"] = body.IMEI
		envelope["packet_type"] = body.TopicSuffix
		if body.MsgID != "" {
			envelope["msgid"] = body.MsgID
		}
		blob, err := json.Marshal(envelope)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"message": "Invalid telemetry ingest payload"})
		}
		if err := ingestService.ProcessPacket("http/telemetry/"+body.TopicSuffix, blob, ""); err != nil {
			return c.Status(422).JSON(fiber.Map{"message": err.Error()})
		}
		return c.Status(202).JSON(fiber.Map{"telemetry_id": uuid.NewString()})
	})
	protected.Get("/telemetry/devices/:device_uuid/history", api.RequireCapability([]string{"telemetry:read"}, false), func(c *fiber.Ctx) error {
		deviceRef := strings.TrimSpace(c.Params("device_uuid"))
		if deviceRef == "" {
			return c.Status(400).JSON(fiber.Map{"message": "Invalid device identifier provided"})
		}
		fromStr := strings.TrimSpace(c.Query("from"))
		toStr := strings.TrimSpace(c.Query("to"))
		start := time.Now().Add(-24 * time.Hour)
		end := time.Now()
		if fromStr != "" {
			if parsed, err := time.Parse(time.RFC3339, fromStr); err == nil {
				start = parsed
			}
		}
		if toStr != "" {
			if parsed, err := time.Parse(time.RFC3339, toStr); err == nil {
				end = parsed
			}
		}
		limit, _ := strconv.Atoi(c.Query("limit", "0"))
		packetType := strings.TrimSpace(c.Query("topic_suffix"))
		if packetType == "" {
			packetType = strings.TrimSpace(c.Query("topicSuffix"))
		}
		records, err := analyticsService.GetHistory(deviceRef, packetType, start, end, limit, 0)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"message": err.Error()})
		}
		return c.JSON(fiber.Map{
			"device_uuid": deviceRef,
			"count":       len(records),
			"records":     records,
		})
	})
	protected.Get("/telemetry/devices/imei/:imei/history", api.RequireCapability([]string{"telemetry:read"}, false), func(c *fiber.Ctx) error {
		imei := strings.TrimSpace(c.Params("imei"))
		if imei == "" {
			return c.Status(400).JSON(fiber.Map{"message": "Invalid device identifier provided"})
		}
		fromStr := strings.TrimSpace(c.Query("from"))
		toStr := strings.TrimSpace(c.Query("to"))
		start := time.Now().Add(-24 * time.Hour)
		end := time.Now()
		if fromStr != "" {
			if parsed, err := time.Parse(time.RFC3339, fromStr); err == nil {
				start = parsed
			}
		}
		if toStr != "" {
			if parsed, err := time.Parse(time.RFC3339, toStr); err == nil {
				end = parsed
			}
		}
		limit, _ := strconv.Atoi(c.Query("limit", "0"))
		packetType := strings.TrimSpace(c.Query("topic_suffix"))
		if packetType == "" {
			packetType = strings.TrimSpace(c.Query("topicSuffix"))
		}
		records, err := analyticsService.GetHistory(imei, packetType, start, end, limit, 0)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"message": err.Error()})
		}
		deviceUuid := ""
		if device, _ := deviceService.GetDeviceByIDOrIMEI(imei); device != nil {
			if id, ok := device["id"].(string); ok {
				deviceUuid = id
			}
		}
		return c.JSON(fiber.Map{
			"device_uuid": deviceUuid,
			"imei":        imei,
			"count":       len(records),
			"records":     records,
		})
	})
	protected.Get("/telemetry/devices/:device_uuid/latest", api.RequireCapability([]string{"telemetry:read"}, false), analyticsController.GetLatest)
	protected.Get("/telemetry/devices/imei/:imei/latest", api.RequireCapability([]string{"telemetry:read"}, false), analyticsController.GetLatest)
	protected.Get("/v1/telemetry/devices/:device_uuid/latest", api.RequireCapability([]string{"telemetry:read"}, false), analyticsController.GetLatest)
	protected.Get("/v1/telemetry/devices/imei/:imei/latest", api.RequireCapability([]string{"telemetry:read"}, false), analyticsController.GetLatest)
	protected.Get("/v1/telemetry/devices/:device_uuid/history", api.RequireCapability([]string{"telemetry:read"}, false), func(c *fiber.Ctx) error {
		deviceRef := strings.TrimSpace(c.Params("device_uuid"))
		c.Context().QueryArgs().Set("device", deviceRef)
		return analyticsController.GetHistory(c)
	})
	protected.Get("/v1/telemetry/devices/imei/:imei/history", api.RequireCapability([]string{"telemetry:read"}, false), func(c *fiber.Ctx) error {
		c.Context().QueryArgs().Set("imei", c.Params("imei"))
		return analyticsController.GetHistory(c)
	})
	protected.Get("/telemetry/devices/:device_uuid/live", api.RequireCapability([]string{"telemetry:live:device", "telemetry:live:all"}, false), telemetryLiveController.StreamDevice)
	protected.Post("/telemetry/devices/:device_uuid/live-token", api.RequireCapability([]string{"telemetry:live:device", "telemetry:live:all"}, false), telemetryLiveController.IssueLiveToken)
	protected.Get("/v1/telemetry/devices/:device_uuid/live", api.RequireCapability([]string{"telemetry:live:device", "telemetry:live:all"}, false), telemetryLiveController.StreamDevice)
	protected.Post("/v1/telemetry/devices/:device_uuid/live-token", api.RequireCapability([]string{"telemetry:live:device", "telemetry:live:all"}, false), telemetryLiveController.IssueLiveToken)
	protected.Get("/telemetry/thresholds/:device_uuid", api.RequireCapability([]string{"alerts:manage"}, false), telemetryThresholdsController.GetDeviceThresholds)
	protected.Put("/telemetry/thresholds/:device_uuid", api.RequireCapability([]string{"alerts:manage"}, false), telemetryThresholdsController.UpsertDeviceThresholds)
	protected.Delete("/telemetry/thresholds/:device_uuid", api.RequireCapability([]string{"alerts:manage"}, false), telemetryThresholdsController.DeleteDeviceThresholds)
	exportSvc := services.NewExportService()
	exportController := api.NewTelemetryExportController(pgRepo, exportSvc)
	protected.Get("/telemetry/export", api.RequireCapability([]string{"telemetry:export"}, false), exportController.Export)

	// Broker sync
	protected.Post("/broker/sync", api.RequireCapability([]string{"devices:credentials"}, false), brokerController.Sync)
	protected.Post("/v1/broker/sync", api.RequireCapability([]string{"devices:credentials"}, false), brokerController.Sync)

	// Master Data (catalogs)
	masterDataController := api.NewMasterDataController(pgRepo)
	protected.Get("/master-data/:type", masterDataController.List)
	protected.Post("/master-data/:type", masterDataController.Create)
	protected.Put("/master-data/:type/:id", masterDataController.Update)
	protected.Delete("/master-data/:type/:id", masterDataController.Delete)

	// Master Data
	protected.Get("/admin/users", api.RequireCapability([]string{"users:manage"}, false), adminController.GetUsers)
	protected.Post("/admin/users", api.RequireCapability([]string{"users:manage"}, false), adminController.CreateUser)
	protected.Get("/admin/users/:id", api.RequireCapability([]string{"users:manage"}, false), adminController.GetUser)
	protected.Patch("/admin/users/:id", api.RequireCapability([]string{"users:manage"}, false), adminController.UpdateUser)
	protected.Post("/admin/users/:id/password", api.RequireCapability([]string{"users:manage"}, false), adminController.ResetUserPassword)
	protected.Delete("/admin/users/:id", api.RequireCapability([]string{"users:manage"}, false), adminController.DeleteUser)
	protected.Get("/users/roles", api.RequireCapability([]string{"users:manage"}, false), adminController.GetUserRoles)
	protected.Get("/users/capabilities", api.RequireCapability([]string{"users:manage"}, false), adminController.GetUserCapabilities)
	protected.Get("/v1/users/roles", api.RequireCapability([]string{"users:manage"}, false), adminController.GetUserRoles)
	protected.Get("/v1/users/capabilities", api.RequireCapability([]string{"users:manage"}, false), adminController.GetUserCapabilities)
	protected.Get("/users", api.RequireCapability([]string{"users:manage"}, false), adminController.ListUsers)
	protected.Get("/users/", api.RequireCapability([]string{"users:manage"}, false), adminController.ListUsers)
	protected.Post("/users", api.RequireCapability([]string{"users:manage"}, false), adminController.CreateUser)
	protected.Post("/users/", api.RequireCapability([]string{"users:manage"}, false), adminController.CreateUser)
	protected.Get("/users/:id", api.RequireCapability([]string{"users:manage"}, false), adminController.GetUser)
	protected.Patch("/users/:id", api.RequireCapability([]string{"users:manage"}, false), adminController.UpdateUser)
	protected.Post("/users/:id/password", api.RequireCapability([]string{"users:manage"}, false), adminController.ResetUserPassword)
	protected.Post("/users/:id/roles", api.RequireCapability([]string{"users:manage"}, false), adminController.AssignUserRole)
	protected.Delete("/users/:id/roles/:bindingId", api.RequireCapability([]string{"users:manage"}, false), adminController.RemoveUserRole)
	protected.Get("/v1/users", api.RequireCapability([]string{"users:manage"}, false), adminController.ListUsers)
	protected.Post("/v1/users", api.RequireCapability([]string{"users:manage"}, false), adminController.CreateUser)
	protected.Get("/v1/users/:id", api.RequireCapability([]string{"users:manage"}, false), adminController.GetUser)
	protected.Patch("/v1/users/:id", api.RequireCapability([]string{"users:manage"}, false), adminController.UpdateUser)
	protected.Post("/v1/users/:id/password", api.RequireCapability([]string{"users:manage"}, false), adminController.ResetUserPassword)
	protected.Post("/v1/users/:id/roles", api.RequireCapability([]string{"users:manage"}, false), adminController.AssignUserRole)
	protected.Delete("/v1/users/:id/roles/:bindingId", api.RequireCapability([]string{"users:manage"}, false), adminController.RemoveUserRole)
	protected.Get("/admin/states", api.RequireCapability([]string{"hierarchy:manage"}, false), adminController.GetStates)
	protected.Post("/admin/state", api.RequireCapability([]string{"hierarchy:manage"}, false), adminController.CreateState)
	protected.Post("/admin/states", api.RequireCapability([]string{"hierarchy:manage"}, false), adminController.CreateState)
	protected.Patch("/admin/states/:id", api.RequireCapability([]string{"hierarchy:manage"}, false), adminController.UpdateState)
	protected.Put("/admin/states/:id", api.RequireCapability([]string{"hierarchy:manage"}, false), adminController.UpdateState)
	protected.Delete("/admin/states/:id", api.RequireCapability([]string{"hierarchy:manage"}, false), adminController.DeleteState)
	protected.Get("/admin/protocol-versions", api.RequireCapability([]string{"catalog:protocols"}, false), adminController.ListProtocolVersions)
	protected.Post("/admin/protocol-versions", api.RequireCapability([]string{"catalog:protocols"}, false), adminController.CreateProtocolVersion)
	protected.Patch("/admin/protocol-versions/:id", api.RequireCapability([]string{"catalog:protocols"}, false), adminController.UpdateProtocolVersion)
	protected.Get("/states", api.RequireCapability([]string{"hierarchy:manage"}, false), adminController.GetStates)
	protected.Post("/states", api.RequireCapability([]string{"hierarchy:manage"}, false), adminController.CreateState)
	protected.Put("/states/:id", api.RequireCapability([]string{"hierarchy:manage"}, false), adminController.UpdateState)
	protected.Delete("/states/:id", api.RequireCapability([]string{"hierarchy:manage"}, false), adminController.DeleteState)
	protected.Get("/authorities", api.RequireCapability([]string{"hierarchy:manage"}, false), adminController.GetAuthorities)
	protected.Post("/authorities", api.RequireCapability([]string{"hierarchy:manage"}, false), adminController.CreateAuthority)
	protected.Put("/authorities/:id", api.RequireCapability([]string{"hierarchy:manage"}, false), adminController.UpdateAuthority)
	protected.Delete("/authorities/:id", api.RequireCapability([]string{"hierarchy:manage"}, false), adminController.DeleteAuthority)
	protected.Post("/admin/authorities", api.RequireCapability([]string{"hierarchy:manage"}, false), adminController.CreateAuthority)
	protected.Put("/admin/authorities/:id", api.RequireCapability([]string{"hierarchy:manage"}, false), adminController.UpdateAuthority)
	protected.Delete("/admin/authorities/:id", api.RequireCapability([]string{"hierarchy:manage"}, false), adminController.DeleteAuthority)
	protected.Get("/admin/state-authorities", api.RequireCapability([]string{"hierarchy:manage"}, false), adminController.GetAuthorities)
	protected.Post("/admin/state-authorities", api.RequireCapability([]string{"hierarchy:manage"}, false), adminController.CreateAuthority)
	protected.Patch("/admin/state-authorities/:id", api.RequireCapability([]string{"hierarchy:manage"}, false), adminController.UpdateAuthority)
	protected.Delete("/admin/state-authorities/:id", api.RequireCapability([]string{"hierarchy:manage"}, false), adminController.DeleteAuthority)
	protected.Get("/lookup/states", api.RequireCapability([]string{"hierarchy:manage"}, false), adminController.GetStates)
	protected.Get("/lookup/authorities", api.RequireCapability([]string{"hierarchy:manage"}, false), adminController.GetAuthorities)
	protected.Get("/admin/server-vendors", api.RequireCapability([]string{"vendors:manage"}, false), func(c *fiber.Ctx) error {
		return adminController.ListVendors(c, "server")
	})
	protected.Post("/admin/server-vendors", api.RequireCapability([]string{"vendors:manage"}, false), func(c *fiber.Ctx) error {
		return adminController.CreateVendor(c, "server")
	})
	protected.Patch("/admin/server-vendors/:id", api.RequireCapability([]string{"vendors:manage"}, false), func(c *fiber.Ctx) error {
		return adminController.UpdateVendor(c, "server")
	})
	protected.Delete("/admin/server-vendors/:id", api.RequireCapability([]string{"vendors:manage"}, false), adminController.DeleteVendor)
	protected.Get("/admin/solar-pump-vendors", api.RequireCapability([]string{"vendors:manage"}, false), func(c *fiber.Ctx) error {
		return adminController.ListVendors(c, "solar_pump")
	})
	protected.Post("/admin/solar-pump-vendors", api.RequireCapability([]string{"vendors:manage"}, false), func(c *fiber.Ctx) error {
		return adminController.CreateVendor(c, "solar_pump")
	})
	protected.Patch("/admin/solar-pump-vendors/:id", api.RequireCapability([]string{"vendors:manage"}, false), func(c *fiber.Ctx) error {
		return adminController.UpdateVendor(c, "solar_pump")
	})
	protected.Delete("/admin/solar-pump-vendors/:id", api.RequireCapability([]string{"vendors:manage"}, false), adminController.DeleteVendor)
	protected.Get("/admin/vfd-drive-manufacturers", api.RequireCapability([]string{"vendors:manage"}, false), func(c *fiber.Ctx) error {
		return adminController.ListVendors(c, "vfd_drive_manufacturer")
	})
	protected.Post("/admin/vfd-drive-manufacturers", api.RequireCapability([]string{"vendors:manage"}, false), func(c *fiber.Ctx) error {
		return adminController.CreateVendor(c, "vfd_drive_manufacturer")
	})
	protected.Patch("/admin/vfd-drive-manufacturers/:id", api.RequireCapability([]string{"vendors:manage"}, false), func(c *fiber.Ctx) error {
		return adminController.UpdateVendor(c, "vfd_drive_manufacturer")
	})
	protected.Delete("/admin/vfd-drive-manufacturers/:id", api.RequireCapability([]string{"vendors:manage"}, false), adminController.DeleteVendor)
	protected.Get("/admin/rms-manufacturers", api.RequireCapability([]string{"vendors:manage"}, false), func(c *fiber.Ctx) error {
		return adminController.ListVendors(c, "rms_manufacturer")
	})
	protected.Post("/admin/rms-manufacturers", api.RequireCapability([]string{"vendors:manage"}, false), func(c *fiber.Ctx) error {
		return adminController.CreateVendor(c, "rms_manufacturer")
	})
	protected.Patch("/admin/rms-manufacturers/:id", api.RequireCapability([]string{"vendors:manage"}, false), func(c *fiber.Ctx) error {
		return adminController.UpdateVendor(c, "rms_manufacturer")
	})
	protected.Delete("/admin/rms-manufacturers/:id", api.RequireCapability([]string{"vendors:manage"}, false), adminController.DeleteVendor)

	// Orgs + API keys
	protected.Get("/orgs", api.RequireCapability([]string{"hierarchy:manage"}, false), orgController.List)
	protected.Post("/orgs", api.RequireCapability([]string{"hierarchy:manage"}, false), orgController.Create)
	protected.Put("/orgs/:id", api.RequireCapability([]string{"hierarchy:manage"}, false), orgController.Update)

	protected.Get("/admin/apikeys", api.RequireCapability([]string{"hierarchy:manage"}, false), apiKeyController.List)
	protected.Post("/admin/apikeys", api.RequireCapability([]string{"hierarchy:manage"}, false), apiKeyController.Create)
	protected.Delete("/admin/apikeys/:id", api.RequireCapability([]string{"hierarchy:manage"}, false), apiKeyController.Revoke)

	// User groups
	protected.Get("/user-groups", api.RequireCapability([]string{"users:manage"}, false), userGroupsController.List)
	protected.Get("/user-groups/", api.RequireCapability([]string{"users:manage"}, false), userGroupsController.List)
	protected.Post("/user-groups", api.RequireCapability([]string{"users:manage"}, false), userGroupsController.Create)
	protected.Patch("/user-groups/:groupId", api.RequireCapability([]string{"users:manage"}, false), userGroupsController.Update)
	protected.Delete("/user-groups/:groupId", api.RequireCapability([]string{"users:manage"}, false), userGroupsController.Delete)
	protected.Get("/user-groups/:groupId/members", api.RequireCapability([]string{"users:manage"}, false), userGroupsController.ListMembers)
	protected.Post("/user-groups/:groupId/members", api.RequireCapability([]string{"users:manage"}, false), userGroupsController.AddMember)
	protected.Delete("/user-groups/:groupId/members/:userId", api.RequireCapability([]string{"users:manage"}, false), userGroupsController.RemoveMember)
	protected.Get("/v1/user-groups", api.RequireCapability([]string{"users:manage"}, false), userGroupsController.List)
	protected.Post("/v1/user-groups", api.RequireCapability([]string{"users:manage"}, false), userGroupsController.Create)
	protected.Patch("/v1/user-groups/:groupId", api.RequireCapability([]string{"users:manage"}, false), userGroupsController.Update)
	protected.Delete("/v1/user-groups/:groupId", api.RequireCapability([]string{"users:manage"}, false), userGroupsController.Delete)
	protected.Get("/v1/user-groups/:groupId/members", api.RequireCapability([]string{"users:manage"}, false), userGroupsController.ListMembers)
	protected.Post("/v1/user-groups/:groupId/members", api.RequireCapability([]string{"users:manage"}, false), userGroupsController.AddMember)
	protected.Delete("/v1/user-groups/:groupId/members/:userId", api.RequireCapability([]string{"users:manage"}, false), userGroupsController.RemoveMember)

	// [PHASE 12] The Final Sweep
	alertsController := api.NewAlertsController(pgRepo)
	schedulerController := api.NewSchedulerController(pgRepo)

	// Alerts
	protected.Get("/alerts", api.RequireCapability([]string{"alerts:manage"}, false), alertsController.GetAlerts)
	protected.Put("/alerts/:id/ack", api.RequireCapability([]string{"alerts:manage"}, false), alertsController.AckAlert)

	// Audit logs
	protected.Get("/audit", api.RequireCapability([]string{"audit:read", "admin:all"}, false), auditController.List)
	protected.Get("/audit/", api.RequireCapability([]string{"audit:read", "admin:all"}, false), auditController.List)
	protected.Get("/profile", func(c *fiber.Ctx) error {
		userID, _ := c.Locals("user_id").(string)
		userID = strings.TrimSpace(userID)
		if userID == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "missing user context"})
		}
		user, err := userRepo.GetUserByID(userID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "failed to load profile"})
		}
		if user == nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "profile not found"})
		}
		phone, _ := user.Metadata["phone"].(string)
		return c.JSON(fiber.Map{
			"id":           user.ID,
			"username":     user.Username,
			"email":        user.Email,
			"display_name": user.DisplayName,
			"phone":        strings.TrimSpace(phone),
			"status":       map[bool]string{true: "active", false: "disabled"}[user.Active],
		})
	})
	protected.Patch("/profile", func(c *fiber.Ctx) error {
		userID, _ := c.Locals("user_id").(string)
		userID = strings.TrimSpace(userID)
		if userID == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "missing user context"})
		}
		var body struct {
			Email            *string `json:"email"`
			DisplayName      *string `json:"display_name"`
			DisplayNameCamel *string `json:"displayName"`
			Phone            *string `json:"phone"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "invalid json"})
		}
		if body.DisplayName == nil {
			body.DisplayName = body.DisplayNameCamel
		}
		if body.Phone != nil {
			v := strings.TrimSpace(*body.Phone)
			if v != "" {
				digits := strings.Map(func(r rune) rune {
					if r >= '0' && r <= '9' {
						return r
					}
					return -1
				}, v)
				if len(digits) != 10 {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "phone must be exactly 10 digits"})
				}
				body.Phone = &digits
			}
		}
		if body.Email != nil {
			email := strings.TrimSpace(*body.Email)
			if email != "" && (!strings.Contains(email, "@") || !strings.Contains(strings.Split(email, "@")[1], ".")) {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "invalid email"})
			}
			body.Email = &email
		}
		updated, err := userRepo.UpdateSelfProfile(userID, body.Email, body.DisplayName, body.Phone)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "failed to update profile"})
		}
		if updated == nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "profile not found"})
		}
		phone, _ := updated.Metadata["phone"].(string)
		return c.JSON(fiber.Map{"ok": true, "profile": fiber.Map{"id": updated.ID, "username": updated.Username, "email": updated.Email, "display_name": updated.DisplayName, "phone": strings.TrimSpace(phone)}})
	})
	protected.Post("/profile/otp/request", func(c *fiber.Ctx) error {
		userID, _ := c.Locals("user_id").(string)
		userID = strings.TrimSpace(userID)
		if userID == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "missing user context"})
		}
		var body struct {
			Channel string `json:"channel"`
			Target  string `json:"target"`
			Purpose string `json:"purpose"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "invalid json"})
		}
		channel := strings.ToLower(strings.TrimSpace(body.Channel))
		target := strings.TrimSpace(body.Target)
		purpose := strings.ToLower(strings.TrimSpace(body.Purpose))
		if channel == "" || target == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "channel and target are required"})
		}
		if purpose == "" {
			purpose = "profile_update"
		}
		otpRef := uuid.NewString()
		otpCode := strings.ReplaceAll(otpRef, "-", "")[:6]
		expiresAt := time.Now().UTC().Add(5 * time.Minute)
		profileOTPMu.Lock()
		profileOTPStore[otpRef] = profileOTPEntry{UserID: userID, Channel: channel, Target: target, Purpose: purpose, Code: otpCode, ExpiresAt: expiresAt}
		profileOTPMu.Unlock()
		return c.JSON(fiber.Map{
			"otp_ref":      otpRef,
			"expires_at":   expiresAt,
			"provider":     "mock",
			"delivery":     "queued",
			"dev_otp_code": otpCode,
		})
	})
	protected.Post("/profile/otp/verify", func(c *fiber.Ctx) error {
		userID, _ := c.Locals("user_id").(string)
		userID = strings.TrimSpace(userID)
		if userID == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "missing user context"})
		}
		var body struct {
			OTPRef           string  `json:"otp_ref"`
			OTP              string  `json:"otp"`
			NewPassword      *string `json:"new_password"`
			NewPasswordCamel *string `json:"newPassword"`
			Email            *string `json:"email"`
			Phone            *string `json:"phone"`
			DisplayName      *string `json:"display_name"`
			DisplayNameCamel *string `json:"displayName"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "invalid json"})
		}
		if body.NewPassword == nil {
			body.NewPassword = body.NewPasswordCamel
		}
		if body.DisplayName == nil {
			body.DisplayName = body.DisplayNameCamel
		}
		otpRef := strings.TrimSpace(body.OTPRef)
		otp := strings.TrimSpace(body.OTP)
		if otpRef == "" || otp == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "otp_ref and otp are required"})
		}
		profileOTPMu.Lock()
		entry, ok := profileOTPStore[otpRef]
		if ok {
			delete(profileOTPStore, otpRef)
		}
		profileOTPMu.Unlock()
		if !ok || entry.UserID != userID {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "invalid otp reference"})
		}
		if time.Now().UTC().After(entry.ExpiresAt) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "otp expired"})
		}
		if otp != entry.Code {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "invalid otp"})
		}

		if body.NewPassword != nil && strings.TrimSpace(*body.NewPassword) != "" {
			if err := authService.ResetPasswordByUserID(userID, strings.TrimSpace(*body.NewPassword)); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "failed to update password"})
			}
		}
		if body.Phone != nil {
			v := strings.TrimSpace(*body.Phone)
			if v != "" {
				digits := strings.Map(func(r rune) rune {
					if r >= '0' && r <= '9' {
						return r
					}
					return -1
				}, v)
				if len(digits) != 10 {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "phone must be exactly 10 digits"})
				}
				body.Phone = &digits
			}
		}
		_, err := userRepo.UpdateSelfProfile(userID, body.Email, body.DisplayName, body.Phone)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "failed to update profile"})
		}
		return c.JSON(fiber.Map{"ok": true, "verified": true})
	})
	protected.Get("/auth/sessions", func(c *fiber.Ctx) error {
		userID, _ := c.Locals("user_id").(string)
		userID = strings.TrimSpace(userID)
		if userID == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "missing user context"})
		}
		limit, _ := strconv.Atoi(strings.TrimSpace(c.Query("limit", "50")))
		sessions, err := userRepo.ListSessionsByUserID(userID, limit)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "failed to list sessions"})
		}
		items := make([]fiber.Map, 0, len(sessions))
		for _, s := range sessions {
			items = append(items, mapSessionView(s))
		}
		return c.JSON(fiber.Map{"count": len(items), "sessions": items})
	})
	protected.Get("/admin/users/:id/sessions", api.RequireCapability([]string{"users:manage", "admin:all"}, false), func(c *fiber.Ctx) error {
		userID := strings.TrimSpace(c.Params("id"))
		if userID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "user id is required"})
		}
		limit, _ := strconv.Atoi(strings.TrimSpace(c.Query("limit", "100")))
		sessions, err := userRepo.ListSessionsByUserID(userID, limit)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "failed to list user sessions"})
		}
		items := make([]fiber.Map, 0, len(sessions))
		for _, s := range sessions {
			items = append(items, mapSessionView(s))
		}
		return c.JSON(fiber.Map{"user_id": userID, "count": len(items), "sessions": items})
	})
	protected.Get("/v1/auth/sessions", func(c *fiber.Ctx) error { return c.Redirect("/api/auth/sessions", fiber.StatusTemporaryRedirect) })
	protected.Get("/v1/profile", func(c *fiber.Ctx) error { return c.Redirect("/api/profile", fiber.StatusTemporaryRedirect) })
	protected.Patch("/v1/profile", func(c *fiber.Ctx) error { return c.Redirect("/api/profile", fiber.StatusTemporaryRedirect) })
	protected.Post("/v1/profile/otp/request", func(c *fiber.Ctx) error { return c.Redirect("/api/profile/otp/request", fiber.StatusTemporaryRedirect) })
	protected.Post("/v1/profile/otp/verify", func(c *fiber.Ctx) error { return c.Redirect("/api/profile/otp/verify", fiber.StatusTemporaryRedirect) })
	protected.Get("/v1/admin/users/:id/sessions", api.RequireCapability([]string{"users:manage", "admin:all"}, false), func(c *fiber.Ctx) error {
		target := "/api/admin/users/" + strings.TrimSpace(c.Params("id")) + "/sessions"
		query := strings.TrimSpace(c.Context().QueryArgs().String())
		if query != "" {
			target += "?" + query
		}
		return c.Redirect(target, fiber.StatusTemporaryRedirect)
	})
	protected.Get("/diagnostics/ingest/deadletter", api.RequireCapability([]string{"diagnostics:commands"}, false), diagnosticsController.GetStatus)
	protected.Post("/diagnostics/ingest/deadletter/replay", api.RequireCapability([]string{"diagnostics:commands"}, false), diagnosticsController.Replay)

	// Intelligence
	protected.Get("/intelligence/anomalies", analyticsController.GetAnomalies)

	// Scheduler
	protected.Get("/scheduler/schedules", api.RequireCapability([]string{"reports:manage", "admin:all"}, false), schedulerController.GetSchedules)
	protected.Post("/scheduler/schedules", api.RequireCapability([]string{"reports:manage", "admin:all"}, false), schedulerController.CreateSchedule)
	protected.Put("/scheduler/schedules/:id/toggle", api.RequireCapability([]string{"reports:manage", "admin:all"}, false), schedulerController.ToggleSchedule)

	// Simulator
	protected.Get("/builder/simulator/:projectId", api.RequireCapability([]string{"simulator:launch"}, false), simController.GetScript)
	protected.Post("/builder/simulator/start", api.RequireCapability([]string{"simulator:launch"}, false), simController.StartSimulation)
	protected.Post("/simulator/sessions", api.RequireCapability([]string{"simulator:launch"}, false), simController.CreateSession)
	protected.Get("/simulator/sessions", api.RequireCapability([]string{"simulator:launch"}, false), simController.ListSessions)
	protected.Delete("/simulator/sessions/:sessionId", api.RequireCapability([]string{"simulator:launch"}, false), simController.RevokeSession)
	protected.Post("/v1/simulator/sessions", api.RequireCapability([]string{"simulator:launch"}, false), simController.CreateSession)
	protected.Get("/v1/simulator/sessions", api.RequireCapability([]string{"simulator:launch"}, false), simController.ListSessions)
	protected.Delete("/v1/simulator/sessions/:sessionId", api.RequireCapability([]string{"simulator:launch"}, false), simController.RevokeSession)

	// Compliance
	protected.Get("/reports/:id/compliance", api.RequireCapability([]string{"reports:manage"}, false), reportController.GenerateComplianceReport)

	// 6. Start HTTP Server
	go func() {
		port := os.Getenv("GO_PORT")
		if port == "" {
			port = "8081"
		}

		if err := app.Listen(":" + port); err != nil {
			log.Fatal(err)
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down...")
}
