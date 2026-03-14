package services

import (
	"fmt"
	"strings"

	"ingestion-go/internal/adapters/secondary"

	"golang.org/x/crypto/bcrypt"
)

type AdminService struct {
	repo     *secondary.PostgresRepo
	userRepo *secondary.PostgresUserRepo
}

type UserRoleSummary struct {
	BindingID       string                 `json:"bindingId"`
	RoleKey         string                 `json:"roleKey"`
	RoleType        string                 `json:"roleType"`
	RoleDescription string                 `json:"roleDescription"`
	Scope           map[string]interface{} `json:"scope"`
}

type UserSummary struct {
	ID                 string                 `json:"id"`
	Username           string                 `json:"username"`
	DisplayName        string                 `json:"displayName"`
	Status             string                 `json:"status"`
	MustRotatePassword bool                   `json:"mustRotatePassword"`
	Metadata           map[string]interface{} `json:"metadata"`
	Roles              []UserRoleSummary      `json:"roles"`
}

type ListUsersInput struct {
	Status      string
	Query       string
	RoleKey     string
	RoleType    string
	StateID     string
	AuthorityID string
	ProjectID   string
	GroupID     string
	Cursor      string
	Limit       int
}

type ListUsersResult struct {
	Users      []UserSummary `json:"users"`
	NextCursor string        `json:"nextCursor"`
}

func NewAdminService(repo *secondary.PostgresRepo, userRepo *secondary.PostgresUserRepo) *AdminService {
	return &AdminService{repo: repo, userRepo: userRepo}
}

func (s *AdminService) GetStates() ([]map[string]interface{}, error) {
	return s.repo.GetStates()
}

func (s *AdminService) CreateState(name string, isoCode *string, metadata map[string]interface{}) (map[string]interface{}, error) {
	return s.repo.CreateState(name, isoCode, metadata)
}

func (s *AdminService) UpdateState(id string, name *string, isoCode *string, metadata map[string]interface{}) (map[string]interface{}, error) {
	return s.repo.UpdateState(id, name, isoCode, metadata)
}

func (s *AdminService) DeleteState(id string) error {
	return s.repo.DeleteState(id)
}

func (s *AdminService) GetAuthorities(stateId string) ([]map[string]interface{}, error) {
	return s.repo.GetAuthorities(stateId)
}

func (s *AdminService) CreateAuthority(stateID, name, authorityType string, contactInfo map[string]interface{}) (map[string]interface{}, error) {
	return s.repo.CreateAuthority(stateID, name, authorityType, contactInfo)
}

func (s *AdminService) UpdateAuthority(id string, name *string, authorityType *string, contactInfo map[string]interface{}) (map[string]interface{}, error) {
	return s.repo.UpdateAuthority(id, name, authorityType, contactInfo)
}

func (s *AdminService) DeleteAuthority(id string) error {
	return s.repo.DeleteAuthority(id)
}

func (s *AdminService) ListVendors(category string) ([]secondary.OrgRecord, error) {
	return s.repo.ListVendors(category)
}

func (s *AdminService) CreateVendor(name, category string, metadata map[string]interface{}) (*secondary.OrgRecord, error) {
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	if category != "" {
		metadata["category"] = category
	}
	return s.repo.CreateOrg(name, "vendor", "", nil, metadata)
}

func (s *AdminService) UpdateVendor(id string, name *string, category *string, metadata map[string]interface{}) (*secondary.OrgRecord, error) {
	var meta map[string]interface{}
	if metadata != nil {
		meta = metadata
		if category != nil {
			meta["category"] = *category
		}
	} else if category != nil {
		meta = map[string]interface{}{"category": *category}
	}
	return s.repo.UpdateVendor(id, name, meta)
}

func (s *AdminService) DeleteVendor(id string) error {
	return s.repo.SoftDeleteOrg(id)
}

func (s *AdminService) GetOrgByID(id string) (*secondary.OrgRecord, error) {
	return s.repo.GetOrgByID(id)
}

func (s *AdminService) GetUsers() ([]secondary.UserRecord, error) {
	return s.userRepo.GetAllUsers()
}

func (s *AdminService) ListUsers(input ListUsersInput) (*ListUsersResult, error) {
	items, nextCursor, err := s.userRepo.ListUsers(secondary.ListUsersFilters{
		Status:      input.Status,
		Query:       input.Query,
		RoleKey:     input.RoleKey,
		RoleType:    input.RoleType,
		StateID:     input.StateID,
		AuthorityID: input.AuthorityID,
		ProjectID:   input.ProjectID,
		GroupID:     input.GroupID,
		Cursor:      input.Cursor,
		Limit:       input.Limit,
	})
	if err != nil {
		return nil, err
	}

	users := make([]UserSummary, 0, len(items))
	for _, item := range items {
		summary, err := s.BuildUserSummaryFromRecord(item)
		if err != nil {
			return nil, err
		}
		if summary != nil {
			users = append(users, *summary)
		}
	}
	return &ListUsersResult{Users: users, NextCursor: nextCursor}, nil
}

