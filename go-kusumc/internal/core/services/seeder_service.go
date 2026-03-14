package services

import (
	"context"
	"log"
	"time"

	"ingestion-go/internal/adapters/secondary"
	"ingestion-go/internal/config/dna"
	"ingestion-go/internal/config/payloadschema"
)

type SeederService struct {
	repo        *secondary.PostgresRepo
	userRepo    *secondary.PostgresUserRepo
	authService *AuthService
}

func NewSeederService(repo *secondary.PostgresRepo, userRepo *secondary.PostgresUserRepo, auth *AuthService) *SeederService {
	return &SeederService{repo: repo, userRepo: userRepo, authService: auth}
}

func (s *SeederService) Seed() {
	log.Println("[Seeder] Checking if seeding is required...")

	// Legacy-compatible hierarchy seed (Maharashtra -> MSEDCL -> PM_KUSUM)
	s.seedOrgHierarchy()

	// Core catalog entries (available for all projects/devices).
	s.seedCoreCommands()

	// 1. Ensure legacy admin users
	s.ensureAdminUser("Him", "0554")
	s.ensureAdminUser("Hadi", "6465")

	// PM-KUSUM bootstrap seed (protocols, VFD)
	s.seedPMKUSUM()

	// PM-KUSUM Project DNA seed (packet validation + config bundle generation)
	s.seedPMKUSUMDNA()

	log.Println("[Seeder] Done.")
}

func (s *SeederService) ensureAdminUser(username, password string) {
	user, _ := s.userRepo.GetUserByUsername(username)
	if user == nil {
		log.Printf("[Seeder] Creating Admin User %q...", username)
		err := s.authService.Register(username, password, "admin")
		if err != nil {
			log.Printf("[Seeder] Failed to create admin user %q: %v", username, err)
		} else {
			log.Printf("[Seeder] ✅ Admin User %q Created", username)
		}
	} else {
		log.Printf("[Seeder] ✅ Admin User %q already exists", username)
	}

	user, _ = s.userRepo.GetUserByUsername(username)
	if user != nil {
		if err := s.userRepo.GrantAllCapabilities(user.ID); err != nil {
			log.Printf("[Seeder] Failed to grant capabilities to %q: %v", username, err)
		} else {
			log.Printf("[Seeder] ✅ Admin capabilities ensured for %q", username)
		}
	}
}

func (s *SeederService) seedOrgHierarchy() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stateID := "11111111-1111-1111-1111-111111111111"
	authorityID := "22222222-2222-2222-2222-222222222222"
	projectNodeID := "33333333-3333-3333-3333-333333333333"

	_, _ = s.repo.Pool.Exec(ctx, `
		INSERT INTO organizations (id, name, type, path, parent_id, metadata)
		VALUES
		  ($1, 'Maharashtra', 'govt', 'India.Maharashtra', NULL, '{}'::jsonb),
		  ($2, 'MSEDCL', 'govt', 'India.Maharashtra.MSEDCL', $1, '{}'::jsonb),
		  ($3, 'PM_KUSUM', 'project', 'India.Maharashtra.MSEDCL.PM_KUSUM', $2, '{}'::jsonb)
		ON CONFLICT (id) DO NOTHING
	`, stateID, authorityID, projectNodeID)

	log.Println("[Seeder] Seeded hierarchy (Maharashtra -> MSEDCL -> PM_KUSUM)")
}

func (s *SeederService) seedCoreCommands() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.repo.Pool.Exec(ctx, `
		INSERT INTO command_catalog (name, scope, project_id, payload_schema, transport)
		SELECT * FROM (
			VALUES
			  ('reboot'::text, 'core'::text, NULL::text, '{"type":"object","properties":{},"additionalProperties":false}'::jsonb, 'mqtt'::text),
			  ('rebootstrap'::text, 'core'::text, NULL::text, '{"type":"object","properties":{},"additionalProperties":false}'::jsonb, 'mqtt'::text),
			  ('set_ping_interval_sec'::text, 'core'::text, NULL::text, '{"type":"object","properties":{"interval_sec":{"type":"integer","minimum":5,"maximum":86400,"examples":[60]}},"required":["interval_sec"],"additionalProperties":false}'::jsonb, 'mqtt'::text),
			  ('send_immediate'::text, 'core'::text, NULL::text, '{"type":"object","properties":{},"additionalProperties":false}'::jsonb, 'mqtt'::text),
			  ('apply_device_configuration'::text, 'core'::text, NULL::text, '{"type":"object","properties":{"config_id":{"type":"string"},"config":{"type":"object"}},"required":["config_id","config"],"additionalProperties":true}'::jsonb, 'mqtt'::text)
		) AS v(name, scope, project_id, payload_schema, transport)
		WHERE NOT EXISTS (
			SELECT 1 FROM command_catalog c WHERE c.scope = 'core' AND c.project_id IS NULL AND c.name = v.name
		)
	`)
	if err != nil {
		log.Printf("[Seeder] Failed to seed core commands: %v", err)
		return
	}
	log.Println("[Seeder] ✅ Core command catalog ensured")
}

