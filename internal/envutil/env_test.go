package envutil

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
	assert.Equal(t, "default", EnvString(key, "default"))

	// Test with set variable
	os.Setenv(key, "custom")
	assert.Equal(t, "custom", EnvString(key, "default"))
}

func TestEnvInt(t *testing.T) {
	key := "TEST_INT"
	defer os.Unsetenv(key)

	// Test with unset variable
	assert.Equal(t, 42, EnvInt(key, 42))

	// Test with valid integer
	os.Setenv(key, "100")
	assert.Equal(t, 100, EnvInt(key, 42))

	// Test with invalid integer (should panic)
	os.Setenv(key, "not-a-number")
	assert.Panics(t, func() {
		EnvInt(key, 42)
	})
}

func TestEnvInt64(t *testing.T) {
	key := "TEST_INT64"
	defer os.Unsetenv(key)

	// Test with unset variable
	assert.Equal(t, int64(123456789), EnvInt64(key, 123456789))

	// Test with valid int64
	os.Setenv(key, "987654321")
	assert.Equal(t, int64(987654321), EnvInt64(key, 123456789))

	// Test with invalid int64 (should panic)
	os.Setenv(key, "invalid")
	assert.Panics(t, func() {
		EnvInt64(key, 123456789)
	})
}

func TestEnvBool(t *testing.T) {
	key := "TEST_BOOL"
	defer os.Unsetenv(key)

	// Test with unset variable
	assert.Equal(t, true, EnvBool(key, true))

	// Test with valid boolean
	os.Setenv(key, "false")
	assert.Equal(t, false, EnvBool(key, true))

	os.Setenv(key, "true")
	assert.Equal(t, true, EnvBool(key, false))

	os.Setenv(key, "1")
	assert.Equal(t, true, EnvBool(key, false))

	os.Setenv(key, "0")
	assert.Equal(t, false, EnvBool(key, true))

	// Test with invalid boolean (should panic)
	os.Setenv(key, "maybe")
	assert.Panics(t, func() {
		EnvBool(key, false)
	})
}

func TestEnvDuration(t *testing.T) {
	key := "TEST_DURATION"
	defer os.Unsetenv(key)

	// Test with unset variable
	assert.Equal(t, 30*time.Second, EnvDuration(key, 30*time.Second))

	// Test with valid duration
	os.Setenv(key, "1h")
	assert.Equal(t, 1*time.Hour, EnvDuration(key, 30*time.Second))

	os.Setenv(key, "500ms")
	assert.Equal(t, 500*time.Millisecond, EnvDuration(key, 30*time.Second))

	// Test with invalid duration (should panic)
	os.Setenv(key, "not-a-duration")
	assert.Panics(t, func() {
		EnvDuration(key, 30*time.Second)
	})
}
