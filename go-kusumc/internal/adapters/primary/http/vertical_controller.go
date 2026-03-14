package http

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ingestion-go/internal/core/services"
	"ingestion-go/internal/models"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type VerticalController struct {
	service *services.VerticalService
}

func NewVerticalController(service *services.VerticalService) *VerticalController {
	return &VerticalController{service: service}
}

// A. Agriculture / General
func (c *VerticalController) CreateBeneficiary(ctx *fiber.Ctx) error {
	var body struct {
		ProjectID          string           `json:"project_id"`
		ProjectIDCamel     string           `json:"projectId"`
		Name               string           `json:"name"`
		Email              *string          `json:"email"`
		Phone              *string          `json:"phone"`
		PhoneCamel         *string          `json:"phoneNumber"`
		Address            *string          `json:"address"`
		Contacts           []map[string]any `json:"contacts"`
		Location           map[string]any   `json:"location"`
		Metadata           map[string]any   `json:"metadata"`
		AccountStatus      *string          `json:"account_status"`
		AccountStatusCamel *string          `json:"accountStatus"`
		Deleted            *bool            `json:"deleted"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return validationError(ctx, "Invalid beneficiary payload", "body", err.Error())
	}
	if body.ProjectID == "" {
		body.ProjectID = body.ProjectIDCamel
	}
	if body.Phone == nil {
		body.Phone = body.PhoneCamel
	}
	if body.AccountStatus == nil {
		body.AccountStatus = body.AccountStatusCamel
	}
	if strings.TrimSpace(body.Name) == "" {
		return validationError(ctx, "Invalid beneficiary payload", "name", "Name is required")
	}

	metadata := body.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	if len(body.Contacts) > 0 {
		metadata["contacts"] = body.Contacts
	}
	if body.Location != nil {
		metadata["location"] = body.Location
	}
	if body.AccountStatus != nil {
		metadata["account_status"] = strings.TrimSpace(*body.AccountStatus)
	}
	if body.Deleted != nil {
		metadata["deleted"] = *body.Deleted
	}

	isActive := interface{}(nil)
	if body.AccountStatus != nil {
		if strings.EqualFold(*body.AccountStatus, "disabled") {
			isActive = false
		} else {
			isActive = true
		}
	}

	address := map[string]any{}
	if body.Address != nil {
		address["address"] = strings.TrimSpace(*body.Address)
	}

	created, err := c.service.CreateBeneficiary(map[string]interface{}{
		"project_id": body.ProjectID,
		"name":       strings.TrimSpace(body.Name),
		"phone":      body.Phone,
		"email":      body.Email,
		"address":    address,
		"metadata":   metadata,
		"is_active":  isActive,
	})
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	c.service.LogAudit(getUserID(ctx), "beneficiary.created", ctx.Path(), ctx.IP(), "success", map[string]interface{}{
		"beneficiaryId": created["id"],
		"project_id":    body.ProjectID,
	})
	return ctx.Status(201).JSON(fiber.Map{"beneficiary": normalizeBeneficiary(created)})
}

func (c *VerticalController) GetBeneficiaries(ctx *fiber.Ctx) error {
	filters := map[string]interface{}{
		"project_id":      verticalQuery(ctx, "project_id", "projectId"),
		"search":          verticalQuery(ctx, "search", "q"),
		"account_status":  verticalQuery(ctx, "account_status", "accountStatus"),
		"installation_id": verticalQuery(ctx, "installationUuid", "installation_id"),
		"include_deleted": verticalQueryBool(ctx, false, "include_soft_deleted", "includeSoftDeleted"),
	}
	if limit := verticalQueryInt(ctx, 0, "limit", "pageSize"); limit > 0 {
		filters["limit"] = limit
	}
	data, err := c.service.ListBeneficiaries(filters)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(fiber.Map{"beneficiaries": normalizeBeneficiaries(data)})
}

func (c *VerticalController) UpdateBeneficiary(ctx *fiber.Ctx) error {
	id := ctx.Params("beneficiaryUuid")
	if id == "" {
		id = ctx.Params("id")
	}
	if id == "" || !isUUID(id) {
		return validationError(ctx, "Invalid beneficiary identifier", "beneficiaryUuid", "Must be a valid UUID")
	}
	var body struct {
		Name               *string          `json:"name"`
		Email              *string          `json:"email"`
		Phone              *string          `json:"phone"`
		PhoneCamel         *string          `json:"phoneNumber"`
		Address            *string          `json:"address"`
		Contacts           []map[string]any `json:"contacts"`
		Location           map[string]any   `json:"location"`
		Metadata           map[string]any   `json:"metadata"`
		AccountStatus      *string          `json:"account_status"`
		AccountStatusCamel *string          `json:"accountStatus"`
		Deleted            *bool            `json:"deleted"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return validationError(ctx, "Invalid beneficiary payload", "body", err.Error())
	}

	if body.Phone == nil {
		body.Phone = body.PhoneCamel
	}
	if body.AccountStatus == nil {
		body.AccountStatus = body.AccountStatusCamel
	}
	if body.Name == nil && body.Email == nil && body.Phone == nil && body.Address == nil && body.Metadata == nil && body.AccountStatus == nil && body.Deleted == nil && len(body.Contacts) == 0 && body.Location == nil {
		return validationError(ctx, "Invalid beneficiary payload", "body", "Provide at least one field to update")
	}

	metadata := body.Metadata
	if metadata == nil && (len(body.Contacts) > 0 || body.Location != nil || body.AccountStatus != nil || body.Deleted != nil) {
		metadata = map[string]any{}
	}
	if len(body.Contacts) > 0 {
		metadata["contacts"] = body.Contacts
	}
	if body.Location != nil {
		metadata["location"] = body.Location
	}
	if body.AccountStatus != nil {
		metadata["account_status"] = strings.TrimSpace(*body.AccountStatus)
	}
	if body.Deleted != nil {
		metadata["deleted"] = *body.Deleted
	}

	isActive := interface{}(nil)
	if body.AccountStatus != nil {
		if strings.EqualFold(*body.AccountStatus, "disabled") {
			isActive = false
		} else {
			isActive = true
		}
	}

	address := map[string]any{}
	if body.Address != nil {
		address["address"] = strings.TrimSpace(*body.Address)
	}

	updated, err := c.service.UpdateBeneficiary(id, map[string]interface{}{
		"name":      body.Name,
		"phone":     body.Phone,
		"email":     body.Email,
		"address":   address,
		"metadata":  metadata,
		"is_active": isActive,
	})
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	c.service.LogAudit(getUserID(ctx), "beneficiary.updated", ctx.Path(), ctx.IP(), "success", map[string]interface{}{
		"beneficiaryId": id,
	})
	return ctx.JSON(fiber.Map{"beneficiary": normalizeBeneficiary(updated)})
}