func (s *SeederService) seedPMKUSUM() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	projectID := "pm-kusum-solar-pump-msedcl"
	primaryProtoID := "proto-pm-primary"
	govtProtoID := "proto-pm-govt"
	manufacturerID := "vfd-manu-seed"
	vfdModelID := "vfd-model-seed"
	assignmentID := "assign-proto-vfd-seed"

	_, _ = s.repo.Pool.Exec(ctx, `
		INSERT INTO projects (id, name, type, location, config)
		VALUES ($1, $2, 'energy', 'Maharashtra', '{}'::jsonb)
		ON CONFLICT (id) DO NOTHING
	`, projectID, "PM-KUSUM Solar Pump RMS")

	_, _ = s.repo.Pool.Exec(ctx, `
		INSERT INTO protocols (id, project_id, kind, protocol, host, port, publish_topics, subscribe_topics, metadata)
		VALUES ($1,$2,'primary','mqtt',$3,$4,$5,$6,$7)
		ON CONFLICT (id) DO NOTHING
	`, primaryProtoID, projectID, "mqtt.local", 1883, []string{"<IMEI>/heartbeat", "<IMEI>/data", "<IMEI>/daq", "<IMEI>/ondemand"}, []string{"<IMEI>/ondemand"}, map[string]any{"seed": true})

	_, _ = s.repo.Pool.Exec(ctx, `
		INSERT INTO protocols (id, project_id, kind, protocol, host, port, publish_topics, subscribe_topics, metadata)
		VALUES ($1,$2,'govt','mqtts',$3,$4,$5,$6,$7)
		ON CONFLICT (id) DO NOTHING
	`, govtProtoID, projectID, "govt-broker.example.com", 8883, []string{"<IMEI>/heartbeat", "<IMEI>/data", "<IMEI>/daq", "<IMEI>/ondemand"}, []string{"<IMEI>/ondemand"}, map[string]any{"seed": true, "tls": true})

	_, _ = s.repo.Pool.Exec(ctx, `
		INSERT INTO vfd_manufacturers (id, project_id, name, metadata)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (id) DO NOTHING
	`, manufacturerID, projectID, "Seed VFD OEM", map[string]any{"seed": true})

	rs485 := map[string]any{"baud_rate": 9600, "data_bits": 8, "parity": "N", "stop_bits": 1}
	realtime := []map[string]any{{"address": 1, "key": "output_freq", "unit": "Hz"}}
	faults := []map[string]any{{"code": 1, "message": "Overvoltage"}}
	commands := []map[string]any{{"address": 100, "label": "start", "value": 1}}
	_, _ = s.repo.Pool.Exec(ctx, `
		INSERT INTO vfd_models (id, project_id, manufacturer_id, model, version, rs485, realtime_parameters, fault_map, command_dictionary, metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (id) DO NOTHING
	`, vfdModelID, projectID, manufacturerID, "Seed-Model", "v1", rs485, realtime, faults, commands, map[string]any{"seed": true})

	_, _ = s.repo.Pool.Exec(ctx, `
		INSERT INTO protocol_vfd_assignments (id, project_id, protocol_id, vfd_model_id, assigned_by, assigned_at, metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (id) DO NOTHING
	`, assignmentID, projectID, primaryProtoID, vfdModelID, "seeder", time.Now(), map[string]any{"seed": true})

	// Optionally attach govt creds seed to a sample device when devices exist; skipped here to avoid tight coupling.

	log.Println("[Seeder] Seeded PM-KUSUM baseline (project, protocols, VFD)")
}

func (s *SeederService) seedPMKUSUMDNA() {
	projectID := "pm-kusum-solar-pump-msedcl"

	record := dna.ProjectPayloadSchema{
		ProjectID: projectID,
		Rows: []payloadschema.Entry{
			{
				PacketType:    "heartbeat",
				ExpectedFor:   "project",
				ScopeID:       projectID,
				Key:           "timestamp",
				Description:   "Telemetry timestamp",
				Required:      true,
				TopicTemplate: "<IMEI>/heartbeat",
			},
			{
				PacketType:    "data",
				ExpectedFor:   "project",
				ScopeID:       projectID,
				Key:           "timestamp",
				Description:   "Telemetry timestamp",
				Required:      true,
				TopicTemplate: "<IMEI>/data",
			},
			{
				PacketType:    "daq",
				ExpectedFor:   "project",
				ScopeID:       projectID,
				Key:           "timestamp",
				Description:   "Telemetry timestamp",
				Required:      true,
				TopicTemplate: "<IMEI>/daq",
			},
			{
				PacketType:    "ondemand_response",
				ExpectedFor:   "project",
				ScopeID:       projectID,
				Key:           "status",
				Description:   "Command response status",
				Required:      false,
				TopicTemplate: "<IMEI>/ondemand",
			},
		},
		Metadata: map[string]any{
			"seededBy": "SeederService",
			"profile":  "pm_kusum_baseline",
		},
	}

	if err := s.repo.UpsertProjectDNA(record); err != nil {
		log.Printf("[Seeder] Failed to seed PM-KUSUM Project DNA: %v", err)
		return
	}

	log.Println("[Seeder] Seeded PM-KUSUM Project DNA")
}
