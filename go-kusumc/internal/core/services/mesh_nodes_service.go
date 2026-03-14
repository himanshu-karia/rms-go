package services

import (
	"fmt"
	"strings"

	"ingestion-go/internal/adapters/secondary"
)

type MeshNodesService struct {
	repo *secondary.PostgresRepo
}

func NewMeshNodesService(repo *secondary.PostgresRepo) *MeshNodesService {
	return &MeshNodesService{repo: repo}
}

func (s *MeshNodesService) resolveGateway(deviceRef, projectOverride string) (deviceID, projectID, imei string, err error) {
	if s.repo == nil {
		return "", "", "", fmt.Errorf("repo unavailable")
	}
	dev, derr := s.repo.GetDeviceByIDOrIMEI(deviceRef)
	if derr != nil {
		return "", "", "", fmt.Errorf("device lookup failed: %w", derr)
	}
	if dev == nil {
		return "", "", "", fmt.Errorf("device not found")
	}
	deviceID, _ = dev["id"].(string)
	imei, _ = dev["imei"].(string)
	projectID = strings.TrimSpace(projectOverride)
	if projectID == "" {
		projectID, _ = dev["project_id"].(string)
	}
	if strings.TrimSpace(deviceID) == "" || strings.TrimSpace(projectID) == "" {
		return "", "", "", fmt.Errorf("device metadata incomplete")
	}
	return deviceID, projectID, imei, nil
}

func (s *MeshNodesService) ListGatewayNodes(deviceRef, projectOverride string, includeDisabled bool) ([]map[string]any, error) {
	gatewayDeviceID, _, _, err := s.resolveGateway(deviceRef, projectOverride)
	if err != nil {
		return nil, err
	}
	return s.repo.ListMeshNodesForGateway(gatewayDeviceID, includeDisabled)
}

func (s *MeshNodesService) AttachNode(deviceRef, projectOverride, nodeID, label, kind string, attributes map[string]any) (map[string]any, error) {
	gatewayDeviceID, projectID, _, err := s.resolveGateway(deviceRef, projectOverride)
	if err != nil {
		return nil, err
	}
	nodeUUID, err := s.repo.UpsertMeshNode(projectID, nodeID, label, kind, attributes)
	if err != nil {
		return nil, err
	}
	if err := s.repo.AttachMeshNodeToGateway(gatewayDeviceID, nodeUUID, false, map[string]any{"source": "ui"}); err != nil {
		return nil, err
	}
	return map[string]any{"nodeUuid": nodeUUID, "nodeId": nodeID}, nil
}

func (s *MeshNodesService) DetachNode(deviceRef, projectOverride, nodeUUID string) error {
	gatewayDeviceID, _, _, err := s.resolveGateway(deviceRef, projectOverride)
	if err != nil {
		return err
	}
	if strings.TrimSpace(nodeUUID) == "" {
		return fmt.Errorf("nodeUuid is required")
	}
	return s.repo.DetachMeshNodeFromGateway(gatewayDeviceID, nodeUUID)
}

func (s *MeshNodesService) ReportDiscovery(deviceRef, projectOverride string, nodeIDs []string) (int, error) {
	gatewayDeviceID, projectID, _, err := s.resolveGateway(deviceRef, projectOverride)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, raw := range nodeIDs {
		nodeID := strings.TrimSpace(raw)
		if nodeID == "" {
			continue
		}
		nodeUUID, err := s.repo.UpsertMeshNode(projectID, nodeID, "", "mesh", nil)
		if err != nil {
			continue
		}
		_ = s.repo.AttachMeshNodeToGateway(gatewayDeviceID, nodeUUID, true, map[string]any{"source": "discovery"})
		count++
	}
	return count, nil
}
