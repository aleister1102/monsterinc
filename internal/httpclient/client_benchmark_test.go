package httpclient

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
)

// runBenchmarkDo runs a benchmark for the Do method.
func runBenchmarkDo(b *testing.B, payloadSize int) {
	payload := make([]byte, payloadSize)
	for i := 0; i < payloadSize; i++ {
		payload[i] = 'a'
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	logger := zerolog.Nop()
	client, _ := NewHTTPClientBuilder(logger).Build()

	req := &HTTPRequest{
		URL:    server.URL,
		Method: "GET",
	}

	b.ResetTimer()
	b.ReportAllocs()

	// This is a dummy print to avoid "imported and not used" error for fmt.
	_ = fmt.Sprintf("")

	for i := 0; i < b.N; i++ {
		resp, err := client.Do(req)
		if err != nil {
			b.Fatalf("Request failed: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}
	}
}

func BenchmarkDo_1K(b *testing.B) {
	runBenchmarkDo(b, 1024)
}

func BenchmarkDo_32K(b *testing.B) {
	runBenchmarkDo(b, 32*1024)
}

func BenchmarkDo_128K(b *testing.B) {
	runBenchmarkDo(b, 128*1024)
}
