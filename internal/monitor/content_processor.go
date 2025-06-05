package monitor

import (
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/aleister1102/monsterinc/internal/models"

	"github.com/rs/zerolog"
)

// ContentProcessor handles processing of fetched file content, like hashing.
type ContentProcessor struct {
	logger zerolog.Logger
}

// NewContentProcessor creates a new ContentProcessor.
func NewContentProcessor(logger zerolog.Logger) *ContentProcessor {
	return &ContentProcessor{
		logger: logger.With().Str("component", "ContentProcessor").Logger(),
	}
}

// ProcessContent takes the file content and processes it.
// Currently, it calculates a SHA256 hash of the content.
// It returns a MonitoredFileUpdate struct with the new hash and other relevant info.
func (cp *ContentProcessor) ProcessContent(url string, content []byte, contentType string) (*models.MonitoredFileUpdate, error) {
	hash := sha256.Sum256(content)
	hashStr := fmt.Sprintf("%x", hash)

	return &models.MonitoredFileUpdate{
		URL:         url,
		NewHash:     hashStr,
		ContentType: contentType,
		FetchedAt:   time.Now(),
		Content:     content, // Pass content through; service layer decides if it's stored
	}, nil
}
