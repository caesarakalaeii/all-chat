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

package models

import (
	"errors"
	"fmt"
	"strings"
)

// ValidProviders lists all supported emote providers
var ValidProviders = []string{"twitch", "7tv", "bttv", "ffz"}

// Emote represents a chat emote from any provider
type Emote struct {
	Code     string `json:"code"`     // e.g., "Kappa", "OMEGALUL"
	URL      string `json:"url"`      // Image URL
	Provider string `json:"provider"` // "twitch", "7tv", "bttv", "ffz"
	Channel  string `json:"channel"`  // Channel name (lowercase)
}

// EmoteResponse is the response structure for emote endpoints
type EmoteResponse struct {
	Channel string  `json:"channel"`
	Emotes  []Emote `json:"emotes"`
}

// Validate checks if the emote has valid fields
func (e *Emote) Validate() error {
	if e.Code == "" {
		return errors.New("code cannot be empty")
	}

	if len(e.Code) > 100 {
		return errors.New("code cannot exceed 100 characters")
	}

	if e.URL == "" {
		return errors.New("url cannot be empty")
	}

	if !isValidURL(e.URL) {
		return errors.New("invalid url format")
	}

	if e.Provider == "" {
		return errors.New("provider cannot be empty")
	}

	if !IsValidProvider(e.Provider) {
		return fmt.Errorf("provider must be one of: %s", strings.Join(ValidProviders, ", "))
	}

	if e.Channel == "" {
		return errors.New("channel cannot be empty")
	}

	return nil
}

// Validate checks if the emote response has valid fields
func (r *EmoteResponse) Validate() error {
	if r.Channel == "" {
		return errors.New("channel cannot be empty")
	}

	// Validate each emote in the list
	for i, emote := range r.Emotes {
		if err := emote.Validate(); err != nil {
			return fmt.Errorf("emote at index %d: %w", i, err)
		}
	}

	return nil
}

// IsValidProvider checks if the provider is supported
func IsValidProvider(provider string) bool {
	for _, valid := range ValidProviders {
		if provider == valid {
			return true
		}
	}
	return false
}

// isValidURL checks if a string is a valid HTTP/HTTPS URL
func isValidURL(urlStr string) bool {
	return strings.HasPrefix(urlStr, "http://") || strings.HasPrefix(urlStr, "https://")
}
