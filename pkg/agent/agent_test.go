package agent

import (
	"context"
	"testing"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/identity"
)

// TestAgentStates tests the agent state machine transitions
func TestAgentStates(t *testing.T) {
	tests := []struct {
		name          string
		initialState  State
		action        func(*Agent) error
		expectedState State
		expectError   bool
	}{
		{
			name:          "start_from_stopped",
			initialState:  StateStopped,
			action:        func(a *Agent) error { return a.Start(context.Background()) },
			expectedState: StateRunning,
			expectError:   false,
		},
		{
			name:          "stop_from_running",
			initialState:  StateRunning,
			action:        func(a *Agent) error { return a.Stop(context.Background()) },
			expectedState: StateStopped,
			expectError:   false,
		},
		{
			name:          "start_already_running",
			initialState:  StateRunning,
			action:        func(a *Agent) error { return a.Start(context.Background()) },
			expectedState: StateRunning,
			expectError:   true,
		},
		{
			name:          "stop_already_stopped",
			initialState:  StateStopped,
			action:        func(a *Agent) error { return a.Stop(context.Background()) },
			expectedState: StateStopped,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test identity
			testIdentity, err := identity.GenerateIdentity()
			if err != nil {
				t.Fatalf("Failed to generate test identity: %v", err)
			}

			// Create agent with test identity
			agent := New(testIdentity)
			agent.state = tt.initialState

			// Perform action
			err = tt.action(agent)

			// Check error expectation
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Check state transition
			if agent.State() != tt.expectedState {
				t.Errorf("Expected state %v, got %v", tt.expectedState, agent.State())
			}
		})
	}
}

// TestAgentIdentityLoading tests that agent loads and reports identity correctly
func TestAgentIdentityLoading(t *testing.T) {
	// Create test identity
	testIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate test identity: %v", err)
	}

	// Create agent
	agent := New(testIdentity)

	// Check identity is loaded
	if agent.Identity() == nil {
		t.Error("Agent identity should not be nil")
	}

	// Check BID is available
	bid := agent.BID()
	if bid == "" {
		t.Error("Agent BID should not be empty")
	}

	// Check handle generation with nickname
	handle := agent.Handle("alice")
	expectedHandle := testIdentity.Handle("alice")
	if handle != expectedHandle {
		t.Errorf("Handle mismatch: expected %s, got %s", expectedHandle, handle)
	}
}

// TestAgentLifecycle tests the complete agent lifecycle
func TestAgentLifecycle(t *testing.T) {
	// Create test identity
	testIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate test identity: %v", err)
	}

	// Create agent
	agent := New(testIdentity)

	// Initial state should be stopped
	if agent.State() != StateStopped {
		t.Errorf("Initial state should be %v, got %v", StateStopped, agent.State())
	}

	// Start agent
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := agent.Start(ctx); err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}

	// Should be running
	if agent.State() != StateRunning {
		t.Errorf("After start, state should be %v, got %v", StateRunning, agent.State())
	}

	// Stop agent
	if err := agent.Stop(ctx); err != nil {
		t.Fatalf("Failed to stop agent: %v", err)
	}

	// Should be stopped
	if agent.State() != StateStopped {
		t.Errorf("After stop, state should be %v, got %v", StateStopped, agent.State())
	}
}

// TestAgentSupervisor tests the supervisor retry logic
func TestAgentSupervisor(t *testing.T) {
	// Create test identity
	testIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate test identity: %v", err)
	}

	// Create agent with supervisor
	agent := New(testIdentity)
	supervisor := NewSupervisor(agent)

	// Start supervisor
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := supervisor.Start(ctx); err != nil {
		t.Fatalf("Failed to start supervisor: %v", err)
	}

	// Agent should be running
	if agent.State() != StateRunning {
		t.Errorf("Agent should be running under supervisor, got %v", agent.State())
	}

	// Stop supervisor
	if err := supervisor.Stop(ctx); err != nil {
		t.Fatalf("Failed to stop supervisor: %v", err)
	}

	// Agent should be stopped
	if agent.State() != StateStopped {
		t.Errorf("Agent should be stopped after supervisor stop, got %v", agent.State())
	}
}
