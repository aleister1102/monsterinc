package crawler

import (
	"testing"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestURLPatternDetector_ShouldSkipURL(t *testing.T) {
	logger := zerolog.Nop()

	tests := []struct {
		name            string
		config          config.AutoCalibrateConfig
		urls            []string
		expectedSkipped []bool
		description     string
	}{
		{
			name: "Should skip similar forum URLs",
			config: config.AutoCalibrateConfig{
				Enabled:           true,
				MaxSimilarURLs:    1,
				IgnoreParameters:  []string{"tid", "fid", "page"},
				EnableSkipLogging: true,
			},
			urls: []string{
				"https://forum.cdo.oppomobile.com/read.php?tid=3801598&fid=1226&page=e#a",
				"https://forum.cdo.oppomobile.com/read.php?tid=5409440&fid=4552&page=e#a",
				"https://forum.cdo.oppomobile.com/read.php?tid=5350586&fid=3018&page=e#a",
			},
			expectedSkipped: []bool{false, true, true},
			description:     "First URL should be crawled, subsequent similar URLs should be skipped",
		},
		{
			name: "Disabled auto-calibrate should not skip",
			config: config.AutoCalibrateConfig{
				Enabled:           false,
				MaxSimilarURLs:    1,
				IgnoreParameters:  []string{"tid", "fid", "page"},
				EnableSkipLogging: true,
			},
			urls: []string{
				"https://forum.cdo.oppomobile.com/read.php?tid=3801598&fid=1226&page=e#a",
				"https://forum.cdo.oppomobile.com/read.php?tid=5409440&fid=4552&page=e#a",
			},
			expectedSkipped: []bool{false, false},
			description:     "When disabled, no URLs should be skipped",
		},
		{
			name: "Different paths should not be skipped",
			config: config.AutoCalibrateConfig{
				Enabled:           true,
				MaxSimilarURLs:    1,
				IgnoreParameters:  []string{"tid", "fid", "page"},
				EnableSkipLogging: true,
			},
			urls: []string{
				"https://forum.cdo.oppomobile.com/read.php?tid=3801598&fid=1226&page=e#a",
				"https://forum.cdo.oppomobile.com/post.php?tid=5409440&fid=4552&page=e#a",
			},
			expectedSkipped: []bool{false, false},
			description:     "URLs with different paths should not be considered similar",
		},
		{
			name: "Max similar URLs = 2",
			config: config.AutoCalibrateConfig{
				Enabled:           true,
				MaxSimilarURLs:    2,
				IgnoreParameters:  []string{"tid", "fid", "page"},
				EnableSkipLogging: true,
			},
			urls: []string{
				"https://forum.cdo.oppomobile.com/read.php?tid=3801598&fid=1226&page=e#a",
				"https://forum.cdo.oppomobile.com/read.php?tid=5409440&fid=4552&page=e#a",
				"https://forum.cdo.oppomobile.com/read.php?tid=5350586&fid=3018&page=e#a",
			},
			expectedSkipped: []bool{false, false, true},
			description:     "Should allow 2 similar URLs before skipping",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector := NewURLPatternDetector(tt.config, logger)

			for i, url := range tt.urls {
				skipped := detector.ShouldSkipURL(url)
				assert.Equal(t, tt.expectedSkipped[i], skipped,
					"URL %d (%s): expected skipped=%v, got skipped=%v. %s",
					i, url, tt.expectedSkipped[i], skipped, tt.description)
			}
		})
	}
}

func TestURLPatternDetector_GenerateURLPattern(t *testing.T) {
	logger := zerolog.Nop()
	config := config.AutoCalibrateConfig{
		Enabled:           true,
		MaxSimilarURLs:    1,
		IgnoreParameters:  []string{"tid", "fid", "page", "id"},
		EnableSkipLogging: true,
	}

	detector := NewURLPatternDetector(config, logger)

	tests := []struct {
		name            string
		url             string
		expectedPattern string
		shouldError     bool
	}{
		{
			name:            "Forum URL with ignored parameters",
			url:             "https://forum.cdo.oppomobile.com/read.php?tid=3801598&fid=1226&page=e#a",
			expectedPattern: "https://forum.cdo.oppomobile.com/read.php",
			shouldError:     false,
		},
		{
			name:            "URL with non-ignored parameters",
			url:             "https://example.com/search?query=test&sort=date",
			expectedPattern: "https://example.com/search?query=*&sort=*",
			shouldError:     false,
		},
		{
			name:            "URL with mixed parameters",
			url:             "https://example.com/page?id=123&query=test&page=2",
			expectedPattern: "https://example.com/page?query=*",
			shouldError:     false,
		},
		{
			name:            "Simple URL without parameters",
			url:             "https://example.com/page",
			expectedPattern: "https://example.com/page",
			shouldError:     false,
		},
		{
			name:            "Invalid URL",
			url:             "not-a-url",
			expectedPattern: "://not-a-url",
			shouldError:     false, // Go's url.Parse is quite lenient
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern, err := detector.generateURLPattern(tt.url)

			if tt.shouldError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedPattern, pattern)
			}
		})
	}
}

func TestURLPatternDetector_GetPatternStats(t *testing.T) {
	logger := zerolog.Nop()
	config := config.AutoCalibrateConfig{
		Enabled:           true,
		MaxSimilarURLs:    2,
		IgnoreParameters:  []string{"tid", "fid"},
		EnableSkipLogging: true,
	}

	detector := NewURLPatternDetector(config, logger)

	// Test initial state
	stats := detector.GetPatternStats()
	assert.Empty(t, stats)

	// Add some URLs
	urls := []string{
		"https://forum.example.com/read.php?tid=1&fid=1",
		"https://forum.example.com/read.php?tid=2&fid=2",
		"https://example.com/different-page",
	}

	for _, url := range urls {
		detector.ShouldSkipURL(url)
	}

	stats = detector.GetPatternStats()
	assert.Equal(t, 2, len(stats))
	assert.Equal(t, 2, stats["https://forum.example.com/read.php"])
	assert.Equal(t, 1, stats["https://example.com/different-page"])
}

func TestURLPatternDetector_Reset(t *testing.T) {
	logger := zerolog.Nop()
	config := config.AutoCalibrateConfig{
		Enabled:           true,
		MaxSimilarURLs:    1,
		IgnoreParameters:  []string{"tid"},
		EnableSkipLogging: true,
	}

	detector := NewURLPatternDetector(config, logger)

	// Add some URLs
	detector.ShouldSkipURL("https://example.com/page?tid=1")
	detector.ShouldSkipURL("https://example.com/page?tid=2")

	// Verify stats exist
	stats := detector.GetPatternStats()
	assert.NotEmpty(t, stats)

	// Reset detector
	detector.Reset()

	// Verify stats are cleared
	stats = detector.GetPatternStats()
	assert.Empty(t, stats)

	// Verify URLs can be processed again after reset
	skipped := detector.ShouldSkipURL("https://example.com/page?tid=1")
	assert.False(t, skipped, "After reset, first URL should not be skipped")
}