// B. Installations
func (c *VerticalController) CreateInstallation(ctx *fiber.Ctx) error {
	var body struct {
		DeviceID              string         `json:"device_id"`
		DeviceIDCamel         string         `json:"deviceUuid"`
		Status                *string        `json:"status"`
		VfdDriveModelID       *string        `json:"vfd_model_id"`
		VfdDriveModelCamel    *string        `json:"vfdDriveModelId"`
		Metadata              map[string]any `json:"metadata"`
		Notes                 *string        `json:"notes"`
		GeoLocation           map[string]any `json:"geo_location"`
		GeoLocationCamel      map[string]any `json:"geoLocation"`
		Location              map[string]any `json:"location"`
		ActivatedAt           *string        `json:"activated_at"`
		ActivatedAtCamel      *string        `json:"activatedAt"`
		DecommissionedAt      *string        `json:"decommissioned_at"`
		DecommissionedAtCamel *string        `json:"decommissionedAt"`
		BeneficiaryID         *string        `json:"beneficiary_id"`
		BeneficiaryCamel      *string        `json:"beneficiaryUuid"`
		ProtocolID            *string        `json:"protocol_id"`
		ProtocolCamel         *string        `json:"protocolId"`
		ProjectID             *string        `json:"project_id"`
		ProjectCamel          *string        `json:"projectId"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return validationError(ctx, "Invalid installation payload", "body", err.Error())
	}
	if strings.TrimSpace(body.DeviceID) == "" {
		body.DeviceID = body.DeviceIDCamel
	}
	if body.VfdDriveModelID == nil {
		body.VfdDriveModelID = body.VfdDriveModelCamel
	}
	if body.GeoLocation == nil {
		body.GeoLocation = body.GeoLocationCamel
	}
	if body.GeoLocation == nil {
		body.GeoLocation = body.Location
	}
	if body.ActivatedAt == nil {
		body.ActivatedAt = body.ActivatedAtCamel
	}
	if body.DecommissionedAt == nil {
		body.DecommissionedAt = body.DecommissionedAtCamel
	}
	if body.BeneficiaryID == nil {
		body.BeneficiaryID = body.BeneficiaryCamel
	}
	if body.ProtocolID == nil {
		body.ProtocolID = body.ProtocolCamel
	}
	if body.ProjectID == nil {
		body.ProjectID = body.ProjectCamel
	}
	if !isUUID(body.DeviceID) {
		return validationError(ctx, "Invalid installation payload", "device_id", "device_id must be a valid UUID")
	}

	metadata := body.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	if body.Notes != nil {
		metadata["notes"] = strings.TrimSpace(*body.Notes)
	}

	created, err := c.service.CreateInstallation(map[string]interface{}{
		"device_id":         body.DeviceID,
		"project_id":        stringOrNil(body.ProjectID),
		"beneficiary_id":    stringOrNil(body.BeneficiaryID),
		"geo_location":      body.GeoLocation,
		"protocol_id":       stringOrNil(body.ProtocolID),
		"vfd_model_id":      stringOrNil(body.VfdDriveModelID),
		"status":            body.Status,
		"metadata":          metadata,
		"activated_at":      parseTimePointer(body.ActivatedAt),
		"decommissioned_at": parseTimePointer(body.DecommissionedAt),
	})
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	c.service.LogAudit(getUserID(ctx), "installation.created", ctx.Path(), ctx.IP(), "success", map[string]interface{}{
		"installationId": created["id"],
		"device_id":      body.DeviceID,
	})
	return ctx.Status(201).JSON(fiber.Map{"installation": normalizeInstallation(created)})
}

func (c *VerticalController) GetInstallations(ctx *fiber.Ctx) error {
	filters := map[string]interface{}{
		"project_id": verticalQuery(ctx, "project_id", "projectId"),
		"device_id":  verticalQuery(ctx, "device_id", "deviceUuid", "deviceId"),
		"status":     verticalQuery(ctx, "status", "status_filter"),
	}
	data, err := c.service.ListInstallations(filters)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(fiber.Map{"installations": normalizeInstallations(data)})
}

func (c *VerticalController) UpdateInstallation(ctx *fiber.Ctx) error {
	id := ctx.Params("installationUuid")
	if id == "" {
		id = ctx.Params("id")
	}
	if id == "" || !isUUID(id) {
		return validationError(ctx, "Invalid installation identifier", "installationUuid", "Must be a valid UUID")
	}
	var body struct {
		Status                *string        `json:"status"`
		VfdDriveModelID       *string        `json:"vfd_model_id"`
		VfdDriveModelCamel    *string        `json:"vfdDriveModelId"`
		Metadata              map[string]any `json:"metadata"`
		Notes                 *string        `json:"notes"`
		GeoLocation           map[string]any `json:"geo_location"`
		GeoLocationCamel      map[string]any `json:"geoLocation"`
		Location              map[string]any `json:"location"`
		ActivatedAt           *string        `json:"activated_at"`
		ActivatedAtCamel      *string        `json:"activatedAt"`
		DecommissionedAt      *string        `json:"decommissioned_at"`
		DecommissionedAtCamel *string        `json:"decommissionedAt"`
		BeneficiaryID         *string        `json:"beneficiary_id"`
		BeneficiaryCamel      *string        `json:"beneficiaryUuid"`
		ProtocolID            *string        `json:"protocol_id"`
		ProtocolCamel         *string        `json:"protocolId"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return validationError(ctx, "Invalid installation payload", "body", err.Error())
	}
	if body.VfdDriveModelID == nil {
		body.VfdDriveModelID = body.VfdDriveModelCamel
	}
	if body.GeoLocation == nil {
		body.GeoLocation = body.GeoLocationCamel
	}
	if body.GeoLocation == nil {
		body.GeoLocation = body.Location
	}
	if body.ActivatedAt == nil {
		body.ActivatedAt = body.ActivatedAtCamel
	}
	if body.DecommissionedAt == nil {
		body.DecommissionedAt = body.DecommissionedAtCamel
	}
	if body.BeneficiaryID == nil {
		body.BeneficiaryID = body.BeneficiaryCamel
	}
	if body.ProtocolID == nil {
		body.ProtocolID = body.ProtocolCamel
	}
	if body.Status == nil && body.VfdDriveModelID == nil && body.Metadata == nil && body.Notes == nil && body.GeoLocation == nil && body.ActivatedAt == nil && body.DecommissionedAt == nil && body.BeneficiaryID == nil && body.ProtocolID == nil {
		return validationError(ctx, "Invalid installation payload", "body", "Provide at least one field to update")
	}
	metadata := body.Metadata
	if metadata == nil && body.Notes != nil {
		metadata = map[string]any{}
	}
	if body.Notes != nil {
		metadata["notes"] = strings.TrimSpace(*body.Notes)
	}

	updated, err := c.service.UpdateInstallation(id, map[string]interface{}{
		"beneficiary_id":    stringOrNil(body.BeneficiaryID),
		"geo_location":      body.GeoLocation,
		"protocol_id":       stringOrNil(body.ProtocolID),
		"vfd_model_id":      stringOrNil(body.VfdDriveModelID),
		"status":            body.Status,
		"metadata":          metadata,
		"activated_at":      parseTimePointer(body.ActivatedAt),
		"decommissioned_at": parseTimePointer(body.DecommissionedAt),
	})
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	c.service.LogAudit(getUserID(ctx), "installation.updated", ctx.Path(), ctx.IP(), "success", map[string]interface{}{
		"installationId": id,
	})
	return ctx.JSON(fiber.Map{"installation": normalizeInstallation(updated)})
}

