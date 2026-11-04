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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap/zaptest"
)

// TestListUsers_PaginatesAndFilters verifies the admin user listing passes the
// search, tab and page through to the repository and answers with one page,
// the total for the selected tab, and every tab's count.
func TestListUsers_PaginatesAndFilters(t *testing.T) {
	counts := map[string]int{"all": 120, "active": 110, "banned": 10, "premium": 7, "beta": 3, "unconfigured": 40}

	cases := []struct {
		name       string
		url        string
		wantFilter repository.UserListFilter
		wantLimit  int
		wantOffset int
		wantTotal  int
	}{
		{"defaults", "/admin/users", repository.UserListFilter{}, 50, 0, 120},
		{"tab, search and page", "/admin/users?filter=banned&q=+bob+&limit=20&offset=40", repository.UserListFilter{Query: "bob", Status: "banned"}, 20, 40, 10},
		{"limit is capped", "/admin/users?limit=5000", repository.UserListFilter{}, 100, 0, 120},
		{"junk paging falls back", "/admin/users?limit=-3&offset=x", repository.UserListFilter{}, 50, 0, 120},
		{"unknown tab totals all", "/admin/users?filter=nope", repository.UserListFilter{Status: "nope"}, 50, 0, 120},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotFilter repository.UserListFilter
			var gotLimit, gotOffset int
			var gotCountQuery string
			repo := &mockAdminUserRepo{
				listUsers: func(_ context.Context, filter repository.UserListFilter, limit, offset int) ([]*models.User, error) {
					gotFilter, gotLimit, gotOffset = filter, limit, offset
					return []*models.User{{ID: "u1", Username: "bob", CreatedAt: time.Unix(0, 0)}}, nil
				},
				countUsers: func(_ context.Context, query string) (map[string]int, error) {
					gotCountQuery = query
					return counts, nil
				},
			}
			h := NewAdminHandler(repo, nil, zaptest.NewLogger(t), nil, nil, time.Hour, time.Hour)

			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.GET("/admin/users", h.ListUsers)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.url, nil))

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
			}
			if gotFilter != tc.wantFilter || gotLimit != tc.wantLimit || gotOffset != tc.wantOffset {
				t.Errorf("ListUsers(%+v, %d, %d), want (%+v, %d, %d)", gotFilter, gotLimit, gotOffset, tc.wantFilter, tc.wantLimit, tc.wantOffset)
			}
			if gotCountQuery != tc.wantFilter.Query {
				t.Errorf("CountUsers(%q), want %q", gotCountQuery, tc.wantFilter.Query)
			}

			var body struct {
				Users  []map[string]any `json:"users"`
				Total  int              `json:"total"`
				Counts map[string]int   `json:"counts"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode: %v (%s)", err, w.Body.String())
			}
			if len(body.Users) != 1 || body.Users[0]["username"] != "bob" {
				t.Errorf("users = %v, want the one stubbed row", body.Users)
			}
			if body.Total != tc.wantTotal {
				t.Errorf("total = %d, want %d", body.Total, tc.wantTotal)
			}
			if body.Counts["unconfigured"] != 40 {
				t.Errorf("counts = %v, want the repository counts", body.Counts)
			}
		})
	}
}
