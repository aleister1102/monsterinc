package differ

import (
	"fmt"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"
	"time"

	"github.com/rs/zerolog"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// ContentDiffer is responsible for generating differences between content versions.
type ContentDiffer struct {
	logger     zerolog.Logger
	diffConfig *config.DiffReporterConfig
}

// NewContentDiffer creates a new instance of ContentDiffer.
func NewContentDiffer(logger zerolog.Logger, diffCfg *config.DiffReporterConfig) *ContentDiffer {
	return &ContentDiffer{
		logger:     logger,
		diffConfig: diffCfg,
	}
}

// GenerateDiff compares two byte slices of content and returns a structured diff result.
// It also considers a maximum file size for generating diffs.
func (cd *ContentDiffer) GenerateDiff(previousContent []byte, currentContent []byte, contentType string, oldHash string, newHash string) (*models.ContentDiffResult, error) {
	startTime := time.Now()

	// Check for large files (Task 5.1)
	maxSizeBytes := int64(cd.diffConfig.MaxDiffFileSizeMB) * 1024 * 1024
	if int64(len(previousContent)) > maxSizeBytes || int64(len(currentContent)) > maxSizeBytes {
		cd.logger.Warn().Str("contentType", contentType).Int64("prevSize", int64(len(previousContent))).Int64("currSize", int64(len(currentContent))).Int("maxMB", cd.diffConfig.MaxDiffFileSizeMB).Msg("File too large for detailed diff.")
		return &models.ContentDiffResult{
			Timestamp:        time.Now().UnixMilli(),
			ContentType:      contentType,
			IsIdentical:      false, // Cannot determine without diffing, but it has changed enough to be checked
			ErrorMessage:     fmt.Sprintf("Content changed, but file is too large for detailed diff (limit: %dMB). Previous size: %d bytes, Current size: %d bytes.", cd.diffConfig.MaxDiffFileSizeMB, len(previousContent), len(currentContent)),
			ProcessingTimeMs: time.Since(startTime).Milliseconds(),
			OldHash:          oldHash,
			NewHash:          newHash,
			SecretFindings:   []models.SecretFinding{}, // Initialize empty slice
		}, nil
	}

	dmp := diffmatchpatch.New()

	// Convert byte slices to strings for diffing
	text1 := string(previousContent)
	text2 := string(currentContent)

	diffs := dmp.DiffMain(text1, text2, true) // true for checkLines (line-based diff)

	// Optional: Clean up semantics for better readability
	diffs = dmp.DiffCleanupSemantic(diffs)

	// Convert to models.ContentDiff and count added/deleted lines
	var resultDiffs []models.ContentDiff // Use the new struct
	linesAdded := 0
	linesDeleted := 0
	// linesChanged := 0 // diffmatchpatch doesn't directly give 'changed' lines, only insert/delete

	for _, diff := range diffs {
		operation := models.DiffEqual
		switch diff.Type {
		case diffmatchpatch.DiffInsert:
			operation = models.DiffInsert
			linesAdded++ // Simplistic line count; might need adjustment based on newline characters
		case diffmatchpatch.DiffDelete:
			operation = models.DiffDelete
			linesDeleted++ // Simplistic line count
		}
		resultDiffs = append(resultDiffs, models.ContentDiff{ // Use models.ContentDiff
			Operation: operation,
			Text:      diff.Text,
		})
	}

	// Determine if content is identical.
	// A direct byte comparison is more reliable than just checking diffs array length,
	// especially after semantic cleanup which might alter the diffs.
	isIdentical := len(previousContent) == len(currentContent) && string(previousContent) == string(currentContent)
	if !isIdentical && len(diffs) == 1 && diffs[0].Type == diffmatchpatch.DiffEqual {
		// This case can happen if only whitespace/semantic changes are present after cleanup
		// Or if the content is indeed identical. Double check by direct comparison.
		isIdentical = true
	}

	processingTimeMs := time.Since(startTime).Milliseconds()
	cd.logger.Debug().
		Str("contentType", contentType).
		Bool("is_identical", isIdentical).
		Int("diffs_count", len(diffs)).
		Int("lines_added", linesAdded).
		Int("lines_deleted", linesDeleted).
		Int64("processing_time_ms", processingTimeMs).
		Msg("Content diff generated")

	return &models.ContentDiffResult{
			Timestamp:        time.Now().UnixMilli(),
			ContentType:      contentType,
		Diffs:            resultDiffs,
			LinesAdded:       linesAdded,
			LinesDeleted:     linesDeleted,
		LinesChanged:     0, // Not implemented yet
			IsIdentical:      isIdentical,
			ProcessingTimeMs: processingTimeMs,
			OldHash:          oldHash,
			NewHash:          newHash,
		SecretFindings:   []models.SecretFinding{}, // Initialize empty slice
	}, nil
	// If an error occurs during diffing (though dmp.DiffMain doesn't return one, future libraries might)
	// return nil, fmt.Errorf("error generating diff: %w", err)
}

// func (cd *ContentDiffer) GenerateDiffWithHashes(previousContent []byte, currentContent []byte, contentType string, oldHash string, newHash string) (*models.ContentDiffResult, error) {
// 	// ... existing code ...
// }
