package differ

import (
	"fmt"

	"github.com/aleister1102/monsterinc/internal/common"
)

// ContentSizeValidator validates content size against limits
type ContentSizeValidator struct {
	maxSizeBytes int64
}

// NewContentSizeValidator creates a new content size validator
func NewContentSizeValidator(maxSizeMB int) *ContentSizeValidator {
	return &ContentSizeValidator{
		maxSizeBytes: int64(maxSizeMB) * 1024 * 1024,
	}
}

// ValidateSize checks if content sizes are within limits
func (csv *ContentSizeValidator) ValidateSize(previousContent, currentContent []byte) error {
	if err := csv.validateSingleContent(previousContent, "previous_content"); err != nil {
		return err
	}

	return csv.validateSingleContent(currentContent, "current_content")
}

// validateSingleContent validates a single content size
func (csv *ContentSizeValidator) validateSingleContent(content []byte, fieldName string) error {
	if int64(len(content)) > csv.maxSizeBytes {
		return common.NewValidationError(fieldName, len(content),
			fmt.Sprintf("%s too large (%d bytes > %d bytes limit)",
				fieldName, len(content), csv.maxSizeBytes))
	}
	return nil
}

// InputValidator validates diff generation inputs
type InputValidator struct{}

// NewInputValidator creates a new input validator
func NewInputValidator() *InputValidator {
	return &InputValidator{}
}

// ValidateInputs validates the input parameters for diff generation
func (iv *InputValidator) ValidateInputs(contentType string) error {
	if contentType == "" {
		return common.NewValidationError("content_type", contentType, "content type cannot be empty")
	}
	return nil
}
