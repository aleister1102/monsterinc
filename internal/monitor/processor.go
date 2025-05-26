package monitor

import (
	"crypto/sha256"
	"fmt"
	"monsterinc/internal/models" // Assuming MonitoredFileUpdate is here
	"time"

	"github.com/rs/zerolog"
)

// Processor handles processing of fetched file content, like hashing.
type Processor struct {
	logger zerolog.Logger
	// Add any dependencies if needed, e.g., config for hash algorithm choice
}

// NewProcessor creates a new Processor.
func NewProcessor(logger zerolog.Logger) *Processor {
	return &Processor{
		logger: logger.With().Str("component", "Processor").Logger(),
	}
}

// ProcessContent takes the file content and processes it.
// Currently, it calculates a SHA256 hash of the content.
// It returns a MonitoredFileUpdate struct with the new hash and other relevant info.
func (p *Processor) ProcessContent(url string, content []byte, contentType string) (*models.MonitoredFileUpdate, error) {
	if content == nil {
		// This case might occur if FetchFileContent returns no error but empty content for some reason (e.g. 304 handled by caller but content still passed)
		// Or if a file is genuinely empty. For hashing, an empty file has a valid hash.
		p.logger.Debug().Str("url", url).Msg("Processing nil/empty content.")
		// Decide if an error should be returned or proceed with hashing empty content
	}

	hash := sha256.Sum256(content)
	hashStr := fmt.Sprintf("%x", hash)

	fetchedAt := time.Now()

	update := &models.MonitoredFileUpdate{
		URL:         url,
		NewHash:     hashStr,
		ContentType: contentType,
		FetchedAt:   fetchedAt,
		Content:     content, // Pass content through; service layer decides if it's stored
	}

	p.logger.Debug().Str("url", url).Str("hash", hashStr).Msg("Content processed, hash calculated.")
	return update, nil
}
