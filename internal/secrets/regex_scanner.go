package secrets

import (
	"bufio"
	"bytes"
	"embed"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"time"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

//go:embed patterns.yaml
var embeddedPatterns embed.FS

// RegexScanner is responsible for scanning content using a list of regex patterns.
type RegexScanner struct {
	config   *config.SecretsConfig
	logger   zerolog.Logger
	patterns []RegexPattern
}

// NewRegexScanner creates a new RegexScanner with compiled patterns.
func NewRegexScanner(cfg *config.SecretsConfig, logger zerolog.Logger) (*RegexScanner, error) {
	scanner := &RegexScanner{
		config:   cfg,
		logger:   logger.With().Str("component", "RegexScanner").Logger(),
		patterns: []RegexPattern{},
	}

	// Load default patterns first
	defaultPatterns, err := DefaultRegexPatterns()
	if err != nil {
		scanner.logger.Error().Err(err).Msg("Failed to load default regex patterns")
		return nil, fmt.Errorf("failed to load default regex patterns: %w", err)
	}
	scanner.patterns = append(scanner.patterns, defaultPatterns...)
	scanner.logger.Debug().Int("count", len(defaultPatterns)).Msg("Loaded default regex patterns")

	// Load custom regex patterns from file if specified
	if cfg.CustomRegexPatternsFile != "" {
		customPatterns, customErr := loadCustomPatternsFromFile(cfg.CustomRegexPatternsFile, scanner.logger)
		if customErr != nil {
			scanner.logger.Error().Err(customErr).Str("file", cfg.CustomRegexPatternsFile).Msg("Failed to load custom regex patterns")
		} else if len(customPatterns) > 0 {
			scanner.patterns = append(scanner.patterns, customPatterns...)
			scanner.logger.Debug().Int("count", len(customPatterns)).Str("file", cfg.CustomRegexPatternsFile).Msg("Loaded custom regex patterns from file")
		}
	}

	// Load Mantra patterns
	mantraPatterns, mantraErr := loadMantraPatternsFromFile("internal/secrets/patterns.yaml", scanner.logger)
	if mantraErr != nil {
		scanner.logger.Error().Err(mantraErr).Msg("Failed to load Mantra patterns")
	} else if len(mantraPatterns) > 0 {
		scanner.patterns = append(scanner.patterns, mantraPatterns...)
		scanner.logger.Debug().Int("count", len(mantraPatterns)).Msg("Loaded Mantra regex patterns")
	}

	// TODO: Load additional patterns from config - compilePatterns function needs to be implemented
	// if len(cfg.DefaultRegexPatterns) > 0 {
	//     configPatterns, configErr := compilePatterns(cfg.DefaultRegexPatterns, scanner.logger)
	//     if configErr != nil {
	//         scanner.logger.Error().Err(configErr).Msg("Failed to compile config regex patterns")
	//     } else {
	//         scanner.patterns = append(scanner.patterns, configPatterns...)
	//         scanner.logger.Debug().Int("count", len(configPatterns)).Msg("Loaded config regex patterns")
	//     }
	// }

	// Compile all patterns
	for i := range scanner.patterns {
		compiled, err := compileRegex(scanner.patterns[i].Pattern)
		if err != nil {
			scanner.logger.Error().Err(err).Str("rule_id", scanner.patterns[i].RuleID).Str("pattern", scanner.patterns[i].Pattern).Msg("Failed to compile regex pattern")
			return nil, fmt.Errorf("failed to compile pattern %s: %w", scanner.patterns[i].RuleID, err)
		}
		scanner.patterns[i].Compiled = compiled
	}

	scanner.logger.Debug().Int("total_patterns", len(scanner.patterns)).Msg("RegexScanner initialized successfully")
	return scanner, nil
}

// loadCustomPatternsFromFile loads regex patterns from a YAML or JSON file.
func loadCustomPatternsFromFile(filePath string, logger zerolog.Logger) ([]RegexPattern, error) {
	logger.Debug().Str("file", filePath).Msg("Loading custom regex patterns from file")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read custom patterns file %s: %w", filePath, err)
	}

	var customPatterns []RegexPattern
	err = yaml.Unmarshal(data, &customPatterns)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal custom patterns file %s as YAML: %w", filePath, err)
	}

	// Compile loaded custom patterns
	for i := range customPatterns {
		if customPatterns[i].Pattern == "" {
			logger.Warn().Str("rule_id", customPatterns[i].RuleID).Msg("Custom pattern has empty regex, skipping")
			continue // Skip invalid pattern
		}
		compiledRegex, err := compileRegex(customPatterns[i].Pattern) // Using existing compileRegex helper
		if err != nil {
			logger.Error().Err(err).Str("rule_id", customPatterns[i].RuleID).Str("pattern", customPatterns[i].Pattern).Msg("Failed to compile custom regex pattern, skipping")
			continue // Skip invalid pattern
		}
		customPatterns[i].Compiled = compiledRegex
	}
	// Filter out patterns that failed to compile or were empty
	validPatterns := make([]RegexPattern, 0, len(customPatterns))
	for _, p := range customPatterns {
		if p.Compiled != nil {
			validPatterns = append(validPatterns, p)
		}
	}

	logger.Debug().Int("loaded_count", len(validPatterns)).Str("file", filePath).Msg("Successfully loaded and compiled custom patterns")
	return validPatterns, nil
}

