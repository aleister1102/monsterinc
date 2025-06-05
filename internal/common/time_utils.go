package common

import (
	"fmt"
	"time"
)

// Common time layout constants
const (
	LayoutRFC3339      = time.RFC3339
	LayoutRFC3339Nano  = time.RFC3339Nano
	LayoutDateOnly     = "2006-01-02"
	LayoutTimeOnly     = "15:04:05"
	LayoutDateTime     = "2006-01-02 15:04:05"
	LayoutDisplayFull  = "Jan 2, 2006 at 3:04 PM MST"
	LayoutDisplayShort = "Jan 2, 2006"
	LayoutLogFormat    = "2006-01-02T15:04:05.000Z07:00"
	LayoutISO8601      = "2006-01-02T15:04:05Z07:00"
)

// TimeFormatter defines interface for time formatting strategies
type TimeFormatter interface {
	Format(t time.Time) string
	FormatOptional(t time.Time) string
	CanFormat(t time.Time) bool
}

// RFC3339Formatter formats times in RFC3339 format
type RFC3339Formatter struct{}

// Format formats time in RFC3339
func (rf *RFC3339Formatter) Format(t time.Time) string {
	return t.Format(LayoutRFC3339)
}

// FormatOptional formats time in RFC3339 if not zero
func (rf *RFC3339Formatter) FormatOptional(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return rf.Format(t)
}

// CanFormat checks if time can be formatted
func (rf *RFC3339Formatter) CanFormat(t time.Time) bool {
	return !t.IsZero()
}

// DisplayFormatter formats times for display purposes
type DisplayFormatter struct {
	layout string
}

// NewDisplayFormatter creates a display formatter
func NewDisplayFormatter(layout string) *DisplayFormatter {
	return &DisplayFormatter{layout: layout}
}

// Format formats time for display
func (df *DisplayFormatter) Format(t time.Time) string {
	return t.Format(df.layout)
}

// FormatOptional formats time for display if not zero
func (df *DisplayFormatter) FormatOptional(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return df.Format(t)
}

// CanFormat checks if time can be formatted
func (df *DisplayFormatter) CanFormat(t time.Time) bool {
	return !t.IsZero()
}

// LogFormatter formats times for logging
type LogFormatter struct{}

// Format formats time for logging
func (lf *LogFormatter) Format(t time.Time) string {
	return t.Format(LayoutLogFormat)
}

// FormatOptional formats time for logging if not zero
func (lf *LogFormatter) FormatOptional(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return lf.Format(t)
}

// CanFormat checks if time can be formatted
func (lf *LogFormatter) CanFormat(t time.Time) bool {
	return !t.IsZero()
}

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

