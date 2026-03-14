package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"ingestion-go/internal/adapters/secondary"
	"ingestion-go/internal/models"
)

// DnaSpecService handles per-sensor specs and thresholds (canonical tables).
type DnaSpecService struct {
	repo *secondary.PostgresRepo
	sync configSynchronizer
}

func NewDnaSpecService(repo *secondary.PostgresRepo, sync configSynchronizer) *DnaSpecService {
	return &DnaSpecService{repo: repo, sync: sync}
}

func (s *DnaSpecService) ListSensors(ctx context.Context, projectID string) ([]models.DnaSensor, error) {
	return s.repo.ListDnaSensors(ctx, projectID)
}

func (s *DnaSpecService) UpsertSensors(ctx context.Context, projectID string, sensors []models.DnaSensor) error {
	for i := range sensors {
		sensors[i].ProjectID = projectID
	}
	if err := s.repo.UpsertDnaSensors(ctx, projectID, sensors); err != nil {
		return err
	}
	if s.sync != nil {
		s.sync.InvalidateScopes()
		go s.sync.SyncProject(projectID)
	}
	return nil
}

// GetThresholds merges defaults with optional device overrides.
func (s *DnaSpecService) GetThresholds(ctx context.Context, projectID string, deviceID *string) ([]models.DnaThreshold, string, error) {
	defaults, err := s.repo.ListDnaThresholds(ctx, projectID, "project", nil)
	if err != nil {
		return nil, "", err
	}
	if deviceID == nil {
		return defaults, "default", nil
	}
	overrides, err := s.repo.ListDnaThresholds(ctx, projectID, "device", deviceID)
	if err != nil {
		return nil, "", err
	}
	if len(overrides) == 0 {
		return defaults, "default", nil
	}

	merged := make([]models.DnaThreshold, 0, len(defaults))
	ov := make(map[string]models.DnaThreshold)
	for _, t := range overrides {
		ov[t.Param] = t
	}
	for _, d := range defaults {
		if o, ok := ov[d.Param]; ok {
			merged = append(merged, mergeThreshold(d, o))
		} else {
			merged = append(merged, d)
		}
	}
	for param, t := range ov {
		found := false
		for _, d := range defaults {
			if d.Param == param {
				found = true
				break
			}
		}
		if !found {
			merged = append(merged, t)
		}
	}
	return merged, "override-merged", nil
}

// ListThresholds returns raw threshold entries for a scope.
func (s *DnaSpecService) ListThresholds(ctx context.Context, projectID, scope string, deviceID *string) ([]models.DnaThreshold, error) {
	return s.repo.ListDnaThresholds(ctx, projectID, scope, deviceID)
}

func mergeThreshold(base models.DnaThreshold, override models.DnaThreshold) models.DnaThreshold {
	res := base
	if override.MinValue != nil {
		res.MinValue = override.MinValue
	}
	if override.MaxValue != nil {
		res.MaxValue = override.MaxValue
	}
	if override.Target != nil {
		res.Target = override.Target
	}
	if override.Unit != nil {
		res.Unit = override.Unit
	}
	if override.DecimalPlaces != nil {
		res.DecimalPlaces = override.DecimalPlaces
	}
	if override.TemplateID != nil {
		res.TemplateID = override.TemplateID
	}
	if override.Metadata != nil {
		res.Metadata = override.Metadata
	}
	if override.Reason != nil {
		res.Reason = override.Reason
	}
	if override.UpdatedBy != nil {
		res.UpdatedBy = override.UpdatedBy
	}
	if override.WarnLow != nil {
		res.WarnLow = override.WarnLow
	}
	if override.WarnHigh != nil {
		res.WarnHigh = override.WarnHigh
	}
	if override.AlertLow != nil {
		res.AlertLow = override.AlertLow
	}
	if override.AlertHigh != nil {
		res.AlertHigh = override.AlertHigh
	}
	if override.Origin != nil {
		res.Origin = override.Origin
	}
	res.Scope = override.Scope
	res.DeviceID = override.DeviceID
	res.UpdatedAt = override.UpdatedAt
	return res
}

