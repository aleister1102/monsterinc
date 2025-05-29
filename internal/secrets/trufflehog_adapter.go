package secrets

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"

	"github.com/rs/zerolog"
)

// TruffleHogAdapter is responsible for interacting with the TruffleHog tool via CLI.
type TruffleHogAdapter struct {
	config *config.SecretsConfig
	logger zerolog.Logger
}

// NewTruffleHogAdapter creates a new TruffleHogAdapter.
func NewTruffleHogAdapter(cfg *config.SecretsConfig, log zerolog.Logger) *TruffleHogAdapter {
	return &TruffleHogAdapter{
		config: cfg,
		logger: log.With().Str("adapter", "TruffleHogAdapter").Logger(),
	}
}

// TruffleHogFindingV3 defines the structure for a single finding from TruffleHog v3 JSONL output.
type TruffleHogFindingV3 struct {
	SourceMetadata struct {
		Data struct {
			Git *struct { // Git specific metadata
				Commit     string    `json:"commit"`
				File       string    `json:"file"`
				Email      string    `json:"email"`
				Repository string    `json:"repository"`
				Timestamp  time.Time `json:"timestamp"`
				Line       int       `json:"line"`
			} `json:"Git"`
			File *struct { // Filesystem specific metadata
				Path string `json:"path"`
				Line int    `json:"line"`
			} `json:"File"`
			// Other source types like S3, GitHub, etc. can be added here
		} `json:"Data"`
	} `json:"SourceMetadata"`
	SourceID       int                    `json:"SourceID"`
	SourceType     int                    `json:"SourceType"`
	SourceName     string                 `json:"SourceName"`
	DetectorType   int                    `json:"DetectorType"`
	DetectorName   string                 `json:"DetectorName"`
	DecoderName    string                 `json:"DecoderName"`
	Verified       bool                   `json:"Verified"`
	Raw            string                 `json:"Raw"`
	Redacted       string                 `json:"Redacted"`
	ExtraData      map[string]interface{} `json:"ExtraData"`      // Catch-all for other fields
	StructuredData interface{}            `json:"StructuredData"` // Can be any structure based on detector
	RuleName       string                 `json:"RuleName"`       // Often present, good for description
}

// ScanWithTruffleHog executes TruffleHog CLI and parses its JSONL output.
func (a *TruffleHogAdapter) ScanWithTruffleHog(content []byte, filenameHint string) ([]models.SecretFinding, error) {
	a.logger.Debug().Str("filenameHint", filenameHint).Int("content_length", len(content)).Msg("Scanning with TruffleHog CLI")

	if a.config.TruffleHogPath == "" {
		return nil, fmt.Errorf("TruffleHog path is not configured")
	}

	// Create a temporary file to scan
	tmpFile, err := os.CreateTemp("", "trufflehog_scan_*.tmp")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file for TruffleHog: %w", err)
	}
	defer os.Remove(tmpFile.Name()) // Clean up the temp file

	if _, err := tmpFile.Write(content); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("failed to write content to temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("failed to close temp file: %w", err)
	}

	// Prepare TruffleHog command
	cmd := exec.Command(a.config.TruffleHogPath, "filesystem", tmpFile.Name(), "--json")

	if a.config.TruffleHogNoVerification {
		cmd.Args = append(cmd.Args, "--no-verification")
	}

	a.logger.Debug().Str("command", cmd.String()).Msg("Executing TruffleHog")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.config.TruffleHogTimeoutSeconds)*time.Second)
	defer cancel()

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	if err != nil {
		// Check if it's a timeout or other context error
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("trufflehog scan timed out after %d seconds", a.config.TruffleHogTimeoutSeconds)
		}
		a.logger.Warn().Err(err).Str("stderr", stderrBuf.String()).Msg("TruffleHog command failed, but will still parse stdout")
	}

	var findings []models.SecretFinding
	scanner := bufio.NewScanner(&stdoutBuf)
	lineNumberInOutput := 0
	for scanner.Scan() {
		lineNumberInOutput++
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue // Skip empty lines
		}

		var thFinding TruffleHogFindingV3
		if err := json.Unmarshal(line, &thFinding); err != nil {
			a.logger.Error().Err(err).Str("line_content", string(line)).Int("output_line", lineNumberInOutput).Msg("Failed to unmarshal TruffleHog JSONL line")
			continue // Skip this finding
		}

		// Determine line number from finding data
		var secretLineNumber int
		if thFinding.SourceMetadata.Data.File != nil { // Filesystem source
			secretLineNumber = thFinding.SourceMetadata.Data.File.Line
		} else if thFinding.SourceMetadata.Data.Git != nil { // Git source (less likely for direct content scan but handle if CLI outputs it)
			secretLineNumber = thFinding.SourceMetadata.Data.Git.Line
		}

		finding := models.SecretFinding{
			SourceURL:         filenameHint,   // Use the original URL/hint as the source
			FilePathInArchive: tmpFile.Name(), // Path of the temp file scanned
			RuleID:            thFinding.DetectorName,
			Description:       fmt.Sprintf("TruffleHog: %s (Rule: %s)", thFinding.DetectorName, thFinding.RuleName),
			Severity:          mapTruffleHogSeverity(thFinding.Verified, thFinding.DetectorName, thFinding.RuleName),
			SecretText:        thFinding.Raw, // Store full secret without truncation
			LineNumber:        secretLineNumber,
			Timestamp:         time.Now(),
			ToolName:          "TruffleHog",
			VerificationState: func() string {
				if thFinding.Verified {
					return "Verified"
				}
				return "Unverified"
			}(),
		}
		// Convert thFinding.ExtraData map[string]interface{} to JSON string
		if thFinding.ExtraData != nil {
			extraDataBytes, err := json.Marshal(thFinding.ExtraData)
			if err != nil {
				a.logger.Warn().Err(err).Msg("Failed to marshal ExtraData to JSON, skipping")
			} else {
				finding.ExtraDataJSON = string(extraDataBytes)
			}
		}

		findings = append(findings, finding)
	}

	if err := scanner.Err(); err != nil {
		a.logger.Error().Err(err).Msg("Error reading TruffleHog stdout")
		// Return findings parsed so far, along with the scanner error.
		return findings, fmt.Errorf("error reading TruffleHog stdout: %w", err)
	}

	a.logger.Debug().Int("findings_count", len(findings)).Msg("TruffleHog CLI scan complete.")
	return findings, nil
}

