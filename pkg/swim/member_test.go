package swim

import (
	"testing"
	"time"
)

func TestMemberState(t *testing.T) {
	tests := []struct {
		name       string
		state      MemberState
		wantString string
	}{
		{"alive", StateAlive, "alive"},
		{"suspect", StateSuspect, "suspect"},
		{"failed", StateFailed, "failed"},
		{"left", StateLeft, "left"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.state.String() != tt.wantString {
				t.Errorf("Expected state string %s, got %s", tt.wantString, tt.state.String())
			}
		})
	}
}

func TestNewMember(t *testing.T) {
	bid := "test-bid-12345"
	addrs := []string{"/ip4/127.0.0.1/tcp/27487"}

	member := NewMember(bid, addrs)

	if member.BID != bid {
		t.Errorf("Expected BID %s, got %s", bid, member.BID)
	}

	if len(member.Addrs) != len(addrs) {
		t.Errorf("Expected %d addresses, got %d", len(addrs), len(member.Addrs))
	}

	if member.Addrs[0] != addrs[0] {
		t.Errorf("Expected address %s, got %s", addrs[0], member.Addrs[0])
	}

	if member.State != StateAlive {
		t.Errorf("Expected initial state %s, got %s", StateAlive, member.State)
	}

	if member.Incarnation != 0 {
		t.Errorf("Expected initial incarnation 0, got %d", member.Incarnation)
	}

	// Check that timestamps are recent
	now := time.Now()
	if member.StateTime.After(now) || member.StateTime.Before(now.Add(-time.Second)) {
		t.Errorf("Expected StateTime to be recent, got %v", member.StateTime)
	}
}

func TestMemberStateTransitions(t *testing.T) {
	member := NewMember("test-bid", []string{"/ip4/127.0.0.1/tcp/27487"})

	// Test alive -> suspect transition
	member.SetState(StateSuspect, 1)
	if member.State != StateSuspect {
		t.Errorf("Expected state %s, got %s", StateSuspect, member.State)
	}
	if member.Incarnation != 1 {
		t.Errorf("Expected incarnation 1, got %d", member.Incarnation)
	}

	// Test suspect -> failed transition
	member.SetState(StateFailed, 1)
	if member.State != StateFailed {
		t.Errorf("Expected state %s, got %s", StateFailed, member.State)
	}

	// Test that we can't go backwards in incarnation
	member.SetState(StateAlive, 0) // Should be ignored
	if member.State != StateFailed {
		t.Errorf("Expected state to remain %s, got %s", StateFailed, member.State)
	}
	if member.Incarnation != 1 {
		t.Errorf("Expected incarnation to remain 1, got %d", member.Incarnation)
	}

	// Test that higher incarnation can override
	member.SetState(StateAlive, 2)
	if member.State != StateAlive {
		t.Errorf("Expected state %s, got %s", StateAlive, member.State)
	}
	if member.Incarnation != 2 {
		t.Errorf("Expected incarnation 2, got %d", member.Incarnation)
	}
}

func TestMemberIsSuspicious(t *testing.T) {
	member := NewMember("test-bid", []string{"/ip4/127.0.0.1/tcp/27487"})

	// Initially not suspicious
	if member.IsSuspicious(10 * time.Second) {
		t.Error("New member should not be suspicious")
	}

	// Set to suspect state
	member.SetState(StateSuspect, 1)

	// Should not be suspicious immediately (timeout hasn't passed)
	if member.IsSuspicious(10 * time.Second) {
		t.Error("Suspect member should not be suspicious immediately with long timeout")
	}

	// Should be suspicious if timeout has passed (use zero timeout)
	if !member.IsSuspicious(0) {
		t.Error("Suspect member should be suspicious with zero timeout")
	}

	// Non-suspect states should not be suspicious
	member.SetState(StateAlive, 2)
	if member.IsSuspicious(10 * time.Second) {
		t.Error("Alive member should not be suspicious")
	}

	member.SetState(StateFailed, 2)
	if member.IsSuspicious(10 * time.Second) {
		t.Error("Failed member should not be suspicious")
	}
}

func TestMemberIsSuspect(t *testing.T) {
	member := NewMember("test-bid", []string{"/ip4/127.0.0.1/tcp/27487"})

	// Initially not suspect
	if member.IsSuspect() {
		t.Error("New member should not be suspect")
	}

	// Set to suspect state
	member.SetState(StateSuspect, 1)

	// Should be suspect immediately
	if !member.IsSuspect() {
		t.Error("Member should be suspect after setting to suspect state")
	}

	// Non-suspect states should not be suspect
	member.SetState(StateAlive, 2)
	if member.IsSuspect() {
		t.Error("Alive member should not be suspect")
	}

	member.SetState(StateFailed, 2)
	if member.IsSuspect() {
		t.Error("Failed member should not be suspect")
	}
}

func TestMemberIsAlive(t *testing.T) {
	member := NewMember("test-bid", []string{"/ip4/127.0.0.1/tcp/27487"})

	// Initially alive
	if !member.IsAlive() {
		t.Error("New member should be alive")
	}

	// Test other states
	member.SetState(StateSuspect, 1)
	if member.IsAlive() {
		t.Error("Suspect member should not be alive")
	}

	member.SetState(StateFailed, 1)
	if member.IsAlive() {
		t.Error("Failed member should not be alive")
	}

	member.SetState(StateLeft, 1)
	if member.IsAlive() {
		t.Error("Left member should not be alive")
	}

	// Back to alive
	member.SetState(StateAlive, 2)
	if !member.IsAlive() {
		t.Error("Member should be alive again")
	}
}

func TestMemberUpdateAddresses(t *testing.T) {
	member := NewMember("test-bid", []string{"/ip4/127.0.0.1/tcp/27487"})

	newAddrs := []string{
		"/ip4/192.168.1.100/tcp/27487",
		"/ip4/10.0.0.1/tcp/27487",
	}

	member.UpdateAddresses(newAddrs)

	if len(member.Addrs) != len(newAddrs) {
		t.Errorf("Expected %d addresses, got %d", len(newAddrs), len(member.Addrs))
	}

	for i, addr := range newAddrs {
		if member.Addrs[i] != addr {
			t.Errorf("Expected address %s at index %d, got %s", addr, i, member.Addrs[i])
		}
	}
}
