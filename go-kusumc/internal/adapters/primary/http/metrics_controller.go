package http

import (
	"fmt"
	"runtime"
	"strings"

	"ingestion-go/internal/core/services"

	"github.com/gofiber/fiber/v2"
)

type MetricsController struct {
	reverify *services.ReverificationService
}

func NewMetricsController(reverify *services.ReverificationService) *MetricsController {
	return &MetricsController{reverify: reverify}
}

// GetMetrics returns basic Go runtime stats in Prometheus format
func (c *MetricsController) GetMetrics(ctx *fiber.Ctx) error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Format: <metric_name> <value>
	metrics := ""

	// Memory
	metrics += fmt.Sprintf("# HELP go_mem_alloc_bytes Current bytes allocated to heap\n")
	metrics += fmt.Sprintf("# TYPE go_mem_alloc_bytes gauge\n")
	metrics += fmt.Sprintf("go_mem_alloc_bytes %d\n", m.Alloc)

	metrics += fmt.Sprintf("# HELP go_mem_sys_bytes Total bytes obtained from system\n")
	metrics += fmt.Sprintf("# TYPE go_mem_sys_bytes gauge\n")
	metrics += fmt.Sprintf("go_mem_sys_bytes %d\n", m.Sys)

	metrics += fmt.Sprintf("# HELP go_gc_count Total number of completed GC cycles\n")
	metrics += fmt.Sprintf("# TYPE go_gc_count counter\n")
	metrics += fmt.Sprintf("go_gc_count %d\n", m.NumGC)

	// Goroutines
	metrics += fmt.Sprintf("# HELP go_goroutines Number of goroutines that currently exist\n")
	metrics += fmt.Sprintf("# TYPE go_goroutines gauge\n")
	metrics += fmt.Sprintf("go_goroutines %d\n", runtime.NumGoroutine())

	// CPU (Basic)
	metrics += fmt.Sprintf("# HELP go_threads Number of OS threads created\n")
	metrics += fmt.Sprintf("# TYPE go_threads gauge\n")
	metrics += fmt.Sprintf("go_threads %d\n", runtime.NumCPU()) // Not exactly threads but useful context

	metrics += fmt.Sprintf("# HELP mobile_auth_request_otp_total Total mobile OTP request calls\n")
	metrics += fmt.Sprintf("# TYPE mobile_auth_request_otp_total counter\n")
	metrics += fmt.Sprintf("mobile_auth_request_otp_total %d\n", mobileAuthRequestOtpTotal.Load())

	metrics += fmt.Sprintf("# HELP mobile_auth_verify_total Total mobile OTP verify calls\n")
	metrics += fmt.Sprintf("# TYPE mobile_auth_verify_total counter\n")
	metrics += fmt.Sprintf("mobile_auth_verify_total %d\n", mobileAuthVerifyTotal.Load())

	metrics += fmt.Sprintf("# HELP mobile_auth_refresh_total Total mobile refresh calls\n")
	metrics += fmt.Sprintf("# TYPE mobile_auth_refresh_total counter\n")
	metrics += fmt.Sprintf("mobile_auth_refresh_total %d\n", mobileAuthRefreshTotal.Load())

	metrics += fmt.Sprintf("# HELP mobile_auth_logout_total Total mobile logout calls\n")
	metrics += fmt.Sprintf("# TYPE mobile_auth_logout_total counter\n")
	metrics += fmt.Sprintf("mobile_auth_logout_total %d\n", mobileAuthLogoutTotal.Load())

	metrics += fmt.Sprintf("# HELP mobile_assignments_total Total mobile assignments reads\n")
	metrics += fmt.Sprintf("# TYPE mobile_assignments_total counter\n")
	metrics += fmt.Sprintf("mobile_assignments_total %d\n", mobileAssignmentsTotal.Load())

	metrics += fmt.Sprintf("# HELP mobile_ingest_accepted_total Total accepted mobile packets\n")
	metrics += fmt.Sprintf("# TYPE mobile_ingest_accepted_total counter\n")
	metrics += fmt.Sprintf("mobile_ingest_accepted_total %d\n", mobileIngestAcceptedTotal.Load())

	metrics += fmt.Sprintf("# HELP mobile_ingest_duplicate_total Total duplicate mobile packets\n")
	metrics += fmt.Sprintf("# TYPE mobile_ingest_duplicate_total counter\n")
	metrics += fmt.Sprintf("mobile_ingest_duplicate_total %d\n", mobileIngestDuplicateTotal.Load())

	metrics += fmt.Sprintf("# HELP mobile_ingest_rejected_total Total rejected mobile packets\n")
	metrics += fmt.Sprintf("# TYPE mobile_ingest_rejected_total counter\n")
	metrics += fmt.Sprintf("mobile_ingest_rejected_total %d\n", mobileIngestRejectedTotal.Load())

	metrics += fmt.Sprintf("# HELP mobile_command_status_total Total mobile command status reads\n")
	metrics += fmt.Sprintf("# TYPE mobile_command_status_total counter\n")
	metrics += fmt.Sprintf("mobile_command_status_total %d\n", mobileCommandStatusTotal.Load())

	metrics += renderMobileEndpointMetrics()

	if c.reverify != nil {
		m := c.reverify.SnapshotMetrics()
		timestamp := int64(0)
		if !m.LastRunAt.IsZero() {
			timestamp = m.LastRunAt.Unix()
		}

		metrics += fmt.Sprintf("# HELP reverify_last_run_timestamp_seconds Unix timestamp of last reverification batch start\n")
		metrics += fmt.Sprintf("# TYPE reverify_last_run_timestamp_seconds gauge\n")
		metrics += fmt.Sprintf("reverify_last_run_timestamp_seconds %d\n", timestamp)

		metrics += fmt.Sprintf("# HELP reverify_last_processed_total Packets inspected in last reverification batch\n")
		metrics += fmt.Sprintf("# TYPE reverify_last_processed_total gauge\n")
		metrics += fmt.Sprintf("reverify_last_processed_total %d\n", m.LastProcessed)

		metrics += fmt.Sprintf("# HELP reverify_last_recovered_total Packets recovered in last reverification batch\n")
		metrics += fmt.Sprintf("# TYPE reverify_last_recovered_total gauge\n")
		metrics += fmt.Sprintf("reverify_last_recovered_total %d\n", m.LastRecovered)

		projectLabel := strings.ReplaceAll(m.LastProject, "\"", "")
		metrics += fmt.Sprintf("# HELP reverify_last_run_info Metadata for last reverification batch\n")
		metrics += fmt.Sprintf("# TYPE reverify_last_run_info gauge\n")
		metrics += fmt.Sprintf("reverify_last_run_info{project=\"%s\"} 1\n", projectLabel)

		totals := c.reverify.SnapshotTotals()
		metrics += fmt.Sprintf("# HELP reverify_processed_total_total Cumulative packets inspected per project across reverification batches\n")
		metrics += fmt.Sprintf("# TYPE reverify_processed_total_total counter\n")
		for pid, t := range totals {
			p := strings.ReplaceAll(pid, "\"", "")
			metrics += fmt.Sprintf("reverify_processed_total_total{project=\"%s\"} %d\n", p, t.Processed)
		}

		metrics += fmt.Sprintf("# HELP reverify_recovered_total_total Cumulative packets recovered per project across reverification batches\n")
		metrics += fmt.Sprintf("# TYPE reverify_recovered_total_total counter\n")
		for pid, t := range totals {
			p := strings.ReplaceAll(pid, "\"", "")
			metrics += fmt.Sprintf("reverify_recovered_total_total{project=\"%s\"} %d\n", p, t.Recovered)
		}
	}

	ctx.Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	return ctx.SendString(metrics)
}
