// Package domain contains the analytics-service aggregate roots and value objects.
package domain

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Date is a calendar-date value that marshals to and from "YYYY-MM-DD" JSON strings.
type Date struct{ time.Time }

// NewDate parses a Date from a string. It returns a zero Date for an empty string.
func NewDate(v string) (Date, error) {
	if v == "" {
		return Date{}, nil
	}
	t, err := time.Parse(time.DateOnly, v)
	if err != nil {
		return Date{}, err
	}
	return Date{t}, nil
}

// String returns the date in YYYY-MM-DD format.
func (d Date) String() string {
	if d.IsZero() {
		return ""
	}
	return d.Format(time.DateOnly)
}

// MarshalJSON implements json.Marshaler.
func (d Date) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// UnmarshalJSON implements json.Unmarshaler.
func (d *Date) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == "" {
		d.Time = time.Time{}
		return nil
	}
	parsed, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return err
	}
	d.Time = parsed
	return nil
}

// IsEmpty reports whether the date is the zero value.
func (d Date) IsEmpty() bool { return d.IsZero() }

// Unit enumerates the supported metric aggregation kinds.
type Unit string

const (
	UnitCount      Unit = "count"
	UnitSum        Unit = "sum"
	UnitAverage    Unit = "average"
	UnitPercentage Unit = "percentage"
)

func IsValidUnit(u Unit) bool {
	switch u {
	case UnitCount, UnitSum, UnitAverage, UnitPercentage:
		return true
	}
	return false
}

// Dimensions is an opaque JSONB map used to slice a metric (e.g. subject_id).
type Dimensions map[string]string

// Key returns a deterministic string key for equality comparisons.
func (d Dimensions) Key() string {
	if len(d) == 0 {
		return ""
	}
	keys := make([]string, 0, len(d))
	for k := range d {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(d))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, d[k]))
	}
	return strings.Join(parts, "|")
}

// Metric is the aggregate root for a time-bucketed projection. Each row uniquely
// identifies a (tenant, metric_name, bucket_date, dimensions) tuple.
type Metric struct {
	ID          string     `json:"id"`
	TenantID    string     `json:"tenant_id"`
	MetricName  string     `json:"metric_name"`
	BucketDate  Date       `json:"bucket_date"`
	Value       float64    `json:"value"`
	Unit        Unit       `json:"unit"`
	Dimensions  Dimensions `json:"dimensions,omitempty"`
	SampleCount *int64     `json:"sample_count,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// NewMetric constructs a Metric, enforcing invariants.
func NewMetric(tenantID, metricName, bucketDate string, value float64, unit Unit, dims Dimensions) (*Metric, error) {
	if tenantID == "" {
		return nil, ErrMissingTenant
	}
	if strings.TrimSpace(metricName) == "" {
		return nil, fmt.Errorf("%w: metric_name is required", ErrValidation)
	}
	date, err := NewDate(bucketDate)
	if err != nil || date.IsEmpty() {
		return nil, fmt.Errorf("%w: bucket_date must be a valid date (YYYY-MM-DD)", ErrValidation)
	}
	if !IsValidUnit(unit) {
		return nil, fmt.Errorf("%w: unit must be count, sum, average or percentage", ErrValidation)
	}
	if value < 0 && unit != UnitSum {
		return nil, fmt.Errorf("%w: value cannot be negative for unit %s", ErrValidation, unit)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("analytics: generate id: %w", err)
	}
	now := time.Now().UTC()
	m := &Metric{
		ID:         id.String(),
		TenantID:   tenantID,
		MetricName: strings.TrimSpace(metricName),
		BucketDate: date,
		Value:      value,
		Unit:       unit,
		Dimensions: normalizeDimensions(dims),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if unit == UnitAverage {
		count := int64(1)
		m.SampleCount = &count
	}
	return m, nil
}

// Validate checks that the aggregate is well-formed.
func (m Metric) Validate() error {
	if m.TenantID == "" {
		return ErrMissingTenant
	}
	if strings.TrimSpace(m.MetricName) == "" {
		return fmt.Errorf("%w: metric_name is required", ErrValidation)
	}
	if m.BucketDate.IsEmpty() {
		return fmt.Errorf("%w: bucket_date must be a valid date", ErrValidation)
	}
	if !IsValidUnit(m.Unit) {
		return fmt.Errorf("%w: unit must be count, sum, average or percentage", ErrValidation)
	}
	if m.Value < 0 && m.Unit != UnitSum {
		return fmt.Errorf("%w: value cannot be negative for unit %s", ErrValidation, m.Unit)
	}
	if m.Unit == UnitAverage && (m.SampleCount == nil || *m.SampleCount <= 0) {
		return fmt.Errorf("%w: average metrics require a positive sample_count", ErrValidation)
	}
	return nil
}

// Increment adds delta to a count metric.
func (m *Metric) Increment(delta float64) error {
	if m.Unit != UnitCount {
		return fmt.Errorf("%w: cannot increment a %s metric", ErrValidation, m.Unit)
	}
	if delta < 0 {
		return fmt.Errorf("%w: increment delta cannot be negative", ErrValidation)
	}
	m.Value += delta
	m.UpdatedAt = time.Now().UTC()
	return nil
}

// Add adds amount to a sum metric.
func (m *Metric) Add(amount float64) error {
	if m.Unit != UnitSum {
		return fmt.Errorf("%w: cannot add to a %s metric", ErrValidation, m.Unit)
	}
	m.Value += amount
	m.UpdatedAt = time.Now().UTC()
	return nil
}

// AddSample incorporates a new sample into an average metric using a weighted update.
func (m *Metric) AddSample(sample float64) error {
	if m.Unit != UnitAverage {
		return fmt.Errorf("%w: cannot add sample to a %s metric", ErrValidation, m.Unit)
	}
	if m.SampleCount == nil {
		count := int64(0)
		m.SampleCount = &count
	}
	count := *m.SampleCount
	if count <= 0 {
		m.Value = sample
		count = 1
	} else {
		m.Value = (m.Value*float64(count) + sample) / float64(count+1)
		count++
	}
	m.SampleCount = &count
	m.UpdatedAt = time.Now().UTC()
	return nil
}

// ValueString returns the value formatted for persistence as a numeric string.
func (m Metric) ValueString() string {
	return strconv.FormatFloat(m.Value, 'f', -1, 64)
}

func normalizeDimensions(d Dimensions) Dimensions {
	if len(d) == 0 {
		return nil
	}
	out := make(Dimensions, len(d))
	for k, v := range d {
		out[k] = v
	}
	return out
}
