package summary

// ScanStatus defines the possible states of a scan.
type ScanStatus string

const (
	ScanStatusStarted             ScanStatus = "STARTED"
	ScanStatusCompleted           ScanStatus = "COMPLETED"
	ScanStatusFailed              ScanStatus = "FAILED"
	ScanStatusCriticalError       ScanStatus = "CRITICAL_ERROR"
	ScanStatusPartialComplete     ScanStatus = "PARTIAL_COMPLETE"
	ScanStatusInterrupted         ScanStatus = "INTERRUPTED"
	ScanStatusUnknown             ScanStatus = "UNKNOWN"
	ScanStatusNoTargets           ScanStatus = "NO_TARGETS"
	ScanStatusCompletedWithIssues ScanStatus = "COMPLETED_WITH_ISSUES"
)

// IsSuccess checks if scan status indicates success
func (ss ScanStatus) IsSuccess() bool {
	return ss == ScanStatusCompleted
}

// IsFailure checks if scan status indicates failure
func (ss ScanStatus) IsFailure() bool {
	return ss == ScanStatusFailed || ss == ScanStatusCriticalError
}

// IsInProgress checks if scan status indicates in progress
func (ss ScanStatus) IsInProgress() bool {
	return ss == ScanStatusStarted
}

// GetColor returns appropriate Discord color for the status
func (ss ScanStatus) GetColor() int {
	switch ss {
	case ScanStatusCompleted:
		return DiscordColorSuccess
	case ScanStatusFailed, ScanStatusCriticalError:
		return DiscordColorError
	case ScanStatusPartialComplete, ScanStatusCompletedWithIssues:
		return DiscordColorWarning
	case ScanStatusStarted:
		return DiscordColorInfo
	default:
		return DiscordColorDefault
	}
}