// ScanWithRegexes scans the given content using the configured regex patterns.
func (s *RegexScanner) ScanWithRegexes(content []byte, sourceURL string) ([]models.SecretFinding, error) {
	s.logger.Debug().Str("sourceURL", sourceURL).Int("contentLength", len(content)).Int("patternCount", len(s.patterns)).Msg("Scanning content with custom regexes")
	var findings []models.SecretFinding
	lineNumber := 0

	// Create a reader for line-by-line processing to get line numbers
	contentReader := bytes.NewReader(content)
	lineScanner := bufio.NewScanner(contentReader)

	// Increase buffer size to handle large JS files (default is 64KB, increase to 1MB)
	const maxScanTokenSize = 1024 * 1024 // 1MB
	buf := make([]byte, maxScanTokenSize)
	lineScanner.Buffer(buf, maxScanTokenSize)

	for lineScanner.Scan() {
		lineNumber++
		lineBytes := lineScanner.Bytes()
		lineStr := string(lineBytes) // For regex matching, string conversion is often needed

		for _, p := range s.patterns {
			if p.Compiled == nil {
				continue // Skip patterns that failed to compile
			}

			// Check line length limit if specified
			if p.LineLength > 0 && len(lineStr) > p.LineLength {
				continue
			}

			matches := p.Compiled.FindAllStringSubmatchIndex(lineStr, -1)
			for _, matchIndices := range matches {
				// matchIndices[0] is start of full match, matchIndices[1] is end of full match
				// If the regex has capturing groups, they are at matchIndices[2*n] and matchIndices[2*n+1]
				// We are interested in the full match or a specific group if defined for the secret.
				// For simplicity, we take the full match or the first capture group if available.

				var secretText string
				if len(matchIndices) > 2 { // Has at least one capture group
					// Check if the capture group indices are valid
					if matchIndices[2] >= 0 && matchIndices[3] >= 0 && matchIndices[2] < matchIndices[3] {
						secretText = lineStr[matchIndices[2]:matchIndices[3]]
					} else {
						// Fallback to full match if capture group is weird (e.g. optional and not matched)
						secretText = lineStr[matchIndices[0]:matchIndices[1]]
					}
				} else { // No capture groups, use full match
					secretText = lineStr[matchIndices[0]:matchIndices[1]]
				}

				// Basic check for entropy if defined in pattern (simplified example)
				if p.Entropy > 0 {
					entropy := calculateShannonEntropy(secretText)
					if entropy < p.Entropy {
						continue // Skip if below entropy threshold
					}
				}

				finding := models.SecretFinding{
					SourceURL:         sourceURL,
					RuleID:            p.RuleID,
					Description:       p.Description,
					Severity:          p.Severity,
					SecretText:        secretText, // Store full secret without truncation
					LineNumber:        lineNumber,
					Timestamp:         time.Now(),
					ToolName:          "RegexScanner",
					VerificationState: "Unverified", // Regex patterns cannot verify secrets
				}
				findings = append(findings, finding)

				// Handle MaxFinds for the rule
				if p.MaxFinds > 0 {
					currentFindsForRule := 0
					for _, f := range findings {
						if f.RuleID == p.RuleID {
							currentFindsForRule++
						}
					}
					if currentFindsForRule >= p.MaxFinds {
						break // Stop processing this rule for the current line/file if max finds reached
					}
				}
			}
		}
	}

	if err := lineScanner.Err(); err != nil && err != io.EOF {
		s.logger.Error().Err(err).Str("sourceURL", sourceURL).Msg("Error scanning content lines")
		return findings, fmt.Errorf("error reading content for regex scan on %s: %w", sourceURL, err)
	}

	if len(findings) > 0 {
		s.logger.Debug().Int("count", len(findings)).Str("sourceURL", sourceURL).Msg("Custom regex scanner found secrets")
	}

	return findings, nil
}