// GET /installations/:installationUuid
func (c *VerticalController) GetInstallation(ctx *fiber.Ctx) error {
	installationID := ctx.Params("installationUuid")
	if installationID == "" || !isUUID(installationID) {
		return validationError(ctx, "Invalid installation identifier", "installationUuid", "Must be a valid UUID")
	}
	inst, err := c.service.GetInstallation(installationID)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	if inst == nil {
		return ctx.Status(404).JSON(fiber.Map{"message": "Installation not found"})
	}
	return ctx.JSON(fiber.Map{"installation": normalizeInstallation(inst)})
}

// GET /installations/:installationUuid/beneficiaries
func (c *VerticalController) GetInstallationBeneficiaries(ctx *fiber.Ctx) error {
	installationID := ctx.Params("installationUuid")
	if installationID == "" || !isUUID(installationID) {
		return validationError(ctx, "Invalid installation identifier", "installation_id", "Must be a valid UUID")
	}
	includeRemoved := queryAliasBool(ctx, false, "include_removed", "includeRemoved")
	assignments, err := c.service.ListInstallationBeneficiaries(installationID, includeRemoved)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	wired := make([]map[string]interface{}, 0, len(assignments))
	for _, a := range assignments {
		wired = append(wired, normalizeInstallationAssignment(a))
	}
	return ctx.JSON(fiber.Map{"assignments": wired})
}

