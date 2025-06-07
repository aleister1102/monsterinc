package common

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestRetryHandler_ShouldRetry(t *testing.T) {
	logger := zerolog.Nop()
	config := RetryHandlerConfig{
		MaxRetries:       3,
		BaseDelay:        time.Second,
		MaxDelay:         time.Minute,
		EnableJitter:     false,
		RetryStatusCodes: []int{429, 502},
	}

	handler := NewRetryHandler(config, logger)

	tests := []struct {
		name       string
		statusCode int
		attempt    int
		expected   bool
	}{
		{"Should retry 429 on first attempt", 429, 0, true},
		{"Should retry 502 on first attempt", 502, 0, true},
		{"Should not retry 200", 200, 0, false},
		{"Should not retry 404", 404, 0, false},
		{"Should not retry after max attempts", 429, 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.ShouldRetry(tt.statusCode, tt.attempt)
			if result != tt.expected {
				t.Errorf("ShouldRetry() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestRetryHandler_CalculateDelay(t *testing.T) {
	logger := zerolog.Nop()
	config := RetryHandlerConfig{
		MaxRetries:       3,
		BaseDelay:        time.Second,
		MaxDelay:         10 * time.Second,
		EnableJitter:     false,
		RetryStatusCodes: []int{429},
	}

	handler := NewRetryHandler(config, logger)

	tests := []struct {
		name     string
		attempt  int
		expected time.Duration
	}{
		{"First attempt", 0, time.Second},
		{"Second attempt", 1, 2 * time.Second},
		{"Third attempt", 2, 4 * time.Second},
		{"Fourth attempt (capped)", 3, 8 * time.Second},
		{"Fifth attempt (capped)", 4, 10 * time.Second}, // Should be capped at maxDelay
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.CalculateDelay(tt.attempt)
			if result != tt.expected {
				t.Errorf("CalculateDelay() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestRetryHandler_DoWithRetry_Success(t *testing.T) {
	logger := zerolog.Nop()

	// Create test server that returns 200 OK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	config := RetryHandlerConfig{
		MaxRetries:       3,
		BaseDelay:        10 * time.Millisecond,
		MaxDelay:         100 * time.Millisecond,
		EnableJitter:     false,
		RetryStatusCodes: []int{429},
	}

	handler := NewRetryHandler(config, logger)
	client, err := NewHTTPClient(DefaultHTTPClientConfig(), logger)
	if err != nil {
		t.Fatalf("Failed to create HTTP client: %v", err)
	}

	req := &HTTPRequest{
		URL:     server.URL,
		Method:  "GET",
		Headers: make(map[string]string),
		Context: context.Background(),
	}

	resp, err := handler.DoWithRetry(context.Background(), client, req)
	if err != nil {
		t.Fatalf("DoWithRetry failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	if string(resp.Body) != "success" {
		t.Errorf("Expected body 'success', got '%s'", string(resp.Body))
	}
}

func TestRetryHandler_DoWithRetry_RateLimited(t *testing.T) {
	logger := zerolog.Nop()

	attemptCount := 0
	// Create test server that returns 429 for first 2 attempts, then 200
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("rate limited"))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
		}
	}))
	defer server.Close()

	config := RetryHandlerConfig{
		MaxRetries:       3,
		BaseDelay:        10 * time.Millisecond,
		MaxDelay:         100 * time.Millisecond,
		EnableJitter:     false,
		RetryStatusCodes: []int{429},
	}

	handler := NewRetryHandler(config, logger)
	client, err := NewHTTPClient(DefaultHTTPClientConfig(), logger)
	if err != nil {
		t.Fatalf("Failed to create HTTP client: %v", err)
	}

	req := &HTTPRequest{
		URL:     server.URL,
		Method:  "GET",
		Headers: make(map[string]string),
		Context: context.Background(),
	}

	resp, err := handler.DoWithRetry(context.Background(), client, req)
	if err != nil {
		t.Fatalf("DoWithRetry failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected final status 200, got %d", resp.StatusCode)
	}

	if string(resp.Body) != "success" {
		t.Errorf("Expected final body 'success', got '%s'", string(resp.Body))
	}

	if attemptCount != 3 {
		t.Errorf("Expected 3 attempts, got %d", attemptCount)
	}
}
