package services

import (
	"log"
	"os"
	"strconv"
	"time"

	"ingestion-go/internal/adapters/secondary"
)

type NotificationProcessorService struct {
	repo     *secondary.PostgresRepo
	interval time.Duration
	batch    int
}

func NewNotificationProcessorService(repo *secondary.PostgresRepo) *NotificationProcessorService {
	interval := 1 * time.Minute
	batch := 100
	if v := os.Getenv("NOTIFICATION_PROCESSOR_INTERVAL_SECONDS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			interval = time.Duration(parsed) * time.Second
		}
	}
	if v := os.Getenv("NOTIFICATION_PROCESSOR_BATCH_LIMIT"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			batch = parsed
		}
	}
	return &NotificationProcessorService{repo: repo, interval: interval, batch: batch}
}

func (s *NotificationProcessorService) Start() {
	if s.repo == nil {
		return
	}
	log.Printf("[NotificationProcessor] Started (interval=%s)", s.interval)
	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			s.runOnce()
			<-ticker.C
		}
	}()
}

func (s *NotificationProcessorService) runOnce() {
	items, err := s.repo.ListPendingNotifications(s.batch)
	if err != nil {
		log.Printf("[NotificationProcessor] Error fetching pending: %v", err)
		return
	}
	if len(items) == 0 {
		return
	}
	for _, item := range items {
		id, _ := item["id"].(string)
		if id == "" {
			continue
		}
		dispatchMeta := map[string]interface{}{
			"event":           "notification_dispatched",
			"dispatched_at":   time.Now().UTC().Format(time.RFC3339),
			"processor":       "notification-processor",
			"delivery_status": "sent",
		}
		if channel, _ := item["channel"].(string); channel != "" {
			dispatchMeta["channel"] = channel
		}
		if target, _ := item["target"].(string); target != "" {
			dispatchMeta["target"] = target
		}
		if err := s.repo.UpdateNotificationStatus(id, "sent", dispatchMeta); err != nil {
			log.Printf("[NotificationProcessor] Failed to mark sent %s: %v", id, err)
			failedMeta := map[string]interface{}{
				"event":           "notification_dispatch_failed",
				"failed_at":       time.Now().UTC().Format(time.RFC3339),
				"processor":       "notification-processor",
				"delivery_status": "failed",
				"error":           err.Error(),
			}
			_ = s.repo.UpdateNotificationStatus(id, "failed", failedMeta)
		}
	}
}
