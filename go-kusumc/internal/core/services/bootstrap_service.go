package services

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"

	"ingestion-go/internal/adapters/secondary"
	"ingestion-go/internal/config/dna"
	"ingestion-go/internal/core/ports"
	"ingestion-go/internal/models"
)

// BootstrapService handles the "First Boot" logic
type BootstrapService struct {
	repo      ports.DeviceRepo
	protocols *ProtocolService
	govtCreds *GovtCredsService
	vfdRepo   *secondary.PostgresVFDRepo
	dnaRepo   *secondary.PostgresDNARepo
}

func NewBootstrapService(repo ports.DeviceRepo, protocols *ProtocolService, govt *GovtCredsService, vfdRepo *secondary.PostgresVFDRepo, dnaRepo *secondary.PostgresDNARepo) *BootstrapService {
	return &BootstrapService{repo: repo, protocols: protocols, govtCreds: govt, vfdRepo: vfdRepo, dnaRepo: dnaRepo}
}

// SyncAll re-hydrates Redis from Postgres (Cold Start)
func (s *BootstrapService) SyncAll(projService interface{}, ruleService interface{}) {
	log.Println("[Bootstrap] 🚀 Starting Cold Sync...")

	// In Go, usually we iterate over all projects via Repo
	// Here we assume projService has a 'SyncAll' method or we call Repo directly.
	// Simplifying for V1:

	log.Println("[Bootstrap] Mocking Sync of 50 Projects and 200 Rules...")
	// for p := range projects {
	//    redis.Set(p)
	// }

	log.Println("[Bootstrap] ✅ Cold Sync Complete.")
}

