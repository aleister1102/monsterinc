package models

import (
	"time"

	httpx "github.com/aleister1102/go-telescope"
)

// Validator interface for models that can validate themselves
type Validator interface {
	Validate() error
}

// Timestamped interface for models that have timestamp information
type Timestamped interface {
	GetTimestamp() time.Time
}

// Identifiable interface for models that have unique identifiers
type Identifiable interface {
	GetID() string
}

// Serializable interface for models that can serialize themselves
type Serializable interface {
	ToJSON() ([]byte, error)
	FromJSON([]byte) error
}

// StatusProvider interface for models that have status information
type StatusProvider interface {
	GetStatus() string
	SetStatus(string)
}

// Counter interface for models that can count specific items
type Counter interface {
	Count() int
}

// URLProvider interface for models that contain URL information
type URLProvider interface {
	GetURL() string
}

// ContentProvider interface for models that contain content
type ContentProvider interface {
	GetContent() []byte
	GetContentType() string
}

// ErrorProvider interface for models that can contain error information
type ErrorProvider interface {
	GetError() string
	HasError() bool
}

// TechnologyDetector interface for models that can detect technologies
type TechnologyDetector interface {
	HasTechnologies() bool
	GetTechnologies() []httpx.Technology
}

// DiffResult interface for models that represent diff results
type DiffResult interface {
	GetDiffs() interface{}
	IsIdentical() bool
}
