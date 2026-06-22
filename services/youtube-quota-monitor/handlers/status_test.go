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

	"github.com/caesar/all-chat/services/youtube-quota-monitor/monitor"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/caesar/all-chat/shared/quota"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type statusFakeReader struct{ snap monitor.Snapshot }

func (f statusFakeReader) Read(_ context.Context) (monitor.Snapshot, error) { return f.snap, nil }

func TestGetQuotaStatus_Envelope(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reader := statusFakeReader{snap: monitor.Snapshot{Used: 480, Reserved: 20, Available: 500, Limit: 1000, Percentage: 50}}
	// A disabled notifier is a no-op and satisfies monitor.Notifier without Redis.
	notifier := quota.NewNotifier(nil, zap.NewNop(), false, "")
	mon := monitor.New(reader, notifier, metrics.NewListenerMetrics("youtube", "youtube-quota-monitor-statustest"), quota.DefaultThresholds(), time.Minute, zap.NewNop())
	mon.RunOnce(context.Background())

	h := NewStatusHandler(mon)
	router := gin.New()
	router.GET("/quota/status", h.GetQuotaStatus)

	req := httptest.NewRequest(http.MethodGet, "/quota/status", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var top map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &top))
	require.Contains(t, top, "global")
	require.Contains(t, top, "channels")
	assert.JSONEq(t, `[]`, string(top["channels"]), "channels must be an empty array, not null")

	var g globalStatus
	require.NoError(t, json.Unmarshal(top["global"], &g))
	assert.Equal(t, "HEALTHY", g.State)
	assert.Equal(t, 500, g.Used) // used + reserved
	assert.Equal(t, 1000, g.Limit)
	assert.Equal(t, 500, g.Remaining)
	assert.InDelta(t, 50.0, g.Percentage, 0.001)
	assert.Equal(t, 1.0, g.PollingMultiplier)
	assert.NotEmpty(t, g.ResetsAt)
}
