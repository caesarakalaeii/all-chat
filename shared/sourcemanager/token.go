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

package sourcemanager

import (
	"context"
	"sync"
	"time"

	"github.com/caesar/all-chat/shared/auth"
)

// TokenSource returns a bearer token for Source Manager requests.
type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

// StaticTokenSource always returns the same token string.
type StaticTokenSource struct {
	token string
}

// NewStaticTokenSource creates a token source that always yields the provided token.
func NewStaticTokenSource(token string) *StaticTokenSource {
	return &StaticTokenSource{token: token}
}

// Token implements TokenSource.
func (s *StaticTokenSource) Token(context.Context) (string, error) {
	return s.token, nil
}

// SigningTokenSource signs short-lived JWTs on demand using the shared secret.
type SigningTokenSource struct {
	serviceName string
	secret      string
	ttl         time.Duration

	mu      sync.Mutex
	token   string
	expires time.Time
	refresh time.Duration
	nowFunc func() time.Time
}

// NewSigningTokenSource builds a token source that signs tokens for serviceName.
func NewSigningTokenSource(serviceName, secret string, ttl time.Duration) *SigningTokenSource {
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}

	return &SigningTokenSource{
		serviceName: serviceName,
		secret:      secret,
		ttl:         ttl,
		refresh:     ttl / 5, // Refresh when 80% of TTL remains
		nowFunc:     time.Now,
	}
}

// Token implements TokenSource and caches tokens until they are close to expiry.
func (s *SigningTokenSource) Token(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.secret == "" {
		return "", ErrMissingSecret
	}

	now := s.nowFunc()
	if s.token != "" && now.Add(s.refresh).Before(s.expires) {
		return s.token, nil
	}

	token, err := auth.GenerateServiceJWT(s.serviceName, s.secret, s.ttl)
	if err != nil {
		return "", err
	}

	s.token = token
	s.expires = now.Add(s.ttl)
	return s.token, nil
}
