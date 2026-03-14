package pipeline

import (
	"context"
	"ingestion-go/internal/models"
	"ingestion-go/internal/repository"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
)

type Batcher struct {
	InputChan chan *models.EnrichedPayload
	Buffer    []*models.EnrichedPayload
	BatchSize int
	Timeout   time.Duration
}

func NewBatcher(batchSize int) *Batcher {
	return &Batcher{
		InputChan: make(chan *models.EnrichedPayload, batchSize*2),
		Buffer:    make([]*models.EnrichedPayload, 0, batchSize),
		BatchSize: batchSize,
		Timeout:   500 * time.Millisecond,
	}
}

func (b *Batcher) Start() {
	go b.loop()
}

func (b *Batcher) loop() {
	ticker := time.NewTicker(b.Timeout)
	defer ticker.Stop()

	for {
		select {
		case payload := <-b.InputChan:
			b.Buffer = append(b.Buffer, payload)
			if len(b.Buffer) >= b.BatchSize {
				b.flush()
			}
		case <-ticker.C:
			if len(b.Buffer) > 0 {
				b.flush()
			}
		}
	}
}

func (b *Batcher) flush() {
	if repository.DB == nil {
		// Mock Mode or DB not ready
		b.Buffer = b.Buffer[:0]
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows := make([][]interface{}, len(b.Buffer))
	for i, p := range b.Buffer {
		// Map struct fields to DB columns
		// Handle Timestamp (interface{})
		var ts time.Time
		switch v := p.Timestamp.(type) {
		case int64:
			ts = time.Unix(v, 0)
		case float64:
			ts = time.Unix(int64(v), 0)
		case string:
			parsed, err := time.Parse(time.RFC3339, v)
			if err != nil {
				ts = time.Now() // Default fallback
			} else {
				ts = parsed
			}
		default:
			ts = time.Now()
		}

		rows[i] = []interface{}{
			ts,
			p.DeviceUUID, // Was Imei
			p.Data,
			p.MsgID, // Added MsgID
			p.ProjectID,
		}
	}

	// High-speed COPY protocol (Aligned with telemetry_hyper)
	count, err := repository.DB.CopyFrom(
		ctx,
		pgx.Identifier{"telemetry_hyper"}, // Correct Table Name
		[]string{"time", "device_uuid", "data", "msgid", "project_id"}, // Correct Columns
		pgx.CopyFromRows(rows),
	)

	if err != nil {
		log.Printf("❌ Batch Flush Error: %v", err)
	} else {
		// log.Printf("💾 Flushed %d records to Timescale", count)
	}
	_ = count // suppress unused

	// Reset Buffer
	b.Buffer = b.Buffer[:0]
}
