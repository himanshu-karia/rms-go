package http

import (
	"strconv"
	"strings"

	"ingestion-go/internal/core/services"

	"github.com/gofiber/fiber/v2"
)

type UserGroupsController struct {
	svc *services.UserGroupService
}

func NewUserGroupsController(svc *services.UserGroupService) *UserGroupsController {
	return &UserGroupsController{svc: svc}
}

func (c *UserGroupsController) List(ctx *fiber.Ctx) error {
	filters := services.UserGroupFilters{
		StateID:     userGroupsFilterQuery(ctx, "state_id", "stateId"),
		AuthorityID: userGroupsFilterQuery(ctx, "authority_id", "authorityId"),
		ProjectID:   userGroupsFilterQuery(ctx, "project_id", "projectId"),
	}
	limit := 0
	if raw := userGroupsLimitQuery(ctx); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	if limit < 0 || limit > 100 {
		return validationError(ctx, "Invalid query parameters", "limit", "limit must be between 1 and 100")
	}
	groups, err := c.svc.List(filters)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	if limit > 0 && len(groups) > limit {
		groups = groups[:limit]
	}
	wired := make([]wireUserGroup, 0, len(groups))
	for _, g := range groups {
		wired = append(wired, toWireUserGroup(g))
	}
	return ctx.JSON(fiber.Map{"groups": wired})
}

func userGroupsFilterQuery(ctx *fiber.Ctx, primaryKey, aliasKey string) string {
	return queryAlias(ctx, primaryKey, aliasKey)
}

func userGroupsLimitQuery(ctx *fiber.Ctx) string {
	return queryAlias(ctx, "limit", "pageSize")
}

func (c *UserGroupsController) Create(ctx *fiber.Ctx) error {
	var body struct {
		Name                string                 `json:"name"`
		Description         *string                `json:"description"`
		Scope               map[string]string      `json:"scope"`
		DefaultRoleIDs      []string               `json:"default_role_ids"`
		DefaultRoleIDsCamel []string               `json:"defaultRoleIds"`
		Metadata            map[string]interface{} `json:"metadata"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return validationError(ctx, "Invalid group payload", "body", "invalid body")
	}
	if len(body.DefaultRoleIDs) == 0 {
		body.DefaultRoleIDs = body.DefaultRoleIDsCamel
	}
	if strings.TrimSpace(body.Name) == "" {
		return validationError(ctx, "Invalid group payload", "name", "name is required")
	}
	internalScope := normalizeUserGroupScopeIn(body.Scope)
	if len(internalScope) == 0 {
		return validationError(ctx, "Invalid group payload", "scope", "scope is required")
	}
	if len(body.DefaultRoleIDs) == 0 {
		return validationError(ctx, "Invalid group payload", "default_role_ids", "Provide at least one default role identifier")
	}
	group, err := c.svc.Create(services.UserGroupCreateInput{
		Name:           body.Name,
		Description:    body.Description,
		Scope:          internalScope,
		DefaultRoleIDs: body.DefaultRoleIDs,
		Metadata:       body.Metadata,
	})
	if err != nil {
		return ctx.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(201).JSON(fiber.Map{"group": toWireUserGroup(*group)})
}

func (c *UserGroupsController) Update(ctx *fiber.Ctx) error {
	groupID := ctx.Params("groupId")
	if strings.TrimSpace(groupID) == "" {
		return validationError(ctx, "Invalid group identifier", "group_id", "group_id is required")
	}
	var body struct {
		Name                *string                 `json:"name"`
		Description         *string                 `json:"description"`
		DefaultRoleIDs      *[]string               `json:"default_role_ids"`
		DefaultRoleIDsCamel *[]string               `json:"defaultRoleIds"`
		Metadata            *map[string]interface{} `json:"metadata"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return validationError(ctx, "Invalid group update payload", "body", "invalid body")
	}
	if body.DefaultRoleIDs == nil {
		body.DefaultRoleIDs = body.DefaultRoleIDsCamel
	}
	if body.Name == nil && body.Description == nil && body.DefaultRoleIDs == nil && body.Metadata == nil {
		return validationError(ctx, "Invalid group update payload", "body", "Provide at least one field to update")
	}

	group, err := c.svc.Update(groupID, services.UserGroupUpdateInput{
		Name:           body.Name,
		Description:    body.Description,
		DefaultRoleIDs: body.DefaultRoleIDs,
		Metadata:       body.Metadata,
	})
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.JSON(fiber.Map{"group": toWireUserGroup(*group)})
}

func (c *UserGroupsController) Delete(ctx *fiber.Ctx) error {
	groupID := ctx.Params("groupId")
	if strings.TrimSpace(groupID) == "" {
		return validationError(ctx, "Invalid group identifier", "groupId", "groupId is required")
	}
	if err := c.svc.Delete(groupID); err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.SendStatus(204)
}

func (c *UserGroupsController) ListMembers(ctx *fiber.Ctx) error {
	groupID := ctx.Params("groupId")
	if strings.TrimSpace(groupID) == "" {
		return validationError(ctx, "Invalid group identifier", "group_id", "group_id is required")
	}
	members, err := c.svc.ListMembers(groupID)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	wired := make([]wireUserGroupMember, 0, len(members))
	for _, m := range members {
		wired = append(wired, toWireUserGroupMember(m))
	}
	return ctx.JSON(fiber.Map{"members": wired})
}

func (c *UserGroupsController) AddMember(ctx *fiber.Ctx) error {
	groupID := ctx.Params("groupId")
	var body struct {
		UserID      string `json:"user_id"`
		UserIDCamel string `json:"userId"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return validationError(ctx, "Invalid group membership payload", "body", "invalid body")
	}
	if strings.TrimSpace(body.UserID) == "" {
		body.UserID = body.UserIDCamel
	}
	if strings.TrimSpace(groupID) == "" || strings.TrimSpace(body.UserID) == "" {
		return validationError(ctx, "Invalid group membership payload", "group_id", "group_id and user_id are required")
	}

	var addedBy *string
	if val, ok := ctx.Locals("user_id").(string); ok && strings.TrimSpace(val) != "" {
		addedBy = &val
	}

	member, err := c.svc.AddMember(groupID, body.UserID, addedBy)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(201).JSON(fiber.Map{"membership": toWireUserGroupMember(*member)})
}

func (c *UserGroupsController) RemoveMember(ctx *fiber.Ctx) error {
	groupID := ctx.Params("groupId")
	userID := ctx.Params("userId")
	if strings.TrimSpace(groupID) == "" || strings.TrimSpace(userID) == "" {
		return validationError(ctx, "Invalid group membership payload", "group_id", "group_id and user_id are required")
	}
	if err := c.svc.RemoveMember(groupID, userID); err != nil {
		return ctx.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.JSON(fiber.Map{"status": "removed"})
}
