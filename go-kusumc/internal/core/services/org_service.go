package services

import "ingestion-go/internal/adapters/secondary"

type OrgService struct {
	repo *secondary.PostgresRepo
}

func NewOrgService(repo *secondary.PostgresRepo) *OrgService {
	return &OrgService{repo: repo}
}

func (s *OrgService) List() ([]secondary.OrgRecord, error) {
	return s.repo.ListOrgs()
}

func (s *OrgService) Create(name, orgType, path string, parentID *string, metadata map[string]interface{}) (*secondary.OrgRecord, error) {
	return s.repo.CreateOrg(name, orgType, path, parentID, metadata)
}

func (s *OrgService) Update(id, name, orgType, path string, parentID *string, metadata map[string]interface{}) (*secondary.OrgRecord, error) {
	return s.repo.UpdateOrg(id, name, orgType, path, parentID, metadata)
}
