package utils

// ParseBoolQuery converts a query string value to *bool.
// Returns nil when the value is absent or not a recognised boolean.
func ParseBoolQuery(v string) *bool {
	switch v {
	case "true", "1":
		t := true
		return &t
	case "false", "0":
		f := false
		return &f
	default:
		return nil
	}
}
