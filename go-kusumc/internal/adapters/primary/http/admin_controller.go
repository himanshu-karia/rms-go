package http

import (
	"strconv"
	"strings"

	"ingestion-go/internal/core/services"
	"ingestion-go/internal/models"

	"github.com/gofiber/fiber/v2"
)

type AdminController struct {
	service   *services.AdminService
	protocols *services.ProtocolService
}

func NewAdminController(service *services.AdminService, protocols *services.ProtocolService) *AdminController {
	return &AdminController{service: service, protocols: protocols}
}

func (c *AdminController) GetStates(ctx *fiber.Ctx) error {
	data, err := c.service.GetStates()
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(normalizeToSnakeKeys(data))
}

func (c *AdminController) CreateState(ctx *fiber.Ctx) error {
	var body struct {
		Name     string                 `json:"name"`
		IsoCode  *string                `json:"iso_code"`
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString(err.Error())
	}
	if body.Name == "" {
		return ctx.Status(400).SendString("name required")
	}
	state, err := c.service.CreateState(body.Name, body.IsoCode, body.Metadata)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	c.service.LogAudit(getUserID(ctx), "admin.state.created", ctx.Path(), ctx.IP(), "success", map[string]interface{}{
		"stateId":   state["id"],
		"stateName": state["name"],
		"isoCode":   state["iso_code"],
	})
	wire := normalizeToSnakeKeys(state)
	return ctx.Status(201).JSON(fiber.Map{"state": wire, "state_record": wire})
}

func (c *AdminController) UpdateState(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	if id == "" {
		return ctx.Status(400).SendString("id required")
	}
	var body struct {
		Name     *string                `json:"name"`
		IsoCode  *string                `json:"iso_code"`
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString(err.Error())
	}
	if body.Name == nil && body.IsoCode == nil && body.Metadata == nil {
		return ctx.Status(400).SendString("no fields to update")
	}
	state, err := c.service.UpdateState(id, body.Name, body.IsoCode, body.Metadata)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	c.service.LogAudit(getUserID(ctx), "admin.state.updated", ctx.Path(), ctx.IP(), "success", map[string]interface{}{
		"stateId":   state["id"],
		"stateName": state["name"],
		"isoCode":   state["iso_code"],
	})
	wire := normalizeToSnakeKeys(state)
	return ctx.JSON(fiber.Map{"state": wire, "state_record": wire})
}

func (c *AdminController) DeleteState(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	if id == "" {
		return ctx.Status(400).SendString("id required")
	}
	if err := c.service.DeleteState(id); err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	c.service.LogAudit(getUserID(ctx), "admin.state.deleted", ctx.Path(), ctx.IP(), "success", map[string]interface{}{
		"stateId": id,
	})
	return ctx.SendStatus(204)
}

func (c *AdminController) GetAuthorities(ctx *fiber.Ctx) error {
	stateId := adminFilterQuery(ctx, "state_id", "stateId")
	if strings.Contains(ctx.Path(), "/lookup/") && strings.TrimSpace(stateId) == "" {
		return validationError(ctx, "Invalid query parameters", "stateId", "stateId is required")
	}
	data, err := c.service.GetAuthorities(stateId)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(normalizeToSnakeKeys(data))
}

