package services

import (
	"ingestion-go/internal/adapters/secondary"
	"log"
	"time"
)

type AnalyticsWorker struct {
	repo     *secondary.PostgresRepo
	jobQueue chan string // Channel for job IDs
}

func NewAnalyticsWorker(repo *secondary.PostgresRepo) *AnalyticsWorker {
	return &AnalyticsWorker{
		repo:     repo,
		jobQueue: make(chan string, 100),
	}
}

func (w *AnalyticsWorker) Start() {
	log.Println("[AnalyticsWorker] Started Background Worker")

	// Poller for Pending Jobs (Pull Mode)
	// In a real system, we might use LISTEN/NOTIFY or Redis Queue
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		for range ticker.C {
			w.processPendingJobs()
		}
	}()
}

func (w *AnalyticsWorker) processPendingJobs() {
	// 1. Fetch Job
	job, err := w.repo.GetPendingAnalyticsJob()
	if err != nil {
		log.Printf("[AnalyticsWorker] Error fetching job: %v", err)
		return
	}
	if job == nil {
		return // No jobs
	}

	jobID := job["id"].(string)
	jobType := job["type"].(string)
	params, _ := job["parameters"].(map[string]interface{})

	log.Printf("[AnalyticsWorker] Processing Job %s (Type: %s)", jobID, jobType)

	// 2. Execute Actual SQL
	result, err := w.repo.RunAggregationQuery(jobType, params)
	if err != nil {
		log.Printf("[AnalyticsWorker] Query Failed: %v", err)
		w.repo.UpdateAnalyticsJob(jobID, "failed", map[string]string{"error": err.Error()})
		return
	}

	// 3. Update Job Status
	w.repo.UpdateAnalyticsJob(jobID, "completed", result)
	log.Printf("[AnalyticsWorker] Job %s Completed", jobID)
}
