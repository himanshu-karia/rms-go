package workers

import (
	"encoding/json"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type deadLetterStore interface {
	PopDeadLetters(queue string, count int) ([]string, error)
	PushDeadLetter(queue string, val string, maxLen int, ttl time.Duration) error
	ListLength(key string) (int64, error)
	IncrementCounter(key string, ttl time.Duration) (int64, error)
}

type deadLetterIngestor interface {
	ProcessPacket(topic string, payload []byte, projectID string) error
}

type DeadLetterReplayResult struct {
	QueueLenBefore int64 `json:"queue_len_before"`
	Pulled         int   `json:"pulled"`
	Replayed       int   `json:"replayed"`
	Failed         int   `json:"failed"`
	Requeued       int   `json:"requeued"`
	BufferFull     int   `json:"buffer_full"`
	QueueLenAfter  int64 `json:"queue_len_after"`
}

type DeadLetterReplayWorker struct {
	store       deadLetterStore
	ingestor    deadLetterIngestor
	ticker      *time.Ticker
	queue       string
	batch       int
	enabled     bool
	requeueTTL  time.Duration
	requeueKeep int
}

type deadLetterEntry struct {
	Topic      string                 `json:"topic"`
	IMEI       string                 `json:"imei,omitempty"`
	ProjectID  string                 `json:"project_id,omitempty"`
	Payload    map[string]interface{} `json:"payload"`
	Reason     string                 `json:"reason,omitempty"`
	OccurredAt string                 `json:"occurred_at,omitempty"`
	RetryCount int                    `json:"retry_count,omitempty"`
	LastError  string                 `json:"last_error,omitempty"`
}

func NewDeadLetterReplayWorker(store deadLetterStore, ingestor deadLetterIngestor) *DeadLetterReplayWorker {
	enabled := false
	if v := strings.TrimSpace(os.Getenv("INGEST_DEADLETTER_REPLAY_ENABLED")); v != "" {
		enabled = strings.EqualFold(v, "true") || strings.EqualFold(v, "1") || strings.EqualFold(v, "yes")
	}
	batch := 50
	if v := strings.TrimSpace(os.Getenv("INGEST_DEADLETTER_REPLAY_BATCH")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			batch = n
		}
	}
	requeueKeep := 10000
	if v := strings.TrimSpace(os.Getenv("INGEST_DEADLETTER_REQUEUE_MAX")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			requeueKeep = n
		}
	}
	requeueTTL := 7 * 24 * time.Hour
	if v := strings.TrimSpace(os.Getenv("INGEST_DEADLETTER_REQUEUE_TTL_HOURS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			requeueTTL = time.Duration(n) * time.Hour
		}
	}
	return &DeadLetterReplayWorker{
		store:       store,
		ingestor:    ingestor,
		queue:       "ingest:deadletter",
		batch:       batch,
		enabled:     enabled,
		requeueTTL:  requeueTTL,
		requeueKeep: requeueKeep,
	}
}

func (w *DeadLetterReplayWorker) Start() {
	if !w.enabled {
		log.Println("[DeadLetterReplayWorker] disabled (INGEST_DEADLETTER_REPLAY_ENABLED=false)")
		return
	}
	if w.store == nil || w.ingestor == nil {
		log.Println("[DeadLetterReplayWorker] missing dependencies; not starting")
		return
	}
	interval := 15 * time.Second
	if v := strings.TrimSpace(os.Getenv("INGEST_DEADLETTER_REPLAY_INTERVAL_MS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			interval = time.Duration(n) * time.Millisecond
		}
	}
	w.ticker = time.NewTicker(interval)
	log.Printf("[DeadLetterReplayWorker] started interval=%v batch=%d", interval, w.batch)
	go func() {
		for range w.ticker.C {
			res := w.ReplayNow(w.batch)
			if res.Pulled > 0 {
				log.Printf("[DeadLetterReplayWorker] replayed=%d failed=%d requeued=%d buffer_full=%d queue_before=%d queue_after=%d",
					res.Replayed, res.Failed, res.Requeued, res.BufferFull, res.QueueLenBefore, res.QueueLenAfter)
			}
		}
	}()
}

func (w *DeadLetterReplayWorker) QueueLength() int64 {
	if w.store == nil {
		return 0
	}
	n, err := w.store.ListLength(w.queue)
	if err != nil {
		return 0
	}
	return n
}

func (w *DeadLetterReplayWorker) ReplayNow(limit int) DeadLetterReplayResult {
	res := DeadLetterReplayResult{}
	if w.store == nil || w.ingestor == nil {
		return res
	}
	if limit <= 0 {
		limit = w.batch
	}
	if q, err := w.store.ListLength(w.queue); err == nil {
		res.QueueLenBefore = q
	}
	items, err := w.store.PopDeadLetters(w.queue, limit)
	if err != nil {
		log.Printf("[DeadLetterReplayWorker] pop error: %v", err)
		if q, e2 := w.store.ListLength(w.queue); e2 == nil {
			res.QueueLenAfter = q
		}
		return res
	}
	res.Pulled = len(items)
	for _, raw := range items {
		entry := deadLetterEntry{}
		if err := json.Unmarshal([]byte(raw), &entry); err != nil {
			res.Failed++
			res.Requeued++
			_ = w.store.PushDeadLetter(w.queue, raw, w.requeueKeep, w.requeueTTL)
			_, _ = w.store.IncrementCounter("metrics:ingest:deadletter_replay_failed_total", 0)
			continue
		}
		payloadBlob, err := json.Marshal(entry.Payload)
		if err != nil {
			res.Failed++
			entry.RetryCount++
			entry.LastError = err.Error()
			entry.Reason = "deadletter_replay_marshal_failed"
			if b, e := json.Marshal(entry); e == nil {
				_ = w.store.PushDeadLetter(w.queue, string(b), w.requeueKeep, w.requeueTTL)
				res.Requeued++
			}
			_, _ = w.store.IncrementCounter("metrics:ingest:deadletter_replay_failed_total", 0)
			continue
		}
		projectID := strings.TrimSpace(entry.ProjectID)
		if projectID == "" {
			if v, ok := entry.Payload["project_id"].(string); ok {
				projectID = strings.TrimSpace(v)
			}
		}
		err = w.ingestor.ProcessPacket(strings.TrimSpace(entry.Topic), payloadBlob, projectID)
		if err != nil {
			res.Failed++
			entry.RetryCount++
			entry.LastError = err.Error()
			entry.Reason = "deadletter_replay_failed"
			if b, e := json.Marshal(entry); e == nil {
				_ = w.store.PushDeadLetter(w.queue, string(b), w.requeueKeep, w.requeueTTL)
				res.Requeued++
			}
			_, _ = w.store.IncrementCounter("metrics:ingest:deadletter_replay_failed_total", 0)
			if strings.Contains(strings.ToLower(err.Error()), "buffer full") {
				res.BufferFull++
				break
			}
			continue
		}
		res.Replayed++
		_, _ = w.store.IncrementCounter("metrics:ingest:deadletter_replay_success_total", 0)
	}
	if q, err := w.store.ListLength(w.queue); err == nil {
		res.QueueLenAfter = q
	}
	return res
}
