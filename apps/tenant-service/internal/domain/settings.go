package domain

// Settings is a tenant's operational configuration served to the web shell at
// startup (agent_plan AURA-5.2, spec §3).
type Settings struct {
	Locale                 string `json:"locale,omitempty"`
	Timezone               string `json:"timezone,omitempty"`
	DateFormat             string `json:"date_format,omitempty"`
	AcademicYearStartMonth int    `json:"academic_year_start_month,omitempty"`
	PrimaryContactEmail    string `json:"primary_contact_email,omitempty"`
}

// ValidateSettings checks that settings values are within acceptable bounds.
func ValidateSettings(s Settings) error {
	if s.AcademicYearStartMonth < 0 || s.AcademicYearStartMonth > 12 {
		return ErrValidation
	}
	return nil
}
