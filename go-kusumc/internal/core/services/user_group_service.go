package services

import (
	"fmt"
	"strings"

	"ingestion-go/internal/adapters/secondary"
)

type UserGroupService struct {
	repo     *secondary.PostgresRepo
	userRepo *secondary.PostgresUserRepo
}

type UserGroupFilters struct {
	StateID     string
	AuthorityID string
	ProjectID   string
}

type UserGroupCreateInput struct {
	Name           string
	Description    *string
	Scope          map[string]string
	DefaultRoleIDs []string
	Metadata       map[string]interface{}
}

type UserGroupUpdateInput struct {
	Name           *string
	Description    *string
	DefaultRoleIDs *[]string
	Metadata       *map[string]interface{}
}

func NewUserGroupService(repo *secondary.PostgresRepo, userRepo *secondary.PostgresUserRepo) *UserGroupService {
	return &UserGroupService{repo: repo, userRepo: userRepo}
}

func (s *UserGroupService) List(filters UserGroupFilters) ([]secondary.UserGroupRecord, error) {
	scope := map[string]string{}
	if strings.TrimSpace(filters.StateID) != "" {
		scope["stateId"] = filters.StateID
	}
	if strings.TrimSpace(filters.AuthorityID) != "" {
		scope["authorityId"] = filters.AuthorityID
	}
	if strings.TrimSpace(filters.ProjectID) != "" {
		scope["projectId"] = filters.ProjectID
	}
	return s.repo.ListUserGroups(scope)
}

func (s *UserGroupService) Create(input UserGroupCreateInput) (*secondary.UserGroupRecord, error) {
	if strings.TrimSpace(input.Name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	if len(input.Scope) == 0 {
		return nil, fmt.Errorf("scope is required")
	}
	if len(input.DefaultRoleIDs) == 0 {
		return nil, fmt.Errorf("defaultRoleIds is required")
	}
	if input.Metadata == nil {
		input.Metadata = map[string]interface{}{}
	}
	return s.repo.CreateUserGroup(input.Name, input.Description, input.Scope, input.DefaultRoleIDs, input.Metadata)
}

func (s *UserGroupService) Update(groupID string, input UserGroupUpdateInput) (*secondary.UserGroupRecord, error) {
	if groupID == "" {
		return nil, fmt.Errorf("groupId is required")
	}
	return s.repo.UpdateUserGroup(groupID, input.Name, input.Description, input.DefaultRoleIDs, input.Metadata)
}

func (s *UserGroupService) Delete(groupID string) error {
	if groupID == "" {
		return fmt.Errorf("groupId is required")
	}
	return s.repo.SoftDeleteUserGroup(groupID)
}

func (s *UserGroupService) ListMembers(groupID string) ([]secondary.UserGroupMemberRecord, error) {
	if groupID == "" {
		return nil, fmt.Errorf("groupId is required")
	}
	return s.repo.ListUserGroupMembers(groupID)
}

func (s *UserGroupService) AddMember(groupID, userID string, addedBy *string) (*secondary.UserGroupMemberRecord, error) {
	if groupID == "" || userID == "" {
		return nil, fmt.Errorf("groupId and userId are required")
	}
	group, err := s.repo.GetUserGroupByID(groupID)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, fmt.Errorf("group not found")
	}

	member, err := s.repo.AddUserToGroup(groupID, userID, addedBy)
	if err != nil {
		return nil, err
	}

	scope := map[string]interface{}{}
	for key, value := range group.Scope {
		if strings.TrimSpace(value) != "" {
			scope[key] = value
		}
	}

	for _, roleKey := range group.DefaultRoleIDs {
		roleKey = strings.TrimSpace(roleKey)
		if roleKey == "" {
			continue
		}
		roleType, err := s.resolveRoleType(roleKey)
		if err != nil {
			return nil, err
		}
		if _, err := s.userRepo.UpsertRoleBinding(userID, roleKey, roleType, scope); err != nil {
			return nil, err
		}
	}

	return member, nil
}

func (s *UserGroupService) RemoveMember(groupID, userID string) error {
	if groupID == "" || userID == "" {
		return fmt.Errorf("groupId and userId are required")
	}
	group, err := s.repo.GetUserGroupByID(groupID)
	if err != nil {
		return err
	}
	if group == nil {
		return fmt.Errorf("group not found")
	}

	if err := s.repo.RemoveUserFromGroup(groupID, userID); err != nil {
		return err
	}

	scope := map[string]interface{}{}
	for key, value := range group.Scope {
		if strings.TrimSpace(value) != "" {
			scope[key] = value
		}
	}

	for _, roleKey := range group.DefaultRoleIDs {
		roleKey = strings.TrimSpace(roleKey)
		if roleKey == "" {
			continue
		}
		roleType, err := s.resolveRoleType(roleKey)
		if err != nil {
			return err
		}
		if _, err := s.userRepo.DeleteRoleBindingByScope(userID, roleKey, roleType, scope); err != nil {
			return err
		}
	}

	return nil
}

func (s *UserGroupService) resolveRoleType(roleKey string) (string, error) {
	if role, err := s.repo.GetOrgRole(roleKey); err != nil {
		return "", err
	} else if role != nil {
		return "org", nil
	}
	if role, err := s.repo.GetProjectRole(roleKey); err != nil {
		return "", err
	} else if role != nil {
		return "project", nil
	}
	if role, err := s.repo.GetLinkRole(roleKey); err != nil {
		return "", err
	} else if role != nil {
		return "link", nil
	}
	return "", fmt.Errorf("role not found")
}
