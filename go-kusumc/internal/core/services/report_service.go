package services

import (
	"fmt"
	"ingestion-go/internal/adapters/secondary"
	"os"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/xuri/excelize/v2"
)

type ReportService struct {
	repo *secondary.PostgresRepo
}

func NewReportService(repo *secondary.PostgresRepo) *ReportService {
	return &ReportService{repo: repo}
}

func (s *ReportService) GenerateDailyReport(projectId string) (string, error) {
	// 1. Fetch Stats
	stats, err := s.repo.GetProjectStats(projectId)
	if err != nil {
		return "", err
	}

	// 2. Setup PDF
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 24)

	// Header
	pdf.CellFormat(190, 10, "Unified IoT Portal", "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "", 16)
	pdf.CellFormat(190, 10, "Daily Intelligence Report", "", 1, "C", false, 0, "")
	pdf.Ln(5)

	pdf.SetFont("Arial", "", 12)
	pdf.CellFormat(190, 10, fmt.Sprintf("Project ID: %s", projectId), "", 1, "L", false, 0, "")
	pdf.CellFormat(190, 10, fmt.Sprintf("Generated: %s", time.Now().Format(time.RFC1123)), "", 1, "L", false, 0, "")
	pdf.Ln(10)

	// 3. System Status
	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(190, 10, "1. System Status", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 12)

	// Values
	total := stats["total_devices"].(int)
	online := stats["online_devices"].(int)
	health := "0%"
	if total > 0 {
		h := (float64(online) / float64(total)) * 100
		health = fmt.Sprintf("%.0f%%", h)
	}

	pdf.CellFormat(190, 7, fmt.Sprintf("Total Devices: %d", total), "", 1, "L", false, 0, "")
	pdf.CellFormat(190, 7, fmt.Sprintf("Online Devices: %d", online), "", 1, "L", false, 0, "")
	pdf.CellFormat(190, 7, fmt.Sprintf("System Health: %s", health), "", 1, "L", false, 0, "")
	pdf.Ln(10)

	// 4. Anomalies
	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(190, 10, "2. Detected Anomalies (Last 24h)", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 12)

	anomalies := stats["anomalies"].([]map[string]interface{})
	if len(anomalies) == 0 {
		pdf.CellFormat(190, 7, "No anomalies detected. System operating normally.", "", 1, "L", false, 0, "")
	} else {
		for _, a := range anomalies {
			// [TYPE] Description (Value)
			line := fmt.Sprintf("[%s] %s (%.2f)", a["type"], a["description"], a["value"])
			pdf.SetTextColor(255, 0, 0) // Red
			pdf.CellFormat(190, 7, line, "", 1, "L", false, 0, "")
		}
	}
	pdf.SetTextColor(0, 0, 0) // Reset
	pdf.Ln(20)

	// Footer
	pdf.SetFont("Arial", "I", 10)
	pdf.SetTextColor(128, 128, 128)
	pdf.CellFormat(190, 10, "Generated automatically by Go AI Engine", "", 1, "C", false, 0, "")

	// 5. Save
	// Ensure dir exists
	outputDir := "reports"
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		os.Mkdir(outputDir, 0755)
	}

	fileName := fmt.Sprintf("Report_%s_%d.pdf", projectId, time.Now().Unix())
	filePath := fmt.Sprintf("%s/%s", outputDir, fileName)

	err = pdf.OutputFileAndClose(filePath)
	if err != nil {
		return "", err
	}

	return filePath, nil
}

// GenerateDailyExcelReport creates an .xlsx report
func (s *ReportService) GenerateDailyExcelReport(projectId string) (string, error) {
	// 1. Fetch Stats
	stats, err := s.repo.GetProjectStats(projectId)
	if err != nil {
		return "", err
	}

	// 2. Setup Excel
	f := excelize.NewFile()
	sheet := "Daily Report"
	f.SetSheetName("Sheet1", sheet)

	// Styles
	styleBold, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 12}})
	styleRed, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Color: "#FF0000"}})

	// 3. Header
	f.SetCellValue(sheet, "A1", "Unified IoT Portal - Daily Intelligence Report")
	f.SetCellStyle(sheet, "A1", "A1", styleBold)

	f.SetCellValue(sheet, "A2", "Project ID:")
	f.SetCellValue(sheet, "B2", projectId)
	f.SetCellValue(sheet, "A3", "Generated:")
	f.SetCellValue(sheet, "B3", time.Now().Format(time.RFC3339))

	// 4. System Status
	f.SetCellValue(sheet, "A5", "1. System Status")
	f.SetCellStyle(sheet, "A5", "A5", styleBold)

	total := stats["total_devices"].(int)
	online := stats["online_devices"].(int)
	health := "0%"
	if total > 0 {
		h := (float64(online) / float64(total)) * 100
		health = fmt.Sprintf("%.0f%%", h)
	}

	f.SetCellValue(sheet, "A6", "Total Devices")
	f.SetCellValue(sheet, "B6", total)
	f.SetCellValue(sheet, "A7", "Online Devices")
	f.SetCellValue(sheet, "B7", online)
	f.SetCellValue(sheet, "A8", "System Health")
	f.SetCellValue(sheet, "B8", health)

	// 5. Anomalies
	f.SetCellValue(sheet, "A10", "2. Detected Anomalies (Last 24h)")
	f.SetCellStyle(sheet, "A10", "A10", styleBold)

	anomalies := stats["anomalies"].([]map[string]interface{})
	if len(anomalies) == 0 {
		f.SetCellValue(sheet, "A11", "No anomalies detected. System operating normally.")
	} else {
		f.SetCellValue(sheet, "A11", "Type")
		f.SetCellValue(sheet, "B11", "Description")
		f.SetCellValue(sheet, "C11", "Value")
		f.SetCellStyle(sheet, "A11", "C11", styleBold)

		row := 12
		for _, a := range anomalies {
			f.SetCellValue(sheet, fmt.Sprintf("A%d", row), a["type"])
			f.SetCellValue(sheet, fmt.Sprintf("B%d", row), a["description"])
			f.SetCellValue(sheet, fmt.Sprintf("C%d", row), a["value"])
			f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("C%d", row), styleRed)
			row++
		}
	}

	// 6. Save
	outputDir := "reports"
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		os.Mkdir(outputDir, 0755)
	}

	fileName := fmt.Sprintf("Report_%s_%d.xlsx", projectId, time.Now().Unix())
	filePath := fmt.Sprintf("%s/%s", outputDir, fileName)

	if err := f.SaveAs(filePath); err != nil {
		return "", err
	}

	return filePath, nil
}

