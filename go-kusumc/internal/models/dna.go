package models

import "time"

// DnaSensor represents a sensor spec persisted per project.
type DnaSensor struct {
	ProjectID     string     `json:"project_id"`
	Param         string     `json:"param"`
	Label         string     `json:"label"`
	Unit          *string    `json:"unit,omitempty"`
	MinValue      *float64   `json:"min,omitempty"`
	MaxValue      *float64   `json:"max,omitempty"`
	Resolution    *float64   `json:"resolution,omitempty"`
	Required      bool       `json:"required"`
	Notes         *string    `json:"notes,omitempty"`
	TopicTemplate *string    `json:"topic_template,omitempty"`
	UpdatedAt     *time.Time `json:"updated_at,omitempty"`
}

// DnaThreshold represents threshold bands for a parameter.
type DnaThreshold struct {
	ProjectID     string                 `json:"project_id"`
	Param         string                 `json:"param"`
	Scope         string                 `json:"scope"` // project | device
	DeviceID      *string                `json:"device_id,omitempty"`
	MinValue      *float64               `json:"min_value,omitempty"`
	MaxValue      *float64               `json:"max_value,omitempty"`
	Target        *float64               `json:"target,omitempty"`
	Unit          *string                `json:"unit,omitempty"`
	DecimalPlaces *int                   `json:"decimal_places,omitempty"`
	TemplateID    *string                `json:"template_id,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Reason        *string                `json:"reason,omitempty"`
	UpdatedBy     *string                `json:"updated_by,omitempty"`
	WarnLow       *float64               `json:"warn_low,omitempty"`
	WarnHigh      *float64               `json:"warn_high,omitempty"`
	AlertLow      *float64               `json:"alert_low,omitempty"`
	AlertHigh     *float64               `json:"alert_high,omitempty"`
	Origin        *string                `json:"origin,omitempty"`
	UpdatedAt     *time.Time             `json:"updated_at,omitempty"`
}

// DnaSensorVersion tracks CSV-based version snapshots for sensors.
type DnaSensorVersion struct {
	ID            int64      `json:"id"`
	ProjectID     string     `json:"project_id"`
	Label         string     `json:"label"`
	Status        string     `json:"status"`
	ImportedCount int        `json:"imported_count"`
	CreatedAt     time.Time  `json:"created_at"`
	PublishedAt   *time.Time `json:"published_at,omitempty"`
	RolledBackAt  *time.Time `json:"rolled_back_at,omitempty"`
	CreatedBy     *string    `json:"created_by,omitempty"`
	PublishedBy   *string    `json:"published_by,omitempty"`
	RolledBackBy  *string    `json:"rolled_back_by,omitempty"`
}
