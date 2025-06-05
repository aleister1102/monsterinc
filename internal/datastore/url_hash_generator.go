package datastore

import (
	"crypto/sha256"
	"encoding/hex"
)

// URLHashGenerator handles URL hash generation
type URLHashGenerator struct {
	hashLength int
}

// NewURLHashGenerator creates a new URL hash generator
func NewURLHashGenerator(hashLength int) *URLHashGenerator {
	if hashLength <= 0 || hashLength > 64 {
		hashLength = 16 // Default hash length
	}
	return &URLHashGenerator{
		hashLength: hashLength,
	}
}

// GenerateHash creates a unique hash for the URL
func (uhg *URLHashGenerator) GenerateHash(url string) string {
	hasher := sha256.New()
	hasher.Write([]byte(url))
	return hex.EncodeToString(hasher.Sum(nil))[:uhg.hashLength]
}
