package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestURLStatus_Constants(t *testing.T) {
	// Test that URL status constants are properly defined
	assert.Equal(t, URLStatus("new"), StatusNew)
	assert.Equal(t, URLStatus("old"), StatusOld)
	assert.Equal(t, URLStatus("existing"), StatusExisting)
}

func TestDiffedURL_Creation(t *testing.T) {
	now := time.Now()
	probeResult := ProbeResult{
		InputURL:    "https://example.com/test",
		StatusCode:  200,
		ContentType: "text/html",
		Title:       "Test Page",
		Timestamp:   now,
		URLStatus:   "new",
	}

	diffedURL := DiffedURL{
		ProbeResult: probeResult,
	}

	// Test that DiffedURL contains the probe result
	assert.Equal(t, "https://example.com/test", diffedURL.ProbeResult.InputURL)
	assert.Equal(t, 200, diffedURL.ProbeResult.StatusCode)
	assert.Equal(t, "text/html", diffedURL.ProbeResult.ContentType)
	assert.Equal(t, "Test Page", diffedURL.ProbeResult.Title)
	assert.Equal(t, now, diffedURL.ProbeResult.Timestamp)
	assert.Equal(t, "new", diffedURL.ProbeResult.URLStatus)
}

func TestURLDiffResult_Creation(t *testing.T) {
	probeResults := []ProbeResult{
		{
			InputURL:   "https://example.com/new",
			StatusCode: 200,
			URLStatus:  "new",
		},
		{
			InputURL:   "https://example.com/existing",
			StatusCode: 200,
			URLStatus:  "existing",
		},
		{
			InputURL:   "https://example.com/old",
			StatusCode: 404,
			URLStatus:  "old",
		},
	}

	diffedURLs := make([]DiffedURL, len(probeResults))
	for i, pr := range probeResults {
		diffedURLs[i] = DiffedURL{ProbeResult: pr}
	}

	urlDiffResult := URLDiffResult{
		RootTargetURL: "https://example.com",
		Results:       diffedURLs,
		New:           1,
		Old:           1,
		Existing:      1,
		Error:         "",
	}

	// Test URLDiffResult fields
	assert.Equal(t, "https://example.com", urlDiffResult.RootTargetURL)
	assert.Equal(t, 3, len(urlDiffResult.Results))
	assert.Equal(t, 1, urlDiffResult.New)
	assert.Equal(t, 1, urlDiffResult.Old)
	assert.Equal(t, 1, urlDiffResult.Existing)
	assert.Empty(t, urlDiffResult.Error)
}

func TestURLDiffResult_CountStatuses(t *testing.T) {
	probeResults := []ProbeResult{
		{InputURL: "https://example.com/new1", URLStatus: "new"},
		{InputURL: "https://example.com/new2", URLStatus: "new"},
		{InputURL: "https://example.com/existing1", URLStatus: "existing"},
		{InputURL: "https://example.com/existing2", URLStatus: "existing"},
		{InputURL: "https://example.com/existing3", URLStatus: "existing"},
		{InputURL: "https://example.com/old1", URLStatus: "old"},
	}

	diffedURLs := make([]DiffedURL, len(probeResults))
	for i, pr := range probeResults {
		diffedURLs[i] = DiffedURL{ProbeResult: pr}
	}

	urlDiffResult := URLDiffResult{
		RootTargetURL: "https://example.com",
		Results:       diffedURLs,
	}

	// Test counting different statuses
	newCount := urlDiffResult.CountStatuses(StatusNew)
	existingCount := urlDiffResult.CountStatuses(StatusExisting)
	oldCount := urlDiffResult.CountStatuses(StatusOld)

	assert.Equal(t, 2, newCount)
	assert.Equal(t, 3, existingCount)
	assert.Equal(t, 1, oldCount)
}

func TestURLDiffResult_CountStatuses_EmptyResults(t *testing.T) {
	urlDiffResult := URLDiffResult{
		RootTargetURL: "https://example.com",
		Results:       []DiffedURL{},
	}

	// Test counting with empty results
	newCount := urlDiffResult.CountStatuses(StatusNew)
	existingCount := urlDiffResult.CountStatuses(StatusExisting)
	oldCount := urlDiffResult.CountStatuses(StatusOld)

	assert.Equal(t, 0, newCount)
	assert.Equal(t, 0, existingCount)
	assert.Equal(t, 0, oldCount)
}

func TestURLDiffResult_CountStatuses_InvalidStatus(t *testing.T) {
	probeResults := []ProbeResult{
		{InputURL: "https://example.com/test1", URLStatus: "new"},
		{InputURL: "https://example.com/test2", URLStatus: "existing"},
		{InputURL: "https://example.com/test3", URLStatus: "invalid"},
	}

	diffedURLs := make([]DiffedURL, len(probeResults))
	for i, pr := range probeResults {
		diffedURLs[i] = DiffedURL{ProbeResult: pr}
	}

	urlDiffResult := URLDiffResult{
		RootTargetURL: "https://example.com",
		Results:       diffedURLs,
	}

	// Test counting with invalid status
	invalidCount := urlDiffResult.CountStatuses(URLStatus("invalid"))
	unknownCount := urlDiffResult.CountStatuses(URLStatus("unknown"))

	assert.Equal(t, 1, invalidCount)
	assert.Equal(t, 0, unknownCount)
}

