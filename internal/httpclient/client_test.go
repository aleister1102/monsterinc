package comet

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestHTTPClient_Do_Post(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.Equal(t, `{"key":"value"}`, string(body))

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"received":true}`))
	}))
	defer server.Close()

	logger := zerolog.Nop()
	client, err := NewHTTPClientBuilder(logger).Build()
	require.NoError(t, err)

	requestBody := []byte(`{"key":"value"}`)
	req := &HTTPRequest{
		URL:    server.URL,
		Method: "POST",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: bytes.NewReader(requestBody),
	}

	resp, err := client.Do(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, `{"received":true}`, string(resp.Body))
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

func TestHTTPClient_FetchContent_Simple(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("ETag", "v1")
		_, _ = w.Write([]byte("some content"))
	}))
	defer server.Close()

	logger := zerolog.Nop()
	client, err := NewHTTPClientBuilder(logger).Build()
	require.NoError(t, err)

	input := FetchContentInput{
		URL: server.URL,
	}

	result, err := client.FetchContent(input)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, result.HTTPStatusCode)
	assert.Equal(t, "some content", string(result.Content))
	assert.Equal(t, "text/plain", result.ContentType)
	assert.Equal(t, "v1", result.ETag)
}

func TestHTTPClient_FetchContent_NotModified(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		etag := r.Header.Get("If-None-Match")
		if etag == "v1" {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", "v1")
		_, _ = w.Write([]byte("some content"))
	}))
	defer server.Close()

	logger := zerolog.Nop()
	client, err := NewHTTPClientBuilder(logger).Build()
	require.NoError(t, err)

	input := FetchContentInput{
		URL:          server.URL,
		PreviousETag: "v1",
	}

	result, err := client.FetchContent(input)
	require.ErrorIs(t, err, ErrNotModified)
	assert.Equal(t, http.StatusNotModified, result.HTTPStatusCode)
}

func TestHTTPClient_FetchContent_MaxSize(t *testing.T) {
	longContent := "this is some very long content"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(longContent))
	}))
	defer server.Close()

	logger := zerolog.Nop()
	client, err := NewHTTPClientBuilder(logger).WithMaxContentSize(10).Build()
	require.NoError(t, err)

	input := FetchContentInput{
		URL: server.URL,
	}

	result, err := client.FetchContent(input)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, result.HTTPStatusCode)
	assert.Equal(t, "this is so", string(result.Content)) // Truncated to 10 bytes
	assert.Len(t, result.Content, 10)
}

func TestHTTPClient_FetchContent_BypassCache(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "no-cache", r.Header.Get("Pragma"))
		assert.Equal(t, "no-cache, no-store, must-revalidate", r.Header.Get("Cache-Control"))
		_, _ = w.Write([]byte("fresh content"))
	}))
	defer server.Close()

	logger := zerolog.Nop()
	client, err := NewHTTPClientBuilder(logger).Build()
	require.NoError(t, err)

	input := FetchContentInput{
		URL:         server.URL,
		BypassCache: true,
	}

	result, err := client.FetchContent(input)
	require.NoError(t, err)
	assert.Equal(t, "fresh content", string(result.Content))
}