func (c *AdminController) CreateAuthority(ctx *fiber.Ctx) error {
	var body struct {
		StateID      string                 `json:"state_id"`
		StateIDCamel string                 `json:"stateId"`
		Name         string                 `json:"name"`
		Type         string                 `json:"type"`
		ContactInfo  map[string]interface{} `json:"contact_info"`
		Metadata     map[string]interface{} `json:"metadata"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString(err.Error())
	}
	if strings.TrimSpace(body.StateID) == "" {
		body.StateID = strings.TrimSpace(body.StateIDCamel)
	}
	if body.ContactInfo == nil && body.Metadata != nil {
		body.ContactInfo = body.Metadata
	}
	if body.StateID == "" || body.Name == "" {
		return ctx.Status(400).SendString("state_id and name required")
	}
	authority, err := c.service.CreateAuthority(body.StateID, body.Name, body.Type, body.ContactInfo)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	c.service.LogAudit(getUserID(ctx), "admin.authority.created", ctx.Path(), ctx.IP(), "success", map[string]interface{}{
		"authorityId":   authority["id"],
		"authorityName": authority["name"],
		"stateId":       authority["state_id"],
		"type":          authority["type"],
	})
	wire := normalizeToSnakeKeys(authority)
	return ctx.Status(201).JSON(fiber.Map{"authority": wire, "state_authority": wire, "stateAuthority": wire})
}

func (c *AdminController) UpdateAuthority(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	if id == "" {
		return ctx.Status(400).SendString("id required")
	}
	var body struct {
		Name        *string                `json:"name"`
		Type        *string                `json:"type"`
		ContactInfo map[string]interface{} `json:"contact_info"`
		Metadata    map[string]interface{} `json:"metadata"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString(err.Error())
	}
	if body.ContactInfo == nil && body.Metadata != nil {
		body.ContactInfo = body.Metadata
	}
	if body.Name == nil && body.Type == nil && body.ContactInfo == nil {
		return ctx.Status(400).SendString("no fields to update")
	}
	authority, err := c.service.UpdateAuthority(id, body.Name, body.Type, body.ContactInfo)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	c.service.LogAudit(getUserID(ctx), "admin.authority.updated", ctx.Path(), ctx.IP(), "success", map[string]interface{}{
		"authorityId":   authority["id"],
		"authorityName": authority["name"],
		"stateId":       authority["state_id"],
		"type":          authority["type"],
	})
	wire := normalizeToSnakeKeys(authority)
	return ctx.JSON(fiber.Map{"authority": wire, "state_authority": wire, "stateAuthority": wire})
}

func (c *AdminController) DeleteAuthority(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	if id == "" {
		return ctx.Status(400).SendString("id required")
	}
	if err := c.service.DeleteAuthority(id); err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	c.service.LogAudit(getUserID(ctx), "admin.authority.deleted", ctx.Path(), ctx.IP(), "success", map[string]interface{}{
		"authorityId": id,
	})
	return ctx.SendStatus(204)
}
func (c *AdminController) GetUsers(ctx *fiber.Ctx) error {
	data, err := c.service.GetUsers()
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	resp := make([]wireUserSummary, 0, len(data))
	for _, u := range data {
		resp = append(resp, toWireUserSummaryFromRecord(u))
	}
	return ctx.JSON(fiber.Map{"users": resp})
}

func (c *AdminController) ListUsers(ctx *fiber.Ctx) error {
	limit := 0
	if raw := adminLimitQuery(ctx); raw != "" {
		if val, err := strconv.Atoi(raw); err == nil {
			limit = val
		} else {
			return validationError(ctx, "Invalid query parameters", "limit", "limit must be an integer")
		}
	}
	if limit < 0 || limit > 100 {
		return validationError(ctx, "Invalid query parameters", "limit", "limit must be between 1 and 100")
	}

	status := adminStatusQuery(ctx)
	if status != "" && status != "active" && status != "disabled" {
		return validationError(ctx, "Invalid query parameters", "status", "status must be active or disabled")
	}

	cursor := adminCursorQuery(ctx)
	if cursor != "" && !isUUID(cursor) {
		return validationError(ctx, "Invalid query parameters", "cursor", "cursor must be a valid UUID")
	}

	result, err := c.service.ListUsers(services.ListUsersInput{
		Status:      status,
		Query:       adminFilterQuery(ctx, "query", "q"),
		RoleKey:     adminFilterQuery(ctx, "role_key", "roleKey"),
		RoleType:    adminFilterQuery(ctx, "role_type", "roleType"),
		StateID:     adminFilterQuery(ctx, "state_id", "stateId"),
		AuthorityID: adminFilterQuery(ctx, "authority_id", "authorityId"),
		ProjectID:   adminFilterQuery(ctx, "project_id", "projectId"),
		GroupID:     adminFilterQuery(ctx, "group_id", "groupId"),
		Cursor:      cursor,
		Limit:       limit,
	})
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(toWireListUsersResult(result))
}

