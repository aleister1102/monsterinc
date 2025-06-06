package crawler

import (
	"testing"

	"github.com/rs/zerolog"
)

func TestCheckPathScope_WithQueryParams(t *testing.T) {
	logger := zerolog.Nop()

	settings := &ScopeSettings{
		disallowedFileExtensions: []string{".css", ".js", ".txt"},
		logger:                   logger,
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "CSS file with query params should be blocked",
			path:     "/assets/style.css?v=123",
			expected: false,
		},
		{
			name:     "JS file with fragment should be blocked",
			path:     "/app.js#main",
			expected: false,
		},
		{
			name:     "CSS file with both query and fragment should be blocked",
			path:     "/style.css?v=123#section",
			expected: false,
		},
		{
			name:     "HTML file with query params should be allowed",
			path:     "/page.html?id=123",
			expected: true,
		},
		{
			name:     "Path without extension should be allowed",
			path:     "/api/endpoint?param=value",
			expected: true,
		},
		{
			name:     "TXT file without query should be blocked",
			path:     "/readme.txt",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := settings.checkPathScope(tt.path)
			if result != tt.expected {
				t.Errorf("checkPathScope(%q) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestCheckHostnameScope_WithSeedURLs(t *testing.T) {
	logger := zerolog.Nop()

	settings := &ScopeSettings{
		seedHostnames:        []string{"example.com", "test.com"},
		disallowedHostnames:  []string{"blocked.com"},
		autoAddSeedHostnames: true,
		includeSubdomains:    false,
		logger:               logger,
	}

	tests := []struct {
		name     string
		hostname string
		expected bool
	}{
		{
			name:     "Seed hostname should be allowed",
			hostname: "example.com",
			expected: true,
		},
		{
			name:     "Non-seed hostname should be denied when seeds exist",
			hostname: "nginx.org",
			expected: false,
		},
		{
			name:     "Blocked hostname should be denied",
			hostname: "blocked.com",
			expected: false,
		},
		{
			name:     "Another seed hostname should be allowed",
			hostname: "test.com",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := settings.CheckHostnameScope(tt.hostname)
			if result != tt.expected {
				t.Errorf("CheckHostnameScope(%q) = %v, expected %v", tt.hostname, result, tt.expected)
			}
		})
	}
}

func TestCheckHostnameScope_NoSeeds(t *testing.T) {
	logger := zerolog.Nop()

	// Case 1: No seeds, no disallowed - should allow all
	settingsNoRestrictions := &ScopeSettings{
		seedHostnames:       []string{},
		disallowedHostnames: []string{},
		logger:              logger,
	}

	if !settingsNoRestrictions.CheckHostnameScope("nginx.org") {
		t.Error("Expected nginx.org to be allowed when no restrictions")
	}

	// Case 2: No seeds, but has disallowed - should allow non-disallowed
	settingsWithDisallowed := &ScopeSettings{
		seedHostnames:       []string{},
		disallowedHostnames: []string{"blocked.com"},
		logger:              logger,
	}

	if !settingsWithDisallowed.CheckHostnameScope("nginx.org") {
		t.Error("Expected nginx.org to be allowed when only disallowed list exists")
	}

	if settingsWithDisallowed.CheckHostnameScope("blocked.com") {
		t.Error("Expected blocked.com to be denied")
	}
}