// mapTruffleHogSeverity maps TruffleHog verification status and detector name to an internal severity string.
func mapTruffleHogSeverity(verified bool, detectorName string, ruleName string) string {
	// Normalize detector and rule names for consistent matching
	lowerDetectorName := strings.ToLower(detectorName)
	lowerRuleName := strings.ToLower(ruleName)

	if verified {
		// High confidence if verified by TruffleHog
		if strings.Contains(lowerDetectorName, "privatekey") || strings.Contains(lowerRuleName, "privatekey") ||
			strings.Contains(lowerDetectorName, "pkcs") || strings.Contains(lowerRuleName, "pkcs") {
			return "CRITICAL"
		}
		if strings.Contains(lowerDetectorName, "aws") || strings.Contains(lowerDetectorName, "gcp") || strings.Contains(lowerDetectorName, "azure") ||
			strings.Contains(lowerDetectorName, "github") || strings.Contains(lowerRuleName, "github") {
			return "CRITICAL"
		}
		return "HIGH" // Default for other verified secrets
	}

	// Non-verified, severity based on detector/rule name patterns
	// Critical keywords for unverified
	if strings.Contains(lowerDetectorName, "privatekey") || strings.Contains(lowerRuleName, "privatekey") ||
		strings.Contains(lowerDetectorName, "pkcs") || strings.Contains(lowerRuleName, "pkcs") ||
		strings.Contains(lowerDetectorName, "github_pat") || strings.Contains(lowerRuleName, "github_pat") || // Personal Access Token
		strings.Contains(lowerDetectorName, "github_app_token") || strings.Contains(lowerRuleName, "github_app_token") {
		return "CRITICAL"
	}

	// High severity keywords
	if strings.Contains(lowerDetectorName, "aws") || strings.Contains(lowerRuleName, "aws_access_key") ||
		strings.Contains(lowerDetectorName, "gcp") || strings.Contains(lowerRuleName, "google_api_key") ||
		strings.Contains(lowerDetectorName, "azure") ||
		strings.Contains(lowerDetectorName, "slack") || strings.Contains(lowerRuleName, "slack_token") ||
		strings.Contains(lowerDetectorName, "stripe") || strings.Contains(lowerRuleName, "stripe_api_key") ||
		strings.Contains(lowerDetectorName, "jwt") || strings.Contains(lowerRuleName, "json_web_token") ||
		strings.Contains(lowerDetectorName, "bearer") ||
		strings.Contains(lowerDetectorName, "secret_key") || strings.Contains(lowerRuleName, "api_key") { // More generic API key
		return "HIGH"
	}

	// Medium severity keywords
	if strings.Contains(lowerDetectorName, "generic") || // Generic detectors might be medium
		strings.Contains(lowerDetectorName, "password") && !strings.Contains(lowerRuleName, "example") || // Password not in an example
		strings.Contains(lowerDetectorName, "token") && !strings.Contains(lowerRuleName, "example") { // Token not in an example
		return "MEDIUM"
	}

	// Default for less common or less certain detectors without verification
	return "LOW"
}

// firstNLines returns the first N lines of a string, for cleaner logging.
func firstNLines(s string, n int) string {
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(s))
	for i := 0; i < n && scanner.Scan(); i++ {
		lines = append(lines, scanner.Text())
	}
	return strings.Join(lines, "\\n")
}
