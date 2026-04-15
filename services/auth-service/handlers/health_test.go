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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler_CheckLive(t *testing.T) {
	tests := []struct {
		name           string
		wantStatusCode int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:           "liveness always returns OK",
			wantStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]string
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				if response["status"] != "alive" {
					t.Errorf("CheckLive() status = %v, want alive", response["status"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := setupTestRouter()
			handler := NewHealthHandler(nil, nil) // Liveness doesn't need dependencies

			router.GET("/health/live", handler.CheckLive)

			req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatusCode {
				t.Errorf("CheckLive() status = %v, want %v", w.Code, tt.wantStatusCode)
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}

func TestHealthHandler_CheckReady(t *testing.T) {
	// Note: This test requires actual database and Redis connections to work properly.
	// The handler uses concrete types (*pgxpool.Pool, *redis.Client) which are
	// difficult to mock. These tests verify the handler structure is correct.

	t.Run("handler can be created", func(t *testing.T) {
		handler := NewHealthHandler(nil, nil)
		if handler == nil {
			t.Fatal("NewHealthHandler returned nil")
		}
	})

	// Note: Full readiness tests require integration testing with real DB/Redis
	// or refactoring the handler to use interfaces. For now, we verify the
	// handler compiles and can be constructed.
}

