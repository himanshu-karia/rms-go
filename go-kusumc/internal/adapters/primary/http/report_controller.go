package http

import (
	"ingestion-go/internal/core/services"

	"github.com/gofiber/fiber/v2"
)

type ReportController struct {
	service *services.ReportService
}

func NewReportController(service *services.ReportService) *ReportController {
	return &ReportController{service: service}
}

func (c *ReportController) GenerateReport(ctx *fiber.Ctx) error {
	projectId := ctx.Params("id")
	if projectId == "" {
		return ctx.Status(400).SendString("Project ID required")
	}

	format := ctx.Query("format")
	var path string
	var err error

	if format == "xlsx" {
		path, err = c.service.GenerateDailyExcelReport(projectId)
	} else {
		path, err = c.service.GenerateDailyReport(projectId)
	}

	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}

	// Trigger Download
	return ctx.Download(path)
}

func (c *ReportController) GenerateComplianceReport(ctx *fiber.Ctx) error {
	projectId := ctx.Params("id")
	if projectId == "" {
		return ctx.Status(400).SendString("Project ID required")
	}

	// Default to last 30 days
	path, err := c.service.GenerateComplianceReport(projectId, 30)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.Download(path)
}