// POST /installations/:installationUuid/beneficiaries
func (c *VerticalController) AddInstallationBeneficiary(ctx *fiber.Ctx) error {
	installationID := ctx.Params("installationUuid")
	if installationID == "" || !isUUID(installationID) {
		return validationError(ctx, "Invalid installation identifier", "installation_id", "Must be a valid UUID")
	}
	var body struct {
		BeneficiaryID      string  `json:"beneficiary_id"`
		BeneficiaryIDCamel string  `json:"beneficiaryUuid"`
		Role               *string `json:"role"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return validationError(ctx, "Invalid assignment payload", "body", err.Error())
	}
	if strings.TrimSpace(body.BeneficiaryID) == "" {
		body.BeneficiaryID = body.BeneficiaryIDCamel
	}
	if body.BeneficiaryID == "" || !isUUID(body.BeneficiaryID) {
		return validationError(ctx, "Invalid assignment payload", "beneficiary_id", "beneficiary_id must be a valid UUID")
	}
	assignment, err := c.service.AssignBeneficiaryToInstallation(installationID, body.BeneficiaryID, stringOrDefault(body.Role, "owner"))
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	c.service.LogAudit(getUserID(ctx), "installation.beneficiary.assigned", ctx.Path(), ctx.IP(), "success", map[string]interface{}{
		"installation_id": installationID,
		"beneficiary_id":  body.BeneficiaryID,
		"role":            stringOrDefault(body.Role, "owner"),
	})
	return ctx.Status(201).JSON(fiber.Map{"assignment": normalizeInstallationAssignment(assignment)})
}

// DELETE /installations/:installationUuid/beneficiaries/:beneficiaryUuid
func (c *VerticalController) RemoveInstallationBeneficiary(ctx *fiber.Ctx) error {
	installationID := ctx.Params("installationUuid")
	beneficiaryID := ctx.Params("beneficiaryUuid")
	if installationID == "" || !isUUID(installationID) {
		return validationError(ctx, "Invalid installation identifier", "installation_id", "Must be a valid UUID")
	}
	if beneficiaryID == "" || !isUUID(beneficiaryID) {
		return validationError(ctx, "Invalid beneficiary identifier", "beneficiary_id", "Must be a valid UUID")
	}
	assignment, err := c.service.RemoveBeneficiaryFromInstallation(installationID, beneficiaryID)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	if assignment == nil {
		return ctx.Status(404).JSON(fiber.Map{"message": "Assignment not found"})
	}
	c.service.LogAudit(getUserID(ctx), "installation.beneficiary.removed", ctx.Path(), ctx.IP(), "success", map[string]interface{}{
		"installation_id": installationID,
		"beneficiary_id":  beneficiaryID,
	})
	return ctx.JSON(fiber.Map{"assignment": normalizeInstallationAssignment(assignment)})
}

// GET /beneficiaries/:beneficiaryUuid
func (c *VerticalController) GetBeneficiary(ctx *fiber.Ctx) error {
	beneficiaryID := ctx.Params("beneficiaryUuid")
	if beneficiaryID == "" || !isUUID(beneficiaryID) {
		return validationError(ctx, "Invalid beneficiary identifier", "beneficiaryUuid", "Must be a valid UUID")
	}
	beneficiary, err := c.service.GetBeneficiary(beneficiaryID)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	if beneficiary == nil {
		return ctx.Status(404).JSON(fiber.Map{"message": "Beneficiary not found"})
	}
	return ctx.JSON(fiber.Map{"beneficiary": normalizeBeneficiary(beneficiary)})
}

// POST /beneficiaries/:beneficiaryUuid/archive
func (c *VerticalController) ArchiveBeneficiary(ctx *fiber.Ctx) error {
	beneficiaryID := ctx.Params("beneficiaryUuid")
	if beneficiaryID == "" || !isUUID(beneficiaryID) {
		return validationError(ctx, "Invalid beneficiary identifier", "beneficiaryUuid", "Must be a valid UUID")
	}
	updated, err := c.service.UpdateBeneficiary(beneficiaryID, map[string]interface{}{
		"metadata": map[string]any{"deleted": true},
	})
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	c.service.LogAudit(getUserID(ctx), "beneficiary.archived", ctx.Path(), ctx.IP(), "success", map[string]interface{}{
		"beneficiaryId": beneficiaryID,
	})
	return ctx.JSON(fiber.Map{"beneficiary": normalizeBeneficiary(updated)})
}

func validationError(ctx *fiber.Ctx, message, field, detail string) error {
	return ctx.Status(400).JSON(fiber.Map{
		"message": message,
		"issues": []fiber.Map{
			{
				"path":    []string{field},
				"message": detail,
			},
		},
	})
}

func getUserID(ctx *fiber.Ctx) string {
	if raw, ok := ctx.Locals("user_id").(string); ok && strings.TrimSpace(raw) != "" {
		return raw
	}
	return "system"
}

func isUUID(value string) bool {
	_, err := uuid.Parse(strings.TrimSpace(value))
	return err == nil
}

func parseTimePointer(value *string) *time.Time {
	if value == nil {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*value))
	if err != nil {
		return nil
	}
	return &parsed
}

func stringOrNil(value *string) interface{} {
	if value == nil {
		return nil
	}
	if strings.TrimSpace(*value) == "" {
		return nil
	}
	return strings.TrimSpace(*value)
}

func stringOrDefault(value *string, fallback string) string {
	if value == nil {
		return fallback
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func normalizeBeneficiary(input map[string]interface{}) map[string]interface{} {
	if input == nil {
		return nil
	}
	output := map[string]interface{}{}
	for key, val := range input {
		switch key {
		case "projectId":
			output["project_id"] = val
		case "stateId":
			output["state_id"] = val
		case "isActive":
			output["is_active"] = val
		case "createdAt":
			output["created_at"] = val
		case "updatedAt":
			output["updated_at"] = val
		default:
			output[key] = val
		}
	}
	return output
}

func normalizeBeneficiaries(items []map[string]interface{}) []map[string]interface{} {
	results := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		results = append(results, normalizeBeneficiary(item))
	}
	return results
}

func normalizeInstallation(input map[string]interface{}) map[string]interface{} {
	if input == nil {
		return nil
	}
	output := map[string]interface{}{}
	for key, val := range input {
		switch key {
		case "deviceUuid":
			output["device_id"] = val
		case "projectId":
			output["project_id"] = val
		case "beneficiaryUuid":
			output["beneficiary_id"] = val
		case "geoLocation":
			output["geo_location"] = val
		case "protocolId":
			output["protocol_id"] = val
		case "vfdDriveModelId":
			output["vfd_model_id"] = val
		case "activatedAt":
			output["activated_at"] = val
		case "decommissionedAt":
			output["decommissioned_at"] = val
		case "createdAt":
			output["created_at"] = val
		case "updatedAt":
			output["updated_at"] = val
		default:
			output[key] = val
		}
	}
	return output
}

func normalizeInstallations(items []map[string]interface{}) []map[string]interface{} {
	results := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		results = append(results, normalizeInstallation(item))
	}
	return results
}

func normalizeInstallationAssignment(input map[string]interface{}) map[string]interface{} {
	if input == nil {
		return nil
	}
	output := map[string]interface{}{}
	for key, val := range input {
		switch key {
		case "installationUuid":
			output["installation_id"] = val
		case "beneficiaryUuid":
			output["beneficiary_id"] = val
		case "removedAt":
			output["removed_at"] = val
		case "createdAt":
			output["created_at"] = val
		default:
			output[key] = val
		}
	}
	return output
}

// C. Healthcare
func (c *VerticalController) CreatePatient(ctx *fiber.Ctx) error {
	var body map[string]interface{}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString(err.Error())
	}
	if err := c.service.CreatePatient(body); err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.SendStatus(201)
}

func (c *VerticalController) GetPatients(ctx *fiber.Ctx) error {
	projectID := verticalQuery(ctx, "project_id", "projectId")
	data, err := c.service.GetPatients(projectID)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(normalizeToSnakeKeys(data))
}

func (c *VerticalController) StartMedicalSession(ctx *fiber.Ctx) error {
	var body struct {
		PatientID      string `json:"patient_id"`
		PatientIDCamel string `json:"patientId"`
		DeviceID       string `json:"device_id"`
		DeviceIDCamel  string `json:"deviceId"`
		DoctorID       string `json:"doctor_id"`
		DoctorIDCamel  string `json:"doctorId"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString(err.Error())
	}
	if body.PatientID == "" {
		body.PatientID = body.PatientIDCamel
	}
	if body.DeviceID == "" {
		body.DeviceID = body.DeviceIDCamel
	}
	if body.DoctorID == "" {
		body.DoctorID = body.DoctorIDCamel
	}
	if body.PatientID == "" || body.DeviceID == "" {
		return ctx.Status(400).SendString("patient_id and device_id required")
	}
	sess, err := c.service.StartMedicalSession(body.PatientID, body.DeviceID, body.DoctorID)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(normalizeToSnakeKeys(sess))
}

