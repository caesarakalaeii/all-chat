// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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
