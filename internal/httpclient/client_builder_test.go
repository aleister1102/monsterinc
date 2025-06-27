package comet

import (
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

func TestHTTPClientBuilder_DefaultValues(t *testing.T) {
	logger := zerolog.Nop()
	builder := NewHTTPClientBuilder(logger)

	client, err := builder.Build()
	require.NoError(t, err)

	defaults := DefaultHTTPClientConfig()

	assert.NotNil(t, client)
	assert.Equal(t, defaults.Timeout, client.config.Timeout)
	assert.Equal(t, defaults.UserAgent, client.config.UserAgent)
	assert.Equal(t, defaults.FollowRedirects, client.config.FollowRedirects)
	assert.Equal(t, defaults.InsecureSkipVerify, client.config.InsecureSkipVerify)
	assert.Equal(t, defaults.MaxRedirects, client.config.MaxRedirects)
}