func (c *VerticalController) EndMedicalSession(ctx *fiber.Ctx) error {
	sessionID := ctx.Params("sessionId")
	if sessionID == "" {
		return ctx.Status(400).SendString("sessionId required")
	}
	var body struct {
		Vitals map[string]interface{} `json:"vitals"`
		Notes  string                 `json:"notes"`
	}
	_ = ctx.BodyParser(&body)
	if err := c.service.EndMedicalSession(sessionID, body.Vitals, body.Notes); err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.SendStatus(204)
}

// E. Soil advice
func (c *VerticalController) GenerateSoilAdvice(ctx *fiber.Ctx) error {
	var body map[string]interface{}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString(err.Error())
	}
	crop, _ := body["crop"].(string)
	if crop == "" {
		crop = verticalQuery(ctx, "crop")
	}
	if crop == "" {
		return ctx.Status(400).SendString("crop required")
	}
	data := body
	if v, ok := body["data"].(map[string]interface{}); ok {
		data = v
	}
	resp, err := c.service.GenerateSoilAdvice(data, crop)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(normalizeToSnakeKeys(resp))
}

// D. GIS
func (c *VerticalController) CreateGISLayer(ctx *fiber.Ctx) error {
	var body map[string]interface{}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString(err.Error())
	}
	if err := c.service.CreateGISLayer(body); err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.SendStatus(201)
}

func (c *VerticalController) GetGISLayers(ctx *fiber.Ctx) error {
	projectID := verticalQuery(ctx, "project_id", "projectId")
	data, err := c.service.GetGISLayers(projectID)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(normalizeToSnakeKeys(data))
}

// E. VFD Manufacturers / Models
func (c *VerticalController) CreateVFDManufacturer(ctx *fiber.Ctx) error {
	projectID := ctx.Params("projectId")
	if projectID == "" {
		projectID = ctx.Params("project_id")
	}
	var body struct {
		Name     string         `json:"name"`
		Metadata map[string]any `json:"metadata"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString(err.Error())
	}
	if body.Name == "" || projectID == "" {
		return ctx.Status(400).SendString("missing project_id or name")
	}
	res, err := c.service.CreateVFDManufacturer(ctx.Context(), projectID, body.Name, body.Metadata)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.Status(201).JSON(normalizeToSnakeKeys(res))
}

func (c *VerticalController) GetVFDManufacturers(ctx *fiber.Ctx) error {
	projectID := ctx.Params("projectId")
	if projectID == "" {
		projectID = ctx.Params("project_id")
	}
	if projectID == "" {
		return ctx.Status(400).SendString("missing project_id")
	}
	res, err := c.service.ListVFDManufacturers(ctx.Context(), projectID)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(normalizeToSnakeKeys(res))
}

func (c *VerticalController) CreateVFDModel(ctx *fiber.Ctx) error {
	projectID := ctx.Params("projectId")
	if projectID == "" {
		projectID = ctx.Params("project_id")
	}
	var body struct {
		ManufacturerID      string           `json:"manufacturer_id"`
		ManufacturerIDCamel string           `json:"manufacturerId"`
		Model               string           `json:"model"`
		Version             string           `json:"version"`
		RS485               map[string]any   `json:"rs485"`
		Realtime            []map[string]any `json:"realtime_parameters"`
		RealtimeCamel       []map[string]any `json:"realtimeParameters"`
		Faults              []map[string]any `json:"fault_map"`
		FaultsCamel         []map[string]any `json:"faultMap"`
		Commands            []map[string]any `json:"command_dictionary"`
		CommandsCamel       []map[string]any `json:"commandDictionary"`
		Metadata            map[string]any   `json:"metadata"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString(err.Error())
	}
	if body.ManufacturerID == "" {
		body.ManufacturerID = body.ManufacturerIDCamel
	}
	if len(body.Realtime) == 0 && len(body.RealtimeCamel) > 0 {
		body.Realtime = body.RealtimeCamel
	}
	if len(body.Faults) == 0 && len(body.FaultsCamel) > 0 {
		body.Faults = body.FaultsCamel
	}
	if len(body.Commands) == 0 && len(body.CommandsCamel) > 0 {
		body.Commands = body.CommandsCamel
	}
	if projectID == "" || body.ManufacturerID == "" || body.Model == "" || body.Version == "" {
		return ctx.Status(400).SendString("missing required fields")
	}
	res, err := c.service.CreateVFDModel(ctx.Context(), projectID, body.ManufacturerID, body.Model, body.Version, body.RS485, body.Realtime, body.Faults, body.Commands, body.Metadata)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	c.service.LogAudit(getUserID(ctx), "catalog.vfd_model.created", ctx.Path(), ctx.IP(), "success", map[string]interface{}{
		"model_id":        res.ID,
		"manufacturer_id": body.ManufacturerID,
		"project_id":      projectID,
	})
	return ctx.Status(201).JSON(fiber.Map{"model": res})
}

