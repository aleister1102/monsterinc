package crawler

// getValueOrDefault returns value if not empty, otherwise returns default
func getValueOrDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

// getIntValueOrDefault returns value if greater than 0, otherwise returns default
func getIntValueOrDefault(value, defaultValue int) int {
	if value <= 0 {
		return defaultValue
	}
	return value
}
