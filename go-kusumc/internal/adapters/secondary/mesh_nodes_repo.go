package secondary

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// UpsertMeshNode creates or updates a mesh node by (project_id, node_id).
func (r *PostgresRepo) UpsertMeshNode(projectID, nodeID, label, kind string, attributes map[string]any) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	projectID = strings.TrimSpace(projectID)
	nodeID = strings.TrimSpace(nodeID)
	label = strings.TrimSpace(label)
	kind = strings.TrimSpace(kind)
	if kind == "" {
		kind = "mesh"
	}
	if projectID == "" || nodeID == "" {
		return "", fmt.Errorf("projectID and nodeID are required")
	}

	query := `
		INSERT INTO mesh_nodes (project_id, node_id, label, kind, attributes)
		VALUES ($1, $2, NULLIF($3,''), $4, $5::jsonb)
		ON CONFLICT (project_id, node_id) DO UPDATE SET
			label = COALESCE(EXCLUDED.label, mesh_nodes.label),
			kind = EXCLUDED.kind,
			attributes = COALESCE(EXCLUDED.attributes, mesh_nodes.attributes),
			updated_at = NOW()
		RETURNING id::text;
	`

	var id string
	err := r.Pool.QueryRow(ctx, query, projectID, nodeID, label, kind, attributes).Scan(&id)
	return id, err
}

// AttachMeshNodeToGateway links an existing mesh node to a gateway device (soft-enabled).
func (r *PostgresRepo) AttachMeshNodeToGateway(gatewayDeviceID, nodeUUID string, discovered bool, metadata map[string]any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		INSERT INTO mesh_gateway_nodes (gateway_device_id, node_id, enabled, discovered, last_seen, metadata)
		VALUES ($1::uuid, $2::uuid, TRUE, $3, NOW(), $4::jsonb)
		ON CONFLICT (gateway_device_id, node_id) DO UPDATE SET
			enabled = TRUE,
			discovered = mesh_gateway_nodes.discovered OR EXCLUDED.discovered,
			last_seen = NOW(),
			metadata = COALESCE(EXCLUDED.metadata, mesh_gateway_nodes.metadata),
			updated_at = NOW();
	`

	_, err := r.Pool.Exec(ctx, query, gatewayDeviceID, nodeUUID, discovered, metadata)
	return err
}

// DetachMeshNodeFromGateway soft-disables a gateway-node association.
func (r *PostgresRepo) DetachMeshNodeFromGateway(gatewayDeviceID, nodeUUID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.Pool.Exec(ctx, `
		UPDATE mesh_gateway_nodes
		SET enabled = FALSE, updated_at = NOW()
		WHERE gateway_device_id = $1::uuid AND node_id = $2::uuid
	`, gatewayDeviceID, nodeUUID)
	return err
}

// ListMeshNodesForGateway returns node records attached to a gateway.
func (r *PostgresRepo) ListMeshNodesForGateway(gatewayDeviceID string, includeDisabled bool) ([]map[string]any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT
			n.id::text,
			n.project_id,
			n.node_id,
			n.label,
			n.kind,
			COALESCE(n.attributes, '{}'::jsonb) AS attributes,
			gn.enabled,
			gn.discovered,
			gn.last_seen,
			COALESCE(gn.metadata, '{}'::jsonb) AS link_metadata
		FROM mesh_gateway_nodes gn
		JOIN mesh_nodes n ON n.id = gn.node_id
		WHERE gn.gateway_device_id = $1::uuid
		  AND ($2::bool = TRUE OR gn.enabled = TRUE)
		ORDER BY COALESCE(gn.last_seen, n.updated_at) DESC;
	`

	rows, err := r.Pool.Query(ctx, query, gatewayDeviceID, includeDisabled)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []map[string]any{}
	for rows.Next() {
		var id, projectID, nodeID, label, kind string
		var attrs map[string]any
		var enabled bool
		var discovered bool
		var lastSeen *time.Time
		var linkMeta map[string]any
		if err := rows.Scan(&id, &projectID, &nodeID, &label, &kind, &attrs, &enabled, &discovered, &lastSeen, &linkMeta); err != nil {
			continue
		}
		out = append(out, map[string]any{
			"id":           id,
			"projectId":    projectID,
			"nodeId":       nodeID,
			"label":        label,
			"kind":         kind,
			"attributes":   attrs,
			"enabled":      enabled,
			"discovered":   discovered,
			"lastSeen":     lastSeen,
			"linkMetadata": linkMeta,
		})
	}
	return out, nil
}

// ResolveMeshNodeUUID finds a node UUID by (project_id, node_id).
func (r *PostgresRepo) ResolveMeshNodeUUID(projectID, nodeID string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var id string
	err := r.Pool.QueryRow(ctx, `SELECT id::text FROM mesh_nodes WHERE project_id = $1 AND node_id = $2`, strings.TrimSpace(projectID), strings.TrimSpace(nodeID)).Scan(&id)
	return id, err
}