func (c *VerticalController) UpdateVFDModel(ctx *fiber.Ctx) error {
	projectID := ctx.Params("projectId")
	if projectID == "" {
		projectID = ctx.Params("project_id")
	}
	if projectID == "" {
		projectID = verticalQuery(ctx, "project_id", "projectId")
	}
	modelID := ctx.Params("modelId")
	if modelID == "" {
		modelID = ctx.Params("model_id")
	}
	if modelID == "" {
		modelID = ctx.Params("vfdModelId")
	}
	if projectID == "" || modelID == "" {
		return ctx.Status(400).SendString("project_id and model_id required")
	}
	var body struct {
		Model         *string          `json:"model"`
		Version       *string          `json:"version"`
		RS485         map[string]any   `json:"rs485"`
		Realtime      []map[string]any `json:"realtime_parameters"`
		RealtimeCamel []map[string]any `json:"realtimeParameters"`
		Faults        []map[string]any `json:"fault_map"`
		FaultsCamel   []map[string]any `json:"faultMap"`
		Commands      []map[string]any `json:"command_dictionary"`
		CommandsCamel []map[string]any `json:"commandDictionary"`
		Metadata      map[string]any   `json:"metadata"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString(err.Error())
	}
	if body.Realtime == nil && body.RealtimeCamel != nil {
		body.Realtime = body.RealtimeCamel
	}
	if body.Faults == nil && body.FaultsCamel != nil {
		body.Faults = body.FaultsCamel
	}
	if body.Commands == nil && body.CommandsCamel != nil {
		body.Commands = body.CommandsCamel
	}
	if body.Model == nil && body.Version == nil && body.RS485 == nil && body.Realtime == nil && body.Faults == nil && body.Commands == nil && body.Metadata == nil {
		return ctx.Status(400).SendString("no fields to update")
	}
	input := services.VFDModelUpdateInput{
		Model:   body.Model,
		Version: body.Version,
	}
	if body.RS485 != nil {
		input.RS485 = &body.RS485
	}
	if body.Realtime != nil {
		input.RealtimeParameters = &body.Realtime
	}
	if body.Faults != nil {
		input.FaultMap = &body.Faults
	}
	if body.Commands != nil {
		input.CommandDictionary = &body.Commands
	}
	if body.Metadata != nil {
		input.Metadata = &body.Metadata
	}
	updated, err := c.service.UpdateVFDModel(ctx.Context(), projectID, modelID, input)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	fields := []string{}
	if body.Model != nil {
		fields = append(fields, "model")
	}
	if body.Version != nil {
		fields = append(fields, "version")
	}
	if body.RS485 != nil {
		fields = append(fields, "rs485")
	}
	if body.Realtime != nil {
		fields = append(fields, "realtime_parameters")
	}
	if body.Faults != nil {
		fields = append(fields, "fault_map")
	}
	if body.Commands != nil {
		fields = append(fields, "command_dictionary")
	}
	if body.Metadata != nil {
		fields = append(fields, "metadata")
	}
	c.service.LogAudit(getUserID(ctx), "catalog.vfd_model.updated", ctx.Path(), ctx.IP(), "success", map[string]interface{}{
		"model_id":        updated.ID,
		"manufacturer_id": updated.ManufacturerID,
		"project_id":      projectID,
		"fields_updated":  fields,
		"command_count":   len(updated.CommandDictionary),
	})
	return ctx.JSON(fiber.Map{"model": updated})
}

func (c *VerticalController) GetVFDModels(ctx *fiber.Ctx) error {
	projectID := ctx.Params("projectId")
	if projectID == "" {
		projectID = ctx.Params("project_id")
	}
	if projectID == "" {
		return ctx.Status(400).SendString("missing project_id")
	}
	res, err := c.service.ListVFDModels(ctx.Context(), projectID)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	manufacturerID := verticalQuery(ctx, "manufacturer_id", "manufacturerId")
	protocolID := verticalQuery(ctx, "protocol_version_id", "protocolVersionId")
	filtered := make([]models.VFDModel, 0, len(res))
	for _, model := range res {
		if manufacturerID != "" && model.ManufacturerID != manufacturerID {
			continue
		}
		if protocolID != "" {
			assignments, err := c.service.ListProtocolVFDAssignments(ctx.Context(), projectID, protocolID)
			if err != nil {
				return ctx.Status(500).SendString(err.Error())
			}
			matched := false
			for _, assignment := range assignments {
				if assignment.VFDModelID == model.ID {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		filtered = append(filtered, model)
	}
	return ctx.JSON(fiber.Map{"count": len(filtered), "models": filtered})
}

// GET /api/vfd-models/export.csv?project_id=...
func (c *VerticalController) ExportVFDModelsCSV(ctx *fiber.Ctx) error {
	projectID := verticalQuery(ctx, "project_id", "projectId")
	if projectID == "" {
		return ctx.Status(400).SendString("project_id required")
	}
	models, err := c.service.ListVFDModels(ctx.Context(), projectID)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}

	buf := &bytes.Buffer{}
	writer := csv.NewWriter(buf)
	_ = writer.Write([]string{"id", "manufacturer_id", "model", "version", "rs485", "realtime_parameters", "fault_map", "command_dictionary", "metadata"})

	for _, m := range models {
		rs485, _ := json.Marshal(m.RS485)
		realtime, _ := json.Marshal(m.RealtimeParameters)
		faults, _ := json.Marshal(m.FaultMap)
		commands, _ := json.Marshal(m.CommandDictionary)
		metadata, _ := json.Marshal(m.Metadata)
		_ = writer.Write([]string{m.ID, m.ManufacturerID, m.Model, m.Version, string(rs485), string(realtime), string(faults), string(commands), string(metadata)})
	}
	writer.Flush()

	ctx.Set("Content-Type", "text/csv")
	ctx.Set("Content-Disposition", "attachment; filename=vfd-models.csv")
	return ctx.Send(buf.Bytes())
}

// GET /api/vfd-models/command-dictionaries/import/jobs?project_id=...
func (c *VerticalController) ListVFDImportJobs(ctx *fiber.Ctx) error {
	projectID := verticalQuery(ctx, "project_id", "projectId")
	status := verticalQuery(ctx, "status", "status_filter")
	limit := verticalQueryInt(ctx, 25, "limit", "pageSize")
	statuses := []string{}
	if status != "" {
		statuses = append(statuses, status)
	}
	items, err := c.service.ListVFDCommandImportJobs(ctx.Context(), projectID, statuses, limit)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(fiber.Map{"jobs": items, "count": len(items)})
}

func verticalQuery(ctx *fiber.Ctx, keys ...string) string {
	return queryAlias(ctx, keys...)
}

func verticalQueryBool(ctx *fiber.Ctx, defaultValue bool, keys ...string) bool {
	return queryAliasBool(ctx, defaultValue, keys...)
}

func verticalQueryInt(ctx *fiber.Ctx, defaultValue int, keys ...string) int {
	return queryAliasInt(ctx, defaultValue, keys...)
}

// POST /vfd-models/import
func (c *VerticalController) ImportVFDModels(ctx *fiber.Ctx) error {
	var body struct {
		CSV                    string `json:"csv"`
		ManufacturerID         string `json:"manufacturer_id"`
		ManufacturerIDCamel    string `json:"manufacturerId"`
		ProtocolVersionID      string `json:"protocol_version_id"`
		ProtocolVersionIDCamel string `json:"protocolVersionId"`
		ProjectID              string `json:"project_id"`
		ProjectIDCamel         string `json:"projectId"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return validationError(ctx, "Invalid import payload", "body", err.Error())
	}
	if strings.TrimSpace(body.ManufacturerID) == "" {
		body.ManufacturerID = body.ManufacturerIDCamel
	}
	if strings.TrimSpace(body.ProtocolVersionID) == "" {
		body.ProtocolVersionID = body.ProtocolVersionIDCamel
	}
	if strings.TrimSpace(body.ProjectID) == "" {
		body.ProjectID = body.ProjectIDCamel
	}
	if strings.TrimSpace(body.CSV) == "" {
		return validationError(ctx, "Invalid import payload", "csv", "CSV payload is required")
	}
	projectID := strings.TrimSpace(body.ProjectID)
	if projectID == "" {
		return validationError(ctx, "Invalid import payload", "project_id", "project_id is required")
	}
	rows, err := parseCSVMaps(body.CSV)
	if err != nil {
		return validationError(ctx, "Invalid import payload", "csv", err.Error())
	}

	modelsCreated := []models.VFDModel{}
	for _, row := range rows {
		manufacturerID := strings.TrimSpace(body.ManufacturerID)
		if manufacturerID == "" {
			if val, ok := row["manufacturer_id"].(string); ok {
				manufacturerID = strings.TrimSpace(val)
			}
			if val, ok := row["manufacturerId"].(string); ok && manufacturerID == "" {
				manufacturerID = strings.TrimSpace(val)
			}
		}
		model, _ := row["model"].(string)
		version, _ := row["version"].(string)
		if manufacturerID == "" || strings.TrimSpace(model) == "" || strings.TrimSpace(version) == "" {
			return validationError(ctx, "Invalid import payload", "csv", "manufacturer_id, model, and version are required")
		}

		rs485 := parseJSONField(row["rs485"])
		realtime := parseJSONArray(row["realtime_parameters"])
		faults := parseJSONArray(row["fault_map"])
		commands := parseJSONArray(row["command_dictionary"])
		metadata := parseJSONField(row["metadata"])

		created, err := c.service.CreateVFDModel(ctx.Context(), projectID, manufacturerID, strings.TrimSpace(model), strings.TrimSpace(version), rs485, realtime, faults, commands, metadata)
		if err != nil {
			return ctx.Status(500).SendString(err.Error())
		}
		modelsCreated = append(modelsCreated, created)

		if strings.TrimSpace(body.ProtocolVersionID) != "" {
			_, _ = c.service.AssignProtocolVFD(ctx.Context(), projectID, body.ProtocolVersionID, created.ID, "", nil)
		}
	}
	c.service.LogAudit(getUserID(ctx), "catalog.vfd_model.imported", ctx.Path(), ctx.IP(), "success", map[string]interface{}{
		"project_id": projectID,
		"count":      len(modelsCreated),
	})

	return ctx.Status(201).JSON(fiber.Map{"models": modelsCreated, "count": len(modelsCreated)})
}

