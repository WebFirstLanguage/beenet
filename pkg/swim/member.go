// Package swim implements the SWIM (Scalable Weakly-consistent Infection-style Process Group Membership) protocol
// for failure detection and cluster membership management as specified in Phase 4.
package swim

import (
	"sync"
	"time"
)

// MemberState represents the state of a member in the SWIM protocol
type MemberState int

const (
	StateAlive MemberState = iota
	StateSuspect
	StateFailed
	StateLeft
)

// String returns the string representation of the member state
func (s MemberState) String() string {
	switch s {
	case StateAlive:
		return "alive"
	case StateSuspect:
		return "suspect"
	case StateFailed:
		return "failed"
	case StateLeft:
		return "left"
	default:
		return "unknown"
	}
}

// Member represents a member in the SWIM cluster
type Member struct {
	mu sync.RWMutex

	// Identity and addressing
	BID   string   // BeeNet ID of the member
	Addrs []string // List of multiaddrs for the member

	// SWIM protocol state
	State       MemberState // Current state of the member
	Incarnation uint64      // Incarnation number for conflict resolution
	StateTime   time.Time   // Time when the state was last changed

	// Failure detection state
	LastPingTime time.Time // Last time we sent a ping to this member
	LastSeenTime time.Time // Last time we received any message from this member
}

// NewMember creates a new member with the given BID and addresses
func NewMember(bid string, addrs []string) *Member {
	now := time.Now()
	member := &Member{
		BID:          bid,
		Addrs:        make([]string, len(addrs)),
		State:        StateAlive,
		Incarnation:  0,
		StateTime:    now,
		LastPingTime: time.Time{}, // Zero time indicates never pinged
		LastSeenTime: now,
	}
	copy(member.Addrs, addrs)
	return member
}

// SetState updates the member's state with the given incarnation number
// State changes are only applied if the incarnation number is higher than the current one,
// or if it's the same incarnation but the new state has higher priority
func (m *Member) SetState(state MemberState, incarnation uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Only update if incarnation is higher, or same incarnation with higher priority state
	if incarnation > m.Incarnation || (incarnation == m.Incarnation && m.shouldUpdateState(state)) {
		m.State = state
		m.Incarnation = incarnation
		m.StateTime = time.Now()
	}
}

// shouldUpdateState determines if we should update to the new state given the same incarnation
// Priority order: Failed > Left > Suspect > Alive
func (m *Member) shouldUpdateState(newState MemberState) bool {
	currentPriority := m.getStatePriority(m.State)
	newPriority := m.getStatePriority(newState)
	return newPriority > currentPriority
}

// getStatePriority returns the priority of a state for conflict resolution
func (m *Member) getStatePriority(state MemberState) int {
	switch state {
	case StateAlive:
		return 0
	case StateSuspect:
		return 1
	case StateLeft:
		return 2
	case StateFailed:
		return 3
	default:
		return -1
	}
}

// IsSuspicious returns true if the member is in suspect state and has been
// suspicious for longer than the given timeout
func (m *Member) IsSuspicious(timeout time.Duration) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.State != StateSuspect {
		return false
	}

	return time.Since(m.StateTime) >= timeout
}

// IsSuspect returns true if the member is in suspect state (regardless of timeout)
func (m *Member) IsSuspect() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.State == StateSuspect
}

// IsAlive returns true if the member is in alive state
func (m *Member) IsAlive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.State == StateAlive
}

// IsFailed returns true if the member is in failed state
func (m *Member) IsFailed() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.State == StateFailed
}

// IsLeft returns true if the member has voluntarily left
func (m *Member) IsLeft() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.State == StateLeft
}

// GetState returns the current state and incarnation of the member
func (m *Member) GetState() (MemberState, uint64) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.State, m.Incarnation
}

// UpdateAddresses updates the member's addresses
func (m *Member) UpdateAddresses(addrs []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Addrs = make([]string, len(addrs))
	copy(m.Addrs, addrs)
}

// GetAddresses returns a copy of the member's addresses
func (m *Member) GetAddresses() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	addrs := make([]string, len(m.Addrs))
	copy(addrs, m.Addrs)
	return addrs
}

// UpdateLastSeen updates the last seen time for this member
func (m *Member) UpdateLastSeen() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LastSeenTime = time.Now()
}

// UpdateLastPing updates the last ping time for this member
func (m *Member) UpdateLastPing() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LastPingTime = time.Now()
}

// GetLastSeen returns the last seen time
func (m *Member) GetLastSeen() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.LastSeenTime
}

// GetLastPing returns the last ping time
func (m *Member) GetLastPing() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.LastPingTime
}
