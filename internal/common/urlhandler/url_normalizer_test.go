package urlhandler

import (
	"testing"
)

func TestURLNormalizer_NormalizeURL(t *testing.T) {
	tests := []struct {
		name     string
		config   URLNormalizationConfig
		inputURL string
		expected string
		wantErr  bool
	}{
		{
			name: "strip fragment only",
			config: URLNormalizationConfig{
				StripFragments:      true,
				StripTrackingParams: false,
				CustomStripParams:   []string{},
			},
			inputURL: "https://example.com/page#section",
			expected: "https://example.com/page",
			wantErr:  false,
		},
		{
			name: "strip tracking params only",
			config: URLNormalizationConfig{
				StripFragments:      false,
				StripTrackingParams: true,
				CustomStripParams:   []string{},
			},
			inputURL: "https://example.com/page?utm_source=test&param=value",
			expected: "https://example.com/page?param=value",
			wantErr:  false,
		},
		{
			name: "strip both fragment and tracking params",
			config: URLNormalizationConfig{
				StripFragments:      true,
				StripTrackingParams: true,
				CustomStripParams:   []string{},
			},
			inputURL: "https://example.com/page?utm_source=test&param=value#section",
			expected: "https://example.com/page?param=value",
			wantErr:  false,
		},
		{
			name: "strip custom params",
			config: URLNormalizationConfig{
				StripFragments:      false,
				StripTrackingParams: false,
				CustomStripParams:   []string{"custom", "test"},
			},
			inputURL: "https://example.com/page?custom=value&test=123&keep=this",
			expected: "https://example.com/page?keep=this",
			wantErr:  false,
		},
		{
			name: "no normalization",
			config: URLNormalizationConfig{
				StripFragments:      false,
				StripTrackingParams: false,
				CustomStripParams:   []string{},
			},
			inputURL: "https://example.com/page?utm_source=test#section",
			expected: "https://example.com/page?utm_source=test#section",
			wantErr:  false,
		},
		{
			name: "invalid URL",
			config: URLNormalizationConfig{
				StripFragments:      true,
				StripTrackingParams: true,
				CustomStripParams:   []string{},
			},
			inputURL: "://invalid-url",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalizer := NewURLNormalizer(tt.config)
			result, err := normalizer.NormalizeURL(tt.inputURL)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestDefaultURLNormalizationConfig(t *testing.T) {
	config := DefaultURLNormalizationConfig()

	if !config.StripFragments {
		t.Error("Expected StripFragments to be true")
	}

	if !config.StripTrackingParams {
		t.Error("Expected StripTrackingParams to be true")
	}

	expectedCustomParams := []string{"utm_source", "utm_medium", "utm_campaign", "fbclid", "gclid"}
	if len(config.CustomStripParams) != len(expectedCustomParams) {
		t.Errorf("Expected %d custom params, got %d", len(expectedCustomParams), len(config.CustomStripParams))
	}
}
