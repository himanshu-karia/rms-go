package services

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"strings"

	"ingestion-go/internal/models"
)

type BulkService struct {
	deviceService *DeviceService
	govtService   *GovtCredsService
}

func NewBulkService(dev *DeviceService, govt *GovtCredsService) *BulkService {
	return &BulkService{deviceService: dev, govtService: govt}
}

// ImportDevices processes a CSV stream.
// Header supported: imei,name,project_id,protocol_id,govt_protocol_id,govt_client_id,govt_username,govt_password
func (s *BulkService) ImportDevices(r io.Reader, defaultProject string) (int, []error) {
	csvReader := csv.NewReader(r)
	head, err := csvReader.Read()
	if err != nil {
		return 0, []error{err}
	}

	colIdx := make(map[string]int)
	for i, h := range head {
		colIdx[strings.ToLower(strings.TrimSpace(h))] = i
	}

	required := []string{"imei"}
	for _, key := range required {
		if _, ok := colIdx[key]; !ok {
			return 0, []error{fmt.Errorf("missing required column: %s", key)}
		}
	}

	success := 0
	errors := []error{}
	rowNum := 1
	var govtBatch []models.GovtCredentialBundle

	for {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			errors = append(errors, fmt.Errorf("row %d: parse error", rowNum))
			continue
		}

		lookup := func(key string) string {
			if idx, ok := colIdx[key]; ok && idx < len(row) {
				return strings.TrimSpace(row[idx])
			}
			return ""
		}

		imei := lookup("imei")
		if imei == "" {
			errors = append(errors, fmt.Errorf("row %d: imei is required", rowNum))
			rowNum++
			continue
		}

		name := lookup("name")
		projectId := lookup("project_id")
		if projectId == "" {
			projectId = defaultProject
		}
		if projectId == "" {
			errors = append(errors, fmt.Errorf("row %d (%s): project_id missing", rowNum, imei))
			rowNum++
			continue
		}

		protocolID := lookup("protocol_id")
		govtProtocolID := lookup("govt_protocol_id")
		govtClient := lookup("govt_client_id")
		govtUser := lookup("govt_username")
		govtPass := lookup("govt_password")

		attrs := map[string]any{}
		if protocolID != "" {
			attrs["protocol_id"] = protocolID
		}
		if govtProtocolID != "" {
			attrs["govt_protocol_id"] = govtProtocolID
		}

		resp, err := s.deviceService.CreateDevice(projectId, name, imei, attrs)
		if err != nil {
			errors = append(errors, fmt.Errorf("row %d (%s): %v", rowNum, imei, err))
			rowNum++
			continue
		}
		deviceID := resp["id"]

		if govtProtocolID != "" && deviceID != "" && (govtClient != "" || govtUser != "" || govtPass != "") {
			govtBatch = append(govtBatch, models.GovtCredentialBundle{
				DeviceID:   deviceID,
				ProtocolID: govtProtocolID,
				ClientID:   govtClient,
				Username:   govtUser,
				Password:   govtPass,
			})
		}

		success++
		rowNum++
	}

	if len(govtBatch) > 0 && s.govtService != nil {
		if _, err := s.govtService.BulkUpsert(context.Background(), govtBatch); err != nil {
			errors = append(errors, fmt.Errorf("govt bulk upsert: %v", err))
		}
	}

	log.Printf("Bulk Import: %d success, %d errors", success, len(errors))
	return success, errors
}
