package notifier

// Notifier is an interface for sending notifications.
type Notifier interface {
	Notify(message interface{}) error
} 