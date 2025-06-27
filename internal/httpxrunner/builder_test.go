package httpxrunner_test

import (
	"testing"

	telescope "github.com/aleister1102/monsterinc/internal/httpxrunner"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunnerBuilder_HappyPath(t *testing.T) {
	logger := zerolog.Nop()
	config := telescope.DefaultConfig()
	config.Targets = []string{"example.com"}

	builder := telescope.NewRunnerBuilder(logger).
		WithConfig(config).
		WithRootTargetURL("my-scan")

	runner, err := builder.Build()
	require.NoError(t, err)
	require.NotNil(t, runner)
}

func TestRunnerBuilder_MissingConfig(t *testing.T) {
	logger := zerolog.Nop()

	builder := telescope.NewRunnerBuilder(logger).
		WithRootTargetURL("my-scan")

	runner, err := builder.Build()
	require.Error(t, err)
	assert.Nil(t, runner)

	var builderErr telescope.ErrBuilderErrors
	require.ErrorAs(t, err, &builderErr)
	require.Len(t, builderErr, 1)
	assert.ErrorIs(t, builderErr[0], telescope.ErrConfigNotSet)
}

func TestRunnerBuilder_MissingRootURL(t *testing.T) {
	logger := zerolog.Nop()
	config := telescope.DefaultConfig()

	builder := telescope.NewRunnerBuilder(logger).
		WithConfig(config)

	runner, err := builder.Build()
	require.Error(t, err)
	assert.Nil(t, runner)

	var builderErr telescope.ErrBuilderErrors
	require.ErrorAs(t, err, &builderErr)
	require.Len(t, builderErr, 1)
	assert.ErrorIs(t, builderErr[0], telescope.ErrRootURLNotSet)
}

func TestRunnerBuilder_MultipleErrors(t *testing.T) {
	logger := zerolog.Nop()

	// No config, no root URL
	builder := telescope.NewRunnerBuilder(logger)

	runner, err := builder.Build()
	require.Error(t, err)
	assert.Nil(t, runner)

	var builderErr telescope.ErrBuilderErrors
	require.ErrorAs(t, err, &builderErr)
	assert.Len(t, builderErr, 2)

	// Check that both expected errors are present
	errorMap := make(map[error]bool)
	for _, e := range builderErr {
		errorMap[e] = true
	}
	assert.True(t, errorMap[telescope.ErrConfigNotSet])
	assert.True(t, errorMap[telescope.ErrRootURLNotSet])
}
