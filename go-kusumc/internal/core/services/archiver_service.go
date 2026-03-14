package services

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/csv"
	"fmt"
	"ingestion-go/internal/adapters/secondary"
	"ingestion-go/internal/adapters/secondary/storage"
	"log"
	"time"

	"github.com/robfig/cron/v3"
)

type ArchiverService struct {
	repo     *secondary.PostgresRepo
	projRepo *secondary.PostgresProjectRepo
	provider storage.StorageProvider
}

func NewArchiverService(repo *secondary.PostgresRepo, proj *secondary.PostgresProjectRepo, prov storage.StorageProvider) *ArchiverService {
	return &ArchiverService{repo: repo, projRepo: proj, provider: prov}
}

func (s *ArchiverService) Start() {
	c := cron.New()
	// Run at 2 AM daily
	c.AddFunc("0 2 * * *", func() {
		log.Println("[Archiver] Starting Daily Archive Job...")
		s.ArchiveData(180)
	})
	c.Start()
	log.Println("[Archiver] Service Initialized. Schedule: 0 2 * * *")
}

func (s *ArchiverService) ArchiveData(days int) {
	cutoff := time.Now().AddDate(0, 0, -days)
	startOfDay := time.Date(cutoff.Year(), cutoff.Month(), cutoff.Day(), 0, 0, 0, 0, time.UTC)
	endOfDay := time.Date(cutoff.Year(), cutoff.Month(), cutoff.Day(), 23, 59, 59, 999, time.UTC)
	dateStr := startOfDay.Format("2006-01-02")

	log.Printf("[Archiver] Archiving for %s...", dateStr)

	// 1. Check existing archives to skip
	ctx := context.Background()
	existingFiles, _ := s.provider.List(ctx, "telemetry_archive_")

	existingMap := make(map[string]bool)
	for _, f := range existingFiles {
		existingMap[f] = true
	}

	// 2. Get Projects
	projects, err := s.projRepo.GetAllProjectsWithConfig()
	if err != nil {
		log.Printf("[Archiver] Failed to fetch projects: %v", err)
		return
	}

	for _, p := range projects {
		projectId := p["id"].(string)
		filename := fmt.Sprintf("telemetry_archive_%s_%s.csv.gz", projectId, dateStr)

		if existingMap[filename] {
			log.Printf("   -> Skipped %s (Exists)", projectId)
			continue
		}

		// 3. Fetch Data
		data, err := s.repo.ExportTelemetry(startOfDay, endOfDay, projectId)
		if err != nil || len(data) == 0 {
			continue // No data
		}

		log.Printf("   -> Archiving %s (%d records)", projectId, len(data))

		// 4. Write CSV.GZ to Buffer
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		cw := csv.NewWriter(gw)

		cw.Write([]string{"time", "device_id", "data"})
		for _, row := range data {
			t := row["time"].(time.Time).Format(time.RFC3339)
			d := fmt.Sprintf("%v", row["device_id"])
			p := fmt.Sprintf("%v", row["data"])
			cw.Write([]string{t, d, p})
		}
		cw.Flush()
		gw.Close() // Essential to flush gzip footer

		// 5. Upload via Provider
		url, err := s.provider.Upload(ctx, filename, &buf)
		if err != nil {
			log.Printf("   -> Upload Failed: %v", err)
		} else {
			log.Printf("   -> Uploaded to %s", url)
		}
	}
}

func (s *ArchiverService) RestoreData(start, end time.Time, projectId string) (string, error) {
	tableName := fmt.Sprintf("telemetry_restore_%s_%d", projectId, time.Now().Unix())

	err := s.repo.CreateTempTelemetryTable(tableName)
	if err != nil {
		return "", fmt.Errorf("failed to create temp table: %v", err)
	}

	current := start
	count := 0
	ctx := context.Background()

	// List once optimization? Or check individually.
	// For range restore, listing helps avoid 404s.
	// We'll trust Download returns error if missing.

	for current.Before(end) || current.Equal(end) {
		dateStr := current.Format("2006-01-02")
		filename := fmt.Sprintf("telemetry_archive_%s_%s.csv.gz", projectId, dateStr)

		// Download Stream
		reader, err := s.provider.Download(ctx, filename)
		if err == nil {
			log.Printf("[Archiver] Restoring %s...", filename)

			// Decompress Stream
			gr, gzErr := gzip.NewReader(reader)
			if gzErr == nil {
				if loadErr := s.repo.CopyTelemetryFromReader(tableName, gr); loadErr == nil {
					count++
				}
				gr.Close()
			}
			reader.Close()
		}

		current = current.AddDate(0, 0, 1)
	}

	if count == 0 {
		log.Println("[Archiver] No archives found/restored.")
	}

	return tableName, nil
}
