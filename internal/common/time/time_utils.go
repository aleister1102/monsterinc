package time

import "time"

// TimeUtils provides a unified interface for time operations
type TimeUtils struct {
	converter  *TimeConverter
	formatters map[string]TimeFormatter
}

// NewTimeUtils creates a new TimeUtils instance
func NewTimeUtils() *TimeUtils {
	tu := &TimeUtils{
		converter:  NewTimeConverter(),
		formatters: make(map[string]TimeFormatter),
	}

	// Register default formatters
	tu.formatters["rfc3339"] = &RFC3339Formatter{}
	tu.formatters["log"] = &LogFormatter{}
	tu.formatters["display_full"] = NewDisplayFormatter(LayoutDisplayFull)
	tu.formatters["display_short"] = NewDisplayFormatter(LayoutDisplayShort)

	return tu
}

// UnixMilliToTimeOptional converts optional Unix milliseconds to time.Time
func UnixMilliToTimeOptional(ms *int64) time.Time {
	return NewTimeConverter().UnixMilliToTimeOptional(ms)
}

// FormatTimeOptional formats time if not zero
func FormatTimeOptional(t time.Time, layout string) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(layout)
}
