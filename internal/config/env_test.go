package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEnvString(t *testing.T) {
	key := "TEST_STRING"
	defer os.Unsetenv(key)

	// Test with unset variable
	assert.Equal(t, "default", envString(key, "default"))

	// Test with set variable
	os.Setenv(key, "custom")
	assert.Equal(t, "custom", envString(key, "default"))
}

func TestEnvInt(t *testing.T) {
	key := "TEST_INT"
	defer os.Unsetenv(key)

	// Test with unset variable
	assert.Equal(t, 42, envInt(key, 42))

	// Test with valid integer
	os.Setenv(key, "100")
	assert.Equal(t, 100, envInt(key, 42))

	// Test with invalid integer (should panic)
	os.Setenv(key, "not-a-number")
	assert.Panics(t, func() {
		envInt(key, 42)
	})
}

func TestEnvInt64(t *testing.T) {
	key := "TEST_INT64"
	defer os.Unsetenv(key)

	// Test with unset variable
	assert.Equal(t, int64(123456789), envInt64(key, 123456789))

	// Test with valid int64
	os.Setenv(key, "987654321")
	assert.Equal(t, int64(987654321), envInt64(key, 123456789))

	// Test with invalid int64 (should panic)
	os.Setenv(key, "invalid")
	assert.Panics(t, func() {
		envInt64(key, 123456789)
	})
}

func TestEnvBool(t *testing.T) {
	key := "TEST_BOOL"
	defer os.Unsetenv(key)

	// Test with unset variable
	assert.Equal(t, true, envBool(key, true))

	// Test with valid boolean
	os.Setenv(key, "false")
	assert.Equal(t, false, envBool(key, true))

	os.Setenv(key, "true")
	assert.Equal(t, true, envBool(key, false))

	os.Setenv(key, "1")
	assert.Equal(t, true, envBool(key, false))

	os.Setenv(key, "0")
	assert.Equal(t, false, envBool(key, true))

	// Test with invalid boolean (should panic)
	os.Setenv(key, "maybe")
	assert.Panics(t, func() {
		envBool(key, false)
	})
}

func TestEnvDuration(t *testing.T) {
	key := "TEST_DURATION"
	defer os.Unsetenv(key)

	// Test with unset variable
	assert.Equal(t, 30*time.Second, envDuration(key, 30*time.Second))

	// Test with valid duration
	os.Setenv(key, "1h")
	assert.Equal(t, 1*time.Hour, envDuration(key, 30*time.Second))

	os.Setenv(key, "500ms")
	assert.Equal(t, 500*time.Millisecond, envDuration(key, 30*time.Second))

	// Test with invalid duration (should panic)
	os.Setenv(key, "not-a-duration")
	assert.Panics(t, func() {
		envDuration(key, 30*time.Second)
	})
}

func TestEnvStringSlice(t *testing.T) {
	key := "TEST_STRING_SLICE"
	defer os.Unsetenv(key)

	// Test with unset variable - should return fallback
	fallback := []string{"a", "b", "c"}
	result := envStringSlice(key, fallback)
	assert.Equal(t, fallback, result)

	// Test with single value
	os.Setenv(key, "single")
	result = envStringSlice(key, fallback)
	assert.Equal(t, []string{"single"}, result)

	// Test with comma-separated values
	os.Setenv(key, "one,two,three")
	result = envStringSlice(key, fallback)
	assert.Equal(t, []string{"one", "two", "three"}, result)

	// Test with spaces around values (should be trimmed)
	os.Setenv(key, " one , two , three ")
	result = envStringSlice(key, fallback)
	assert.Equal(t, []string{"one", "two", "three"}, result)

	// Test with empty string - should return empty slice
	os.Setenv(key, "")
	result = envStringSlice(key, fallback)
	assert.Equal(t, fallback, result)

	// Test with trailing/leading commas (empty parts should be ignored)
	os.Setenv(key, ",one,two,")
	result = envStringSlice(key, fallback)
	assert.Equal(t, []string{"one", "two"}, result)

	// Test with multiple consecutive commas (empty parts should be ignored)
	os.Setenv(key, "one,,two,,,three")
	result = envStringSlice(key, fallback)
	assert.Equal(t, []string{"one", "two", "three"}, result)

	// Test with only commas (should return empty slice)
	os.Setenv(key, ",,,")
	result = envStringSlice(key, fallback)
	assert.Equal(t, []string{}, result)
}
