package httpclient

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPClientBuilder(t *testing.T) {
	logger := zerolog.Nop()
	builder := NewHTTPClientBuilder(logger)

	client, err := builder.
		WithTimeout(15 * time.Second).
		WithUserAgent("test-agent").
		WithFollowRedirects(false).
		WithInsecureSkipVerify(true).
		WithMaxRedirects(5).
		Build()

	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, 15*time.Second, client.config.Timeout)
	assert.Equal(t, "test-agent", client.config.UserAgent)
	assert.False(t, client.config.FollowRedirects)
	assert.True(t, client.config.InsecureSkipVerify)
	assert.Equal(t, 5, client.config.MaxRedirects)
}

func TestHTTPClient_Do(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-value", r.Header.Get("X-Test-Header"))
		assert.Equal(t, "test-agent", r.Header.Get("User-Agent"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	logger := zerolog.Nop()
	client, err := NewHTTPClientBuilder(logger).WithUserAgent("test-agent").Build()
	require.NoError(t, err)

	req := &HTTPRequest{
		URL:    server.URL,
		Method: "GET",
		Headers: map[string]string{
			"X-Test-Header": "test-value",
		},
	}

	resp, err := client.Do(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, `{"status":"ok"}`, string(resp.Body))
	assert.Equal(t, "application/json", resp.Headers["Content-Type"])
}

func TestFetcher_FetchFileContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"test-etag"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"test-etag"`)
		w.Header().Set("Last-Modified", "some-date")
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("file content"))
	}))
	defer server.Close()

	logger := zerolog.Nop()
	client, err := NewHTTPClientBuilder(logger).Build()
	require.NoError(t, err)

	fetcher := NewFetcher(client, logger, &HTTPClientFetcherConfig{MaxContentSize: 1024})

	// First fetch
	input := FetchFileContentInput{URL: server.URL}
	result, err := fetcher.FetchFileContent(input)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, result.HTTPStatusCode)
	assert.Equal(t, "file content", string(result.Content))
	assert.Equal(t, `"test-etag"`, result.ETag)
	assert.Equal(t, "some-date", result.LastModified)

	// Second fetch with ETag to get 304 Not Modified
	input.PreviousETag = result.ETag
	input.PreviousLastModified = result.LastModified
	result2, err := fetcher.FetchFileContent(input)
	assert.ErrorIs(t, err, ErrNotModified)
	require.NotNil(t, result2)
	assert.Equal(t, http.StatusNotModified, result2.HTTPStatusCode)
}

func TestFetcher_ContentSizeLimit(t *testing.T) {
	longContent := "this is a very long string that will be truncated"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(longContent))
	}))
	defer server.Close()

	logger := zerolog.Nop()
	client, err := NewHTTPClientBuilder(logger).Build()
	require.NoError(t, err)

	fetcher := NewFetcher(client, logger, &HTTPClientFetcherConfig{MaxContentSize: 10})

	input := FetchFileContentInput{URL: server.URL}
	result, err := fetcher.FetchFileContent(input)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, result.HTTPStatusCode)
	assert.Equal(t, "this is a ", string(result.Content))
	assert.Len(t, result.Content, 10)
}

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

	resp, err := testClient.DoWithRetry(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(3), atomic.LoadInt32(&requestCount))
}

func TestHTTPClient_Redirects(t *testing.T) {
	var requestCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/final", http.StatusFound)
		} else if r.URL.Path == "/final" {
			fmt.Fprint(w, "ok")
		}
	}))
	defer ts.Close()

	logger := zerolog.Nop()

	// Test with follow redirects enabled
	clientFollow, _ := NewHTTPClientBuilder(logger).WithFollowRedirects(true).Build()
	req := &HTTPRequest{URL: ts.URL + "/redirect", Method: "GET"}
	resp, err := clientFollow.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "ok", string(resp.Body))

	// Test with follow redirects disabled
	clientNoFollow, _ := NewHTTPClientBuilder(logger).WithFollowRedirects(false).Build()
	resp, err = clientNoFollow.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusFound, resp.StatusCode)
}
