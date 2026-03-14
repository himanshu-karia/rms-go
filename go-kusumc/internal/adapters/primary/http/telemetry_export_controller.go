package http

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"ingestion-go/internal/adapters/secondary"
	"ingestion-go/internal/core/services"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
)

type TelemetryExportController struct {
	repo      *secondary.PostgresRepo
	exportSvc *services.ExportService
}

func NewTelemetryExportController(repo *secondary.PostgresRepo, exportSvc *services.ExportService) *TelemetryExportController {
	return &TelemetryExportController{repo: repo, exportSvc: exportSvc}
}

// Export is the preferred handler. It supports format=csv|xlsx|pdf and optional filters.
func (c *TelemetryExportController) Export(ctx *fiber.Ctx) error {
	imei := telemetryExportDeviceQuery(ctx)
	projectId := telemetryExportProjectQuery(ctx)
	startStr := telemetryExportStartQuery(ctx)
	endStr := telemetryExportEndQuery(ctx)
	packetType := telemetryExportPacketTypeQuery(ctx)
	quality := telemetryExportQualityQuery(ctx)
	excludeQuality := telemetryExportExcludeQualityQuery(ctx)
	format := strings.ToLower(ctx.Query("format", "csv"))

	end := time.Now()
	start := end.Add(-24 * time.Hour)
	if startStr != "" {
		if t, err := parseFlexibleTime(startStr); err == nil {
			start = t
		}
	}
	if endStr != "" {
		if t, err := parseFlexibleTime(endStr); err == nil {
			end = t
		}
	}

	// If projectId not specified, try auth context (ApiKeyMiddleware sets this)
	if projectId == "" {
		if pid, ok := ctx.Locals("project_id").(string); ok {
			projectId = pid
		}
	}

	var (
		rows []map[string]interface{}
		err  error
	)

	if imei != "" {
		if packetType != "" || quality != "" || excludeQuality != "" {
			rows, err = c.repo.ExportTelemetryByIMEIFiltered(start, end, imei, packetType, quality, excludeQuality)
		} else {
			rows, err = c.repo.ExportTelemetryByIMEI(start, end, imei)
		}
	} else {
		if projectId == "" {
			return ctx.Status(400).JSON(fiber.Map{"error": "imei or project_id required"})
		}
		if packetType != "" || quality != "" || excludeQuality != "" {
			rows, err = c.repo.ExportTelemetryFiltered(start, end, projectId, packetType, quality, excludeQuality)
		} else {
			rows, err = c.repo.ExportTelemetry(start, end, projectId)
		}
	}
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	filename := "telemetry_export.csv"
	q := url.Values{}
	if imei != "" {
		q.Set("imei", imei)
	}
	if projectId != "" {
		q.Set("project_id", projectId)
	}
	if packetType != "" {
		q.Set("packet_type", packetType)
	}
	if quality != "" {
		q.Set("quality", quality)
	}
	if excludeQuality != "" {
		q.Set("exclude_quality", excludeQuality)
	}
	_ = q
	baseName := "telemetry_export"
	if imei != "" {
		baseName = fmt.Sprintf("telemetry_%s", imei)
	} else if projectId != "" {
		baseName = fmt.Sprintf("telemetry_%s", projectId)
	}

	suffix := time.Now().Format("20060102_150405")

	switch format {
	case "xlsx", "excel":
		filename = fmt.Sprintf("%s_%s.xlsx", baseName, suffix)
		b, err := buildTelemetryXLSX(rows)
		if err != nil {
			return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		ctx.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		ctx.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		return ctx.Send(b)
	case "pdf":
		if c.exportSvc == nil {
			return ctx.Status(500).JSON(fiber.Map{"error": "export service not configured"})
		}
		filename = fmt.Sprintf("%s_%s.pdf", baseName, suffix)
		// ExportService expects maps with "timestamp"; normalize from db rows.
		norm := make([]map[string]interface{}, 0, len(rows))
		for _, r := range rows {
			t, _ := r["time"].(time.Time)
			out := map[string]interface{}{"timestamp": t}
			if d, ok := r["data"].(map[string]interface{}); ok {
				for k, v := range d {
					out[k] = v
				}
			}
			norm = append(norm, out)
		}
		pdf, err := c.exportSvc.GeneratePDF(projectId, norm)
		if err != nil {
			return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		ctx.Set("Content-Type", "application/pdf")
		ctx.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		return ctx.Send(pdf)
	default:
		filename = fmt.Sprintf("%s_%s.csv", baseName, suffix)
		b, err := buildTelemetryCSV(rows)
		if err != nil {
			return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		ctx.Set("Content-Type", "text/csv")
		ctx.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		return ctx.Send(b)
	}
}

// ExportCSV is kept for backward compatibility.
func (c *TelemetryExportController) ExportCSV(ctx *fiber.Ctx) error {
	return c.Export(ctx)
}

func buildTelemetryCSV(rows []map[string]interface{}) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write([]string{"time", "device_id", "data"}); err != nil {
		return nil, err
	}
	for _, r := range rows {
		t, _ := r["time"].(time.Time)
		deviceID := fmt.Sprintf("%v", r["device_id"])
		payload, _ := json.Marshal(r["data"])
		_ = w.Write([]string{t.Format(time.RFC3339), deviceID, string(payload)})
	}
	w.Flush()
	return buf.Bytes(), nil
}

func buildTelemetryXLSX(rows []map[string]interface{}) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	sheet := f.GetSheetName(0)
	headers := []string{"Time", "Device ID", "Payload"}
	_ = f.SetSheetRow(sheet, "A1", &headers)

	for i, r := range rows {
		t, _ := r["time"].(time.Time)
		deviceID := fmt.Sprintf("%v", r["device_id"])
		payload, _ := json.Marshal(r["data"])
		row := []interface{}{t.Format(time.RFC3339), deviceID, string(payload)}
		axis, _ := excelize.CoordinatesToCellName(1, i+2)
		_ = f.SetSheetRow(sheet, axis, &row)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func telemetryExportDeviceQuery(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "imei", "device_id", "deviceId")
}

func telemetryExportProjectQuery(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "project_id", "projectId")
}

func telemetryExportStartQuery(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "start", "from")
}

func telemetryExportEndQuery(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "end", "to")
}

func telemetryExportPacketTypeQuery(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "packet_type", "packetType")
}

func telemetryExportQualityQuery(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "quality", "data_quality")
}

func telemetryExportExcludeQualityQuery(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "exclude_quality", "excludeQuality")
}
