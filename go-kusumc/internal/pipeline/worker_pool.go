package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"ingestion-go/internal/engine" // [NEW]
	"ingestion-go/internal/models"
	"ingestion-go/internal/repository"
	"log"
	"time"
)

type WorkerPool struct {
	JobChan    chan []byte
	NumWorkers int
	Batcher    *Batcher
	Publish    func(topic string, payload interface{}) // [NEW] Callback for MQTT
}

func NewWorkerPool(bufferSize, numWorkers int, batcher *Batcher) *WorkerPool {
	return &WorkerPool{
		JobChan:    make(chan []byte, bufferSize),
		NumWorkers: numWorkers,
		Batcher:    batcher,
	}
}

func (wp *WorkerPool) Start() {
	log.Printf("🚀 Starting Worker Pool with %d workers...", wp.NumWorkers)
	for i := 0; i < wp.NumWorkers; i++ {
		go wp.worker(i)
	}
}

func (wp *WorkerPool) worker(id int) {
	for payload := range wp.JobChan {
		ctx := context.Background()
		// 1. Decode
		var packet models.TelemetryPayload
		if err := json.Unmarshal(payload, &packet); err != nil {
			continue
		}

		if packet.MsgID == "" {
			packet.MsgID = "gen-" + time.Now().String()
		}

		// 2. Deduplication
		if repository.Rdb != nil {
			origin := packet.Imei
			packetType := ""
			ts := ""
			if packet.Data != nil {
				if pt, ok := packet.Data["packet_type"].(string); ok {
					packetType = pt
				}
				if meta, ok := packet.Data["metadata"].(map[string]interface{}); ok && meta != nil {
					if nodeID, ok := meta["origin_node_id"].(string); ok && nodeID != "" {
						origin = nodeID
					} else if o, ok := meta["origin_imei"].(string); ok && o != "" {
						origin = o
					}
				}
				if tsv, ok := packet.Data["ts"].(string); ok {
					ts = tsv
				} else if tsv, ok := packet.Data["timestamp"].(string); ok {
					ts = tsv
				}
			}

			dedupKey := ""
			if packet.MsgID != "" {
				dedupKey = fmt.Sprintf("dedup:%s:%s", origin, packet.MsgID)
			} else if packetType != "" && ts != "" {
				dedupKey = fmt.Sprintf("dedup:%s:%s:%s", origin, packetType, ts)
			}
			ttl := repository.DedupTTLFromEnv()
			isUnique, err := repository.DeduplicateKey(ctx, dedupKey, ttl)
			if err != nil {
				log.Printf("⚠️ Redis Dedup Error: %v", err)
			} else if !isUnique {
				log.Printf("♻️ Duplicate message dropped: %s", dedupKey)
				continue
			}
		}

		// 3. Enrichment
		projectID := "unknown"
		deviceUUID := ""

		if repository.Rdb != nil {
			meta, err := repository.GetDeviceMetadata(ctx, packet.Imei)
			if err == nil && len(meta) > 0 {
				if pid, ok := meta["projectId"]; ok {
					projectID = pid
				}
				if uuid, ok := meta["uuid"]; ok {
					deviceUUID = uuid
				}
			}
		}

		// Fallback to envelope fields when Redis metadata is missing (RMS-style payloads)
		if projectID == "unknown" {
			if packet.ProjectID != "" {
				projectID = packet.ProjectID
			} else if packet.ProjectIDAlt != "" {
				projectID = packet.ProjectIDAlt
			}
		}

		if deviceUUID == "" && packet.DeviceUUID != "" {
			deviceUUID = packet.DeviceUUID
		}

		// Final fallback: direct lookup from Timescale devices table
		if repository.DB != nil && (projectID == "unknown" || deviceUUID == "") {
			ctxDb, cancelDb := context.WithTimeout(ctx, 2*time.Second)
			row := repository.DB.QueryRow(ctxDb, `select id, project_id from devices where imei = $1 limit 1`, packet.Imei)
			var dbUUID, dbProjectID string
			if err := row.Scan(&dbUUID, &dbProjectID); err == nil {
				if deviceUUID == "" {
					deviceUUID = dbUUID
				}
				if projectID == "unknown" && dbProjectID != "" {
					projectID = dbProjectID
				}
			}
			cancelDb()
		}

		// --- [NEW] Engine: Payload Verification ---
		quality := "verified"
		var verifyErr string

		if projectID != "unknown" {
			projectConfig, err := engine.Loader.GetProject(ctx, projectID)
			if err == nil && projectConfig != nil {
				// Verify Keys
				valid, unknownKeys := engine.VerifyPayload(projectConfig, packet.Data)
				if !valid {
					quality = "suspicious"
					verifyErr = fmt.Sprintf("Unknown Keys: %v", unknownKeys)
					log.Printf("⚠️ Device %s sent unknown keys: %v", packet.Imei, unknownKeys)
				}

				// Transformation (Virtual Sensor)
				packet.Data = engine.TransformPayload(projectConfig, packet.Data)

				// --- [NEW] Engine: Rule Evaluation ---
				rules, err := engine.Loader.GetRules(ctx, projectID)
				if err == nil && len(rules) > 0 {
					results := engine.EvaluateRules(packet.Data, rules)

					// Execute Actions (with Cooldown & MQTT)
					if len(results) > 0 && wp.Publish != nil {
						engine.ProcessActions(ctx, results, packet.Imei, projectID, wp.Publish)
					}
				}
			} else {
				// Project Config Missing in Redis?
				// Maybe sync is lagging. Treat as trusted or warn?
				// For now: Log
				// log.Printf("⚠️ Config missing for project %s", projectID)
			}
		} else {
			// Unknown Project implies unprovisioned device?
			quality = "unprovisioned"
		}

		// 4. Hot Path (Redis Pipeline)
		if repository.Rdb != nil {
			pipe := repository.Rdb.Pipeline()

			// A. Latest State (Shadow)
			if deviceUUID != "" {
				shadowKey := fmt.Sprintf("device_shadow:%s", deviceUUID)
				// Flatten Data
				flattened := make(map[string]interface{})
				for k, v := range packet.Data {
					flattened[k] = v
				}
				pipe.HSet(ctx, shadowKey, flattened)
			}

			// B. Packet History (Sparklines)
			hotKey := fmt.Sprintf("telemetry:hot:%s", packet.Imei)
			hotPacket := map[string]interface{}{
				"timestamp": packet.Timestamp,
				"payload":   packet.Data,
				"msgid":     packet.MsgID,
			}
			hotPacketJSON, _ := json.Marshal(hotPacket)

			pipe.LPush(ctx, hotKey, hotPacketJSON)
			pipe.LTrim(ctx, hotKey, 0, 49)

			// C. Real-time Feed
			pipe.Publish(ctx, "device:updates", hotPacketJSON)

			// Execute
			_, err := pipe.Exec(ctx)
			if err != nil {
				log.Printf("⚠️ Hot Path Pipeline Error: %v", err)
			}
		}

		// 5. Cold Path (Timescale Batch)
		enriched := &models.EnrichedPayload{
			TelemetryPayload:  packet,
			Imei:              packet.Imei,
			ProjectID:         projectID,
			DeviceUUID:        deviceUUID, // Pass to Batcher
			ReceivedAt:        time.Now(),
			Quality:           quality,
			VerificationError: verifyErr,
		}

		select {
		case wp.Batcher.InputChan <- enriched:
		default:
			log.Println("⚠️ Batcher Full, dropping DB write")
		}
	}
}
