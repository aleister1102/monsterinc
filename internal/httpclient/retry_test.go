package httpclient

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetryHandler(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		if count <= 2 {
			w.WriteHeader(http.StatusTooManyRequests) // 429
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger := zerolog.Nop()
	retryConfig := RetryHandlerConfig{
		MaxRetries:       3,
		BaseDelay:        1 * time.Millisecond,
		MaxDelay:         10 * time.Millisecond,
		RetryStatusCodes: []int{http.StatusTooManyRequests, http.StatusInternalServerError},
	}
	retryHandler := NewRetryHandler(retryConfig, logger)

	client, err := NewHTTPClientBuilder(logger).Build()
	require.NoError(t, err)

	// This is a bit of a hack to test retry logic without exposing the retry handler on the client struct.
	// We create a temporary client and assign our retry handler to it.
	testClient := *client
	testClient.retryHandler = retryHandler

	req := &HTTPRequest{
		URL:    server.URL,
		Method: "GET",
	}

	resp, err := testClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(3), atomic.LoadInt32(&requestCount))
}

func TestRetryHandler_MaxRetriesExceeded(t *testing.T) {
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusServiceUnavailable) // Always fail
	}))
	defer server.Close()

	logger := zerolog.Nop()
	retryConfig := RetryHandlerConfig{
		MaxRetries:       2,
		BaseDelay:        1 * time.Millisecond,
		MaxDelay:         10 * time.Millisecond,
		RetryStatusCodes: []int{http.StatusServiceUnavailable},
	}
	retryHandler := NewRetryHandler(retryConfig, logger)

	client, err := NewHTTPClientBuilder(logger).Build()
	require.NoError(t, err)

	testClient := *client
	testClient.retryHandler = retryHandler

	req := &HTTPRequest{
		URL:    server.URL,
		Method: "GET",
	}

	resp, err := testClient.Do(req)
	require.Error(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int32(3), atomic.LoadInt32(&requestCount)) // Initial call + 2 retries
	var httpErr *HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusServiceUnavailable, httpErr.StatusCode)
}