func (c *AdminController) GetUser(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	if id == "" {
		return ctx.Status(400).SendString("id required")
	}
	user, err := c.service.GetUserByID(id)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	if user == nil {
		return ctx.Status(404).SendString("user not found")
	}
	return ctx.JSON(fiber.Map{"user": toWireUserSummaryFromRecord(*user)})
}

func (c *AdminController) CreateUser(ctx *fiber.Ctx) error {
	var body struct {
		Username           string                 `json:"username"`
		Email              *string                `json:"email"`
		DisplayName        *string                `json:"display_name"`
		DisplayNameCamel   *string                `json:"displayName"`
		Password           string                 `json:"password"`
		Role               *string                `json:"role"`
		Status             *string                `json:"status"`
		MustRotatePassword *bool                  `json:"must_rotate_password"`
		MustRotateCamel    *bool                  `json:"mustRotatePassword"`
		Metadata           map[string]interface{} `json:"metadata"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString(err.Error())
	}
	if body.Username == "" || body.Password == "" {
		return ctx.Status(400).SendString("username and password required")
	}
	if body.DisplayName == nil {
		body.DisplayName = body.DisplayNameCamel
	}
	if body.MustRotatePassword == nil {
		body.MustRotatePassword = body.MustRotateCamel
	}

	active := true
	if body.Status != nil {
		switch *body.Status {
		case "active":
			active = true
		case "disabled":
			active = false
		default:
			return ctx.Status(400).SendString("invalid status")
		}
	}

	mustRotate := false
	if body.MustRotatePassword != nil {
		mustRotate = *body.MustRotatePassword
	}

	role := ""
	if body.Role != nil {
		role = *body.Role
	}

	user, err := c.service.CreateUser(body.Username, body.Email, body.DisplayName, body.Password, role, active, mustRotate, body.Metadata)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.Status(201).JSON(fiber.Map{"user": toWireUserSummaryFromRecord(*user)})
}

func (c *AdminController) UpdateUser(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	if id == "" {
		return ctx.Status(400).SendString("id required")
	}
	var body struct {
		DisplayName        *string                `json:"display_name"`
		DisplayNameCamel   *string                `json:"displayName"`
		Status             *string                `json:"status"`
		MustRotatePassword *bool                  `json:"must_rotate_password"`
		MustRotateCamel    *bool                  `json:"mustRotatePassword"`
		Metadata           map[string]interface{} `json:"metadata"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString(err.Error())
	}
	if body.DisplayName == nil {
		body.DisplayName = body.DisplayNameCamel
	}
	if body.MustRotatePassword == nil {
		body.MustRotatePassword = body.MustRotateCamel
	}
	if body.DisplayName == nil && body.Status == nil && body.MustRotatePassword == nil && body.Metadata == nil {
		return ctx.Status(400).SendString("no fields to update")
	}

	var active *bool
	if body.Status != nil {
		switch *body.Status {
		case "active":
			val := true
			active = &val
		case "disabled":
			val := false
			active = &val
		default:
			return ctx.Status(400).SendString("invalid status")
		}
	}

	user, err := c.service.UpdateUser(id, body.DisplayName, active, body.MustRotatePassword, body.Metadata)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	if user == nil {
		return ctx.Status(404).SendString("user not found")
	}
	return ctx.JSON(fiber.Map{"user": toWireUserSummaryFromRecord(*user)})
}

