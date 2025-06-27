package reporter

import (
	"testing"

	"github.com/aleister1102/monsterinc/internal/config"
	httpx "github.com/aleister1102/monsterinc/internal/httpxrunner"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

// Since processProbeResults is a method on HtmlReporter, we need to instantiate it.
// We can use a minimal configuration for this test.
func newTestHtmlReporter(t *testing.T) *HtmlReporter {
	logger := zerolog.Nop()
	// Pass a pointer to the config
	cfg := config.NewDefaultReporterConfig()
	reporter, err := NewHtmlReporter(&cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create HtmlReporter: %v", err)
	}
	return reporter
}

func TestProcessProbeResults_SecretAssociation(t *testing.T) {
	reporter := newTestHtmlReporter(t)

	// --- Test Data ---
	probeResults := []*httpx.ProbeResult{
		{InputURL: "https://example.com/page1", StatusCode: 200},
		{InputURL: "https://example.com/page2", StatusCode: 404},
		{InputURL: "https://example.com/page3", StatusCode: 200},
	}

	secretFindings := []models.SecretFinding{
		{SourceURL: "https://example.com/page1", RuleID: "rule-A", SecretText: "secret-for-page1"},
		{SourceURL: "https://example.com/page3", RuleID: "rule-B", SecretText: "secret1-for-page3"},
		{SourceURL: "https://example.com/page3", RuleID: "rule-C", SecretText: "secret2-for-page3"},
	}

	pageData := &models.ReportPageData{
		SecretFindings: secretFindings,
		// Initialize other slices to avoid nil panics
		ProbeResults:       []models.ProbeResultDisplay{},
		UniqueHostnames:    []string{},
		UniqueStatusCodes:  []int{},
		UniqueContentTypes: []string{},
		UniqueTechnologies: []string{},
		UniqueURLStatuses:  []string{},
	}

	// --- Call the method under test using the test helper ---
	reporter.TestProcessProbeResults(probeResults, pageData)

	// --- Assertions ---

	// Check total results
	assert.Len(t, pageData.ProbeResults, 3)

	// Find the results for each page and check secrets
	var resultPage1, resultPage2, resultPage3 models.ProbeResultDisplay
	foundPage1, foundPage2, foundPage3 := false, false, false
	for _, r := range pageData.ProbeResults {
		switch r.InputURL {
		case "https://example.com/page1":
			resultPage1 = r
			foundPage1 = true
		case "https://example.com/page2":
			resultPage2 = r
			foundPage2 = true
		case "https://example.com/page3":
			resultPage3 = r
			foundPage3 = true
		}
	}

	// Assert that all pages were found in the results
	assert.True(t, foundPage1, "Result for page1 should exist")
	assert.True(t, foundPage2, "Result for page2 should exist")
	assert.True(t, foundPage3, "Result for page3 should exist")

	// Page 1 should have one secret
	assert.Len(t, resultPage1.SecretFindings, 1)
	assert.Equal(t, "rule-A", resultPage1.SecretFindings[0].RuleID)

	// Page 2 should have no secrets
	assert.Empty(t, resultPage2.SecretFindings)

	// Page 3 should have two secrets
	assert.Len(t, resultPage3.SecretFindings, 2)
	// The order of secrets for the same URL is not guaranteed, so check for presence instead of exact order.
	foundRuleB := false
	foundRuleC := false
	for _, secret := range resultPage3.SecretFindings {
		if secret.RuleID == "rule-B" {
			foundRuleB = true
		}
		if secret.RuleID == "rule-C" {
			foundRuleC = true
		}
	}
	assert.True(t, foundRuleB, "Should have found rule-B for page 3")
	assert.True(t, foundRuleC, "Should have found rule-C for page 3")
}
