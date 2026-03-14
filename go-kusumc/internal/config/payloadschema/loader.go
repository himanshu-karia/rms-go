package payloadschema

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Entry represents a single CSV row describing a telemetry key.
type Entry struct {
	PacketType       string
	ExpectedFor      string
	ScopeID          string
	Key              string
	Description      string
	Unit             string
	Required         bool
	MaxLength        *int
	TopicTemplate    string
	EnvelopeRequired bool
	Notes            string
	ValueMin         *float64
	ValueMax         *float64
	Resolution       *float64
}

// KeySpec is the distilled shape consumed by downstream ingestion and rules logic.
type KeySpec struct {
	Key         string   `json:"key"`
	Description string   `json:"description,omitempty"`
	Unit        string   `json:"unit,omitempty"`
	Required    bool     `json:"required,omitempty"`
	MaxLength   *int     `json:"maxLength,omitempty"`
	Notes       string   `json:"notes,omitempty"`
	ValueMin    *float64 `json:"valueMin,omitempty"`
	ValueMax    *float64 `json:"valueMax,omitempty"`
	Resolution  *float64 `json:"resolution,omitempty"`
}

// PacketSchema gathers keys for a specific packet type.
type PacketSchema struct {
	PacketType    string    `json:"packetType"`
	TopicTemplate string    `json:"topicTemplate,omitempty"`
	Keys          []KeySpec `json:"keys"`
	EnvelopeKeys  []KeySpec `json:"envelopeKeys,omitempty"`
}

// ScopeSchema groups packet schemas for a logical scope (project, tenant, device class, etc.).
type ScopeSchema struct {
	ScopeKey      string                  `json:"scopeKey"`
	ExpectedFor   string                  `json:"expectedFor"`
	ScopeID       string                  `json:"scopeId,omitempty"`
	PacketSchemas map[string]PacketSchema `json:"packetSchemas"`
}

const (
	columnPacketType       = "packet_type"
	columnExpectedFor      = "expected_for"
	columnScopeID          = "scope_id"
	columnKey              = "key"
	columnDescription      = "description"
	columnUnit             = "unit"
	columnRequired         = "required"
	columnMaxLength        = "max_length"
	columnTopicTemplate    = "topic_template"
	columnEnvelopeRequired = "envelope_required"
	columnNotes            = "notes"
	columnValueMin         = "value_min"
	columnValueMax         = "value_max"
	columnResolution       = "resolution"
)