func (s *AdminService) GetUserByID(id string) (*secondary.UserRecord, error) {
	return s.userRepo.GetUserByID(id)
}

func (s *AdminService) CreateUser(username string, email, displayName *string, password string, role string, active, mustRotate bool, metadata map[string]interface{}) (*secondary.UserRecord, error) {
	if role == "" {
		role = "viewer"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	return s.userRepo.CreateUserWithProfile(username, email, displayName, string(hash), role, active, mustRotate, metadata)
}

func (s *AdminService) UpdateUser(id string, displayName *string, active *bool, mustRotate *bool, metadata map[string]interface{}) (*secondary.UserRecord, error) {
	updated, err := s.userRepo.UpdateUserProfile(id, displayName, active, mustRotate, metadata)
	if err != nil {
		return nil, err
	}
	if updated != nil && active != nil && !*active {
		if err := s.userRepo.RevokeSessionsByUserID(id); err != nil {
			return nil, err
		}
	}
	return updated, nil
}

func (s *AdminService) ResetUserPassword(id string, password string, mustRotate bool) (*secondary.UserRecord, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	if err := s.userRepo.UpdatePassword(id, string(hash), mustRotate); err != nil {
		return nil, err
	}
	return s.userRepo.GetUserByID(id)
}

func (s *AdminService) AssignUserRole(userID, roleKey, roleType string, scope map[string]interface{}) (*UserSummary, error) {
	resolvedType, desc, err := s.resolveRole(roleKey, roleType)
	if err != nil {
		return nil, err
	}
	if _, err := s.userRepo.UpsertRoleBinding(userID, roleKey, resolvedType, scope); err != nil {
		return nil, err
	}
	return s.BuildUserSummary(userID, desc)
}

func (s *AdminService) RemoveUserRole(userID, bindingID string) (*UserSummary, error) {
	deleted, err := s.userRepo.DeleteRoleBinding(userID, bindingID)
	if err != nil {
		return nil, err
	}
	if !deleted {
		return nil, fmt.Errorf("role binding not found")
	}
	return s.BuildUserSummary(userID, "")
}

func (s *AdminService) BuildUserSummary(userID string, fallbackRoleDesc string) (*UserSummary, error) {
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, nil
	}
	return s.BuildUserSummaryFromRecord(*user, fallbackRoleDesc)
}

func (s *AdminService) BuildUserSummaryFromRecord(user secondary.UserRecord, fallbackRoleDesc ...string) (*UserSummary, error) {
	roles, err := s.ListUserRoles(user.ID, func() string {
		if len(fallbackRoleDesc) > 0 {
			return fallbackRoleDesc[0]
		}
		return ""
	}())
	if err != nil {
		return nil, err
	}
	status := "disabled"
	if user.Active {
		status = "active"
	}
	displayName := user.Username
	if user.DisplayName != nil && *user.DisplayName != "" {
		displayName = *user.DisplayName
	}
	return &UserSummary{
		ID:                 user.ID,
		Username:           user.Username,
		DisplayName:        displayName,
		Status:             status,
		MustRotatePassword: user.MustRotatePassword,
		Metadata:           user.Metadata,
		Roles:              roles,
	}, nil
}

func (s *AdminService) ListUserRoles(userID string, fallbackRoleDesc string) ([]UserRoleSummary, error) {
	bindings, err := s.userRepo.ListRoleBindingsByUserID(userID)
	if err != nil {
		return nil, err
	}
	roles := make([]UserRoleSummary, 0, len(bindings))
	for _, binding := range bindings {
		desc, err := s.resolveRoleDescription(binding.RoleKey, binding.RoleType)
		if err != nil {
			return nil, err
		}
		if desc == "" && fallbackRoleDesc != "" {
			desc = fallbackRoleDesc
		}
		roles = append(roles, UserRoleSummary{
			BindingID:       binding.ID,
			RoleKey:         binding.RoleKey,
			RoleType:        binding.RoleType,
			RoleDescription: desc,
			Scope:           binding.Scope,
		})
	}
	return roles, nil
}

func (s *AdminService) LogAudit(userId, action, resource, ip, status string, metadata map[string]interface{}) {
	_ = s.repo.LogAudit(userId, action, resource, ip, status, metadata)
}