// TimeToUnixMilli converts time.Time to Unix milliseconds
func (tc *TimeConverter) TimeToUnixMilli(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

// TimeToUnixMilliPtr converts time.Time to Unix milliseconds pointer
func (tc *TimeConverter) TimeToUnixMilliPtr(t time.Time) *int64 {
	if t.IsZero() {
		return nil
	}
	ms := t.UnixMilli()
	return &ms
}

// UnixSecToTime converts Unix seconds to time.Time
func (tc *TimeConverter) UnixSecToTime(sec int64) time.Time {
	return time.Unix(sec, 0)
}

// TimeToUnixSec converts time.Time to Unix seconds
func (tc *TimeConverter) TimeToUnixSec(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

// TimeValidator validates time values
type TimeValidator struct{}

// NewTimeValidator creates a new time validator
func NewTimeValidator() *TimeValidator {
	return &TimeValidator{}
}

// IsValid checks if time is valid (not zero and not too far in past/future)
func (tv *TimeValidator) IsValid(t time.Time) bool {
	if t.IsZero() {
		return false
	}

	// Check if time is reasonable (not too far in past or future)
	now := time.Now()
	oneHundredYearsAgo := now.AddDate(-100, 0, 0)
	oneHundredYearsFromNow := now.AddDate(100, 0, 0)

	return t.After(oneHundredYearsAgo) && t.Before(oneHundredYearsFromNow)
}

// IsInFuture checks if time is in the future
func (tv *TimeValidator) IsInFuture(t time.Time) bool {
	if t.IsZero() {
		return false
	}
	return t.After(time.Now())
}

// IsInPast checks if time is in the past
func (tv *TimeValidator) IsInPast(t time.Time) bool {
	if t.IsZero() {
		return false
	}
	return t.Before(time.Now())
}

// ValidateTimeRange validates if time is within a range
func (tv *TimeValidator) ValidateTimeRange(t, start, end time.Time) error {
	if t.IsZero() {
		return NewValidationError("time", t, "time cannot be zero")
	}

	if !start.IsZero() && t.Before(start) {
		return NewValidationError("time", t, "time is before start time")
	}

	if !end.IsZero() && t.After(end) {
		return NewValidationError("time", t, "time is after end time")
	}

	return nil
}

// TimeUtils provides a comprehensive set of time utilities
type TimeUtils struct {
	converter  *TimeConverter
	validator  *TimeValidator
	formatters map[string]TimeFormatter
}

// NewTimeUtils creates a new time utilities instance
func NewTimeUtils() *TimeUtils {
	return &TimeUtils{
		converter: NewTimeConverter(),
		validator: NewTimeValidator(),
		formatters: map[string]TimeFormatter{
			"rfc3339":  &RFC3339Formatter{},
			"display":  NewDisplayFormatter(LayoutDisplayFull),
			"short":    NewDisplayFormatter(LayoutDisplayShort),
			"log":      &LogFormatter{},
			"datetime": NewDisplayFormatter(LayoutDateTime),
			"date":     NewDisplayFormatter(LayoutDateOnly),
			"time":     NewDisplayFormatter(LayoutTimeOnly),
		},
	}
}

// Convert returns the time converter
func (tu *TimeUtils) Convert() *TimeConverter {
	return tu.converter
}

// Validate returns the time validator
func (tu *TimeUtils) Validate() *TimeValidator {
	return tu.validator
}

// FormatWith formats time using specified formatter
func (tu *TimeUtils) FormatWith(formatterName string, t time.Time) string {
	formatter, exists := tu.formatters[formatterName]
	if !exists {
		// Fallback to RFC3339
		formatter = &RFC3339Formatter{}
	}
	return formatter.Format(t)
}

// FormatOptionalWith formats time using specified formatter if not zero
func (tu *TimeUtils) FormatOptionalWith(formatterName string, t time.Time) string {
	formatter, exists := tu.formatters[formatterName]
	if !exists {
		// Fallback to RFC3339
		formatter = &RFC3339Formatter{}
	}
	return formatter.FormatOptional(t)
}

// AddFormatter adds a custom formatter
func (tu *TimeUtils) AddFormatter(name string, formatter TimeFormatter) {
	tu.formatters[name] = formatter
}

// Now returns current time
func (tu *TimeUtils) Now() time.Time {
	return time.Now()
}

// NowUTC returns current UTC time
func (tu *TimeUtils) NowUTC() time.Time {
	return time.Now().UTC()
}

// Legacy functions for backward compatibility

// UnixMilliToTimeOptional converts an optional int64 (Unix milliseconds) to time.Time
func UnixMilliToTimeOptional(ms *int64) time.Time {
	return NewTimeConverter().UnixMilliToTimeOptional(ms)
}

// FormatTimeOptional formats a time.Time object into a string using the specified layout
func FormatTimeOptional(t time.Time, layout string) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(layout)
}

// Additional utility functions

// TimePtr creates a pointer to time.Time
func TimePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// TimePtrToTime converts time pointer to time.Time
func TimePtrToTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

// DurationPtr creates a pointer to time.Duration
func DurationPtr(d time.Duration) *time.Duration {
	return &d
}

// DurationPtrToDuration converts duration pointer to time.Duration
func DurationPtrToDuration(d *time.Duration) time.Duration {
	if d == nil {
		return 0
	}
	return *d
}

// FormatDuration formats duration in human-readable format
func FormatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}

	if d < time.Second {
		return d.String()
	}

	// For longer durations, use more readable format
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh%dm%ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}