// POST /vfd-models/command-dictionaries/import
func (c *VerticalController) ImportVFDCommandDictionary(ctx *fiber.Ctx) error {
	var body struct {
		ProjectID          string `json:"project_id"`
		ProjectIDCamel     string `json:"projectId"`
		ModelID            string `json:"model_id"`
		ModelIDCamel       string `json:"modelId"`
		CSV                string `json:"csv"`
		JSON               string `json:"json"`
		MergeStrategy      string `json:"merge_strategy"`
		MergeStrategyCamel string `json:"mergeStrategy"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return validationError(ctx, "Invalid import payload", "body", err.Error())
	}
	if strings.TrimSpace(body.ProjectID) == "" {
		body.ProjectID = body.ProjectIDCamel
	}
	if strings.TrimSpace(body.ModelID) == "" {
		body.ModelID = body.ModelIDCamel
	}
	if strings.TrimSpace(body.MergeStrategy) == "" {
		body.MergeStrategy = body.MergeStrategyCamel
	}
	if strings.TrimSpace(body.ProjectID) == "" || strings.TrimSpace(body.ModelID) == "" {
		return validationError(ctx, "Invalid import payload", "project_id", "project_id and model_id are required")
	}
	if strings.TrimSpace(body.CSV) == "" && strings.TrimSpace(body.JSON) == "" {
		return validationError(ctx, "Invalid import payload", "csv", "Provide either csv or json payload")
	}
	if strings.TrimSpace(body.CSV) != "" && strings.TrimSpace(body.JSON) != "" {
		return validationError(ctx, "Invalid import payload", "csv", "Provide either csv or json payload")
	}
	commands := []map[string]any{}
	if strings.TrimSpace(body.CSV) != "" {
		parsed, err := parseCSVMaps(body.CSV)
		if err != nil {
			return validationError(ctx, "Invalid import payload", "csv", err.Error())
		}
		commands = parsed
	} else {
		parsed := parseJSONArray(body.JSON)
		commands = parsed
	}

	model, err := c.service.ImportVFDArtifacts(ctx.Context(), body.ProjectID, body.ModelID, body.MergeStrategy, nil, nil, nil, commands)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	c.service.LogAudit(getUserID(ctx), "catalog.vfd_command_dictionary.imported", ctx.Path(), ctx.IP(), "success", map[string]interface{}{
		"project_id": body.ProjectID,
		"model_id":   body.ModelID,
		"count":      len(commands),
	})
	return ctx.Status(200).JSON(fiber.Map{"model": model})
}

func (c *VerticalController) ImportVFDModelArtifacts(ctx *fiber.Ctx) error {
	projectID := ctx.Params("projectId")
	modelID := ctx.Params("modelId")
	if projectID == "" || modelID == "" {
		return ctx.Status(400).SendString("missing project_id or model_id")
	}

	var body struct {
		MergeStrategy      string           `json:"merge_strategy"`
		MergeStrategyCamel string           `json:"mergeStrategy"`
		RS485              map[string]any   `json:"rs485"`
		Realtime           []map[string]any `json:"realtime_parameters"`
		Faults             []map[string]any `json:"fault_map"`
		Commands           []map[string]any `json:"command_dictionary"`
		FaultsCSV          string           `json:"fault_map_csv"`
		CommandsCSV        string           `json:"command_dictionary_csv"`
	}

	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString(err.Error())
	}
	if strings.TrimSpace(body.MergeStrategy) == "" {
		body.MergeStrategy = body.MergeStrategyCamel
	}

	if len(body.Commands) == 0 && strings.TrimSpace(body.CommandsCSV) != "" {
		parsed, err := parseCSVMaps(body.CommandsCSV)
		if err != nil {
			return ctx.Status(400).SendString(fmt.Sprintf("invalid command_dictionary_csv: %v", err))
		}
		body.Commands = parsed
	}

	if len(body.Faults) == 0 && strings.TrimSpace(body.FaultsCSV) != "" {
		parsed, err := parseCSVMaps(body.FaultsCSV)
		if err != nil {
			return ctx.Status(400).SendString(fmt.Sprintf("invalid fault_map_csv: %v", err))
		}
		body.Faults = parsed
	}

	res, err := c.service.ImportVFDArtifacts(ctx.Context(), projectID, modelID, body.MergeStrategy, body.RS485, body.Realtime, body.Faults, body.Commands)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.Status(200).JSON(res)
}

// F. Protocol ↔ VFD assignments
func (c *VerticalController) CreateProtocolVFDAssignment(ctx *fiber.Ctx) error {
	projectID := ctx.Params("projectId")
	protocolID := ctx.Params("protocolId")
	var body struct {
		VFDModelID string         `json:"vfd_model_id"`
		AssignedBy string         `json:"assigned_by"`
		Metadata   map[string]any `json:"metadata"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString(err.Error())
	}
	if projectID == "" || protocolID == "" || body.VFDModelID == "" {
		return ctx.Status(400).SendString("missing required fields")
	}
	res, err := c.service.AssignProtocolVFD(ctx.Context(), projectID, protocolID, body.VFDModelID, body.AssignedBy, body.Metadata)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	c.service.LogAudit(getUserID(ctx), "catalog.vfd_assignment.created", ctx.Path(), ctx.IP(), "success", map[string]interface{}{
		"assignment_id": res.ID,
		"protocol_id":   protocolID,
		"model_id":      body.VFDModelID,
		"project_id":    projectID,
	})
	return ctx.Status(201).JSON(fiber.Map{"assignment": res})
}

