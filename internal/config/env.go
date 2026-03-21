package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// envString returns the value of the environment variable with the given key,
// or the fallback value if the environment variable is not set.
func envString(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// envInt returns the integer value of the environment variable with the given key,
// or the fallback value if the environment variable is not set.
// Panics if the value cannot be converted to an integer.
func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	intVal, err := strconv.Atoi(value)
	if err != nil {
		panic(fmt.Sprintf("invalid integer value for %s: %v", key, value))
	}
	return intVal
}

// envInt64 returns the int64 value of the environment variable with the given key,
// or the fallback value if the environment variable is not set.
// Panics if the value cannot be converted to an int64.
func envInt64(key string, fallback int64) int64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	int64Val, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		panic(fmt.Sprintf("invalid int64 value for %s: %v", key, value))
	}
	return int64Val
}

// envBool returns the boolean value of the environment variable with the given key,
// or the fallback value if the environment variable is not set.
// Panics if the value cannot be converted to a boolean.
func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	boolVal, err := strconv.ParseBool(value)
	if err != nil {
		panic(fmt.Sprintf("invalid boolean value for %s: %v", key, value))
	}
	return boolVal
}

// envDuration returns the duration value of the environment variable with the given key,
// or the fallback value if the environment variable is not set.
// Panics if the value cannot be converted to a duration.
func envDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	durationVal, err := time.ParseDuration(value)
	if err != nil {
		panic(fmt.Sprintf("invalid duration value for %s: %v", key, value))
	}
	return durationVal
}

// envStringSlice returns a slice of strings from a comma-separated environment variable,
// or the fallback value if the environment variable is not set.
func envStringSlice(key string, fallback []string) []string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
