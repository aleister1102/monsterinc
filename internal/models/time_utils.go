package models

import "time"

// TimeToUnixMilliOptional converts a time.Time (passed by pointer) to Unix milliseconds (int64 pointer).
// Returns nil if the input time.Time pointer is nil or if the time is zero.
func TimeToUnixMilliOptional(t *time.Time) *int64 {
	if t == nil || (*t).IsZero() {
		return nil
	}
	val := (*t).UnixMilli()
	return &val
}

// UnixMilliToTimeOptional converts an optional int64 (Unix milliseconds) to time.Time.
// Returns zero time.Time if the pointer is nil or the value is 0.
func UnixMilliToTimeOptional(ms *int64) time.Time {
	if ms == nil || *ms == 0 {
		return time.Time{}
	}
	return time.UnixMilli(*ms)
}

// FormatTimeOptional formats a time.Time object into a string using the specified layout.
// Returns "N/A" if the time is zero.
// If layout is empty, it defaults to "2006-01-02 15:04:05 MST".
func FormatTimeOptional(t time.Time, layout string) string {
	if t.IsZero() {
		return "N/A"
	}
	if layout == "" {
		layout = "2006-01-02 15:04:05 MST"
	}
	return t.Format(layout)
}