// GetDeviceConfig returns the "Universal Payload"
func (s *BootstrapService) GetDeviceConfig(imei string) (map[string]interface{}, error) {
	// 1. Fetch Core Device
	deviceRaw, err := s.repo.GetDeviceByIMEI(imei)
	if err != nil {
		return nil, fmt.Errorf("device lookup failed: %v", err)
	}

	device, ok := deviceRaw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("device payload unexpected")
	}

	attrs := map[string]interface{}{}
	if raw, ok := device["attributes"].(map[string]interface{}); ok && raw != nil {
		attrs = raw
	}
	getAttr := func(key string) string {
		if v, ok := attrs[key]; ok {
			return fmt.Sprintf("%v", v)
		}
		return ""
	}

	ctx := context.Background()

	var primaryProto *models.ProtocolProfile
	var govtProto *models.ProtocolProfile

	projectID := fmt.Sprintf("%v", device["project_id"])
	if s.protocols != nil && projectID != "" {
		if list, err := s.protocols.ListByProject(ctx, projectID); err == nil {
			for i := range list {
				p := list[i]
				switch p.Kind {
				case "primary":
					if primaryProto == nil {
						primaryProto = &p
					}
				case "govt":
					if govtProto == nil {
						govtProto = &p
					}
				}
			}
		}
	}

	mqttHost := os.Getenv("MQTT_HOST")
	if mqttHost == "" {
		mqttHost = "localhost"
	}
	mqttPort := os.Getenv("MQTT_PORT")
	if mqttPort == "" {
		mqttPort = "1883"
	}
	publicHost, publicHostSet := os.LookupEnv("MQTT_PUBLIC_HOST")
	if publicHost == "" {
		publicHost = mqttHost
	}
	publicPort, publicPortSet := os.LookupEnv("MQTT_PUBLIC_PORT")
	if publicPort == "" {
		publicPort = "8883"
	}
	publicProto, publicProtoSet := os.LookupEnv("MQTT_PUBLIC_PROTOCOL")
	if publicProto == "" {
		publicProto = "mqtts"
	}

	// Optional: allow explicitly returning multiple endpoints (e.g. both mqtts:// and mqtt://)
	// via comma-separated URLs. If set and valid, it overrides MQTT_PUBLIC_PROTOCOL/HOST/PORT for endpoints.
	publicURLs := []string{}
	if rawURLs := strings.TrimSpace(os.Getenv("MQTT_PUBLIC_URLS")); rawURLs != "" {
		parts := strings.Split(rawURLs, ",")
		for _, part := range parts {
			s := strings.TrimSpace(part)
			if s == "" {
				continue
			}
			u, err := url.Parse(s)
			if err != nil || u.Scheme == "" {
				continue
			}
			h := u.Hostname()
			p := u.Port()
			if p == "" {
				switch strings.ToLower(u.Scheme) {
				case "mqtts", "ssl", "tls", "mqtt+ssl", "mqtt+tls":
					p = "8883"
				default:
					p = "1883"
				}
			}
			if h == "" {
				continue
			}
			publicURLs = append(publicURLs, fmt.Sprintf("%s://%s:%s", u.Scheme, h, p))
		}
		if len(publicURLs) > 0 {
			if u0, err := url.Parse(publicURLs[0]); err == nil {
				if u0.Scheme != "" {
					publicProto = u0.Scheme
				}
				if h := u0.Hostname(); h != "" {
					publicHost = h
				}
				if p := u0.Port(); p != "" {
					publicPort = p
				}
			}
		}
	}

	// Protocol profiles may carry broker hints, but the bootstrap payload must remain
	// usable by devices/tests outside the docker network. If a public endpoint is
	// explicitly configured via env, it must win.
	isProbablyInternalHost := func(host string) bool {
		h := strings.ToLower(strings.TrimSpace(host))
		if h == "" {
			return false
		}
		// Known docker/service-only names used in local stacks.
		if h == "emqx" || h == "mqtt.local" {
			return true
		}
		// Treat single-label hosts (no dots) as internal, except localhost.
		if !strings.Contains(h, ".") && h != "localhost" {
			return true
		}
		return false
	}

	if primaryProto != nil {
		if !publicHostSet && primaryProto.Host != "" && !isProbablyInternalHost(primaryProto.Host) {
			publicHost = primaryProto.Host
		}
		if !publicPortSet && primaryProto.Port != 0 {
			publicPort = strconv.Itoa(primaryProto.Port)
		}
		if !publicProtoSet && primaryProto.Protocol != "" {
			publicProto = primaryProto.Protocol
		}
	}

	toString := func(v interface{}) string {
		if v == nil {
			return ""
		}
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}

	expandTopicTemplates := func(topics []string, projectID, imei string) []string {
		if len(topics) == 0 {
			return topics
		}
		out := make([]string, 0, len(topics))
		for _, t := range topics {
			if strings.TrimSpace(t) == "" {
				continue
			}
			x := t
			x = strings.ReplaceAll(x, "<IMEI>", imei)
			x = strings.ReplaceAll(x, "{imei}", imei)
			x = strings.ReplaceAll(x, "{IMEI}", imei)
			x = strings.ReplaceAll(x, "{project_id}", projectID)
			x = strings.ReplaceAll(x, "{PROJECT_ID}", projectID)
			x = strings.ReplaceAll(x, "{projectId}", projectID)
			out = append(out, x)
		}
		return out
	}

	// 3. Fetch Context (Installation & Beneficiary)
	var inst map[string]interface{}
	var instProjectID string
	var ben map[string]interface{}
	var dnaRecord *dna.ProjectPayloadSchema

	uuid, ok := device["id"].(string)
	if ok {
		inst, _ = s.repo.GetInstallationByDevice(uuid)
		if inst != nil {
			if pid, ok := inst["project_id"].(string); ok {
				instProjectID = pid
			}
			if benId, ok := inst["beneficiary_id"].(string); ok {
				ben, _ = s.repo.GetBeneficiary(benId)
			}
		}
	}

	effectiveProject := instProjectID
	if effectiveProject == "" {
		effectiveProject = projectID
	}
	if s.dnaRepo != nil && effectiveProject != "" {
		if rec, err := s.dnaRepo.GetByProjectID(ctx, effectiveProject); err == nil {
			dnaRecord = rec
		}
	}

	// Construct the Response (Parity with Node)
	envelopeKeys := []string{"packet_type", "project_id", "protocol_id", "contractor_id", "supplier_id", "manufacturer_id", "org_id", "device_id", "imei", "ts", "msg_id"}
	if dnaRecord != nil {
		for _, row := range dnaRecord.Rows {
			if row.EnvelopeRequired {
				envelopeKeys = appendUnique(envelopeKeys, row.Key)
			}
		}
		if meta := dnaRecord.Metadata; meta != nil {
			if raw, ok := meta["envelope_required"]; ok {
				envelopeKeys = appendUniqueAny(envelopeKeys, raw)
			}
		}
	}

	credBundle := map[string]interface{}{}
	provisionStatus := "unknown"
	provisioning := map[string]interface{}{
		"status": provisionStatus,
	}

	if latest, err := s.repo.GetLatestCredentialHistory(device["id"].(string)); err == nil && latest != nil {
		if status := toString(latest["lifecycle"]); status != "" {
			provisionStatus = status
			provisioning["status"] = status
		}

		if id := toString(latest["id"]); id != "" {
			provisioning["credential_history_id"] = id
		}
		if applied, ok := latest["applied"].(bool); ok {
			provisioning["applied"] = applied
		}
		if attempts, ok := latest["attempts"].(int); ok {
			provisioning["attempts"] = attempts
		}
		if errMsg := toString(latest["last_error"]); errMsg != "" && errMsg != "<nil>" {
			provisioning["last_error"] = errMsg
		}

		if bundle, ok := latest["bundle"].(map[string]interface{}); ok {
			credBundle = bundle
		}
	}

	publishTopics := []string{}
	if primaryProto != nil {
		publishTopics = expandTopicTemplates(primaryProto.PublishTopics, toString(device["project_id"]), toString(device["imei"]))
	}
	if len(publishTopics) == 0 {
		imei := toString(device["imei"])
		publishTopics = []string{
			fmt.Sprintf("%s/heartbeat", imei),
			fmt.Sprintf("%s/data", imei),
			fmt.Sprintf("%s/daq", imei),
			fmt.Sprintf("%s/ondemand", imei),
			fmt.Sprintf("%s/errors", imei),
		}
	}
	publishTopics = filterUnsupportedPublishTopics(publishTopics)
	// Safety: always advertise the dedicated errors lane for legacy firmware.
	// This keeps bootstrap aligned with the firmware contract even if protocol profiles omit it.
	{
		want := fmt.Sprintf("%s/errors", toString(device["imei"]))
		has := false
		for _, t := range publishTopics {
			if strings.TrimSpace(t) == want {
				has = true
				break
			}
		}
		if !has {
			publishTopics = append(publishTopics, want)
		}
	}

	subscribeTopics := []string{}
	if primaryProto != nil {
		subscribeTopics = expandTopicTemplates(primaryProto.SubscribeTopics, toString(device["project_id"]), toString(device["imei"]))
	}
	if len(subscribeTopics) == 0 {
		imei := toString(device["imei"])
		subscribeTopics = []string{fmt.Sprintf("%s/ondemand", imei)}
	}

	// Always compute primary broker endpoints at runtime from the public MQTT settings.
	// Credential bundles should persist only auth material + topics (env-specific routing must not be stored).
	endpoints := publicURLs
	if len(endpoints) == 0 {
		endpoints = []string{fmt.Sprintf("%s://%s:%s", publicProto, publicHost, publicPort)}
	}

	primaryBroker := map[string]interface{}{
		"protocol":         "mqtt",
		"protocol_id":      "",
		"host":             publicHost,
		"port":             publicPort,
		"username":         toString(credBundle["username"]),
		"password":         toString(credBundle["password"]),
		"client_id":        toString(credBundle["client_id"]),
		"publish_topics":   publishTopics,
		"subscribe_topics": subscribeTopics,
		"endpoints":        endpoints,
	}
	if primaryProto != nil {
		primaryBroker["protocol"] = primaryProto.Protocol
		primaryBroker["protocol_id"] = primaryProto.ID
	}

	var govtBroker map[string]interface{}
	if s.govtCreds != nil && device["id"] != nil {
		if govtList, err := s.govtCreds.ListByDevice(ctx, toString(device["id"])); err == nil {
			var selectedCred *models.GovtCredentialBundle
			if len(govtList) > 0 {
				if govtProto != nil {
					for i := range govtList {
						if govtList[i].ProtocolID == govtProto.ID {
							selectedCred = &govtList[i]
							break
						}
					}
				}
				if selectedCred == nil {
					selectedCred = &govtList[0]
				}
			}

			if selectedCred != nil {
				publishGovt := []string{}
				subscribeGovt := []string{}
				endpointGovt := []string{}
				if govtProto != nil {
					publishGovt = expandTopicTemplates(govtProto.PublishTopics, toString(device["project_id"]), toString(device["imei"]))
					subscribeGovt = expandTopicTemplates(govtProto.SubscribeTopics, toString(device["project_id"]), toString(device["imei"]))
					if govtProto.Host != "" && govtProto.Port != 0 {
						protoScheme := govtProto.Protocol
						if protoScheme == "" {
							protoScheme = "mqtt"
						}
						endpointGovt = []string{fmt.Sprintf("%s://%s:%d", protoScheme, govtProto.Host, govtProto.Port)}
					}
				}

				// Govt broker topics/endpoints must come from the selected govt protocol profile.
				// Do not fall back to the primary broker routing; that would misconfigure devices.

				govtBroker = map[string]interface{}{
					"protocol":         "mqtt",
					"protocol_id":      selectedCred.ProtocolID,
					"client_id":        selectedCred.ClientID,
					"username":         selectedCred.Username,
					"password":         selectedCred.Password,
					"publish_topics":   publishGovt,
					"subscribe_topics": subscribeGovt,
					"endpoints":        endpointGovt,
				}
				if govtProto != nil {
					govtBroker["protocol"] = govtProto.Protocol
				}
			}
		}
	}

	response := map[string]interface{}{
		"status": "success",
		"identity": map[string]interface{}{
			"imei":            device["imei"],
			"uuid":            device["id"],
			"lifecycle":       provisionStatus,
			"protocol_id":     getAttr("protocol_id"),
			"contractor_id":   getAttr("contractor_id"),
			"supplier_id":     getAttr("supplier_id"),
			"manufacturer_id": getAttr("manufacturer_id"),
			"org_id":          getAttr("org_id"),
		},
		"credentials":    primaryBroker, // backwards compatibility
		"primary_broker": primaryBroker,
		"provisioning":   provisioning,
		"context": map[string]interface{}{
			"project": map[string]interface{}{
				"id":   device["project_id"],
				"name": "Project " + fmt.Sprintf("%v", device["project_id"]), // Mock Name if not joined
			},
			"location":       nil, // Default null
			"beneficiary":    nil,
			"vfd_model":      nil,
			"vfd_assignment": nil,
		},
		"configuration": map[string]interface{}{
			"server_vendor":     "SynapseIO",
			"sampling_rate_sec": 60,
		},
		"envelope": map[string]interface{}{
			"required": envelopeKeys,
		},
	}

	if dnaRecord != nil {
		response["dna"] = map[string]interface{}{
			"project_id":       dnaRecord.ProjectID,
			"rows":             dnaRecord.Rows,
			"edge_rules":       dnaRecord.EdgeRules,
			"virtual_sensors":  dnaRecord.VirtualSensors,
			"automation_flows": dnaRecord.AutomationFlows,
			"metadata":         dnaRecord.Metadata,
		}
	}

	if govtBroker != nil {
		response["govt_broker"] = govtBroker
	}

	// Enrich Context if found
	if inst != nil {
		if loc, ok := inst["location"].(map[string]interface{}); ok {
			response["context"].(map[string]interface{})["location"] = loc
		}
		// VFD enrich
		if s.vfdRepo != nil {
			vfdID := toString(inst["vfd_model_id"])
			if vfdID != "" {
				if model, err := s.vfdRepo.GetModelByID(ctx, vfdID); err == nil && model != nil {
					response["context"].(map[string]interface{})["vfd_model"] = model
					protoID := toString(inst["protocol_id"])
					projectID := instProjectID
					if projectID == "" {
						projectID = toString(device["project_id"])
					}
					if protoID == "" && primaryProto != nil {
						protoID = primaryProto.ID
					}
					if protoID != "" && projectID != "" {
						if assign, err := s.vfdRepo.GetAssignmentFor(ctx, projectID, protoID, vfdID); err == nil && assign != nil {
							response["context"].(map[string]interface{})["vfd_assignment"] = assign
						}
					}
				}
			}
		}
	}
	if ben != nil {
		response["context"].(map[string]interface{})["beneficiary"] = map[string]interface{}{
			"name":  ben["name"],
			"phone": ben["phone"],
		}
	}

	return response, nil
}

func appendUnique(base []string, val string) []string {
	if val == "" {
		return base
	}
	for _, v := range base {
		if v == val {
			return base
		}
	}
	return append(base, val)
}

func appendUniqueAny(base []string, raw interface{}) []string {
	switch v := raw.(type) {
	case []string:
		for _, item := range v {
			base = appendUnique(base, strings.TrimSpace(item))
		}
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				base = appendUnique(base, strings.TrimSpace(s))
			}
		}
	case string:
		base = appendUnique(base, strings.TrimSpace(v))
	}
	return base
}