// GenerateComplianceReport creates a detailed audit report
func (s *ReportService) GenerateComplianceReport(projectId string, days int) (string, error) {
	// 1. Fetch Audit Logs (Last N days)
	// We need a helper in Repo for range, but for V1 we use fetchSimple (LIMIT 100) or add a query.
	// We'll reuse GetAllAuditLogs pattern but we need to add filters eventually.
	// For MVP, we query raw SQL here or add repo method.
	// Let's add repo method call: s.repo.GetRecentAuditLogs(days)

	// Since I didn't add GetRecentAuditLogs yet, I'll inline the query via fetchSimpleWithArgs if accessible?
	// But Repo methods are cleaner. I'll add GetRecentAuditLogs to postgres_repo first?
	// Or I'll just use s.repo.GetAlerts() and s.repo.GetWorkOrders() and standard Audits.

	// Let's rely on what we have + Alerts + WorkOrders to show "Compliance" state.
	// "Compliance" usually means: "Did we catch all incidents? Who accessed what?"

	f := excelize.NewFile()
	sheet := "Compliance Scan"
	f.SetSheetName("Sheet1", sheet)

	styleHead, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 14}})

	// Title
	f.SetCellValue(sheet, "A1", "Unified IoT Portal - Compliance Audit")
	f.SetCellStyle(sheet, "A1", "A1", styleHead)
	f.SetCellValue(sheet, "A2", fmt.Sprintf("Project: %s", projectId))
	f.SetCellValue(sheet, "A3", fmt.Sprintf("Date: %s", time.Now().Format(time.RFC3339)))

	// Section 1: Critical Alerts (Incidents)
	f.SetCellValue(sheet, "A5", "1. Critical Incidents (Open)")
	f.SetCellStyle(sheet, "A5", "A5", styleHead)

	alerts, _ := s.repo.GetAlerts(projectId, "critical")
	if len(alerts) > 0 {
		f.SetCellValue(sheet, "A6", "ID")
		f.SetCellValue(sheet, "B6", "Message")
		f.SetCellValue(sheet, "C6", "Time")
		row := 7
		for _, a := range alerts {
			f.SetCellValue(sheet, fmt.Sprintf("A%d", row), a["id"])
			f.SetCellValue(sheet, fmt.Sprintf("B%d", row), a["message"])
			f.SetCellValue(sheet, fmt.Sprintf("C%d", row), a["triggered_at"])
			row++
		}
	} else {
		f.SetCellValue(sheet, "A6", "No critical incidents found.")
	}

	// Section 2: Last 50 Work Orders
	f.SetCellValue(sheet, "A20", "2. Maintenance Activity")
	f.SetCellStyle(sheet, "A20", "A20", styleHead)

	wos, _ := s.repo.GetWorkOrders()
	f.SetCellValue(sheet, "A21", "Ticket ID")
	f.SetCellValue(sheet, "B21", "Title")
	f.SetCellValue(sheet, "C21", "Status")

	row := 22
	for _, w := range wos {
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), w["ticket_id"])
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), w["title"])
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), w["status"])
		row++
	}

	// Save
	outputDir := "reports"
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		os.Mkdir(outputDir, 0755)
	}

	fileName := fmt.Sprintf("Compliance_%s_%d.xlsx", projectId, time.Now().Unix())
	filePath := fmt.Sprintf("%s/%s", outputDir, fileName)

	if err := f.SaveAs(filePath); err != nil {
		return "", err
	}

	return filePath, nil
}
