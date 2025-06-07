package crawler

import (
	"fmt"
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
			name: "Should skip similar support URLs with different locales",
			config: config.AutoCalibrateConfig{
				Enabled:           true,
				MaxSimilarURLs:    1,
				IgnoreParameters:  []string{},
				AutoDetectLocales: true, // Enable auto locale detection
				EnableSkipLogging: true,
			},
			urls: []string{
				"https://support.oppo.com/chde/premium/find-n3-1/",
				"https://support.oppo.com/tw/premium/find-n3-1/",
				"https://support.oppo.com/kzkk/premium/find-n3-1/",
				"https://support.oppo.com/chfr/premium/find-n3-1/",
				"https://support.oppo.com/jp/premium/findx6-series/",
				"https://support.oppo.com/en/premium/findx6-series/",
				"https://support.oppo.com/fr/premium/findx6-series/",
			},
			expectedSkipped: []bool{false, true, true, true, false, true, true},
			description:     "Should detect two patterns: find-n3-1 and findx6-series, skip duplicates",
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

	tests := []struct {
		name            string
		config          config.AutoCalibrateConfig
		url             string
		expectedPattern string
		shouldError     bool
	}{
		{
			name: "Forum URL with ignored parameters",
			config: config.AutoCalibrateConfig{
				Enabled:           true,
				MaxSimilarURLs:    1,
				IgnoreParameters:  []string{"tid", "fid", "page", "id"},
				EnableSkipLogging: true,
			},
			url:             "https://forum.cdo.oppomobile.com/read.php?tid=3801598&fid=1226&page=e#a",
			expectedPattern: "https://forum.cdo.oppomobile.com/read.php",
			shouldError:     false,
		},
		{
			name: "Support URL with ignored path segment",
			config: config.AutoCalibrateConfig{
				Enabled:           true,
				MaxSimilarURLs:    1,
				IgnoreParameters:  []string{},
				AutoDetectLocales: true, // Enable auto locale detection
				EnableSkipLogging: true,
			},
			url:             "https://support.oppo.com/chde/premium/find-n3-1/",
			expectedPattern: "https://support.oppo.com/*/premium/find-n3-1/",
			shouldError:     false,
		},
		{
			name: "URL with non-ignored parameters",
			config: config.AutoCalibrateConfig{
				Enabled:           true,
				MaxSimilarURLs:    1,
				IgnoreParameters:  []string{},
				EnableSkipLogging: true,
			},
			url:             "https://example.com/search?query=test&sort=date",
			expectedPattern: "https://example.com/search?query=*&sort=*",
			shouldError:     false,
		},
		{
			name: "URL with mixed parameters and path segments",
			config: config.AutoCalibrateConfig{
				Enabled:           true,
				MaxSimilarURLs:    1,
				IgnoreParameters:  []string{"id", "page"},
				AutoDetectLocales: true,
				EnableSkipLogging: true,
			},
			url:             "https://example.com/api/v1/users?id=123&query=test&page=2",
			expectedPattern: "https://example.com/api/v1/users?query=*",
			shouldError:     false,
		},
		{
			name: "Simple URL without parameters",
			config: config.AutoCalibrateConfig{
				Enabled:           true,
				MaxSimilarURLs:    1,
				EnableSkipLogging: true,
			},
			url:             "https://example.com/page",
			expectedPattern: "https://example.com/page",
			shouldError:     false,
		},
		{
			name: "Invalid URL",
			config: config.AutoCalibrateConfig{
				Enabled:           true,
				MaxSimilarURLs:    1,
				EnableSkipLogging: true,
			},
			url:             "not-a-url",
			expectedPattern: "://not-a-url",
			shouldError:     false, // Go's url.Parse is quite lenient
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector := NewURLPatternDetector(tt.config, logger)
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

func TestURLPatternDetector_FragmentHandling(t *testing.T) {
	config := config.AutoCalibrateConfig{
		Enabled:           true,
		MaxSimilarURLs:    1,
		IgnoreParameters:  []string{"id", "page", "tid", "fid"},
		AutoDetectLocales: false,
	}

	detector := NewURLPatternDetector(config, zerolog.Nop())

	// Test fragment URLs with id=number pattern
	testURLs := []struct {
		url         string
		shouldSkip  bool
		description string
	}{
		{
			url:         "https://open.oppomobile.com/wiki/doc#id=10071",
			shouldSkip:  false,
			description: "First URL with id fragment should not be skipped",
		},
		{
			url:         "https://open.oppomobile.com/wiki/doc#id=10294",
			shouldSkip:  true,
			description: "Second URL with same pattern should be skipped",
		},
		{
			url:         "https://open.oppomobile.com/wiki/doc#id=10456",
			shouldSkip:  true,
			description: "Third URL with same pattern should be skipped",
		},
		{
			url:         "https://open.oppomobile.com/wiki/doc#section=intro",
			shouldSkip:  false,
			description: "URL with different fragment type should not be skipped",
		},
	}

	for _, tt := range testURLs {
		t.Run(tt.description, func(t *testing.T) {
			result := detector.ShouldSkipURL(tt.url)
			if result != tt.shouldSkip {
				t.Errorf("Expected ShouldSkipURL(%s) = %v, got %v", tt.url, tt.shouldSkip, result)
			}
		})
	}
}

func TestURLPatternDetector_isVariableFragment(t *testing.T) {
	config := config.AutoCalibrateConfig{
		Enabled:           true,
		MaxSimilarURLs:    1,
		IgnoreParameters:  []string{"id", "page"},
		AutoDetectLocales: false,
	}

	detector := NewURLPatternDetector(config, zerolog.Nop())

	tests := []struct {
		fragment string
		expected bool
	}{
		{"id=10071", true},       // Variable ID
		{"id=10294", true},       // Variable ID
		{"page=1", true},         // Variable page
		{"page=2", true},         // Variable page
		{"section=intro", false}, // Non-variable content
		{"about", false},         // Static fragment
		{"a", true},              // Short fragment
		{"123", true},            // Numeric fragment
		{"abc123", true},         // Hex-like fragment
		{"introduction", false},  // Long static fragment
		{"", false},              // Empty fragment
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("fragment_%s", tt.fragment), func(t *testing.T) {
			result := detector.isVariableFragment(tt.fragment)
			if result != tt.expected {
				t.Errorf("Expected isVariableFragment(%s) = %v, got %v", tt.fragment, tt.expected, result)
			}
		})
	}
}
