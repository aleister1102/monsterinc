package secretscanner

import (
	"context"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/datastore"
	"github.com/aleister1102/monsterinc/internal/notifier"
	"github.com/rs/zerolog"
)

// StatsCallback is an interface for reporting secret scanning statistics.
type StatsCallback interface {
	OnSecretFound(count int64)
}

// Detector orchestrates the secret scanning process.
type Detector struct {
	config        *config.SecretsConfig
	logger        zerolog.Logger
	scanner       *RegexScanner
	store         *datastore.SecretsStore
	notifier      notifier.Notifier
	statsCallback StatsCallback
}

// NewDetector creates a new Detector service.
func NewDetector(
	cfg *config.SecretsConfig,
	store *datastore.SecretsStore,
	ntf notifier.Notifier,
	logger zerolog.Logger,
) (*Detector, error) {
	return &Detector{
		config:   cfg,
		logger:   logger.With().Str("module", "SecretDetector").Logger(),
		scanner:  NewRegexScanner(),
		store:    store,
		notifier: ntf,
	}, nil
}

// SetStatsCallback sets the callback for reporting statistics.
func (d *Detector) SetStatsCallback(callback StatsCallback) {
	d.statsCallback = callback
}

// ScanAndProcess orchestrates scanning, storage, and notification.
func (d *Detector) ScanAndProcess(sourceURL string, content []byte) {
	if !d.config.Enabled {
		return
	}

	d.logger.Debug().Str("url", sourceURL).Msg("Scanning for secrets")
	findings := d.scanner.Scan(sourceURL, content)

	if len(findings) == 0 {
		return
	}

	d.logger.Info().Int("count", len(findings)).Str("url", sourceURL).Msg("Found secrets")

	if d.statsCallback != nil {
		d.statsCallback.OnSecretFound(int64(len(findings)))
	}

	// Store findings
	if err := d.store.StoreFindings(context.Background(), findings); err != nil {
		d.logger.Error().Err(err).Msg("Failed to store secret findings")
	}

	// Notify if configured
	if d.config.NotifyOnFound && d.notifier != nil {
		// This is a placeholder for the actual notification logic.
		// We'd need to format the findings into a DiscordMessagePayload.
		// For now, just log that we would notify.
		d.logger.Info().Msg("Notification for secrets would be sent here.")
		// d.notifier.Notify(message)
	}
}
