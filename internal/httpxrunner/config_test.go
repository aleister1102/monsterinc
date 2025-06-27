package httpxrunner_test

import (
	"testing"

	telescope "github.com/aleister1102/monsterinc/internal/httpxrunner"
	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	config := telescope.DefaultConfig()

	assert.NotNil(t, config)
	assert.Equal(t, 25, config.Threads)
	assert.Equal(t, 10, config.Timeout)
	assert.Equal(t, 1, config.Retries)
	assert.Equal(t, "GET", config.Method)
	assert.True(t, config.FollowRedirects)
	assert.Equal(t, 0, config.RateLimit)
	assert.Empty(t, config.CustomHeaders)
	assert.Empty(t, config.RequestURIs)
	assert.True(t, config.ExtractTitle)
	assert.True(t, config.ExtractStatusCode)
	assert.True(t, config.ExtractContentLength)
	assert.True(t, config.ExtractHeaders)
	assert.False(t, config.ExtractBody)
	assert.True(t, config.TechDetect)
	assert.False(t, config.Verbose)
}
