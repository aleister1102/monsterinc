package timeutils

import "time"

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
