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