func (s *AdminService) resolveRole(roleKey, roleType string) (string, string, error) {
	if roleKey == "" {
		return "", "", fmt.Errorf("role key required")
	}
	if roleType != "" {
		desc, err := s.resolveRoleDescription(roleKey, roleType)
		if err != nil {
			return "", "", err
		}
		if desc == "" {
			return "", "", fmt.Errorf("role not found")
		}
		return roleType, desc, nil
	}
	if role, err := s.repo.GetOrgRole(roleKey); err != nil {
		return "", "", err
	} else if role != nil {
		desc := ""
		if val, ok := role["description"].(string); ok {
			desc = val
		}
		return "org", desc, nil
	}
	if role, err := s.repo.GetProjectRole(roleKey); err != nil {
		return "", "", err
	} else if role != nil {
		desc := ""
		if val, ok := role["description"].(string); ok {
			desc = val
		}
		return "project", desc, nil
	}
	if role, err := s.repo.GetLinkRole(roleKey); err != nil {
		return "", "", err
	} else if role != nil {
		desc := ""
		if val, ok := role["description"].(string); ok {
			desc = val
		}
		return "link", desc, nil
	}
	return "", "", fmt.Errorf("role not found")
}

func (s *AdminService) resolveRoleDescription(roleKey, roleType string) (string, error) {
	switch roleType {
	case "org":
		role, err := s.repo.GetOrgRole(roleKey)
		if err != nil || role == nil {
			return "", err
		}
		if desc, ok := role["description"].(string); ok {
			return desc, nil
		}
		return "", nil
	case "project":
		role, err := s.repo.GetProjectRole(roleKey)
		if err != nil || role == nil {
			return "", err
		}
		if desc, ok := role["description"].(string); ok {
			return desc, nil
		}
		return "", nil
	case "link":
		role, err := s.repo.GetLinkRole(roleKey)
		if err != nil || role == nil {
			return "", err
		}
		if desc, ok := role["description"].(string); ok {
			return desc, nil
		}
		return "", nil
	default:
		return "", nil
	}
}

func (s *AdminService) DeleteUser(id string) error {
	return s.userRepo.SoftDeleteUser(id)
}

func (s *AdminService) ListCapabilities() ([]map[string]interface{}, error) {
	return s.repo.ListCapabilities()
}

func (s *AdminService) ListRoles() (map[string]interface{}, error) {
	orgRoles, err := s.repo.ListOrgRoles()
	if err != nil {
		return nil, err
	}
	projectRoles, err := s.repo.ListProjectRoles()
	if err != nil {
		return nil, err
	}
	linkRoles, err := s.repo.ListLinkRoles()
	if err != nil {
		return nil, err
	}
	capabilities, err := s.repo.ListCapabilities()
	if err != nil {
		return nil, err
	}
	allCaps := make([]string, 0, len(capabilities))
	for _, cap := range capabilities {
		if key, ok := cap["key"].(string); ok {
			allCaps = append(allCaps, key)
		}
	}

	orgDefs := buildRoleDefinitions("org", orgRoles, allCaps)
	projectDefs := buildRoleDefinitions("project", projectRoles, allCaps)
	linkDefs := buildRoleDefinitions("link", linkRoles, allCaps)
	return map[string]interface{}{
		"org_roles":     orgDefs,
		"project_roles": projectDefs,
		"link_roles":    linkDefs,
	}, nil
}

func buildRoleDefinitions(roleType string, roles []map[string]interface{}, allCaps []string) []map[string]interface{} {
	defs := make([]map[string]interface{}, 0, len(roles))
	for _, role := range roles {
		slug, _ := role["slug"].(string)
		desc, _ := role["description"].(string)
		def := map[string]interface{}{
			"key":          slug,
			"name":         titleizeRole(slug),
			"description":  desc,
			"capabilities": roleCapabilities(roleType, slug, allCaps),
		}
		if val, ok := role["is_unique_per_device"]; ok {
			def["is_unique_per_device"] = val
		}
		defs = append(defs, def)
	}
	return defs
}

func roleCapabilities(roleType string, key string, allCaps []string) []string {
	switch roleType {
	case "org":
		switch key {
		case "owner", "admin":
			return allCaps
		case "operator":
			return []string{
				"devices:read",
				"devices:write",
				"devices:commands",
				"telemetry:read",
				"telemetry:live:device",
				"alerts:manage",
				"installations:manage",
				"beneficiaries:manage",
			}
		case "viewer":
			return []string{"devices:read", "telemetry:read"}
		}
	case "project":
		switch key {
		case "maintainer":
			return []string{
				"devices:read",
				"devices:write",
				"telemetry:read",
				"telemetry:export",
				"alerts:manage",
			}
		case "analyst":
			return []string{"telemetry:read", "telemetry:export"}
		case "viewer":
			return []string{"devices:read", "telemetry:read"}
		case "service":
			return []string{"devices:commands", "telemetry:read"}
		}
	case "link":
		return []string{"devices:commands"}
	}
	return []string{}
}

func titleizeRole(value string) string {
	value = strings.ReplaceAll(value, "_", " ")
	value = strings.ReplaceAll(value, "-", " ")
	parts := strings.Fields(value)
	for i, part := range parts {
		if len(part) == 0 {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
	}
	return strings.Join(parts, " ")
}
