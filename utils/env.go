package utils

import (
	"os"
	"strings"
)

// getFromEnvOrDefault trims the value for key and returns def if it's empty.
func getFromEnvOrDefault(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// GetFromEnvOrDefault exposes getFromEnvOrDefault for other packages.
func GetFromEnvOrDefault(key, def string) string {
	return getFromEnvOrDefault(key, def)
}