// LoadCSV parses the payload schema CSV and returns every valid row as an Entry slice.
func LoadCSV(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open schema csv: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read schema csv: %w", err)
	}
	if len(records) == 0 {
		return nil, errors.New("schema csv is empty")
	}

	headers := records[0]
	if len(headers) == 0 {
		return nil, errors.New("schema csv missing header row")
	}

	colIndex := func(name string) (int, error) {
		for i, header := range headers {
			if strings.EqualFold(strings.TrimSpace(header), name) {
				return i, nil
			}
		}
		return -1, fmt.Errorf("required column %q not found", name)
	}

	optionalIndex := func(name string) int {
		for i, header := range headers {
			if strings.EqualFold(strings.TrimSpace(header), name) {
				return i
			}
		}
		return -1
	}

	requiredColumns := []string{
		columnPacketType,
		columnExpectedFor,
		columnScopeID,
		columnKey,
		columnDescription,
		columnUnit,
		columnRequired,
		columnMaxLength,
		columnTopicTemplate,
		columnEnvelopeRequired,
		columnNotes,
	}

	indices := make(map[string]int, len(requiredColumns))
	for _, column := range requiredColumns {
		idx, err := colIndex(column)
		if err != nil {
			return nil, err
		}
		indices[column] = idx
	}
	indices[columnValueMin] = optionalIndex(columnValueMin)
	indices[columnValueMax] = optionalIndex(columnValueMax)
	indices[columnResolution] = optionalIndex(columnResolution)

	entries := make([]Entry, 0, len(records)-1)

	for _, record := range records[1:] {
		if len(record) == 0 {
			continue
		}
		firstCell := strings.TrimSpace(record[0])
		if strings.HasPrefix(firstCell, "#") {
			continue
		}

		get := func(column string) string {
			idx := indices[column]
			if idx < 0 || idx >= len(record) {
				return ""
			}
			return strings.TrimSpace(record[idx])
		}

		packetType := get(columnPacketType)
		key := get(columnKey)
		if packetType == "" && key == "" {
			continue
		}

		required := parseBool(get(columnRequired))
		envelope := parseBool(get(columnEnvelopeRequired))

		var maxLength *int
		if raw := get(columnMaxLength); raw != "" {
			if value, convErr := strconv.Atoi(raw); convErr == nil {
				maxLength = &value
			}
		}

		valueMin := parseFloat(get(columnValueMin))
		valueMax := parseFloat(get(columnValueMax))
		resolution := parseFloat(get(columnResolution))

		entry := Entry{
			PacketType:       packetType,
			ExpectedFor:      get(columnExpectedFor),
			ScopeID:          get(columnScopeID),
			Key:              key,
			Description:      get(columnDescription),
			Unit:             get(columnUnit),
			Required:         required,
			MaxLength:        maxLength,
			TopicTemplate:    get(columnTopicTemplate),
			EnvelopeRequired: envelope,
			Notes:            get(columnNotes),
			ValueMin:         valueMin,
			ValueMax:         valueMax,
			Resolution:       resolution,
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// BuildScopes groups entries by scope and packet type, ready for JSON serialisation.
func BuildScopes(entries []Entry) map[string]ScopeSchema {
	result := make(map[string]ScopeSchema)

	for _, entry := range entries {
		scopeKey := buildScopeKey(entry.ExpectedFor, entry.ScopeID)
		scope, ok := result[scopeKey]
		if !ok {
			scope = ScopeSchema{
				ScopeKey:      scopeKey,
				ExpectedFor:   strings.TrimSpace(entry.ExpectedFor),
				ScopeID:       strings.TrimSpace(entry.ScopeID),
				PacketSchemas: make(map[string]PacketSchema),
			}
		}

		packet := scope.PacketSchemas[entry.PacketType]
		packet.PacketType = entry.PacketType
		if packet.TopicTemplate == "" && entry.TopicTemplate != "" {
			packet.TopicTemplate = entry.TopicTemplate
		}

		spec := KeySpec{
			Key:         entry.Key,
			Description: entry.Description,
			Unit:        entry.Unit,
			Required:    entry.Required,
			MaxLength:   entry.MaxLength,
			Notes:       entry.Notes,
			ValueMin:    entry.ValueMin,
			ValueMax:    entry.ValueMax,
			Resolution:  entry.Resolution,
		}

		if entry.EnvelopeRequired {
			packet.EnvelopeKeys = append(packet.EnvelopeKeys, spec)
		} else {
			packet.Keys = append(packet.Keys, spec)
		}

		scope.PacketSchemas[entry.PacketType] = packet
		result[scopeKey] = scope
	}

	return result
}

func buildScopeKey(expectedFor, scopeID string) string {
	expectedFor = strings.TrimSpace(expectedFor)
	scopeID = strings.TrimSpace(scopeID)
	if scopeID == "" {
		return expectedFor
	}
	return fmt.Sprintf("%s:%s", expectedFor, scopeID)
}

// ScopeKeyForProject returns the canonical scope key for a project-specific payload schema.
func ScopeKeyForProject(projectID string) string {
	return buildScopeKey("project", projectID)
}

func parseBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "y", "yes", "true", "1":
		return true
	default:
		return false
	}
}

func parseFloat(raw string) *float64 {
	if raw == "" {
		return nil
	}
	if v, err := strconv.ParseFloat(raw, 64); err == nil {
		return &v
	}
	return nil
}

// ToJSONFriendly converts the grouped scopes into a structure that marshals cleanly.
func ToJSONFriendly(scopes map[string]ScopeSchema) map[string]any {
	out := make(map[string]any, len(scopes))
	for key, schema := range scopes {
		out[key] = schema
	}
	return out
}

// LoadAndGroup is a convenience helper that loads the CSV and groups rows in one go.
func LoadAndGroup(path string) (map[string]ScopeSchema, error) {
	entries, err := LoadCSV(path)
	if err != nil {
		return nil, err
	}
	return BuildScopes(entries), nil
}

// WriteJSON pretty-prints the schema to a writer for snapshots or config distribution.
func WriteJSON(w io.Writer, scopes map[string]ScopeSchema) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(ToJSONFriendly(scopes)); err != nil {
		return fmt.Errorf("encode schema json: %w", err)
	}
	return nil
}
