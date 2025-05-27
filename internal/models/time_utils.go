package models

import "time"

// UnixMilliToTimeOptional converts an optional int64 (Unix milliseconds) to time.Time.
// If the input pointer is nil, it returns a zero time.Time value.
func UnixMilliToTimeOptional(ms *int64) time.Time {
	if ms == nil {
		return time.Time{} // Return zero time if nil
	}
	return time.UnixMilli(*ms)
}

// FormatTimeOptional formats a time.Time object into a string using the specified layout.
// If the time is zero, it returns an empty string.
func FormatTimeOptional(t time.Time, layout string) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(layout)
}
