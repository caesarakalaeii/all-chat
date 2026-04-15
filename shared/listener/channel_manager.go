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

package listener

import "context"

// DemandedSource represents a source that has active overlay clients demanding its data.
// Passed to UpdateDemandedSourceIDs to inform the ChannelManager which sources to connect to.
type DemandedSource struct {
	SourceID  string
	ChannelID string
	Platform  string
	OverlayID string
}

// ChannelManager is the interface that all listener channel managers must satisfy.
type ChannelManager interface {
	Start(ctx context.Context) error
	Stop()
	// UpdateAssignedSourceIDs is a no-op slot retained for interface stability.
	// No SDK code calls it — assignment-based filtering is removed in Phase 06.
	UpdateAssignedSourceIDs(newAssignedIDs map[string]bool)
	// UpdateDemandedSourceIDs is called by the SDK demand subscriber loop whenever the set of
	// demanded sources changes. demanded is a map keyed by source_id filtered by platform.
	// An empty map means no sources are demanded; listeners should disconnect all.
	// nil is never passed — use an empty map to signal "disconnect all".
	UpdateDemandedSourceIDs(demanded map[string]DemandedSource)
	GetFilteredAssignmentCount() int
	GetActiveChannels() []string
	GetActiveChannelCount() int
}
