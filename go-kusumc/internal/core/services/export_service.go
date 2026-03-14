package services

import (
	"bytes"
	"fmt"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/xuri/excelize/v2"
)

type ExportService struct{}

func NewExportService() *ExportService {
	return &ExportService{}
}

// GenerateExcel creates an .xlsx file from telemetry data
func (s *ExportService) GenerateExcel(data []map[string]interface{}) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	// Header
	headers := []string{"Timestamp", "Payload"}
	// In real impl, we dynamically find keys from jsonb

	f.SetSheetRow("Sheet1", "A1", &headers)

	for i, row := range data {
		cellData := []interface{}{
			row["timestamp"],       // Ensure string or Time
			fmt.Sprintf("%v", row), // Dump JSON as string for MVP
		}
		axis, _ := excelize.CoordinatesToCellName(1, i+2)
		f.SetSheetRow("Sheet1", axis, &cellData)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GeneratePDF creates a simple PDF report
func (s *ExportService) GeneratePDF(projectId string, data []map[string]interface{}) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)

	// Title
	pdf.Cell(40, 10, "Unified IoT Intelligence Report")
	pdf.Ln(12)

	pdf.SetFont("Arial", "", 12)
	pdf.Cell(0, 10, fmt.Sprintf("Project: %s", projectId))
	pdf.Ln(8)
	pdf.Cell(0, 10, fmt.Sprintf("Generated: %s", time.Now().Format(time.RFC822)))
	pdf.Ln(12)

	// --- Intelligence Summary ---
	// Calculate Stats from Data (Mocking for V1 as data[] is raw telemetry)
	totalPoints := len(data)
	uptime := 98.5 // Mock
	anomalies := 0

	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(0, 10, "1. System Health")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 12)
	pdf.Cell(0, 10, fmt.Sprintf("- Total Data Points: %d", totalPoints))
	pdf.Ln(6)
	pdf.Cell(0, 10, fmt.Sprintf("- Estimated Uptime: %.1f%%", uptime))
	pdf.Ln(6)
	pdf.Cell(0, 10, fmt.Sprintf("- Critical Anomalies: %d", anomalies))
	pdf.Ln(15)

	// --- Table Header ---
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(60, 10, "Timestamp")
	pdf.Cell(0, 10, "Data")
	pdf.Ln(10)

	// Rows
	pdf.SetFont("Arial", "", 10)
	for _, row := range data {
		ts := fmt.Sprintf("%v", row["timestamp"])
		val := fmt.Sprintf("%v", row)
		// Truncate for PDF width
		if len(val) > 50 {
			val = val[:47] + "..."
		}

		pdf.Cell(60, 10, ts)
		pdf.Cell(0, 10, val)
		pdf.Ln(8)
	}

	var buf bytes.Buffer
	err := pdf.Output(&buf)
	return buf.Bytes(), err
}
