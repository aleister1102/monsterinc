package notifier

import "github.com/aleister1102/monsterinc/internal/models"

// Notifier is an interface for sending notifications.
type Notifier interface {
	SendSecretNotification(finding models.SecretFinding) error
}