func TestURLDiffResult_WithError(t *testing.T) {
	urlDiffResult := URLDiffResult{
		RootTargetURL: "https://example.com",
		Results:       []DiffedURL{},
		New:           0,
		Old:           0,
		Existing:      0,
		Error:         "Failed to load historical data",
	}

	// Test URLDiffResult with error
	assert.Equal(t, "https://example.com", urlDiffResult.RootTargetURL)
	assert.Equal(t, 0, len(urlDiffResult.Results))
	assert.Equal(t, 0, urlDiffResult.New)
	assert.Equal(t, 0, urlDiffResult.Old)
	assert.Equal(t, 0, urlDiffResult.Existing)
	assert.Equal(t, "Failed to load historical data", urlDiffResult.Error)
}

func TestURLDiffResult_CompleteScenario(t *testing.T) {
	// Create a comprehensive test scenario
	now := time.Now()

	probeResults := []ProbeResult{
		{
			InputURL:    "https://example.com/api/v1/users",
			StatusCode:  200,
			ContentType: "application/json",
			Title:       "Users API",
			Timestamp:   now,
			URLStatus:   "new",
		},
		{
			InputURL:    "https://example.com/api/v1/posts",
			StatusCode:  200,
			ContentType: "application/json",
			Title:       "Posts API",
			Timestamp:   now,
			URLStatus:   "existing",
		},
		{
			InputURL:    "https://example.com/api/v1/comments",
			StatusCode:  200,
			ContentType: "application/json",
			Title:       "Comments API",
			Timestamp:   now,
			URLStatus:   "existing",
		},
		{
			InputURL:    "https://example.com/old-endpoint",
			StatusCode:  404,
			ContentType: "text/html",
			Title:       "Not Found",
			Timestamp:   now.Add(-24 * time.Hour),
			URLStatus:   "old",
		},
	}

	diffedURLs := make([]DiffedURL, len(probeResults))
	for i, pr := range probeResults {
		diffedURLs[i] = DiffedURL{ProbeResult: pr}
	}

	urlDiffResult := URLDiffResult{
		RootTargetURL: "https://example.com",
		Results:       diffedURLs,
		New:           1,
		Old:           1,
		Existing:      2,
		Error:         "",
	}

	// Test comprehensive scenario
	assert.Equal(t, "https://example.com", urlDiffResult.RootTargetURL)
	assert.Equal(t, 4, len(urlDiffResult.Results))
	assert.Equal(t, 1, urlDiffResult.New)
	assert.Equal(t, 2, urlDiffResult.Existing)
	assert.Equal(t, 1, urlDiffResult.Old)
	assert.Empty(t, urlDiffResult.Error)

	// Verify counts match actual results
	actualNewCount := urlDiffResult.CountStatuses(StatusNew)
	actualExistingCount := urlDiffResult.CountStatuses(StatusExisting)
	actualOldCount := urlDiffResult.CountStatuses(StatusOld)

	assert.Equal(t, urlDiffResult.New, actualNewCount)
	assert.Equal(t, urlDiffResult.Existing, actualExistingCount)
	assert.Equal(t, urlDiffResult.Old, actualOldCount)

	// Test individual diffed URLs
	newURL := urlDiffResult.Results[0]
	assert.Equal(t, "https://example.com/api/v1/users", newURL.ProbeResult.InputURL)
	assert.Equal(t, "new", newURL.ProbeResult.URLStatus)
	assert.Equal(t, 200, newURL.ProbeResult.StatusCode)

	oldURL := urlDiffResult.Results[3]
	assert.Equal(t, "https://example.com/old-endpoint", oldURL.ProbeResult.InputURL)
	assert.Equal(t, "old", oldURL.ProbeResult.URLStatus)
	assert.Equal(t, 404, oldURL.ProbeResult.StatusCode)
}

func TestURLDiffResult_StatisticsConsistency(t *testing.T) {
	// Test that manual counts match the CountStatuses method
	probeResults := []ProbeResult{
		{InputURL: "https://example.com/1", URLStatus: "new"},
		{InputURL: "https://example.com/2", URLStatus: "new"},
		{InputURL: "https://example.com/3", URLStatus: "new"},
		{InputURL: "https://example.com/4", URLStatus: "existing"},
		{InputURL: "https://example.com/5", URLStatus: "existing"},
		{InputURL: "https://example.com/6", URLStatus: "old"},
	}

	diffedURLs := make([]DiffedURL, len(probeResults))
	for i, pr := range probeResults {
		diffedURLs[i] = DiffedURL{ProbeResult: pr}
	}

	urlDiffResult := URLDiffResult{
		RootTargetURL: "https://example.com",
		Results:       diffedURLs,
		New:           3,
		Old:           1,
		Existing:      2,
	}

	// Test that stored counts match calculated counts
	assert.Equal(t, urlDiffResult.New, urlDiffResult.CountStatuses(StatusNew))
	assert.Equal(t, urlDiffResult.Existing, urlDiffResult.CountStatuses(StatusExisting))
	assert.Equal(t, urlDiffResult.Old, urlDiffResult.CountStatuses(StatusOld))

	// Test total count
	totalStored := urlDiffResult.New + urlDiffResult.Existing + urlDiffResult.Old
	totalCalculated := urlDiffResult.CountStatuses(StatusNew) +
		urlDiffResult.CountStatuses(StatusExisting) +
		urlDiffResult.CountStatuses(StatusOld)

	assert.Equal(t, totalStored, totalCalculated)
	assert.Equal(t, len(urlDiffResult.Results), totalCalculated)
}
