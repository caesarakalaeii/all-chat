package coordination

import (
	"fmt"
	"hash/crc32"
	"sync"

	"github.com/buraksezer/consistent"
)

// Assigner implements bounded-load consistent hashing for channel-to-pod assignment
type Assigner struct {
	ring *consistent.Consistent
	mu   sync.RWMutex // Protects ring operations
}

// hasher implements consistent.Hasher interface using CRC32
type hasher struct{}

// Sum64 implements consistent.Hasher interface
// Uses CRC32 as specified in user constraints (simple, fast, sufficient)
func (h hasher) Sum64(data []byte) uint64 {
	return uint64(crc32.ChecksumIEEE(data))
}

// podMember implements consistent.Member interface
type podMember string

// String implements consistent.Member interface
func (p podMember) String() string {
	return string(p)
}

// NewAssigner creates a new bounded-load consistent hash ring with given pods
//
// Configuration (from RESEARCH.md):
// - PartitionCount: 271 (prime number for uniform distribution)
// - ReplicationFactor: 20 (virtual nodes per pod, balances distribution vs memory)
// - Load: 1.25 (bounded-load factor, no pod exceeds 1.25x average load)
// - Hasher: CRC32-based (user constraint: simple, fast, sufficient)
func NewAssigner(pods []string) *Assigner {
	cfg := consistent.Config{
		PartitionCount:    271,  // Prime number for uniform distribution
		ReplicationFactor: 20,   // Virtual nodes per pod (20-100 typical)
		Load:              1.25, // Bounded-load factor (user constraint)
		Hasher:            hasher{},
	}

	ring := consistent.New(nil, cfg)

	// Add pods as members
	for _, podID := range pods {
		ring.Add(podMember(podID))
	}

	return &Assigner{
		ring: ring,
	}
}

// AssignChannel determines which pod should handle the given source
//
// Uses source_id as hash key (user constraint: no additional context).
// Returns assigned pod ID or error if all pods at capacity.
//
// Thread-safe for concurrent assignment queries.
func (a *Assigner) AssignChannel(sourceID string) (string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	member := a.ring.LocateKey([]byte(sourceID))
	if member == nil {
		return "", fmt.Errorf("no pods available for assignment")
	}

	return member.String(), nil
}

// AddPod adds a new pod to the ring
//
// Causes minimal reassignment due to consistent hashing properties.
// Thread-safe.
func (a *Assigner) AddPod(podID string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.ring.Add(podMember(podID))
}

// RemovePod removes a pod from the ring
//
// Channels previously assigned to this pod will be redistributed
// to remaining pods within bounded-load constraint.
// Thread-safe.
func (a *Assigner) RemovePod(podID string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.ring.Remove(podID)
}

// GetMembers returns all current pod members in the ring
//
// Useful for debugging and monitoring.
// Thread-safe.
func (a *Assigner) GetMembers() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	members := a.ring.GetMembers()
	result := make([]string, len(members))
	for i, m := range members {
		result[i] = m.String()
	}
	return result
}
