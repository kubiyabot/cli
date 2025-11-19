package entities

import (
	"strings"
	"time"
)

// CustomTime handles multiple timestamp formats from the API
type CustomTime struct {
	time.Time
}

// UnmarshalJSON implements custom JSON unmarshaling for timestamps
func (ct *CustomTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "null" || s == "" {
		return nil
	}

	// Try multiple formats
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05.999999",    // Without timezone
		"2006-01-02T15:04:05",           // Without microseconds and timezone
		"2006-01-02 15:04:05.999999",    // Space separator
		"2006-01-02 15:04:05",           // Space separator without microseconds
	}

	var parseErr error
	for _, format := range formats {
		t, err := time.Parse(format, s)
		if err == nil {
			ct.Time = t
			return nil
		}
		parseErr = err
	}

	return parseErr
}

// MarshalJSON implements custom JSON marshaling for timestamps
func (ct CustomTime) MarshalJSON() ([]byte, error) {
	if ct.Time.IsZero() {
		return []byte("null"), nil
	}
	return []byte(`"` + ct.Time.Format(time.RFC3339) + `"`), nil
}
