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

package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestTrackedStatusesArePreSeeded(t *testing.T) {
	// The alert for the "healthy but every patron reads as none" failure compares
	// active against 0. That comparison needs the series to EXIST at zero before the
	// first reconcile pass, or the alert can never fire — the same silent-failure
	// shape these metrics exist to catch.
	for _, s := range trackedStatuses {
		assert.NotNil(t, ConnectionsByStatus.WithLabelValues(s), "status %q not pre-seeded", s)
	}
	assert.Zero(t, testutil.ToFloat64(ConnectionsByStatus.WithLabelValues("active")))
}

func TestObserveReconcileStatusesResetsAbsentStatuses(t *testing.T) {
	ObserveReconcileStatuses(map[string]int{"active": 3, "none": 1})
	assert.Equal(t, 3.0, testutil.ToFloat64(ConnectionsByStatus.WithLabelValues("active")))
	assert.Equal(t, 1.0, testutil.ToFloat64(ConnectionsByStatus.WithLabelValues("none")))

	// A status that stops occurring must fall to zero. Leaving the previous value in
	// place would let a fleet that has lost every active patron keep reporting the
	// old healthy number indefinitely.
	ObserveReconcileStatuses(map[string]int{"none": 4})
	assert.Zero(t, testutil.ToFloat64(ConnectionsByStatus.WithLabelValues("active")))
	assert.Equal(t, 4.0, testutil.ToFloat64(ConnectionsByStatus.WithLabelValues("none")))
}

func TestExportedMetricNames(t *testing.T) {
	// These names are referenced as strings by the PatreonEntitlementsAllUnresolved,
	// PatreonUnmatchedMembersDiscarded and PatreonReconcileStalled alerts in
	// caesar-deployment (apps/platform/allchat-monitoring/allchat-*-alerts.yaml).
	// A rename here would leave those alerts silently unfirable — the same invisible
	// failure they were written to catch — so pin the names and make the coupling
	// break loudly instead.
	// CollectAndCount counts series carrying the given name, so a wrong name yields 0.
	// The exact series count is deliberately not asserted: it tracks the number of
	// status labels seen so far, which depends on test order.
	ObserveReconcileStatuses(map[string]int{"active": 1})
	assert.Positive(t, testutil.CollectAndCount(ConnectionsByStatus, "payment_patreon_connections"))
	assert.Equal(t, 1, testutil.CollectAndCount(UnmatchedMembers, "payment_patreon_unmatched_members_total"))
	assert.Equal(t, 1, testutil.CollectAndCount(ReconcileLastSuccess, "payment_patreon_reconcile_last_success_timestamp_seconds"))
}

func TestObserveReconcileStatusesAcceptsUntrackedStatus(t *testing.T) {
	// An unrecognised status must still be published rather than dropped: a status we
	// forgot to track silently vanishing is how you end up with a fleet whose numbers
	// do not add up to the connection count.
	ObserveReconcileStatuses(map[string]int{"active": 1, "some_new_status": 2})
	assert.Equal(t, 2.0, testutil.ToFloat64(ConnectionsByStatus.WithLabelValues("some_new_status")))

	ObserveReconcileStatuses(map[string]int{})
}