func (c *AdminController) ResetUserPassword(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	if id == "" {
		return ctx.Status(400).SendString("id required")
	}
	var body struct {
		Password              string `json:"password"`
		RequirePasswordChange *bool  `json:"require_password_change"`
		RequireChangeCamel    *bool  `json:"requirePasswordChange"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString(err.Error())
	}
	if body.Password == "" {
		return ctx.Status(400).SendString("password required")
	}
	if body.RequirePasswordChange == nil {
		body.RequirePasswordChange = body.RequireChangeCamel
	}
	mustRotate := true
	if body.RequirePasswordChange != nil {
		mustRotate = *body.RequirePasswordChange
	}

	user, err := c.service.ResetUserPassword(id, body.Password, mustRotate)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	if user == nil {
		return ctx.Status(404).SendString("user not found")
	}
	return ctx.JSON(fiber.Map{"user": toWireUserSummaryFromRecord(*user)})
}

func (c *AdminController) AssignUserRole(ctx *fiber.Ctx) error {
	userID := ctx.Params("id")
	if userID == "" {
		return ctx.Status(400).SendString("id required")
	}
	var body struct {
		RoleID        *string                `json:"role_id"`
		RoleIDCamel   *string                `json:"roleId"`
		RoleKey       *string                `json:"role_key"`
		RoleKeyCamel  *string                `json:"roleKey"`
		RoleType      *string                `json:"role_type"`
		RoleTypeCamel *string                `json:"roleType"`
		Scope         map[string]interface{} `json:"scope"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString(err.Error())
	}
	roleKey := ""
	if body.RoleKey != nil {
		roleKey = *body.RoleKey
	} else if body.RoleKeyCamel != nil {
		roleKey = *body.RoleKeyCamel
	} else if body.RoleID != nil {
		roleKey = *body.RoleID
	} else if body.RoleIDCamel != nil {
		roleKey = *body.RoleIDCamel
	}
	if roleKey == "" {
		return ctx.Status(400).SendString("roleKey required")
	}
	roleType := ""
	if body.RoleType != nil {
		roleType = *body.RoleType
	} else if body.RoleTypeCamel != nil {
		roleType = *body.RoleTypeCamel
	}
	user, err := c.service.AssignUserRole(userID, roleKey, roleType, body.Scope)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	if user == nil {
		return ctx.Status(404).SendString("user not found")
	}
	return ctx.Status(201).JSON(fiber.Map{"user": toWireUserSummary(*user)})
}

func (c *AdminController) RemoveUserRole(ctx *fiber.Ctx) error {
	userID := ctx.Params("id")
	bindingID := ctx.Params("bindingId")
	if userID == "" || bindingID == "" {
		return ctx.Status(400).SendString("id and bindingId required")
	}
	user, err := c.service.RemoveUserRole(userID, bindingID)
	if err != nil {
		if err.Error() == "role binding not found" {
			return ctx.Status(404).SendString("role binding not found")
		}
		return ctx.Status(500).SendString(err.Error())
	}
	if user == nil {
		return ctx.Status(404).SendString("user not found")
	}
	return ctx.JSON(fiber.Map{"user": toWireUserSummary(*user)})
}

func (c *AdminController) DeleteUser(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	if id == "" {
		return ctx.Status(400).SendString("id required")
	}
	if err := c.service.DeleteUser(id); err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.SendStatus(204)
}

func (c *AdminController) GetUserCapabilities(ctx *fiber.Ctx) error {
	items, err := c.service.ListCapabilities()
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(fiber.Map{"capabilities": items})
}

func (c *AdminController) GetUserRoles(ctx *fiber.Ctx) error {
	roles, err := c.service.ListRoles()
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(fiber.Map{"roles": roles})
}

func (c *AdminController) ListVendors(ctx *fiber.Ctx, category string) error {
	items, err := c.service.ListVendors(category)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	return ctx.JSON(fiber.Map{"vendors": items})
}

func (c *AdminController) CreateVendor(ctx *fiber.Ctx, category string) error {
	var body struct {
		Name     string                 `json:"name"`
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString(err.Error())
	}
	if body.Name == "" {
		return ctx.Status(400).SendString("name required")
	}
	item, err := c.service.CreateVendor(body.Name, category, body.Metadata)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	c.service.LogAudit(getUserID(ctx), "admin.vendor.created", ctx.Path(), ctx.IP(), "success", map[string]interface{}{
		"vendorId":   item.ID,
		"vendorName": item.Name,
		"category":   category,
	})
	return ctx.Status(201).JSON(fiber.Map{"vendor": item})
}

