package secrets

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/datastore"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/notifier"

	"github.com/rs/zerolog"
)

// SecretDetectorService is responsible for orchestrating the detection of secrets in content.
type SecretDetectorService struct {
	secretsConfig      *config.SecretsConfig        // Configuration for secret detection.
	secretsStore       datastore.SecretsStore       // Interface for storing found secrets.
	logger             zerolog.Logger               // Logger instance for logging.
	notificationHelper *notifier.NotificationHelper // Helper for sending notifications.
	trufflehogAdapter  *TruffleHogAdapter           // Adapter for TruffleHog.
	regexScanner       *RegexScanner                // Scanner for custom regex patterns.
}

// NewSecretDetectorService creates a new instance of SecretDetectorService.
func NewSecretDetectorService(
	cfg *config.GlobalConfig, // Changed to GlobalConfig to access SecretsConfig and other potential future configs
	store datastore.SecretsStore,
	log zerolog.Logger,
	notifyHelper *notifier.NotificationHelper,
) (*SecretDetectorService, error) { // Added error return

	rs, err := NewRegexScanner(&cfg.SecretsConfig, log)
	if err != nil {
		// Log the error but allow service creation if regex scanner fails to init (e.g. bad custom patterns file)
		// The service can still potentially run TrufflHog or other detectors.
		log.Error().Err(err).Msg("Failed to initialize RegexScanner in SecretDetectorService, regex scanning will be disabled for this instance.")
		rs = nil // Ensure regexScanner is nil if initialization failed
	}

	return &SecretDetectorService{
		secretsConfig:      &cfg.SecretsConfig,
		secretsStore:       store,
		logger:             log.With().Str("service", "SecretDetectorService").Logger(),
		notificationHelper: notifyHelper,
		trufflehogAdapter:  NewTruffleHogAdapter(&cfg.SecretsConfig, log),
		regexScanner:       rs, // Assign the initialized (or nil) regex scanner
	}, nil
}

