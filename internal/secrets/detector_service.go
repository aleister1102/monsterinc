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
	s.logger.Info().Str("sourceURL", sourceURL).Str("contentType", contentType).Int("contentLength", len(content)).Msg("Starting secret detection scan")

	var allFindings []models.SecretFinding
	scanStartTime := time.Now()

	// Implement file size check based on s.secretsConfig.MaxFileSizeToScanMB (Task 7.1)
	if s.secretsConfig.MaxFileSizeToScanMB > 0 && len(content) > s.secretsConfig.MaxFileSizeToScanMB*1024*1024 {
		s.logger.Warn().Str("sourceURL", sourceURL).Int("contentLength", len(content)).Int("maxSizeMB", s.secretsConfig.MaxFileSizeToScanMB).Msg("Content exceeds max file size for secret scanning, skipping.")
		return nil, fmt.Errorf("content size %d bytes exceeds max limit of %d MB for secret scanning", len(content), s.secretsConfig.MaxFileSizeToScanMB)
	}

	// Scan with TruffleHog if enabled
	if s.secretsConfig.EnableTruffleHog && s.trufflehogAdapter != nil {
		start := time.Now()
		truffleHogFindings, err := s.trufflehogAdapter.ScanWithTruffleHog(content, sourceURL)
		duration := time.Since(start)

		if err != nil {
			// Check if it's an updater-related error - less critical
			errStr := err.Error()
			isUpdaterError := strings.Contains(errStr, "updater failed") ||
				strings.Contains(errStr, "cmd") ||
				strings.Contains(errStr, "cannot move binary")

			if isUpdaterError {
				s.logger.Warn().Err(err).Str("sourceURL", sourceURL).Dur("duration", duration).
					Msg("TruffleHog scan failed due to updater issues, continuing with regex scanning only")
				// Continue without TruffleHog findings, don't fail the entire scan
			} else {
				s.logger.Error().Err(err).Str("sourceURL", sourceURL).Dur("duration", duration).
					Msg("TruffleHog scan failed")
				// For other errors, still continue but log as error
			}
		} else {
			s.logger.Info().Int("findings", len(truffleHogFindings)).Str("sourceURL", sourceURL).
				Dur("duration", duration).Msg("TruffleHog scan completed")
			allFindings = append(allFindings, truffleHogFindings...)
		}
	}

	// Call custom regex scanner if s.secretsConfig.EnableCustomRegex (Task 3 & 4.1)
	if s.secretsConfig.Enabled && s.secretsConfig.EnableCustomRegex && s.regexScanner != nil {
		s.logger.Debug().Str("sourceURL", sourceURL).Msg("Custom regex scanning enabled, starting scan")
		regexStartTime := time.Now()

		regexFindings, err := s.regexScanner.ScanWithRegexes(content, sourceURL)
		regexDuration := time.Since(regexStartTime)

		if err != nil {
			s.logger.Error().Err(err).Str("sourceURL", sourceURL).Dur("duration", regexDuration).Msg("Custom regex scan failed")
		} else {
			if len(regexFindings) > 0 {
				s.logger.Info().Int("count", len(regexFindings)).Str("sourceURL", sourceURL).Dur("duration", regexDuration).Msg("Custom regex scan completed with findings")
				allFindings = append(allFindings, regexFindings...)

				// Log severity breakdown for regex findings
				severityCount := make(map[string]int)
				ruleCount := make(map[string]int)
				for _, finding := range regexFindings {
					severityCount[finding.Severity]++
					ruleCount[finding.RuleID]++
				}
				s.logger.Debug().Interface("severity_breakdown", severityCount).Interface("rule_breakdown", ruleCount).Str("tool", "CustomRegex").Msg("Custom regex findings breakdown")
			} else {
				s.logger.Debug().Str("sourceURL", sourceURL).Dur("duration", regexDuration).Msg("Custom regex scan completed with no findings")
			}
		}
	} else {
		s.logger.Debug().Bool("secrets_enabled", s.secretsConfig.Enabled).Bool("regex_enabled", s.secretsConfig.EnableCustomRegex).Bool("scanner_available", s.regexScanner != nil).Msg("Custom regex scanning is disabled or scanner not initialized")
	}

	// Combine and deduplicate findings (Task 4.1)
	deduplicationStartTime := time.Now()
	deduplicatedFindings := s.deduplicateFindings(allFindings)
	deduplicationDuration := time.Since(deduplicationStartTime)

	s.logger.Debug().Int("original_count", len(allFindings)).Int("deduplicated_count", len(deduplicatedFindings)).Dur("deduplication_duration", deduplicationDuration).Msg("Finished deduplicating findings")

	// Store findings using s.secretsStore (Task 4.3)
	if s.secretsStore != nil && len(deduplicatedFindings) > 0 {
		storeStartTime := time.Now()
		if err := s.secretsStore.StoreSecretFindings(deduplicatedFindings); err != nil {
			storeDuration := time.Since(storeStartTime)
			s.logger.Error().Err(err).Dur("duration", storeDuration).Msg("Failed to store secret findings")
			// Depending on policy, this might be a critical error or just logged.
			// For now, we log it and the scan itself is still considered "successful" in terms of detection.
		} else {
			storeDuration := time.Since(storeStartTime)
			s.logger.Info().Int("count", len(deduplicatedFindings)).Dur("duration", storeDuration).Msg("Successfully stored secret findings")
		}
	} else if len(deduplicatedFindings) > 0 {
		s.logger.Warn().Msg("SecretsStore is not initialized, cannot store findings")
	}

	// TODO: Send notification for high-severity secrets if s.secretsConfig.NotifyOnHighSeveritySecret (Task 5.3)

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