func (c *AdminController) UpdateVendor(ctx *fiber.Ctx, category string) error {
	id := ctx.Params("id")
	if id == "" {
		return ctx.Status(400).SendString("id required")
	}
	var body struct {
		Name     *string                `json:"name"`
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString(err.Error())
	}
	if body.Name == nil && body.Metadata == nil {
		return ctx.Status(400).SendString("no fields to update")
	}
	item, err := c.service.UpdateVendor(id, body.Name, &category, body.Metadata)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	c.service.LogAudit(getUserID(ctx), "admin.vendor.updated", ctx.Path(), ctx.IP(), "success", map[string]interface{}{
		"vendorId":   item.ID,
		"vendorName": item.Name,
		"category":   category,
	})
	return ctx.JSON(fiber.Map{"vendor": item})
}

func (c *AdminController) DeleteVendor(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	if id == "" {
		return ctx.Status(400).SendString("id required")
	}
	if err := c.service.DeleteVendor(id); err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	c.service.LogAudit(getUserID(ctx), "admin.vendor.deleted", ctx.Path(), ctx.IP(), "success", map[string]interface{}{
		"vendorId": id,
	})
	return ctx.SendStatus(204)
}

func (c *AdminController) ListProtocolVersions(ctx *fiber.Ctx) error {
	projectID := adminFilterQuery(ctx, "project_id", "projectId")
	if projectID == "" {
		return ctx.Status(400).SendString("project_id required")
	}
	filter := services.ProtocolVersionFilter{
		StateID:        adminFilterQuery(ctx, "state_id", "stateId"),
		AuthorityID:    adminFilterQuery(ctx, "state_authority_id", "stateAuthorityId"),
		ProjectID:      projectID,
		ServerVendorID: adminFilterQuery(ctx, "server_vendor_id", "serverVendorId"),
	}
	items, err := c.protocols.ListProtocolVersions(ctx.Context(), filter)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	orgNames := map[string]string{}
	for _, item := range items {
		if item.ServerVendor != "" {
			if _, ok := orgNames[item.ServerVendor]; !ok {
				if org, err := c.service.GetOrgByID(item.ServerVendor); err == nil && org != nil {
					orgNames[item.ServerVendor] = org.Name
				}
			}
		}
	}
	resp := make([]fiber.Map, 0, len(items))
	for _, item := range items {
		resp = append(resp, protocolVersionResponse(item, orgNames))
	}
	return ctx.JSON(fiber.Map{"protocol_versions": resp, "protocolVersions": resp, "count": len(resp)})
}

func adminFilterQuery(ctx *fiber.Ctx, primaryKey, aliasKey string) string {
	return queryAlias(ctx, primaryKey, aliasKey)
}

func adminCursorQuery(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "cursor", "after_id", "afterId")
}

func adminLimitQuery(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "limit", "pageSize")
}

func adminStatusQuery(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "status", "status_filter")
}

