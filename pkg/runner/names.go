package runner

import (
	"regexp"
	"strings"
)

var (
	// sanitizeRegex matches characters that are not allowed in container names
	sanitizeRegex = regexp.MustCompile(`[^a-zA-Z0-9_.-]`)
	// leadingInvalidRegex matches leading characters that are not alphanumeric
	leadingInvalidRegex = regexp.MustCompile(`^[^a-zA-Z0-9]+`)
)

// SanitizeName converts a string to a valid container/hostname name.
// It replaces invalid characters with hyphens and removes leading non-alphanumeric characters.
func SanitizeName(name string) string {
	// Replace invalid characters with hyphens
	sanitized := sanitizeRegex.ReplaceAllString(name, "-")
	// Remove leading non-alphanumeric characters
	sanitized = leadingInvalidRegex.ReplaceAllString(sanitized, "")
	// Collapse multiple hyphens
	for strings.Contains(sanitized, "--") {
		sanitized = strings.ReplaceAll(sanitized, "--", "-")
	}
	// Trim trailing hyphens
	sanitized = strings.TrimSuffix(sanitized, "-")
	// Convert to lowercase
	sanitized = strings.ToLower(sanitized)
	// Ensure non-empty
	if sanitized == "" {
		sanitized = "container"
	}
	return sanitized
}
