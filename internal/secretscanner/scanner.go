package secretscanner

import (
	"bufio"
	"bytes"

	"github.com/aleister1102/monsterinc/internal/models"
)

// RegexScanner scans content for secrets using a set of regex rules.
type RegexScanner struct {
	rules []RegexRule
}

// NewRegexScanner creates a new scanner with the default rules.
func NewRegexScanner() *RegexScanner {
	return &RegexScanner{
		rules: DefaultRules,
	}
}

// Scan applies the regex patterns to the input content and returns findings.
func (s *RegexScanner) Scan(sourceURL string, content []byte) []models.SecretFinding {
	var findings []models.SecretFinding
	foundSecrets := make(map[string]bool)

	for _, rule := range s.rules {
		// We scan line by line to get the line number
		scanner := bufio.NewScanner(bytes.NewReader(content))
		lineNumber := 1
		for scanner.Scan() {
			line := scanner.Text()
			matches := rule.Regex.FindAllStringSubmatch(line, -1)

			if len(matches) > 0 {
				for _, match := range matches {
					secretText := match[0]
					if len(match) > 1 {
						// If there's a capture group, use it as the secret.
						// This helps in getting the exact secret from a larger match.
						secretText = match[1]
					}

					if secretText != "" && !foundSecrets[secretText] {
						finding := models.SecretFinding{
							SourceURL:  sourceURL,
							RuleID:     rule.ID,
							SecretText: secretText,
							LineNumber: lineNumber,
							Context:    line,
						}
						findings = append(findings, finding)
						foundSecrets[secretText] = true
					}
				}
			}
			lineNumber++
		}
	}

	return findings
}
