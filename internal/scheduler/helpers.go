package scheduler

import (
	"time"
)

const (
	// DefaultRetryDelay is the default delay between retry attempts
	DefaultRetryDelay = 5 * time.Minute

	// ScanStatusStarted represents a started scan
	ScanStatusStarted = "STARTED"

	// ScanStatusCompleted represents a completed scan
	ScanStatusCompleted = "COMPLETED"

	// ScanStatusFailed represents a failed scan
	ScanStatusFailed = "FAILED"
)

// TimeStampGenerator generates timestamp strings
type TimeStampGenerator struct{}

// NewTimeStampGenerator creates a new timestamp generator
func NewTimeStampGenerator() *TimeStampGenerator {
	return &TimeStampGenerator{}
}

// GenerateSessionID generates a scan session ID based on current time
func (t *TimeStampGenerator) GenerateSessionID() string {
	return time.Now().Format("20060102-150405")
}

// GenerateInterruptSessionID generates an interrupt session ID
func (t *TimeStampGenerator) GenerateInterruptSessionID(serviceType string) string {
	timestamp := time.Now().Format("20060102-150405")
	return "scheduler_interrupted_" + serviceType + "_" + timestamp
}
