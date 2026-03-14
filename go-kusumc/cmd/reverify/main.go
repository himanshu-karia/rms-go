package main

import (
	"flag"
	"log"
	"os"
	"strings"
	"time"

	"ingestion-go/internal/adapters/secondary"
	"ingestion-go/internal/core/services"
)

func main() {
	project := flag.String("project", "", "project ID to reverify")
	projectsCSV := flag.String("projects", "", "comma-separated project IDs to reverify")
	flag.Parse()

	targets := collectTargets(*project, *projectsCSV)
	if len(targets) == 0 {
		log.Fatal("specify -project or -projects")
	}

	redisURL := os.Getenv("REDIS_URL")
	pgURL := os.Getenv("TIMESCALE_URI")

	redisStore := secondary.NewRedisStore(redisURL)
	pgRepo, err := secondary.NewPostgresRepo(pgURL)
	if err != nil {
		log.Fatalf("failed to connect to Postgres: %v", err)
	}

	svc := services.NewReverificationService(pgRepo, redisStore)

	for _, pid := range targets {
		log.Printf("[CLI] Reverify %s", pid)
		svc.ReverifyProject(pid)
	}

	m := svc.SnapshotMetrics()
	log.Printf("[CLI] Last run project=%s processed=%d recovered=%d at=%s", m.LastProject, m.LastProcessed, m.LastRecovered, m.LastRunAt.Format(time.RFC3339))
}

func collectTargets(single string, csv string) []string {
	targets := []string{}
	if single != "" {
		targets = append(targets, strings.TrimSpace(single))
	}
	if csv != "" {
		for _, p := range strings.Split(csv, ",") {
			if t := strings.TrimSpace(p); t != "" {
				targets = append(targets, t)
			}
		}
	}
	return targets
}
