package datastore_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/datastore"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecretsStore_StoreAndLoadFindings(t *testing.T) {
	logger := zerolog.Nop()
	tempDir, err := os.MkdirTemp("", "secrets-store-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	storageCfg := &config.StorageConfig{ParquetBasePath: tempDir}
	store, err := datastore.NewSecretsStore(storageCfg, logger)
	require.NoError(t, err)

	findings1 := []models.SecretFinding{
		{RuleID: "rule1", SecretText: "secret1", SourceURL: "url1"},
		{RuleID: "rule2", SecretText: "secret2", SourceURL: "url2"},
	}

	// 1. Store and Load
	t.Run("store and load", func(t *testing.T) {
		err := store.StoreFindings(context.Background(), findings1)
		require.NoError(t, err)

		loadedFindings, err := store.LoadFindings(context.Background())
		require.NoError(t, err)
		assert.Equal(t, findings1, loadedFindings)
	})

	// 2. Append new findings
	t.Run("append findings", func(t *testing.T) {
		findings2 := []models.SecretFinding{
			{RuleID: "rule3", SecretText: "secret3", SourceURL: "url3"},
		}
		err := store.StoreFindings(context.Background(), findings2)
		require.NoError(t, err)

		loadedFindings, err := store.LoadFindings(context.Background())
		require.NoError(t, err)
		assert.Len(t, loadedFindings, 3)
		expectedFindings := append(findings1, findings2...)
		assert.Equal(t, expectedFindings, loadedFindings)
	})
}

func TestSecretsStore_EmptyStore(t *testing.T) {
	logger := zerolog.Nop()
	tempDir, err := os.MkdirTemp("", "empty-store-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	storageCfg := &config.StorageConfig{ParquetBasePath: tempDir}
	store, err := datastore.NewSecretsStore(storageCfg, logger)
	require.NoError(t, err)

	// Load from a non-existent file
	loadedFindings, err := store.LoadFindings(context.Background())
	require.NoError(t, err)
	assert.Empty(t, loadedFindings)
}

func TestSecretsStore_NoBasePath(t *testing.T) {
	logger := zerolog.Nop()
	// Empty config
	storageCfg := &config.StorageConfig{}
	store, err := datastore.NewSecretsStore(storageCfg, logger)
	require.NoError(t, err)

	// Store should fail
	err = store.StoreFindings(context.Background(), []models.SecretFinding{{RuleID: "test"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ParquetBasePath is not configured")

	// Load should also fail
	_, err = store.LoadFindings(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ParquetBasePath is not configured")
}

func TestSecretsStore_StoreEmptyFindings(t *testing.T) {
	logger := zerolog.Nop()
	tempDir, err := os.MkdirTemp("", "empty-findings-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	storageCfg := &config.StorageConfig{ParquetBasePath: tempDir}
	store, err := datastore.NewSecretsStore(storageCfg, logger)
	require.NoError(t, err)

	// Storing an empty slice should be a no-op
	err = store.StoreFindings(context.Background(), []models.SecretFinding{})
	require.NoError(t, err)

	// The directory should be created, but the file shouldn't
	secretsDir := filepath.Join(tempDir, "secrets")
	secretsFile := filepath.Join(secretsDir, "secrets.parquet")
	_, err = os.Stat(secretsFile)
	assert.True(t, os.IsNotExist(err), "Parquet file should not be created for empty findings")
}
