package models

import "time"

// FormatTimeOptional formats a time.Time object into a string.
// If the time is zero, it returns an empty string.
func FormatTimeOptional(t time.Time, layout string) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(layout)
}

// UnixMilliToTimeOptional converts a pointer to int64 (Unix milliseconds) to a time.Time object.
// Returns a zero time.Time if the pointer is nil.
func UnixMilliToTimeOptional(millis *int64) time.Time {
	if millis == nil {
		return time.Time{}
	}
	return time.UnixMilli(*millis)
} 