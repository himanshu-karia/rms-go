package services

import (
	"encoding/json"
	"ingestion-go/internal/adapters/secondary"
	"ingestion-go/internal/core/ports"
	"log"
	"sync"
	"time"
)

type ReverifyMetrics struct {
	LastRunAt     time.Time
	LastProject   string
	LastProcessed int
	LastRecovered int
}

type ReverificationService struct {
	repo    *secondary.PostgresRepo
	state   ports.StateStore
	metrics ReverifyMetrics
	totals  map[string]struct {
		Processed int64
		Recovered int64
	}
	mu sync.Mutex
}

func NewReverificationService(repo *secondary.PostgresRepo, state ports.StateStore) *ReverificationService {
	svc := &ReverificationService{repo: repo, state: state, totals: map[string]struct {
		Processed int64
		Recovered int64
	}{}}
	svc.restoreMetrics()
	return svc
}

// ReverifyProject fetches 'suspicious' packets and re-checks them against the Project DNA.
func (s *ReverificationService) ReverifyProject(projectId string) {
	log.Printf("[Reverify] Starting Batch for %s", projectId)
	startedAt := time.Now()

	// 1. Fetch Suspicious Packets
	packets, err := s.repo.GetSuspiciousPackets(projectId)
	if err != nil {
		log.Printf("[Reverify] DB Error: %v", err)
		return
	}

	count := len(packets)
	recovered := 0
	log.Printf("[Reverify] Found %d suspicious packets", count)

	for _, pkt := range packets {
		// Pkt has time, device_id, data
		dataStr, ok := pkt["data"].(string)
		// If driver returns map directly (pgx v5 might), we handle that:
		// But helper returns string for bytes.

		var payload map[string]interface{}
		if ok {
			json.Unmarshal([]byte(dataStr), &payload)
		} else {
			// Maybe it's already a map if driver supported it
			if m, isMap := pkt["data"].(map[string]interface{}); isMap {
				payload = m
			}
		}

		if payload == nil {
			continue
		}

		// 2. Re-Validate
		// [Logic] Check if values are now within bounds (e.g., DNA updated with wider range)
		// For V1, we simulate "If it has 'temp', it is now valid"
		isValid := true
		if _, hasTemp := payload["temp"]; hasTemp {
			// Simulate check
			isValid = true
		}

		if isValid {
			// 3. Update Status
			tVal := pkt["time"].(time.Time)
			dVal := pkt["device_id"].(string)

			err := s.repo.UpdatePacketStatus(tVal, dVal, "verified")
			if err == nil {
				recovered++
				// Optional: Push to Hot Cache
				// s.state.PushPacket(dVal, payload)
			} else {
				log.Printf("[Reverify] Update Failed: %v", err)
			}
		}
	}

	s.recordMetrics(projectId, count, recovered, startedAt)
	log.Printf("[Reverify] Processed %d. Recovered %d packets.", count, recovered)
}

// SnapshotMetrics returns the last run stats for metrics endpoints.
func (s *ReverificationService) SnapshotMetrics() ReverifyMetrics {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.metrics
}

// SnapshotTotals returns cumulative processed/recovered per project.
func (s *ReverificationService) SnapshotTotals() map[string]struct {
	Processed int64
	Recovered int64
} {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]struct {
		Processed int64
		Recovered int64
	}, len(s.totals))
	for k, v := range s.totals {
		out[k] = v
	}
	return out
}

func (s *ReverificationService) recordMetrics(projectId string, processed, recovered int, startedAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metrics = ReverifyMetrics{
		LastRunAt:     startedAt,
		LastProject:   projectId,
		LastProcessed: processed,
		LastRecovered: recovered,
	}

	// Update cumulative totals
	entry := s.totals[projectId]
	entry.Processed += int64(processed)
	entry.Recovered += int64(recovered)
	s.totals[projectId] = entry

	// Persist for visibility across restarts when a raw store is available (e.g., Redis)
	if raw, ok := s.state.(interface {
		SetRaw(key string, val string, ttl time.Duration) error
	}); ok {
		if data, err := json.Marshal(s.metrics); err == nil {
			_ = raw.SetRaw("reverify:metrics", string(data), 0)
		}
		if totalsData, err := json.Marshal(s.totals); err == nil {
			_ = raw.SetRaw("reverify:metrics:totals", string(totalsData), 0)
		}
	}
}

func (s *ReverificationService) restoreMetrics() {
	if raw, ok := s.state.(interface {
		GetRaw(key string) (string, bool, error)
	}); ok {
		if val, found, err := raw.GetRaw("reverify:metrics"); err == nil && found {
			var m ReverifyMetrics
			if jsonErr := json.Unmarshal([]byte(val), &m); jsonErr == nil {
				s.metrics = m
			}
		}
		if val, found, err := raw.GetRaw("reverify:metrics:totals"); err == nil && found {
			var totals map[string]struct {
				Processed int64
				Recovered int64
			}
			if jsonErr := json.Unmarshal([]byte(val), &totals); jsonErr == nil && totals != nil {
				s.totals = totals
			}
		}
	}
}
