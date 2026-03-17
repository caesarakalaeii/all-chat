package listener

import (
	"context"

	"github.com/caesar/all-chat/shared/coordination"
)

// ChannelManager is the interface that all listener channel managers must satisfy.
// Both twitch-listener and kick-listener channels.Manager implement this interface.
type ChannelManager interface {
	Start(ctx context.Context) error
	Stop()
	HandleMigrationEvent(event *coordination.MigrationEvent) error
	UpdateAssignedSourceIDs(newAssignedIDs map[string]bool)
	GetFilteredAssignmentCount() int
	GetActiveChannels() []string
	GetActiveChannelCount() int
}
