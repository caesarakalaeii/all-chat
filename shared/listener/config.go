package listener

import "os"

// Env returns the value of the environment variable key,
// or defaultValue if the variable is unset or empty.
// Drop-in replacement for the getEnvOrDefault pattern used across all listeners.
func Env(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
