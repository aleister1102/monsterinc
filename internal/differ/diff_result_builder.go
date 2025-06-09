package differ

import (
	"time"

	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// ContentDiffResultBuilder builds ContentDiffResult objects
type ContentDiffResultBuilder struct {
	result models.ContentDiffResult
}

// NewContentDiffResultBuilder creates a new result builder
func NewContentDiffResultBuilder() *ContentDiffResultBuilder {
	return &ContentDiffResultBuilder{
		result: models.ContentDiffResult{
			Timestamp: time.Now().UnixMilli(),
		},
	}
}

// WithContentType sets the content type
func (rb *ContentDiffResultBuilder) WithContentType(contentType string) *ContentDiffResultBuilder {
	rb.result.ContentType = contentType
	return rb
}

// WithHashes sets the old and new hashes
func (rb *ContentDiffResultBuilder) WithHashes(oldHash, newHash string) *ContentDiffResultBuilder {
	rb.result.OldHash = oldHash
	rb.result.NewHash = newHash
	return rb
}

// WithError sets an error message
func (rb *ContentDiffResultBuilder) WithError(errorMessage string) *ContentDiffResultBuilder {
	rb.result.ErrorMessage = errorMessage
	return rb
}

// WithProcessingTime sets the processing time
func (rb *ContentDiffResultBuilder) WithProcessingTime(duration time.Duration) *ContentDiffResultBuilder {
	rb.result.ProcessingTimeMs = duration.Milliseconds()
	return rb
}

// WithDiffs sets the diff results and statistics
func (rb *ContentDiffResultBuilder) WithDiffs(diffs []diffmatchpatch.Diff, stats DiffStatistics) *ContentDiffResultBuilder {
	rb.result.Diffs = rb.convertDiffsToModels(diffs)
	rb.setStatsOnResult(stats)
	return rb
}

// convertDiffsToModels converts diffmatchpatch.Diff to models.ContentDiff
func (rb *ContentDiffResultBuilder) convertDiffsToModels(diffs []diffmatchpatch.Diff) []models.ContentDiff {
	resultDiffs := make([]models.ContentDiff, 0, len(diffs))

	for _, diff := range diffs {
		operation := rb.mapDiffOperation(diff.Type)
		resultDiffs = append(resultDiffs, models.ContentDiff{
			Operation: operation,
			Text:      diff.Text,
		})
	}

	return resultDiffs
}

// mapDiffOperation maps diffmatchpatch operation to models operation
func (rb *ContentDiffResultBuilder) mapDiffOperation(diffType diffmatchpatch.Operation) models.DiffOperation {
	switch diffType {
	case diffmatchpatch.DiffInsert:
		return models.DiffInsert
	case diffmatchpatch.DiffDelete:
		return models.DiffDelete
	default:
		return models.DiffEqual
	}
}

// setStatsOnResult sets statistics on the result
func (rb *ContentDiffResultBuilder) setStatsOnResult(stats DiffStatistics) {
	rb.result.LinesAdded = stats.LinesAdded
	rb.result.LinesDeleted = stats.LinesDeleted
	rb.result.LinesChanged = stats.LinesChanged
	rb.result.IsIdentical = stats.IsIdentical
}

// Build creates the final ContentDiffResult
func (rb *ContentDiffResultBuilder) Build() *models.ContentDiffResult {
	return &rb.result
}