// Helper function to compile regex (could be moved to patterns.go or a util package)
func compileRegex(pattern string) (*regexp.Regexp, error) {
	return regexp.Compile(pattern)
}

// Helper function to calculate Shannon entropy (simplified)
// For a more robust implementation, consider a dedicated library or more research.
func calculateShannonEntropy(data string) float64 {
	if data == "" {
		return 0.0
	}
	freq := make(map[rune]int)
	for _, char := range data {
		freq[char]++
	}

	entropy := 0.0
	length := float64(len(data))
	for _, count := range freq {
		probability := float64(count) / length
		entropy -= probability * math.Log2(probability)
	}
	return entropy
}

// loadMantraPatternsFromEmbedded loads regex patterns from a Mantra-style YAML file embedded in the binary.
func loadMantraPatternsFromEmbedded(fs embed.FS, logger zerolog.Logger) ([]RegexPattern, error) {
	// Read embedded file content
	data, err := fs.ReadFile("patterns.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded mantra patterns file: %w", err)
	}

	// Define structure for YAML parsing
	type MantraPatternFile struct {
		Patterns []struct {
			RuleID      string   `yaml:"rule_id"`
			Description string   `yaml:"description"`
			Pattern     string   `yaml:"pattern"`
			Severity    string   `yaml:"severity"`
			Keywords    []string `yaml:"keywords,omitempty"`
			Entropy     float64  `yaml:"entropy,omitempty"`
			MaxFinds    int      `yaml:"max_finds,omitempty"`
			LineLength  int      `yaml:"line_length,omitempty"`
		} `yaml:"patterns"`
	}

	var mantraFile MantraPatternFile
	if err := yaml.Unmarshal(data, &mantraFile); err != nil {
		return nil, fmt.Errorf("failed to parse embedded mantra patterns YAML: %w", err)
	}

	var patterns []RegexPattern
	for _, p := range mantraFile.Patterns {
		pattern := RegexPattern{
			RuleID:      p.RuleID,
			Description: p.Description,
			Pattern:     p.Pattern,
			Severity:    p.Severity,
			Keywords:    p.Keywords,
			Entropy:     p.Entropy,
			MaxFinds:    p.MaxFinds,
			LineLength:  p.LineLength,
		}
		patterns = append(patterns, pattern)
	}

	logger.Debug().Int("count", len(patterns)).Msg("Successfully loaded embedded Mantra patterns")
	return patterns, nil
}

// loadMantraPatternsFromFile loads regex patterns from a Mantra-style YAML file (for backward compatibility).
func loadMantraPatternsFromFile(filePath string, logger zerolog.Logger) ([]RegexPattern, error) {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("mantra patterns file does not exist: %s", filePath)
	}

	// Read file content
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read mantra patterns file: %w", err)
	}

	// Define structure for YAML parsing
	type MantraPatternFile struct {
		Patterns []struct {
			RuleID      string   `yaml:"rule_id"`
			Description string   `yaml:"description"`
			Pattern     string   `yaml:"pattern"`
			Severity    string   `yaml:"severity"`
			Keywords    []string `yaml:"keywords,omitempty"`
			Entropy     float64  `yaml:"entropy,omitempty"`
			MaxFinds    int      `yaml:"max_finds,omitempty"`
			LineLength  int      `yaml:"line_length,omitempty"`
		} `yaml:"patterns"`
	}

	var mantraFile MantraPatternFile
	if err := yaml.Unmarshal(data, &mantraFile); err != nil {
		return nil, fmt.Errorf("failed to parse mantra patterns YAML: %w", err)
	}

	var patterns []RegexPattern
	for _, p := range mantraFile.Patterns {
		pattern := RegexPattern{
			RuleID:      p.RuleID,
			Description: p.Description,
			Pattern:     p.Pattern,
			Severity:    p.Severity,
			Keywords:    p.Keywords,
			Entropy:     p.Entropy,
			MaxFinds:    p.MaxFinds,
			LineLength:  p.LineLength,
		}
		patterns = append(patterns, pattern)
	}

	logger.Debug().Int("count", len(patterns)).Str("file", filePath).Msg("Successfully loaded Mantra patterns from file")
	return patterns, nil
}
