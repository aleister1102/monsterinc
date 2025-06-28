package time

import "time"

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
