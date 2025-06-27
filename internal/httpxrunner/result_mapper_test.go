package telescope_test

import (
	"testing"
	"time"

	"github.com/aleister1102/go-telescope"
	"github.com/projectdiscovery/httpx/runner"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProbeResultMapper_MapResult_HappyPath(t *testing.T) {
	logger := zerolog.Nop()
	mapper := telescope.NewProbeResultMapper(logger)
	now := time.Now()

	httpxResult := runner.Result{
		Input:         "http://test.com",
		URL:           "http://test.com",
		StatusCode:    200,
		ContentLength: 123,
		ContentType:   "text/plain",
		Title:         "Test Title",
		WebServer:     "Test Web Server",
		Timestamp:     now,
		ResponseTime:  "1.2s",
		Technologies:  []string{"Go", "React"},
		A:             []string{"1.2.3.4"},
		ResponseHeaders: map[string]interface{}{
			"Content-Type": "text/html",
			"X-Test":       "value",
		},
	}

	probeResult := mapper.MapResult(httpxResult, "http://root.com")
	require.NotNil(t, probeResult)

	assert.Equal(t, "http://test.com", probeResult.InputURL)
	assert.Equal(t, 200, probeResult.StatusCode)
	assert.Equal(t, int64(123), probeResult.ContentLength)
	assert.Equal(t, "text/plain", probeResult.ContentType)
	assert.Equal(t, "Test Title", probeResult.Title)
	assert.Equal(t, "Test Web Server", probeResult.WebServer)
	assert.Equal(t, now, probeResult.Timestamp)
	assert.InDelta(t, 1.2, probeResult.Duration, 0.01)
	assert.Equal(t, "http://root.com", probeResult.RootTargetURL)

	require.Len(t, probeResult.Technologies, 2)
	assert.Equal(t, "Go", probeResult.Technologies[0].Name)

	assert.Contains(t, probeResult.IPs, "1.2.3.4")

	require.Len(t, probeResult.Headers, 2)
	assert.Equal(t, "text/html", probeResult.Headers["Content-Type"])
}

func TestProbeResultMapper_EdgeCases(t *testing.T) {
	logger := zerolog.Nop()
	mapper := telescope.NewProbeResultMapper(logger)

	t.Run("Empty result", func(t *testing.T) {
		probeResult := mapper.MapResult(runner.Result{}, "")
		require.NotNil(t, probeResult)
		assert.Empty(t, probeResult.InputURL)
		assert.Zero(t, probeResult.StatusCode)
		assert.Empty(t, probeResult.Technologies)
		assert.Empty(t, probeResult.Headers)
	})

	t.Run("Invalid response time", func(t *testing.T) {
		res := runner.Result{ResponseTime: "not-a-duration"}
		probeResult := mapper.MapResult(res, "")
		assert.Zero(t, probeResult.Duration)
	})

	t.Run("Go duration format for response time", func(t *testing.T) {
		res := runner.Result{ResponseTime: "1m30s"}
		probeResult := mapper.MapResult(res, "")
		assert.InDelta(t, 90.0, probeResult.Duration, 0.01)
	})

	t.Run("Various header value types", func(t *testing.T) {
		headers := map[string]interface{}{
			"string-header": "value",
			"slice-header":  []string{"val1", "val2"},
			"iface-slice":   []interface{}{"v1", "v2"},
			"mixed-slice":   []interface{}{"v1", 123, true},
			"other-type":    12345,
		}
		res := runner.Result{ResponseHeaders: headers}
		probeResult := mapper.MapResult(res, "")

		assert.Equal(t, "value", probeResult.Headers["string-header"])
		assert.Equal(t, "val1, val2", probeResult.Headers["slice-header"])
		assert.Equal(t, "v1, v2", probeResult.Headers["iface-slice"])
		assert.Equal(t, "v1", probeResult.Headers["mixed-slice"])
		assert.Equal(t, "", probeResult.Headers["other-type"])
	})
}
