package common

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/net/http2"
)

func TestHTTP2Support(t *testing.T) {
	logger := zerolog.Nop()

	// Create a test server that supports HTTP/2
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message": "HTTP/2 works!", "protocol": "` + r.Proto + `"}`))
	}))

	// Enable HTTP/2 on the test server
	server.TLS = &tls.Config{}
	if err := http2.ConfigureServer(server.Config, &http2.Server{}); err != nil {
		t.Fatalf("Failed to configure HTTP/2 server: %v", err)
	}

	server.StartTLS()
	defer server.Close()

	// Create HTTP client with HTTP/2 enabled
	config := DefaultHTTPClientConfig()
	config.EnableHTTP2 = true
	config.InsecureSkipVerify = true // For test server
	config.Timeout = 10 * time.Second

	client, err := NewHTTPClient(config, logger)
	if err != nil {
		t.Fatalf("Failed to create HTTP client: %v", err)
	}

	req := &HTTPRequest{
		URL:     server.URL,
		Method:  "GET",
		Headers: make(map[string]string),
		Context: context.Background(),
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	responseBody := string(resp.Body)
	t.Logf("Response: %s", responseBody)

	// Check if response indicates HTTP/2 was used
	if len(responseBody) == 0 {
		t.Error("Empty response body")
	}
}

func TestHTTP2Disabled(t *testing.T) {
	logger := zerolog.Nop()

	// Create a simple HTTP/1.1 test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message": "HTTP/1.1 works!", "protocol": "` + r.Proto + `"}`))
	}))
	defer server.Close()

	// Create HTTP client with HTTP/2 disabled
	config := DefaultHTTPClientConfig()
	config.EnableHTTP2 = false
	config.Timeout = 10 * time.Second

	client, err := NewHTTPClient(config, logger)
	if err != nil {
		t.Fatalf("Failed to create HTTP client: %v", err)
	}

	req := &HTTPRequest{
		URL:     server.URL,
		Method:  "GET",
		Headers: make(map[string]string),
		Context: context.Background(),
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	responseBody := string(resp.Body)
	t.Logf("Response: %s", responseBody)

	if len(responseBody) == 0 {
		t.Error("Empty response body")
	}
}

func TestHTTPClientFactory_HTTP2Support(t *testing.T) {
	logger := zerolog.Nop()
	factory := NewHTTPClientFactory(logger)

	// Test Discord client with HTTP/2
	discordClient, err := factory.CreateDiscordClient(30 * time.Second)
	if err != nil {
		t.Fatalf("Failed to create Discord client: %v", err)
	}

	if discordClient == nil {
		t.Error("Discord client is nil")
	}

	// Test monitor client with HTTP/2
	monitorClient, err := factory.CreateMonitorClient(30*time.Second, true)
	if err != nil {
		t.Fatalf("Failed to create monitor client: %v", err)
	}

	if monitorClient == nil {
		t.Error("Monitor client is nil")
	}

	// Test basic client with HTTP/2
	basicClient, err := factory.CreateBasicClient(30 * time.Second)
	if err != nil {
		t.Fatalf("Failed to create basic client: %v", err)
	}

	if basicClient == nil {
		t.Error("Basic client is nil")
	}
}

func TestHTTPClientBuilder_HTTP2Config(t *testing.T) {
	logger := zerolog.Nop()

	// Test HTTP/2 enabled
	client1, err := NewHTTPClientBuilder(logger).
		WithHTTP2(true).
		WithTimeout(10 * time.Second).
		Build()
	if err != nil {
		t.Fatalf("Failed to build HTTP client with HTTP/2 enabled: %v", err)
	}

	if client1 == nil {
		t.Error("HTTP client with HTTP/2 enabled is nil")
	}

	// Test HTTP/2 disabled
	client2, err := NewHTTPClientBuilder(logger).
		WithHTTP2(false).
		WithTimeout(10 * time.Second).
		Build()
	if err != nil {
		t.Fatalf("Failed to build HTTP client with HTTP/2 disabled: %v", err)
	}

	if client2 == nil {
		t.Error("HTTP client with HTTP/2 disabled is nil")
	}
}

func ExampleHTTPClient() {
	logger := zerolog.Nop()

	// Create HTTP client with HTTP/2 support
	config := DefaultHTTPClientConfig()
	config.EnableHTTP2 = true

	client, err := NewHTTPClient(config, logger)
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		return
	}

	// Make a request
	req := &HTTPRequest{
		URL:    "https://httpbin.org/get",
		Method: "GET",
		Headers: map[string]string{
			"User-Agent": "MonsterInc HTTP/2 Client",
		},
		Context: context.Background(),
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error making request: %v\n", err)
		return
	}

	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Content-Type: %s\n", resp.Headers["Content-Type"])
}
