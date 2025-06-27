package httpxrunner_test

import (
	"testing"

	telescope "github.com/aleister1102/monsterinc/internal/httpxrunner"
	"github.com/stretchr/testify/assert"
)

func TestOptionsConfigurator_GetOptions(t *testing.T) {
	config := telescope.DefaultConfig()
	config.Targets = []string{"example.com"}
	config.Threads = 50
	config.Timeout = 20
	config.Retries = 3
	config.Method = "POST"
	config.FollowRedirects = false
	config.RateLimit = 100
	config.CustomHeaders = []string{"X-Test: true", "User-Agent: MyBot"}
	config.RequestURIs = []string{"/testpath"}
	config.Verbose = true
	config.ExtractTitle = false
	config.ExtractStatusCode = false
	config.ExtractContentLength = false
	config.ExtractBody = true
	config.ExtractHeaders = false
	config.TechDetect = false

	configurator := telescope.NewOptionsConfigurator(config, "http://root.url", nil)
	options := configurator.GetOptions()

	assert.Equal(t, config.Method, options.Methods)
	assert.True(t, options.Silent)
	assert.Equal(t, config.Verbose, options.Verbose)
	assert.Equal(t, config.Timeout, options.Timeout)
	assert.Equal(t, config.Retries, options.Retries)
	assert.Equal(t, config.FollowRedirects, options.FollowRedirects)
	assert.Equal(t, config.RateLimit, options.RateLimit)
	assert.Equal(t, config.Targets, []string(options.InputTargetHost))
	assert.Equal(t, "/testpath", options.RequestURI)
	assert.Equal(t, config.Threads, options.Threads)
	assert.Nil(t, options.OnResult)
	assert.Equal(t, config.ExtractTitle, options.ExtractTitle)
	assert.Equal(t, config.ExtractStatusCode, options.StatusCode)
	assert.Equal(t, config.ExtractContentLength, options.ContentLength)
	assert.Equal(t, !config.ExtractBody, options.OmitBody)
	assert.Equal(t, config.ExtractHeaders, options.ResponseHeadersInStdout)
	assert.Equal(t, config.TechDetect, options.TechDetect)

	// Check custom headers
	assert.Len(t, options.CustomHeaders, 2)
	assert.Equal(t, "X-Test: true", options.CustomHeaders[0])
	assert.Equal(t, "User-Agent: MyBot", options.CustomHeaders[1])
}
