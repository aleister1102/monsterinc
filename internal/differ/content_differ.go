package differ

import (
	"fmt"
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"

	"github.com/rs/zerolog"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// ContentDifferConfig holds configuration for ContentDiffer
type ContentDifferConfig struct {
	EnableSemanticCleanup bool
	EnableLineBasedDiff   bool
	ContextLines          int
}

// DefaultContentDifferConfig returns default configuration
func DefaultContentDifferConfig() ContentDifferConfig {
	return ContentDifferConfig{
		EnableSemanticCleanup: true,
		EnableLineBasedDiff:   true,
		ContextLines:          3,
	}
}

// DiffProcessor handles the core diffing logic
type DiffProcessor struct {
	dmp    *diffmatchpatch.DiffMatchPatch
	config ContentDifferConfig
	logger zerolog.Logger
}

// NewDiffProcessor creates a new diff processor
func NewDiffProcessor(config ContentDifferConfig, logger zerolog.Logger) *DiffProcessor {
	return &DiffProcessor{
		dmp:    diffmatchpatch.New(),
		config: config,
		logger: logger.With().Str("component", "DiffProcessor").Logger(),
	}
}

// ProcessDiff generates diff between two content strings
func (dp *DiffProcessor) ProcessDiff(text1, text2 string) []diffmatchpatch.Diff {
	diffs := dp.dmp.DiffMain(text1, text2, dp.config.EnableLineBasedDiff)

	if dp.config.EnableSemanticCleanup {
		diffs = dp.dmp.DiffCleanupSemantic(diffs)
	}

	return diffs
}

// ContentSizeValidator validates content size against limits
type ContentSizeValidator struct {
	maxSizeBytes int64
	logger       zerolog.Logger
}

// NewContentSizeValidator creates a new content size validator
func NewContentSizeValidator(maxSizeMB int, logger zerolog.Logger) *ContentSizeValidator {
	return &ContentSizeValidator{
		maxSizeBytes: int64(maxSizeMB) * 1024 * 1024,
		logger:       logger.With().Str("component", "ContentSizeValidator").Logger(),
	}
}

// ValidateSize checks if content sizes are within limits
func (csv *ContentSizeValidator) ValidateSize(previousContent, currentContent []byte) error {
	if int64(len(previousContent)) > csv.maxSizeBytes {
		return common.NewValidationError("previous_content", len(previousContent),
			fmt.Sprintf("previous content too large (%d bytes > %d bytes limit)", len(previousContent), csv.maxSizeBytes))
	}

	if int64(len(currentContent)) > csv.maxSizeBytes {
		return common.NewValidationError("current_content", len(currentContent),
			fmt.Sprintf("current content too large (%d bytes > %d bytes limit)", len(currentContent), csv.maxSizeBytes))
	}

	return nil
}

// DiffStatsCalculator calculates statistics from diff results
type DiffStatsCalculator struct {
	logger zerolog.Logger
}

// NewDiffStatsCalculator creates a new diff stats calculator
func NewDiffStatsCalculator(logger zerolog.Logger) *DiffStatsCalculator {
	return &DiffStatsCalculator{
		logger: logger.With().Str("component", "DiffStatsCalculator").Logger(),
	}
}

// DiffStatistics holds diff calculation results
type DiffStatistics struct {
	LinesAdded   int
	LinesDeleted int
	LinesChanged int
	IsIdentical  bool
}

// CalculateStats computes statistics from diff results
func (dsc *DiffStatsCalculator) CalculateStats(diffs []diffmatchpatch.Diff, oldHash, newHash string) DiffStatistics {
	stats := DiffStatistics{}

	// Count operations
	for _, diff := range diffs {
		switch diff.Type {
		case diffmatchpatch.DiffInsert:
			stats.LinesAdded++
		case diffmatchpatch.DiffDelete:
			stats.LinesDeleted++
		}
	}

	// Determine if content is identical
	stats.IsIdentical = dsc.determineIdentical(diffs, oldHash, newHash)

	return stats
}

// determineIdentical checks if content is identical based on diffs and hashes
func (dsc *DiffStatsCalculator) determineIdentical(diffs []diffmatchpatch.Diff, oldHash, newHash string) bool {
	// If hashes are different, content cannot be identical
	if oldHash != "" && newHash != "" && oldHash != newHash {
		dsc.logger.Debug().
			Str("old_hash", oldHash).
			Str("new_hash", newHash).
			Msg("Content hashes differ, marking as not identical")
		return false
	}

	// Check if only equal diffs exist
	if len(diffs) == 1 && diffs[0].Type == diffmatchpatch.DiffEqual {
		return true
	}

	// Check if no meaningful changes exist
	hasChanges := false
	for _, diff := range diffs {
		if diff.Type != diffmatchpatch.DiffEqual {
			hasChanges = true
			break
		}
	}

	return !hasChanges
}

// ContentDiffResultBuilder builds ContentDiffResult objects
type ContentDiffResultBuilder struct {
	result models.ContentDiffResult
	logger zerolog.Logger
}

// NewContentDiffResultBuilder creates a new result builder
func NewContentDiffResultBuilder(logger zerolog.Logger) *ContentDiffResultBuilder {
	return &ContentDiffResultBuilder{
		result: models.ContentDiffResult{
			Timestamp: time.Now().UnixMilli(),
		},
		logger: logger.With().Str("component", "ContentDiffResultBuilder").Logger(),
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
	resultDiffs := make([]models.ContentDiff, 0, len(diffs))

	for _, diff := range diffs {
		operation := models.DiffEqual
		switch diff.Type {
		case diffmatchpatch.DiffInsert:
			operation = models.DiffInsert
		case diffmatchpatch.DiffDelete:
			operation = models.DiffDelete
		}

		resultDiffs = append(resultDiffs, models.ContentDiff{
			Operation: operation,
			Text:      diff.Text,
		})
	}

	rb.result.Diffs = resultDiffs
	rb.result.LinesAdded = stats.LinesAdded
	rb.result.LinesDeleted = stats.LinesDeleted
	rb.result.LinesChanged = stats.LinesChanged
	rb.result.IsIdentical = stats.IsIdentical

	return rb
}

// Build creates the final ContentDiffResult
func (rb *ContentDiffResultBuilder) Build() *models.ContentDiffResult {
	return &rb.result
}

// ContentDiffer is responsible for generating differences between content versions
type ContentDiffer struct {
	logger          zerolog.Logger
	diffConfig      *config.DiffReporterConfig
	processor       *DiffProcessor
	sizeValidator   *ContentSizeValidator
	statsCalculator *DiffStatsCalculator
	config          ContentDifferConfig
}

// ContentDifferBuilder provides a fluent interface for creating ContentDiffer
type ContentDifferBuilder struct {
	logger     zerolog.Logger
	diffConfig *config.DiffReporterConfig
	config     ContentDifferConfig
}

// NewContentDifferBuilder creates a new builder
func NewContentDifferBuilder(logger zerolog.Logger) *ContentDifferBuilder {
	return &ContentDifferBuilder{
		logger: logger.With().Str("component", "ContentDiffer").Logger(),
		config: DefaultContentDifferConfig(),
	}
}

// WithDiffReporterConfig sets the diff reporter configuration
func (b *ContentDifferBuilder) WithDiffReporterConfig(cfg *config.DiffReporterConfig) *ContentDifferBuilder {
	b.diffConfig = cfg
	return b
}

// WithContentDifferConfig sets the content differ configuration
func (b *ContentDifferBuilder) WithContentDifferConfig(cfg ContentDifferConfig) *ContentDifferBuilder {
	b.config = cfg
	return b
}

// Build creates a new ContentDiffer instance
func (b *ContentDifferBuilder) Build() (*ContentDiffer, error) {
	if b.diffConfig == nil {
		return nil, common.NewValidationError("diff_config", b.diffConfig, "diff reporter config cannot be nil")
	}

	processor := NewDiffProcessor(b.config, b.logger)
	sizeValidator := NewContentSizeValidator(b.diffConfig.MaxDiffFileSizeMB, b.logger)
	statsCalculator := NewDiffStatsCalculator(b.logger)

	return &ContentDiffer{
		logger:          b.logger,
		diffConfig:      b.diffConfig,
		processor:       processor,
		sizeValidator:   sizeValidator,
		statsCalculator: statsCalculator,
		config:          b.config,
	}, nil
}

// NewContentDiffer creates a new instance of ContentDiffer using builder pattern
func NewContentDiffer(logger zerolog.Logger, diffCfg *config.DiffReporterConfig) (*ContentDiffer, error) {
	return NewContentDifferBuilder(logger).
		WithDiffReporterConfig(diffCfg).
		Build()
}

// validateInputs validates the input parameters for diff generation
func (cd *ContentDiffer) validateInputs(previousContent, currentContent []byte, contentType, oldHash, newHash string) error {
	if contentType == "" {
		return common.NewValidationError("content_type", contentType, "content type cannot be empty")
	}

	if err := cd.sizeValidator.ValidateSize(previousContent, currentContent); err != nil {
		return err
	}

	return nil
}

// createTooLargeResult creates a result for content that's too large to diff
func (cd *ContentDiffer) createTooLargeResult(previousContent, currentContent []byte, contentType, oldHash, newHash string, processingTime time.Duration) *models.ContentDiffResult {
	errorMsg := fmt.Sprintf("Content changed, but file is too large for detailed diff (limit: %dMB). Previous size: %d bytes, Current size: %d bytes.",
		cd.diffConfig.MaxDiffFileSizeMB, len(previousContent), len(currentContent))

	return NewContentDiffResultBuilder(cd.logger).
		WithContentType(contentType).
		WithHashes(oldHash, newHash).
		WithError(errorMsg).
		WithProcessingTime(processingTime).
		Build()
}

// GenerateDiff compares two byte slices of content and returns a structured diff result
func (cd *ContentDiffer) GenerateDiff(previousContent []byte, currentContent []byte, contentType string, oldHash string, newHash string) (*models.ContentDiffResult, error) {
	startTime := time.Now()

	// Validate inputs
	if err := cd.validateInputs(previousContent, currentContent, contentType, oldHash, newHash); err != nil {
		if validationErr, ok := err.(*common.ValidationError); ok && validationErr.Field == "previous_content" || validationErr.Field == "current_content" {
			// Content too large - return special result instead of error
			cd.logger.Warn().
				Str("contentType", contentType).
				Int64("prevSize", int64(len(previousContent))).
				Int64("currSize", int64(len(currentContent))).
				Int("maxMB", cd.diffConfig.MaxDiffFileSizeMB).
				Msg("File too large for detailed diff")

			return cd.createTooLargeResult(previousContent, currentContent, contentType, oldHash, newHash, time.Since(startTime)), nil
		}
		return nil, common.WrapError(err, "failed to validate diff inputs")
	}

	// Convert byte slices to strings for diffing
	text1 := string(previousContent)
	text2 := string(currentContent)

	// Process the diff
	diffs := cd.processor.ProcessDiff(text1, text2)

	// Calculate statistics
	stats := cd.statsCalculator.CalculateStats(diffs, oldHash, newHash)

	// Build result
	result := NewContentDiffResultBuilder(cd.logger).
		WithContentType(contentType).
		WithHashes(oldHash, newHash).
		WithDiffs(diffs, stats).
		WithProcessingTime(time.Since(startTime)).
		Build()

	cd.logger.Debug().
		Str("contentType", contentType).
		Bool("is_identical", stats.IsIdentical).
		Int("diffs_count", len(diffs)).
		Int("lines_added", stats.LinesAdded).
		Int("lines_deleted", stats.LinesDeleted).
		Int64("processing_time_ms", result.ProcessingTimeMs).
		Msg("Content diff generated successfully")

	return result, nil
}
