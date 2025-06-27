package httpxrunner_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	telescope "github.com/aleister1102/monsterinc/internal/httpxrunner"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunner_Run(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Server", "TestServer")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><head><title>Test Page</title></head></html>"))
	}))
	defer server.Close()

	logger := zerolog.Nop()
	config := telescope.DefaultConfig()
	config.Threads = 1
	config.Targets = []string{server.URL}

	r, err := telescope.NewRunnerBuilder(logger).
		WithConfig(config).
		WithRootTargetURL(server.URL).
		Build()
	require.NoError(t, err)

	err = r.Run(context.Background())
	require.NoError(t, err)

	results := r.GetResults()
	require.Len(t, results, 1)

	result := results[0]
	assert.Equal(t, server.URL, result.InputURL)
	assert.Equal(t, http.StatusOK, result.StatusCode)
	assert.Equal(t, "Test Page", result.Title)
	assert.Contains(t, result.ContentType, "text/html")
	assert.Equal(t, "TestServer", result.WebServer)
}
