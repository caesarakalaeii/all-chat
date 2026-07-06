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
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSnapshotOmitsOverlayID locks in G2: the broadcast/HTTP snapshots must NOT
// serialize overlay_id at any depth (it is an overlay bearer capability), while the
// Go field stays populated so the publisher can still route on it.
func TestSnapshotOmitsOverlayID(t *testing.T) {
	oid := uuid.New()

	ps := PollSnapshot{
		Poll:       Poll{ID: uuid.New(), OverlayID: oid, Source: SourceAllChat, State: PollActive},
		TotalVotes: 3,
	}
	b, err := json.Marshal(ps)
	require.NoError(t, err)
	assert.NotContains(t, string(b), "overlay_id", "poll snapshot must not serialize overlay_id")
	assert.NotContains(t, strings.ToLower(string(b)), strings.ToLower(oid.String()), "the overlay UUID must not leak in the poll frame")
	assert.Equal(t, oid, ps.Poll.OverlayID, "the Go field is still populated for routing")

	pr := PredictionSnapshot{
		Prediction: Prediction{ID: uuid.New(), OverlayID: oid, Source: SourceAllChat, State: PredActive},
		TotalPts:   5,
	}
	b2, err := json.Marshal(pr)
	require.NoError(t, err)
	assert.NotContains(t, string(b2), "overlay_id", "prediction snapshot must not serialize overlay_id")
	assert.NotContains(t, strings.ToLower(string(b2)), strings.ToLower(oid.String()), "the overlay UUID must not leak in the prediction frame")
	assert.Equal(t, oid, pr.Prediction.OverlayID)
}
