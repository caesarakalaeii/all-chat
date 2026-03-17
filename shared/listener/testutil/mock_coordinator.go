package testutil

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/caesar/all-chat/shared/coordination"
)

// MockCoordinator is a behavioral in-memory mock for the coordinator client.
// It tracks call counts and supports failure mode simulation.
// Does NOT use real HTTP or Redis — safe with goleak.VerifyNone.
type MockCoordinator struct {
	heartbeatCount  atomic.Int64
	assignmentCount atomic.Int64

	// Failure mode flags — set before calling Start() to simulate failure scenarios
	ShouldFailHeartbeat bool
	ShouldFail401       bool
	ShouldFailTimeout   bool

	// Assignments to return from QueryAssignments (default: empty slice)
	Assignments []*coordination.Assignment
}

func (m *MockCoordinator) PublishHeartbeat(_ context.Context, _ string) error {
	m.heartbeatCount.Add(1)
	if m.ShouldFail401 {
		return errors.New("401 unauthorized")
	}
	if m.ShouldFailHeartbeat {
		return errors.New("coordinator down")
	}
	return nil
}

func (m *MockCoordinator) QueryAssignments(_ context.Context, _ string) ([]*coordination.Assignment, error) {
	m.assignmentCount.Add(1)
	if m.ShouldFail401 {
		return nil, errors.New("401 unauthorized")
	}
	if m.ShouldFailTimeout {
		return nil, errors.New("request timeout")
	}
	if m.Assignments != nil {
		return m.Assignments, nil
	}
	return []*coordination.Assignment{}, nil
}

func (m *MockCoordinator) StartJWTRefresh(_ context.Context) {}
func (m *MockCoordinator) StopJWTRefresh()                    {}

// HeartbeatCallCount returns the number of times PublishHeartbeat was called.
func (m *MockCoordinator) HeartbeatCallCount() int64 { return m.heartbeatCount.Load() }

// AssignmentCallCount returns the number of times QueryAssignments was called.
func (m *MockCoordinator) AssignmentCallCount() int64 { return m.assignmentCount.Load() }