func (c *VerticalController) GetProtocolVFDAssignments(ctx *fiber.Ctx) error {
	projectID := ctx.Params("projectId")
	protocolID := ctx.Params("protocolId")
	if projectID == "" || protocolID == "" {
		return ctx.Status(400).SendString("missing ids")
	}
	res, err := c.service.ListProtocolVFDAssignments(ctx.Context(), projectID, protocolID)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	includeRevoked := verticalQueryBool(ctx, false, "include_revoked", "includeRevoked")
	assignments := make([]models.ProtocolVFDAssignment, 0, len(res))
	for _, assignment := range res {
		if !includeRevoked && assignment.RevokedAt != nil {
			continue
		}
		assignments = append(assignments, assignment)
	}
	return ctx.JSON(fiber.Map{"assignments": assignments})
}

func (c *VerticalController) RevokeProtocolVFDAssignment(ctx *fiber.Ctx) error {
	projectID := ctx.Params("projectId")
	assignmentID := ctx.Params("id")
	var body struct {
		RevokedBy string         `json:"revoked_by"`
		Reason    string         `json:"reason"`
		Metadata  map[string]any `json:"metadata"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString(err.Error())
	}
	if projectID == "" || assignmentID == "" {
		return ctx.Status(400).SendString("missing ids")
	}
	if err := c.service.RevokeProtocolVFDAssignment(ctx.Context(), projectID, assignmentID, body.RevokedBy, body.Reason, body.Metadata); err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	c.service.LogAudit(getUserID(ctx), "catalog.vfd_assignment.revoked", ctx.Path(), ctx.IP(), "success", map[string]interface{}{
		"assignment_id": assignmentID,
		"project_id":    projectID,
		"reason":        body.Reason,
	})
	return ctx.SendStatus(204)
}

func parseCSVMaps(raw string) ([]map[string]any, error) {
	r := csv.NewReader(strings.NewReader(raw))
	r.TrimLeadingSpace = true
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("csv payload is empty")
	}
	header := records[0]
	if len(header) == 0 {
		return nil, fmt.Errorf("csv header is empty")
	}
	var out []map[string]any
	for i, row := range records[1:] {
		if len(row) != len(header) {
			return nil, fmt.Errorf("row %d: expected %d columns, got %d", i+1, len(header), len(row))
		}
		entry := make(map[string]any, len(header))
		for idx, key := range header {
			entry[key] = row[idx]
		}
		out = append(out, entry)
	}
	return out, nil
}

func parseJSONField(value interface{}) map[string]any {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case map[string]any:
		return typed
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return nil
		}
		var out map[string]any
		if err := json.Unmarshal([]byte(trimmed), &out); err == nil {
			return out
		}
	}
	return nil
}

func parseJSONArray(value interface{}) []map[string]any {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case []map[string]any:
		return typed
	case []interface{}:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if asMap, ok := item.(map[string]any); ok {
				out = append(out, asMap)
			}
		}
		return out
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return nil
		}
		var out []map[string]any
		if err := json.Unmarshal([]byte(trimmed), &out); err == nil {
			return out
		}
	}
	return nil
}