func (s *DnaSpecService) UpsertThresholds(ctx context.Context, projectID string, thresholds []models.DnaThreshold, scope string, deviceID *string) error {
	for i := range thresholds {
		thresholds[i].ProjectID = projectID
		thresholds[i].Scope = scope
		thresholds[i].DeviceID = deviceID
	}
	if err := s.repo.UpsertDnaThresholds(ctx, thresholds); err != nil {
		return err
	}
	if s.sync != nil {
		go s.sync.SyncProject(projectID)
	}
	return nil
}

// DeleteThresholds removes thresholds for a scope and returns rows affected.
func (s *DnaSpecService) DeleteThresholds(ctx context.Context, projectID, scope string, deviceID *string) (int64, error) {
	return s.repo.DeleteDnaThresholds(ctx, projectID, scope, deviceID)
}

// ListThresholdDevices returns device IDs that have overrides for a project.
func (s *DnaSpecService) ListThresholdDevices(ctx context.Context, projectID string) ([]string, error) {
	return s.repo.ListDnaThresholdDeviceIDs(ctx, projectID)
}

// ExportSensorsCSV renders the sensors table as CSV.
func (s *DnaSpecService) ExportSensorsCSV(ctx context.Context, projectID string) ([]byte, error) {
	sensors, err := s.repo.ListDnaSensors(ctx, projectID)
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	w := csv.NewWriter(buf)
	_ = w.Write([]string{"param", "label", "unit", "min", "max", "resolution", "required", "notes", "topic_template"})
	for _, s := range sensors {
		row := []string{
			s.Param,
			s.Label,
			stringOrEmpty(s.Unit),
			floatOrEmpty(s.MinValue),
			floatOrEmpty(s.MaxValue),
			floatOrEmpty(s.Resolution),
			boolToString(s.Required),
			stringOrEmpty(s.Notes),
			stringOrEmpty(s.TopicTemplate),
		}
		_ = w.Write(row)
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ImportSensorsCSV ingests sensors from CSV and upserts them.
func (s *DnaSpecService) ImportSensorsCSV(ctx context.Context, projectID string, r io.Reader) (int, error) {
	sensors, err := parseSensorsCSV(projectID, r)
	if err != nil {
		return 0, err
	}

	if err := s.repo.UpsertDnaSensors(ctx, projectID, sensors); err != nil {
		return 0, err
	}
	if s.sync != nil {
		s.sync.InvalidateScopes()
		go s.sync.SyncProject(projectID)
	}
	return len(sensors), nil
}

// CreateSensorVersionFromCSV stores a draft version without applying it.
func (s *DnaSpecService) CreateSensorVersionFromCSV(ctx context.Context, projectID, label string, r io.Reader, createdBy *string) (int64, int, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return 0, 0, err
	}
	// Parse once to validate and count rows.
	sensors, err := parseSensorsCSV(projectID, bytes.NewReader(data))
	if err != nil {
		return 0, 0, err
	}

	versionID, err := s.repo.CreateSensorVersion(ctx, projectID, label, data, len(sensors), createdBy)
	if err != nil {
		return 0, 0, err
	}
	return versionID, len(sensors), nil
}

// PublishSensorVersion applies a stored version and syncs cache.
func (s *DnaSpecService) PublishSensorVersion(ctx context.Context, projectID string, versionID int64, publishedBy *string) (int, error) {
	data, _, err := s.repo.GetSensorVersionCSV(ctx, projectID, versionID)
	if err != nil {
		return 0, err
	}
	sensors, err := parseSensorsCSV(projectID, bytes.NewReader(data))
	if err != nil {
		return 0, err
	}
	if err := s.repo.UpsertDnaSensors(ctx, projectID, sensors); err != nil {
		return 0, err
	}
	if err := s.repo.MarkSensorVersionPublished(ctx, projectID, versionID, publishedBy); err != nil {
		return 0, err
	}
	if s.sync != nil {
		s.sync.InvalidateScopes()
		go s.sync.SyncProject(projectID)
	}
	return len(sensors), nil
}

// RollbackSensorVersion reapplies a previous version and marks rollback metadata.
func (s *DnaSpecService) RollbackSensorVersion(ctx context.Context, projectID string, versionID int64, rolledBackBy *string) (int, error) {
	data, _, err := s.repo.GetSensorVersionCSV(ctx, projectID, versionID)
	if err != nil {
		return 0, err
	}
	sensors, err := parseSensorsCSV(projectID, bytes.NewReader(data))
	if err != nil {
		return 0, err
	}
	if err := s.repo.UpsertDnaSensors(ctx, projectID, sensors); err != nil {
		return 0, err
	}
	if err := s.repo.MarkSensorVersionRolledBack(ctx, projectID, versionID, rolledBackBy); err != nil {
		return 0, err
	}
	if s.sync != nil {
		s.sync.InvalidateScopes()
		go s.sync.SyncProject(projectID)
	}
	return len(sensors), nil
}

// ListSensorVersions exposes metadata for recent versions.
func (s *DnaSpecService) ListSensorVersions(ctx context.Context, projectID string) ([]models.DnaSensorVersion, error) {
	return s.repo.ListSensorVersions(ctx, projectID, 25)
}

// GetSensorVersionCSV returns raw CSV bytes and metadata for a specific version.
func (s *DnaSpecService) GetSensorVersionCSV(ctx context.Context, projectID string, versionID int64) ([]byte, *models.DnaSensorVersion, error) {
	return s.repo.GetSensorVersionCSV(ctx, projectID, versionID)
}

func parseSensorsCSV(projectID string, r io.Reader) ([]models.DnaSensor, error) {
	reader := csv.NewReader(r)
	headers, err := reader.Read()
	if err != nil {
		return nil, err
	}
	index := map[string]int{}
	for i, h := range headers {
		index[strings.ToLower(strings.TrimSpace(h))] = i
	}
	requiredCols := []string{"param", "label"}
	for _, c := range requiredCols {
		if _, ok := index[c]; !ok {
			return nil, fmt.Errorf("missing column %s", c)
		}
	}

	var sensors []models.DnaSensor
	for {
		rec, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		param := valueAt(rec, index, "param")
		label := valueAt(rec, index, "label")
		if param == "" || label == "" {
			continue
		}

		s := models.DnaSensor{
			ProjectID: projectID,
			Param:     param,
			Label:     label,
			Required:  parseBool(valueAt(rec, index, "required")),
		}
		if v := parseFloatPtr(valueAt(rec, index, "min")); v != nil {
			s.MinValue = v
		}
		if v := parseFloatPtr(valueAt(rec, index, "max")); v != nil {
			s.MaxValue = v
		}
		if v := parseFloatPtr(valueAt(rec, index, "resolution")); v != nil {
			s.Resolution = v
		}
		if v := valueAt(rec, index, "unit"); v != "" {
			s.Unit = &v
		}
		if v := valueAt(rec, index, "notes"); v != "" {
			s.Notes = &v
		}
		if v := valueAt(rec, index, "topic_template"); v != "" {
			s.TopicTemplate = &v
		}
		sensors = append(sensors, s)
	}

	if len(sensors) == 0 {
		return nil, fmt.Errorf("no valid rows found")
	}

	return sensors, nil
}

func valueAt(rec []string, index map[string]int, key string) string {
	if idx, ok := index[key]; ok && idx < len(rec) {
		return strings.TrimSpace(rec[idx])
	}
	return ""
}

func parseFloatPtr(s string) *float64 {
	if s == "" {
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &f
}

func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "1" || s == "yes"
}

func stringOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func floatOrEmpty(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
