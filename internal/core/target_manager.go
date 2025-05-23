package core

import (
	"bufio"
	"monsterinc/internal/models"
	"monsterinc/internal/urlhandler"
	"os"
)

// TargetManager handles loading and normalizing targets.
type TargetManager struct {
	// We can add configuration here if needed later, e.g., for concurrent processing
}

// NewTargetManager creates a new TargetManager.
func NewTargetManager() *TargetManager {
	return &TargetManager{}
}

// LoadTargetsFromFile reads URLs from a given file path, normalizes them,
// and returns a slice of Target structs.
// It skips empty lines and lines that result in an error during normalization.
func (tm *TargetManager) LoadTargetsFromFile(filePath string) ([]models.Target, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var targets []models.Target
	scanner := bufio.NewScanner(file)
	// For concurrent normalization if needed, though for now, sequential is fine.
	// var wg sync.WaitGroup
	// mu := &sync.Mutex{}

	for scanner.Scan() {
		originalURL := scanner.Text()
		if originalURL == "" {
			continue // Skip empty lines
		}

		// NormalizeURL can be called concurrently if the list is very large
		// For now, keeping it simple and sequential.
		normalizedURL, err := urlhandler.NormalizeURL(originalURL)
		if err != nil {
			// Optionally log this error or handle it more gracefully
			// fmt.Printf("Skipping URL %s due to normalization error: %v\n", originalURL, err)
			continue
		}
		targets = append(targets, models.Target{OriginalURL: originalURL, NormalizedURL: normalizedURL})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return targets, nil
}
