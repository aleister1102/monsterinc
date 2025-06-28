package crawler

import (
	"github.com/aleister1102/monsterinc/internal/common/errors"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/notifier"
	"github.com/rs/zerolog"
)

// CrawlerBuilder provides a fluent interface for creating Crawler instances
type CrawlerBuilder struct {
	config   *config.CrawlerConfig
	logger   zerolog.Logger
	notifier notifier.Notifier
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

// WithNotifier sets the notifier for alerts
func (cb *CrawlerBuilder) WithNotifier(notifier notifier.Notifier) *CrawlerBuilder {
	cb.notifier = notifier
	return cb
}

// Build creates a new Crawler instance with the configured settings
func (cb *CrawlerBuilder) Build() (*Crawler, error) {
	if cb.config == nil {
		return nil, errors.NewValidationError("config", nil, "crawler config cannot be nil")
	}

	crawler := &Crawler{
		discoveredURLs: make(map[string]bool),
		urlParentMap:   make(map[string]string),
		logger:         cb.logger,
		config:         cb.config,
	}

	if err := crawler.initialize(); err != nil {
		return nil, errors.WrapError(err, "failed to initialize crawler")
	}

	return crawler, nil
}
