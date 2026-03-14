package workers

import (
	"log"
	"os"
	"strconv"
	"time"

	"ingestion-go/internal/core/ports"
	"ingestion-go/internal/core/services"
)

type CommandRetryWorker struct {
	repo   ports.CommandRepo
	svc    *services.CommandsService
	ticker *time.Ticker
	age    time.Duration
	batch  int
	max    int
}

func NewCommandRetryWorker(repo ports.CommandRepo, svc *services.CommandsService) *CommandRetryWorker {
	ageSeconds := 15
	if v := os.Getenv("COMMAND_RETRY_AGE_SECONDS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			ageSeconds = parsed
		}
	}
	batchSize := 10
	if v := os.Getenv("COMMAND_RETRY_BATCH"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			batchSize = parsed
		}
	}
	maxRetries := 3
	if v := os.Getenv("COMMAND_RETRY_MAX"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
			maxRetries = parsed
		}
	}
	return &CommandRetryWorker{
		repo:  repo,
		svc:   svc,
		age:   time.Duration(ageSeconds) * time.Second,
		batch: batchSize,
		max:   maxRetries,
	}
}

func (w *CommandRetryWorker) Start() {
	interval := 5 * time.Second
	if v := os.Getenv("COMMAND_RETRY_INTERVAL_MS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			interval = time.Duration(parsed) * time.Millisecond
		}
	}
	log.Printf("[CommandRetryWorker] starting; interval=%v age=%v batch=%d maxRetries=%d", interval, w.age, w.batch, w.max)
	w.ticker = time.NewTicker(interval)
	go func() {
		for range w.ticker.C {
			w.tick()
		}
	}()
}

func (w *CommandRetryWorker) tick() {
	cutoff := time.Now().Add(-w.age)
	items, err := w.repo.ListPendingRetries(cutoff, w.batch)
	if err != nil {
		log.Printf("[CommandRetryWorker] fetch error: %v", err)
		return
	}
	for _, req := range items {
		if w.max >= 0 && req.Retries >= w.max {
			now := time.Now()
			_ = w.repo.UpdateCommandRequestStatus(req.CorrelationID, "timeout", nil, &now, &req.Retries)
			log.Printf("[CommandRetryWorker] marking timeout for %s after %d retries", req.ID, req.Retries)
			continue
		}
		if err := w.svc.RetryPublish(req); err != nil {
			log.Printf("[CommandRetryWorker] retry failed for %s: %v", req.ID, err)
		}
	}
}
