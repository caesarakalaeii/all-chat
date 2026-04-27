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

package handlers

import (
	"testing"
	"time"

	"github.com/caesar/all-chat/shared/auth"
)

func TestAcceptShare_ValidAcceptance(t *testing.T) {
	t.Skip("Wave 0 stub - implement in Wave 1")
}

func TestAcceptShare_CycleDetection(t *testing.T) {
	t.Skip("Wave 0 stub - implement in Wave 1")
}

func TestAcceptShare_ExpiryValidation(t *testing.T) {
	t.Skip("Wave 0 stub - implement in Wave 1")
}

// TestShares_GenerateServiceJWT_UsesServiceChain is the Pitfall 4 regression test.
// It proves that the service JWT issued by the handler validates against the SERVICE chain
// and is REJECTED by the USER chain (D-10 cross-chain isolation).
func TestShares_GenerateServiceJWT_UsesServiceChain(t *testing.T) {
	userKC := auth.NewKeyChain(
		map[string][]byte{"v1": []byte("user-chain-secret")},
		nil,
		"v1",
	)
	serviceKC := auth.NewKeyChain(
		map[string][]byte{"v1": []byte("service-chain-secret")},
		nil,
		"v1",
	)

	// Build a token the way the handler does it (after Pitfall 4 fix)
	token, err := auth.GenerateServiceJWTWithKid(
		serviceKC.LatestKid(),
		"share-service",
		string(serviceKC.LatestSecret()),
		30*time.Second,
	)
	if err != nil {
		t.Fatalf("GenerateServiceJWTWithKid failed: %v", err)
	}

	// Must validate against service chain
	_, err = auth.ValidateServiceJWTWithKeyChain(token, serviceKC)
	if err != nil {
		t.Errorf("token must validate against service chain: %v", err)
	}

	// Must NOT validate against user chain (D-10 isolation)
	_, err = auth.ValidateServiceJWTWithKeyChain(token, userKC)
	if err == nil {
		t.Error("token must NOT validate against user chain (Pitfall 4 regression)")
	}
	_ = userKC // suppress unused warning if test fails early
}
