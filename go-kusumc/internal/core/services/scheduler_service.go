package services

import (
	"ingestion-go/internal/adapters/secondary"
	"log"
	"time"
)

type SchedulerService struct {
	repo *secondary.PostgresRepo
	// mqtt adapter
}

func NewSchedulerService(repo *secondary.PostgresRepo) *SchedulerService {
	return &SchedulerService{repo: repo}
}

func (s *SchedulerService) Start() {
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for range ticker.C {
			s.processSchedules()
		}
	}()
	log.Println("[Scheduler] Started (1m interval)")
}

func (s *SchedulerService) processSchedules() {
	// 1. Get Due Schedules from DB
	schedules, err := s.repo.GetDueSchedules()
	if err != nil {
		log.Printf("[Scheduler] Error fetching schedules: %v", err)
		return
	}

	// 2. Execute
	for _, sch := range schedules {
		// Publish MQTT Command (Mocked)
		log.Printf("[Scheduler] ⏰ Executing Schedule %s (Command: %v)", sch["id"], sch["command"])
		// s.mqtt.Publish("channels/" + sch["project_id"] + "/cmd", sch["command"])
	}
}
