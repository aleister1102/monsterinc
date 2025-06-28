package crawler

import (
	"github.com/aleister1102/monsterinc/internal/common/errorwrapper"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/rs/zerolog"
)

// CrawlerBuilder provides a fluent interface for creating Crawler instances
type CrawlerBuilder struct {
	config *config.CrawlerConfig
	logger zerolog.Logger
}

// NewCrawlerBuilder creates a new CrawlerBuilder instance
func NewCrawlerBuilder(logger zerolog.Logger) *CrawlerBuilder {
	return &CrawlerBuilder{
		logger: logger.With().Str("module", "Crawler").Logger(),
	}
}

// WithConfig sets the crawler configuration
func (cb *CrawlerBuilder) WithConfig(cfg *config.CrawlerConfig) *CrawlerBuilder {
	cb.config = cfg
	return cb
}

// Build creates a new Crawler instance with the configured settings
func (cb *CrawlerBuilder) Build() (*Crawler, error) {
	if cb.config == nil {
		return nil, errorwrapper.NewValidationError("config", nil, "crawler config cannot be nil")
	}

	crawler := &Crawler{
		discoveredURLs: make(map[string]bool),
		urlParentMap:   make(map[string]string),
		logger:         cb.logger,
		config:         cb.config,
	}

	if err := crawler.initialize(); err != nil {
		return nil, errorwrapper.WrapError(err, "failed to initialize crawler")
	}

	return crawler, nil
}
