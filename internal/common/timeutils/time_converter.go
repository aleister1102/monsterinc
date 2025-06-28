package timeutils

import "time"

// TimeConverter handles time conversions
type TimeConverter struct{}

// NewTimeConverter creates a new time converter
func NewTimeConverter() *TimeConverter {
	return &TimeConverter{}
}

// UnixMilliToTime converts Unix milliseconds to time.Time
func (tc *TimeConverter) UnixMilliToTime(ms int64) time.Time {
	return time.UnixMilli(ms)
}

// UnixMilliToTimeOptional converts optional Unix milliseconds to time.Time
func (tc *TimeConverter) UnixMilliToTimeOptional(ms *int64) time.Time {
	if ms == nil {
		return time.Time{}
	}
	return tc.UnixMilliToTime(*ms)
}
