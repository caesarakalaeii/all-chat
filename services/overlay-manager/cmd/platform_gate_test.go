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

package main

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubGates is a GateChecker with explicit premium and early-access flags.
type stubGates struct{ premium, earlyAccess map[string]bool }

func (s stubGates) IsPremium(key string) bool     { return s.premium[key] }
func (s stubGates) IsEarlyAccess(key string) bool { return s.earlyAccess[key] }

// gateFor returns a platform gate whose caller has the given entitlements.
func gateFor(gates stubGates, premium, earlyAccess bool) platformSourceGate {
	return platformSourceGate{gates: gates, entitlements: func(context.Context, string) (bool, bool, error) {
		return premium, earlyAccess, nil
	}}
}

// Facebook and Instagram run on a Meta app that is not approved yet, so only
// users who can be app testers (beta testers and ambassadors) may add them:
// their gates are premium AND early access.
var metaGates = stubGates{
	premium:     map[string]bool{"platform_facebook": true, "platform_instagram": true},
	earlyAccess: map[string]bool{"platform_facebook": true, "platform_instagram": true},
}

func TestPlatformSourceGate_EarlyAccessRefusesPremiumOnlyUser(t *testing.T) {
	for _, platform := range []string{"facebook", "instagram"} {
		allowed, err := gateFor(metaGates, true, false).PlatformSourceAllowed(context.Background(), "u1", platform)
		require.NoError(t, err)
		assert.False(t, allowed, "%s: a premium user who is not a beta tester must be refused", platform)
	}
}

func TestPlatformSourceGate_EarlyAccessAllowsBetaTester(t *testing.T) {
	// Beta testers and ambassadors are premium too (shared/premium.Recompute).
	allowed, err := gateFor(metaGates, true, true).PlatformSourceAllowed(context.Background(), "u1", "facebook")
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestPlatformSourceGate_PremiumGateStillAllowsPremiumUser(t *testing.T) {
	gates := stubGates{premium: map[string]bool{"platform_goodgame": true}}

	allowed, err := gateFor(gates, true, false).PlatformSourceAllowed(context.Background(), "u1", "goodgame")
	require.NoError(t, err)
	assert.True(t, allowed)

	allowed, err = gateFor(gates, false, false).PlatformSourceAllowed(context.Background(), "u1", "goodgame")
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestPlatformSourceGate_OpenGateSkipsLookup(t *testing.T) {
	g := platformSourceGate{gates: stubGates{}, entitlements: func(context.Context, string) (bool, bool, error) {
		t.Fatal("an open gate must not read the user row")
		return false, false, nil
	}}

	allowed, err := g.PlatformSourceAllowed(context.Background(), "u1", "goodgame")
	require.NoError(t, err)
	assert.True(t, allowed)

	allowed, err = g.PlatformSourceAllowed(context.Background(), "u1", "twitch")
	require.NoError(t, err)
	assert.True(t, allowed, "core platforms are never gated")
}

func TestPlatformSourceGate_LookupErrorFailsClosed(t *testing.T) {
	g := platformSourceGate{gates: metaGates, entitlements: func(context.Context, string) (bool, bool, error) {
		return true, true, errors.New("db down")
	}}

	allowed, err := g.PlatformSourceAllowed(context.Background(), "u1", "instagram")
	assert.Error(t, err)
	assert.False(t, allowed)
}
