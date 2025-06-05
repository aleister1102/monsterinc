package extractor

import (
	"strings"

	"github.com/rs/zerolog"
)

// ContextExtractor handles extraction of context snippets
type ContextExtractor struct {
	logger      zerolog.Logger
	snippetSize int
}

// NewContextExtractor creates a new context extractor
func NewContextExtractor(snippetSize int, logger zerolog.Logger) *ContextExtractor {
	return &ContextExtractor{
		logger:      logger.With().Str("component", "ContextExtractor").Logger(),
		snippetSize: snippetSize,
	}
}

// ExtractContext extracts context around a match in the content
func (ce *ContextExtractor) ExtractContext(contentStr string, match string) string {
	matchStartIndex := strings.Index(contentStr, match)
	if matchStartIndex == -1 {
		ce.logger.Debug().Str("match", match).Msg("Match not found in content")
		return ""
	}

	start := matchStartIndex - ce.snippetSize
	if start < 0 {
		start = 0
	}

	end := matchStartIndex + len(match) + ce.snippetSize
	if end > len(contentStr) {
		end = len(contentStr)
	}

	context := contentStr[start:end]
	ce.logger.Debug().Str("match", match).Int("context_length", len(context)).Msg("Extracted context")

	return context
}
