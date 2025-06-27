package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/projectdiscovery/httpx/runner"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunnerBuilder(t *testing.T) {
	logger := zerolog.Nop()
	config := DefaultConfig()
	config.Threads = 5

	builder := NewRunnerBuilder(logger).
		WithConfig(config).
		WithRootTargetURL("http://example.com")

	r, err := builder.Build()
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, 5, r.config.Threads)
	assert.Equal(t, "http://example.com", r.rootTargetURL)

	// Test build with missing config
	_, err = NewRunnerBuilder(logger).WithRootTargetURL("http://example.com").WithConfig(nil).Build()
	require.Error(t, err)

	// Test build with missing root url
	_, err = NewRunnerBuilder(logger).WithConfig(config).WithRootTargetURL("").Build()
	require.Error(t, err)
}

func TestRunner_Run(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Server", "TestServer")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><head><title>Test Page</title></head></html>"))
	}))
	defer server.Close()

	logger := zerolog.Nop()
	config := DefaultConfig()
	config.Threads = 1
	config.Targets = []string{server.URL}

	r, err := NewRunnerBuilder(logger).
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

func TestResultCollector_Concurrent(t *testing.T) {
	logger := zerolog.Nop()
	collector := NewResultCollector(logger)
	var wg sync.WaitGroup
	numRoutines := 10
	numResultsPerRoutine := 100

	for i := 0; i < numRoutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numResultsPerRoutine; j++ {
				collector.AddResult(&ProbeResult{InputURL: "http://example.com"})
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, numRoutines*numResultsPerRoutine, collector.GetResultsCount())
	results := collector.GetResults()
	assert.Len(t, results, numRoutines*numResultsPerRoutine)
}

func TestProbeResultMapper(t *testing.T) {
	logger := zerolog.Nop()
	mapper := NewProbeResultMapper(logger)
	httpxResult := runner.Result{
		Input:         "http://test.com",
		URL:           "http://test.com",
		StatusCode:    200,
		ContentLength: 123,
		ContentType:   "text/plain",
		Title:         "Test Title",
		WebServer:     "Test Web Server",
		Timestamp:     time.Now(),
		ResponseTime:  "1.2s",
		Technologies:  []string{"Go", "React"},
		A:             []string{"1.2.3.4"},
		CNAMEs:        []string{"cname.test.com"},
	}

	probeResult := mapper.MapResult(httpxResult, "http://root.com")
	require.NotNil(t, probeResult)

	assert.Equal(t, "http://test.com", probeResult.InputURL)
	assert.Equal(t, 200, probeResult.StatusCode)
	assert.Equal(t, int64(123), probeResult.ContentLength)
	assert.Equal(t, "text/plain", probeResult.ContentType)
	assert.Equal(t, "Test Title", probeResult.Title)
	assert.Equal(t, "Test Web Server", probeResult.WebServer)
	assert.InDelta(t, 1.2, probeResult.Duration, 0.01)
	require.Len(t, probeResult.Technologies, 2)
	assert.Equal(t, "Go", probeResult.Technologies[0].Name)
	assert.Equal(t, "React", probeResult.Technologies[1].Name)
	assert.Contains(t, probeResult.IPs, "1.2.3.4")
	assert.Equal(t, "http://root.com", probeResult.RootTargetURL)
}

func TestOptionsConfigurator(t *testing.T) {
	logger := zerolog.Nop()
	configurator := NewHTTPXOptionsConfigurator(logger)

	cfg := DefaultConfig()
	cfg.Threads = 50
	cfg.Timeout = 15
	cfg.Method = "POST"
	cfg.CustomHeaders = map[string]string{"X-Custom": "Value"}
	cfg.ExtractBody = true
	cfg.TechDetect = false

	options := configurator.ConfigureOptions(cfg)
	require.NotNil(t, options)

	assert.Equal(t, 50, options.Threads)
	assert.Equal(t, 15, options.Timeout)
	assert.Equal(t, "POST", options.Methods)
	assert.False(t, options.OmitBody)
	assert.False(t, options.TechDetect)

	require.Len(t, options.CustomHeaders, 1)
	assert.Equal(t, "X-Custom: Value", options.CustomHeaders[0])
}
