package domain

import (
	"encoding/json"
	"time"
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
