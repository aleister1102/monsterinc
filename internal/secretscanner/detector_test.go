package secretscanner_test

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/datastore"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/secretscanner"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockNotifier is a mock implementation of the Notifier interface.
type MockNotifier struct {
	SendSecretNotificationFunc func(finding models.SecretFinding) error
	sentNotifications          []models.SecretFinding
	mu                         sync.Mutex
}

func (m *MockNotifier) SendSecretNotification(finding models.SecretFinding) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentNotifications = append(m.sentNotifications, finding)
	if m.SendSecretNotificationFunc != nil {
		return m.SendSecretNotificationFunc(finding)
	}
	return nil
}

func (m *MockNotifier) GetSentNotifications() []models.SecretFinding {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Return a copy to be safe
	notifications := make([]models.SecretFinding, len(m.sentNotifications))
	copy(notifications, m.sentNotifications)
	return notifications
}

func (m *MockNotifier) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentNotifications = nil
}

// newTestSecretsStore creates a real SecretsStore in a temporary directory for testing.
func newTestSecretsStore(t *testing.T) (*datastore.SecretsStore, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "secrets-store-test")
	require.NoError(t, err)

	storageCfg := &config.StorageConfig{
		ParquetBasePath: tempDir,
	}
	logger := zerolog.Nop()
	store, err := datastore.NewSecretsStore(storageCfg, logger)
	require.NoError(t, err)

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return store, cleanup
}

func TestDetector_ScanAndProcess(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("scanning disabled", func(t *testing.T) {
		cfg := config.SecretsConfig{Enabled: false}
		// No need for store or notifier if disabled
		detector, err := secretscanner.NewDetector(&cfg, nil, nil, logger)
		require.NoError(t, err)

		// This should be a no-op and not panic
		detector.ScanAndProcess("test.js", []byte("var secret = 'sk_live_12345';"))
	})

	t.Run("no secrets found", func(t *testing.T) {
		cfg := config.SecretsConfig{Enabled: true}
		store, cleanup := newTestSecretsStore(t)
		defer cleanup()
		mockNotifier := &MockNotifier{}

		detector, err := secretscanner.NewDetector(&cfg, store, mockNotifier, logger)
		require.NoError(t, err)

		detector.ScanAndProcess("test.js", []byte("var hello = 'world';"))

		// Verify no notifications were sent
		assert.Empty(t, mockNotifier.GetSentNotifications())

		// Verify nothing was written to the store
		findings, err := store.LoadFindings(context.Background())
		require.NoError(t, err)
		assert.Empty(t, findings)
	})

	t.Run("secrets found and notifications enabled", func(t *testing.T) {
		cfg := config.SecretsConfig{Enabled: true, NotifyOnFound: true}
		store, cleanup := newTestSecretsStore(t)
		defer cleanup()
		mockNotifier := &MockNotifier{}
		mockNotifier.Reset()

		detector, err := secretscanner.NewDetector(&cfg, store, mockNotifier, logger)
		require.NoError(t, err)

		// This secret key matches the "Generic API Key" rule
		secret := "sk-a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5" // 43 chars
		content := `{"ApiKey": "` + secret + `"}`
		detector.ScanAndProcess("config.json", []byte(content))

		// Check notifications
		notifications := mockNotifier.GetSentNotifications()
		require.NotEmpty(t, notifications, "Should have sent notifications for a found secret")
		assert.Equal(t, "Generic API Key", notifications[0].RuleID)
		assert.Equal(t, secret, notifications[0].SecretText)

		// Check stored findings
		storedFindings, err := store.LoadFindings(context.Background())
		require.NoError(t, err)
		require.NotEmpty(t, storedFindings, "Should have stored the found secret")
		assert.Equal(t, "Generic API Key", storedFindings[0].RuleID)
	})

	t.Run("secrets found but notifications disabled", func(t *testing.T) {
		cfg := config.SecretsConfig{Enabled: true, NotifyOnFound: false}
		store, cleanup := newTestSecretsStore(t)
		defer cleanup()
		mockNotifier := &MockNotifier{}
		mockNotifier.Reset()

		detector, err := secretscanner.NewDetector(&cfg, store, mockNotifier, logger)
		require.NoError(t, err)

		// This token matches the "GitHub Personal Access Token" rule
		secret := "ghp_a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6" // 36 chars
		content := `const token = "` + secret + `"`
		detector.ScanAndProcess("main.js", []byte(content))

		// Check no notifications were sent
		assert.Empty(t, mockNotifier.GetSentNotifications())

		// Check findings were still stored
		storedFindings, err := store.LoadFindings(context.Background())
		require.NoError(t, err)
		require.NotEmpty(t, storedFindings, "Should have stored the found secret")
		assert.Equal(t, "GitHub Personal Access Token", storedFindings[0].RuleID)
		assert.Equal(t, secret, storedFindings[0].SecretText)
	})
}
