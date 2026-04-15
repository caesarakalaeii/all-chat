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

package cycles

import (
	"context"

	"github.com/caesar/all-chat/services/share-service/models"
)

// ShareRepository defines the interface for querying accepted shares
type ShareRepository interface {
	GetAcceptedSharesByRecipient(ctx context.Context, userID string) ([]models.ShareRequest, error)
}

// CycleDetector implements DFS-based cycle detection for share relationships
type CycleDetector struct {
	repo ShareRepository
}

// NewCycleDetector creates a new cycle detector
func NewCycleDetector(repo ShareRepository) *CycleDetector {
	return &CycleDetector{repo: repo}
}

// HasCycle checks if accepting a share from fromUserID to toUserID would create a cycle.
// It uses DFS to traverse the share graph looking for back edges.
//
// Share graph semantics:
// - If user A shares overlay to user B (A is recipient, B is sender in share_request)
// - The graph edge is B→A (message flow direction)
// - GetAcceptedSharesByRecipient(userA) returns shares where userA is recipient
//   (meaning the senders are users that userA shares TO)
func (d *CycleDetector) HasCycle(ctx context.Context, fromUserID, toUserID string) (bool, error) {
	// Quick check: self-loop is always a cycle
	if fromUserID == toUserID {
		return true, nil
	}

	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	// Start DFS from fromUserID looking for path to toUserID
	return d.dfs(ctx, fromUserID, toUserID, visited, recStack)
}

// dfs performs depth-first search to detect cycles
func (d *CycleDetector) dfs(ctx context.Context, current, target string, visited, recStack map[string]bool) (bool, error) {
	// Mark current node as visited and add to recursion stack
	visited[current] = true
	recStack[current] = true

	// If we've reached the target and it's in the recursion stack, we have a cycle
	if current == target && recStack[target] {
		return true, nil
	}

	// Get all accepted shares where current user is the recipient
	// These represent outgoing edges in the share graph
	shares, err := d.repo.GetAcceptedSharesByRecipient(ctx, current)
	if err != nil {
		return false, err
	}

	// Traverse all neighbors (users that current user shares TO)
	for _, share := range shares {
		neighbor := share.SenderUserID

		// If neighbor is the target, we found a cycle
		if neighbor == target {
			return true, nil
		}

		// If not visited, recurse
		if !visited[neighbor] {
			if hasCycle, err := d.dfs(ctx, neighbor, target, visited, recStack); err != nil {
				return false, err
			} else if hasCycle {
				return true, nil
			}
		} else if recStack[neighbor] {
			// If neighbor is in recursion stack, we have a back edge (cycle)
			return true, nil
		}
	}

	// Remove current from recursion stack before returning
	recStack[current] = false
	return false, nil
}
