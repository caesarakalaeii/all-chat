package listener_test

import (
	"testing"

	"github.com/caesar/all-chat/shared/listener"
	"github.com/stretchr/testify/assert"
)

func TestEnv_DefaultWhenAbsent(t *testing.T) {
	// Ensure key is not set
	t.Setenv("TEST_ENV_ABSENT_KEY_XYZ", "")

	result := listener.Env("TEST_ENV_ABSENT_KEY_XYZ", "default-value")
	assert.Equal(t, "default-value", result)
}

func TestEnv_ValueWhenSet(t *testing.T) {
	t.Setenv("TEST_ENV_PRESENT_KEY_XYZ", "actual-value")

	result := listener.Env("TEST_ENV_PRESENT_KEY_XYZ", "default-value")
	assert.Equal(t, "actual-value", result)
}

func TestEnv_DefaultWhenEmpty(t *testing.T) {
	t.Setenv("TEST_ENV_EMPTY_KEY_XYZ", "")

	result := listener.Env("TEST_ENV_EMPTY_KEY_XYZ", "default-value")
	assert.Equal(t, "default-value", result,
		"Env should treat empty string as absent and return the default")
}
