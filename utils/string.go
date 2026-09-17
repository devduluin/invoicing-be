package utils

import "strings"

// ParseCommaSeparated converts a comma-separated string into a trimmed slice.
func ParseCommaSeparated(fields string) []string {
	if strings.TrimSpace(fields) == "" {
		return nil
	}
	parts := strings.Split(fields, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// Contains reports whether slice contains val.
func Contains(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
