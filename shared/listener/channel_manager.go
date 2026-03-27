package listener

import (
	"context"

	"github.com/caesar/all-chat/shared/coordination"
)

// DemandedSource represents a source that has active overlay clients demanding its data.
// Passed to UpdateDemandedSourceIDs to inform the ChannelManager which sources to connect to.
type DemandedSource struct {
	SourceID  string
	ChannelID string
	Platform  string
	OverlayID string
}

// ChannelManager is the interface that all listener channel managers must satisfy.
// Both twitch-listener and kick-listener channels.Manager implement this interface.
type ChannelManager interface {
	Start(ctx context.Context) error
	Stop()
	HandleMigrationEvent(event *coordination.MigrationEvent) error
	UpdateAssignedSourceIDs(newAssignedIDs map[string]bool)
	// UpdateDemandedSourceIDs is called by the SDK demand subscriber loop whenever the set of
	// demanded sources changes. demanded is a map keyed by source_id containing only the
	// intersection of assigned sources and sources with active overlay clients.
	// An empty map means no sources are demanded; listeners should disconnect all.
	// nil is never passed — use an empty map to signal "disconnect all".
	UpdateDemandedSourceIDs(demanded map[string]DemandedSource)
	GetFilteredAssignmentCount() int
	GetActiveChannels() []string
	GetActiveChannelCount() int
}
