package differ

import (
	"fmt"
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/rs/zerolog"
)

// ContentDiffer generates differences between content versions
type ContentDiffer struct {
	processor       *DiffProcessor
	sizeValidator   *ContentSizeValidator
	inputValidator  *InputValidator
	statsCalculator *DiffStatsCalculator
	maxFileSizeMB   int
}

// ContentDifferBuilder provides a fluent interface for creating ContentDiffer
type ContentDifferBuilder struct {
	diffConfig *config.DiffReporterConfig
	diffCfg    DiffConfig
}

// NewContentDifferBuilder creates a new builder
func NewContentDifferBuilder() *ContentDifferBuilder {
	return &ContentDifferBuilder{
		diffCfg: DefaultDiffConfig(),
	}
}

// WithDiffReporterConfig sets the diff reporter configuration
func (b *ContentDifferBuilder) WithDiffReporterConfig(cfg *config.DiffReporterConfig) *ContentDifferBuilder {
	b.diffConfig = cfg
	return b
}

// WithDiffConfig sets the diff configuration
func (b *ContentDifferBuilder) WithDiffConfig(cfg DiffConfig) *ContentDifferBuilder {
	b.diffCfg = cfg
	return b
}

// Build creates a new ContentDiffer instance
func (b *ContentDifferBuilder) Build() (*ContentDiffer, error) {
	if b.diffConfig == nil {
		return nil, common.NewValidationError("diff_config", b.diffConfig, "diff reporter config cannot be nil")
	}

	return &ContentDiffer{
		processor:       NewDiffProcessor(b.diffCfg),
		sizeValidator:   NewContentSizeValidator(b.diffConfig.MaxDiffFileSizeMB),
		inputValidator:  NewInputValidator(),
		statsCalculator: NewDiffStatsCalculator(),
		maxFileSizeMB:   b.diffConfig.MaxDiffFileSizeMB,
	}, nil
}

// NewContentDiffer creates a new instance of ContentDiffer
func NewContentDiffer(logger zerolog.Logger, diffCfg *config.DiffReporterConfig) (*ContentDiffer, error) {
	return NewContentDifferBuilder().
		WithDiffReporterConfig(diffCfg).
		Build()
}

// GenerateDiff compares two byte slices of content and returns a structured diff result
func (cd *ContentDiffer) GenerateDiff(previousContent []byte, currentContent []byte, contentType string, oldHash string, newHash string) (*models.ContentDiffResult, error) {
	startTime := time.Now()

	if err := cd.validateInputs(previousContent, currentContent, contentType); err != nil {
		if cd.isContentTooLargeError(err) {
			return cd.createTooLargeResult(previousContent, currentContent, contentType, oldHash, newHash, time.Since(startTime)), nil
		}
		return nil, common.WrapError(err, "failed to validate diff inputs")
	}

	return cd.processDiff(previousContent, currentContent, contentType, oldHash, newHash, startTime)
}

// validateInputs validates the input parameters for diff generation
func (cd *ContentDiffer) validateInputs(previousContent, currentContent []byte, contentType string) error {
	if err := cd.inputValidator.ValidateInputs(contentType); err != nil {
		return err
	}

	return cd.sizeValidator.ValidateSize(previousContent, currentContent)
}

// isContentTooLargeError checks if the error is due to content being too large
func (cd *ContentDiffer) isContentTooLargeError(err error) bool {
	if validationErr, ok := err.(*common.ValidationError); ok {
		return validationErr.Field == "previous_content" || validationErr.Field == "current_content"
	}
	return false
}

// processDiff processes the diff and returns the result
func (cd *ContentDiffer) processDiff(previousContent, currentContent []byte, contentType, oldHash, newHash string, startTime time.Time) (*models.ContentDiffResult, error) {
	text1 := string(previousContent)
	text2 := string(currentContent)

	diffs := cd.processor.ProcessDiff(text1, text2)
	stats := cd.statsCalculator.CalculateStats(diffs, oldHash, newHash)

	result := NewContentDiffResultBuilder().
		WithContentType(contentType).
		WithHashes(oldHash, newHash).
		WithDiffs(diffs, stats).
		WithProcessingTime(time.Since(startTime)).
		Build()

	return result, nil
}

// createTooLargeResult creates a result for content that's too large to diff
func (cd *ContentDiffer) createTooLargeResult(previousContent, currentContent []byte, contentType, oldHash, newHash string, processingTime time.Duration) *models.ContentDiffResult {
	errorMsg := fmt.Sprintf("Content changed, but file is too large for detailed diff (limit: %dMB). Previous size: %d bytes, Current size: %d bytes.",
		cd.maxFileSizeMB, len(previousContent), len(currentContent))

	return NewContentDiffResultBuilder().
		WithContentType(contentType).
		WithHashes(oldHash, newHash).
		WithError(errorMsg).
		WithProcessingTime(processingTime).
		Build()
}