// ScanContent scans the provided content for secrets using configured detection methods.
// It returns a slice of SecretFinding and any error encountered.
func (s *SecretDetectorService) ScanContent(sourceURL string, content []byte, contentType string) ([]models.SecretFinding, error) {
	s.logger.Debug().Str("sourceURL", sourceURL).Str("contentType", contentType).Int("contentLength", len(content)).Msg("Starting secret detection scan")

	var allFindings []models.SecretFinding
	scanStartTime := time.Now()

	// Check file size limit
	if s.secretsConfig.MaxFileSizeToScanMB > 0 {
		fileSizeMB := float64(len(content)) / (1024 * 1024)
		if fileSizeMB > float64(s.secretsConfig.MaxFileSizeToScanMB) {
			s.logger.Warn().Float64("file_size_mb", fileSizeMB).Int("max_size_mb", s.secretsConfig.MaxFileSizeToScanMB).Str("source_url", sourceURL).Msg("File too large for secret scanning, skipping")
			return nil, nil
		}
	}

	// Run TruffleHog detection if enabled
	if s.secretsConfig.EnableTruffleHog && s.trufflehogAdapter != nil {
		s.logger.Debug().Str("source_url", sourceURL).Msg("Running TruffleHog secret detection")
		startTime := time.Now()
		truffleFindings, err := s.trufflehogAdapter.ScanWithTruffleHog(content, sourceURL)
		s.logger.Debug().Dur("duration", time.Since(startTime)).Int("findings", len(truffleFindings)).Str("source_url", sourceURL).Msg("TruffleHog scan completed")

		if err != nil {
			s.logger.Error().Err(err).Str("source_url", sourceURL).Msg("TruffleHog scan failed")
			// Continue with other detection methods despite TruffleHog failure
		} else {
			allFindings = append(allFindings, truffleFindings...)
		}
	} else {
		s.logger.Debug().Str("source_url", sourceURL).Msg("TruffleHog detection disabled or not available")
	}

	// Run custom regex scanning if enabled
	if s.secretsConfig.EnableCustomRegex && s.regexScanner != nil {
		s.logger.Debug().Str("source_url", sourceURL).Msg("Running custom regex secret detection")
		startTime := time.Now()
		regexFindings, err := s.regexScanner.ScanWithRegexes(content, sourceURL)
		s.logger.Debug().Dur("duration", time.Since(startTime)).Int("findings", len(regexFindings)).Str("source_url", sourceURL).Msg("Custom regex scan completed")

		if err != nil {
			s.logger.Error().Err(err).Str("source_url", sourceURL).Msg("Custom regex scan failed")
			// Continue despite regex scan failure
		} else {
			allFindings = append(allFindings, regexFindings...)
		}
	} else {
		s.logger.Debug().Str("source_url", sourceURL).Msg("Custom regex detection disabled or not available")
	}

	// Deduplicate findings
	deduplicatedFindings := s.deduplicateFindings(allFindings)
	s.logger.Debug().Int("original_count", len(allFindings)).Int("deduplicated_count", len(deduplicatedFindings)).Str("source_url", sourceURL).Msg("Secret findings deduplicated")

	// Store findings
	if len(deduplicatedFindings) > 0 {
		err := s.secretsStore.StoreSecretFindings(deduplicatedFindings)
		if err != nil {
			s.logger.Error().Err(err).Str("source_url", sourceURL).Msg("Failed to store secret findings")
			return deduplicatedFindings, fmt.Errorf("failed to store secret findings: %w", err)
		}
		s.logger.Info().Int("count", len(deduplicatedFindings)).Str("source_url", sourceURL).Msg("Secret findings stored successfully")
	}

	// TODO: Send notification for high-severity secrets if configured
	for _, finding := range deduplicatedFindings {
		if s.secretsConfig.NotifyOnHighSeveritySecret && (finding.Severity == "HIGH" || finding.Severity == "CRITICAL") {
			s.logger.Info().Str("severity", finding.Severity).Str("rule_id", finding.RuleID).Str("source_url", sourceURL).Msg("High-severity secret detected - notification should be sent")
		}
	}

	totalScanDuration := time.Since(scanStartTime)
	if len(deduplicatedFindings) > 0 {
		// Log final summary with severity breakdown
		finalSeverityCount := make(map[string]int)
		highSeverityFindings := 0
		for _, finding := range deduplicatedFindings {
			finalSeverityCount[finding.Severity]++
			if strings.ToLower(finding.Severity) == "high" || strings.ToLower(finding.Severity) == "critical" {
				highSeverityFindings++
			}
		}

		// Send notifications for high-severity secrets if enabled
		if s.secretsConfig.NotifyOnHighSeveritySecret && s.notificationHelper != nil && highSeverityFindings > 0 {
			s.logger.Info().Int("high_severity_count", highSeverityFindings).Msg("Sending notifications for high-severity secret findings")

			// Send notification for each high-severity finding
			for _, finding := range deduplicatedFindings {
				severity := strings.ToLower(finding.Severity)
				if severity == "high" || severity == "critical" {
					// Use a background context with timeout for notifications to avoid blocking the scan
					notificationCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					s.notificationHelper.SendHighSeveritySecretNotification(notificationCtx, finding, notifier.MonitorServiceNotification)
					cancel()
				}
			}
		}

		s.logger.Info().
			Int("total_findings", len(deduplicatedFindings)).
			Int("high_severity_findings", highSeverityFindings).
			Interface("severity_breakdown", finalSeverityCount).
			Str("sourceURL", sourceURL).
			Dur("total_duration", totalScanDuration).
			Msg("Secret detection scan completed with findings")
	} else {
		s.logger.Info().Str("sourceURL", sourceURL).Dur("total_duration", totalScanDuration).Msg("Secret detection scan completed with no findings")
	}

	return deduplicatedFindings, nil
}

// deduplicateFindings removes duplicate secret findings.
// A simple deduplication strategy based on RuleID, SecretText, and LineNumber.
// More sophisticated strategies might consider location context or fuzzy matching if needed.
func (s *SecretDetectorService) deduplicateFindings(findings []models.SecretFinding) []models.SecretFinding {
	if len(findings) == 0 {
		return findings
	}

	seen := make(map[string]bool)
	uniqueFindings := make([]models.SecretFinding, 0, len(findings))

	// Sort for consistent deduplication, though map iteration order isn't guaranteed relevant here
	// if the primary key for `seen` is robust enough.
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].SourceURL != findings[j].SourceURL {
			return findings[i].SourceURL < findings[j].SourceURL
		}
		if findings[i].RuleID != findings[j].RuleID {
			return findings[i].RuleID < findings[j].RuleID
		}
		if findings[i].LineNumber != findings[j].LineNumber {
			return findings[i].LineNumber < findings[j].LineNumber
		}
		return findings[i].SecretText < findings[j].SecretText
	})

	for _, f := range findings {
		// Create a unique key for the finding
		// Using a combination of critical fields to identify a unique secret instance.
		// Note: SecretText itself can be long, consider hashing if performance becomes an issue
		// or if there are concerns about storing/comparing full secret text even temporarily.
		findingKey := fmt.Sprintf("%s|%s|%d|%s", f.SourceURL, f.RuleID, f.LineNumber, f.SecretText)
		if !seen[findingKey] {
			seen[findingKey] = true
			uniqueFindings = append(uniqueFindings, f)
		}
	}
	return uniqueFindings
}
