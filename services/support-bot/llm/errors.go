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

package llm

import "fmt"

// Kind classifies an LLM API error for retry/abort decisions.
type Kind int

const (
	// KindOther is an unclassified error.
	KindOther Kind = iota
	// KindAuth is 401/403 — bad or missing credentials. Not retryable.
	KindAuth
	// KindInvalidRequest is 400/404/422 — a malformed request. Not retryable; it
	// fails identically on retry.
	KindInvalidRequest
	// KindRetryable is a transient HTTP status (429/5xx). Retryable.
	KindRetryable
	// KindUnavailable is a transport-level failure (connection refused, timeout).
	// Retryable.
	KindUnavailable
)

func (k Kind) String() string {
	switch k {
	case KindAuth:
		return "auth"
	case KindInvalidRequest:
		return "invalid_request"
	case KindRetryable:
		return "retryable"
	case KindUnavailable:
		return "unavailable"
	default:
		return "other"
	}
}

// APIError is a typed LLM error. Its Message is already credential-masked; it is safe
// to log but must NEVER be inserted into the conversation the model sees.
type APIError struct {
	Kind    Kind
	Status  int
	Message string
}

func (e *APIError) Error() string {
	if e.Status != 0 {
		return fmt.Sprintf("llm %s error (status %d): %s", e.Kind, e.Status, e.Message)
	}
	return fmt.Sprintf("llm %s error: %s", e.Kind, e.Message)
}

// mapStatus converts an HTTP status + masked body into a typed APIError.
func mapStatus(status int, maskedBody string) *APIError {
	switch {
	case status == 401 || status == 403:
		return &APIError{Kind: KindAuth, Status: status, Message: maskedBody}
	case status == 400 || status == 404 || status == 422:
		return &APIError{Kind: KindInvalidRequest, Status: status, Message: maskedBody}
	case isRetryableStatus(status):
		return &APIError{Kind: KindRetryable, Status: status, Message: maskedBody}
	default:
		return &APIError{Kind: KindOther, Status: status, Message: maskedBody}
	}
}

// isRetryableStatus reports whether an HTTP status warrants a retry.
func isRetryableStatus(s int) bool {
	switch s {
	case 408, 425, 429, 500, 502, 503, 504:
		return true
	}
	return false
}