func (c *AdminController) CreateProtocolVersion(ctx *fiber.Ctx) error {
	var body struct {
		StateID             string                 `json:"state_id"`
		StateIDCamel        string                 `json:"stateId"`
		AuthorityID         string                 `json:"state_authority_id"`
		AuthorityIDCamel    string                 `json:"stateAuthorityId"`
		ProjectID           string                 `json:"project_id"`
		ProjectIDCamel      string                 `json:"projectId"`
		ServerVendorID      string                 `json:"server_vendor_id"`
		ServerVendorIDCamel string                 `json:"serverVendorId"`
		Version             string                 `json:"version"`
		Name                string                 `json:"name"`
		Metadata            map[string]interface{} `json:"metadata"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString(err.Error())
	}
	if body.ProjectID == "" {
		body.ProjectID = body.ProjectIDCamel
	}
	if body.StateID == "" {
		body.StateID = body.StateIDCamel
	}
	if body.AuthorityID == "" {
		body.AuthorityID = body.AuthorityIDCamel
	}
	if body.ServerVendorID == "" {
		body.ServerVendorID = body.ServerVendorIDCamel
	}
	if body.ProjectID == "" || body.StateID == "" || body.AuthorityID == "" || body.ServerVendorID == "" || body.Version == "" {
		return ctx.Status(400).SendString("state_id, state_authority_id, project_id, server_vendor_id, and version required")
	}
	meta := map[string]any{}
	for k, v := range body.Metadata {
		meta[k] = v
	}
	item, err := c.protocols.CreateProtocolVersion(ctx.Context(), body.ProjectID, body.StateID, body.AuthorityID, body.ServerVendorID, body.Version, body.Name, meta)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	c.service.LogAudit(getUserID(ctx), "admin.protocol_version.created", ctx.Path(), ctx.IP(), "success", map[string]interface{}{
		"protocol_version_id": item.ID,
		"project_id":          item.ProjectID,
		"state_id":            body.StateID,
		"authority_id":        body.AuthorityID,
		"server_vendor_id":    item.ServerVendor,
		"version":             body.Version,
	})
	orgNames := map[string]string{}
	if body.ServerVendorID != "" {
		if org, err := c.service.GetOrgByID(body.ServerVendorID); err == nil && org != nil {
			orgNames[body.ServerVendorID] = org.Name
		}
	}
	wire := protocolVersionResponse(item, orgNames)
	return ctx.Status(201).JSON(fiber.Map{"protocol_version": wire, "protocolVersion": wire})
}

func (c *AdminController) UpdateProtocolVersion(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	if id == "" {
		return ctx.Status(400).SendString("id required")
	}
	var body struct {
		Version             *string                `json:"version"`
		Name                *string                `json:"name"`
		ServerVendorID      *string                `json:"server_vendor_id"`
		ServerVendorIDCamel *string                `json:"serverVendorId"`
		Metadata            map[string]interface{} `json:"metadata"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(400).SendString(err.Error())
	}
	if body.ServerVendorID == nil {
		body.ServerVendorID = body.ServerVendorIDCamel
	}
	if body.Version == nil && body.Name == nil && body.ServerVendorID == nil && body.Metadata == nil {
		return ctx.Status(400).SendString("no fields to update")
	}
	meta := map[string]any{}
	for k, v := range body.Metadata {
		meta[k] = v
	}
	item, err := c.protocols.UpdateProtocolVersion(ctx.Context(), id, body.Version, body.Name, body.ServerVendorID, meta)
	if err != nil {
		return ctx.Status(500).SendString(err.Error())
	}
	stateId := ""
	authorityId := ""
	if item.Metadata != nil {
		if raw, ok := item.Metadata["state_id"].(string); ok {
			stateId = raw
		}
		if raw, ok := item.Metadata["authority_id"].(string); ok {
			authorityId = raw
		}
	}
	c.service.LogAudit(getUserID(ctx), "admin.protocol_version.updated", ctx.Path(), ctx.IP(), "success", map[string]interface{}{
		"protocol_version_id": item.ID,
		"project_id":          item.ProjectID,
		"state_id":            stateId,
		"authority_id":        authorityId,
		"server_vendor_id":    item.ServerVendor,
	})
	orgNames := map[string]string{}
	if item.ServerVendor != "" {
		if org, err := c.service.GetOrgByID(item.ServerVendor); err == nil && org != nil {
			orgNames[item.ServerVendor] = org.Name
		}
	}
	wire := protocolVersionResponse(item, orgNames)
	return ctx.JSON(fiber.Map{"protocol_version": wire, "protocolVersion": wire})
}

func protocolVersionResponse(item models.ProtocolProfile, orgNames map[string]string) fiber.Map {
	stateID, _ := item.Metadata["state_id"].(string)
	if stateID == "" {
		if v, ok := item.Metadata["stateId"].(string); ok {
			stateID = v
		}
	}
	authorityID, _ := item.Metadata["authority_id"].(string)
	if authorityID == "" {
		if v, ok := item.Metadata["authorityId"].(string); ok {
			authorityID = v
		}
	}
	version, _ := item.Metadata["version"].(string)
	name, _ := item.Metadata["name"].(string)
	serverVendorName := ""
	if item.ServerVendor != "" {
		serverVendorName = orgNames[item.ServerVendor]
	}
	return fiber.Map{
		"id":                 item.ID,
		"state_id":           stateID,
		"authority_id":       authorityID,
		"project_id":         item.ProjectID,
		"server_vendor_id":   item.ServerVendor,
		"server_vendor_name": serverVendorName,
		"version":            version,
		"name":               name,
		"metadata":           item.Metadata,
		"created_at":         item.CreatedAt,
		"updated_at":         item.UpdatedAt,
	}
}
