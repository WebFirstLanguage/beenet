// Package agent implements the Beenet agent lifecycle and state management as specified in §2.
package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/identity"
)

// State represents the current state of the agent
type State int

const (
	// StateStopped indicates the agent is not running
	StateStopped State = iota
	// StateStarting indicates the agent is in the process of starting
	StateStarting
	// StateRunning indicates the agent is running normally
	StateRunning
	// StateStopping indicates the agent is in the process of stopping
	StateStopping
	// StateError indicates the agent encountered an error
	StateError
)

// String returns the string representation of the state
func (s State) String() string {
	switch s {
	case StateStopped:
		return "stopped"
	case StateStarting:
		return "starting"
	case StateRunning:
		return "running"
	case StateStopping:
		return "stopping"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// Agent represents a Beenet agent with lifecycle management
type Agent struct {
	mu       sync.RWMutex
	state    State
	identity *identity.Identity
	nickname string

	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// New creates a new agent with the given identity
func New(id *identity.Identity) *Agent {
	return &Agent{
		state:    StateStopped,
		identity: id,
		done:     make(chan struct{}),
	}
}

// State returns the current state of the agent
func (a *Agent) State() State {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

// setState sets the agent state (internal use)
func (a *Agent) setState(state State) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state = state
}

// Identity returns the agent's identity
func (a *Agent) Identity() *identity.Identity {
	return a.identity
}

// BID returns the agent's Bee ID
func (a *Agent) BID() string {
	if a.identity == nil {
		return ""
	}
	return a.identity.BID()
}

// Handle returns the agent's handle with the given nickname
func (a *Agent) Handle(nickname string) string {
	if a.identity == nil {
		return ""
	}
	return a.identity.Handle(nickname)
}

// SetNickname sets the agent's nickname
func (a *Agent) SetNickname(nickname string) error {
	// Normalize the nickname
	normalized, err := identity.NormalizeNickname(nickname)
	if err != nil {
		return fmt.Errorf("invalid nickname: %w", err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.nickname = normalized
	return nil
}

// Nickname returns the agent's current nickname
func (a *Agent) Nickname() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.nickname
}

// Start starts the agent
func (a *Agent) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.state == StateRunning {
		return fmt.Errorf("agent is already running")
	}

	if a.state == StateStarting {
		return fmt.Errorf("agent is already starting")
	}

	a.state = StateStarting

	// Create context for agent lifecycle
	a.ctx, a.cancel = context.WithCancel(ctx)

	// Reset done channel
	a.done = make(chan struct{})

	// Start the agent main loop
	go a.run()

	// Wait a moment for startup
	time.Sleep(10 * time.Millisecond)

	a.state = StateRunning
	return nil
}

// Stop stops the agent
func (a *Agent) Stop(ctx context.Context) error {
	a.mu.Lock()

	if a.state == StateStopped {
		a.mu.Unlock()
		return fmt.Errorf("agent is already stopped")
	}

	if a.state == StateStopping {
		a.mu.Unlock()
		return fmt.Errorf("agent is already stopping")
	}

	a.state = StateStopping

	// Cancel the agent context
	if a.cancel != nil {
		a.cancel()
	}

	// Unlock before waiting
	a.mu.Unlock()

	// Wait for shutdown with timeout
	select {
	case <-a.done:
		// Agent stopped gracefully
	case <-ctx.Done():
		// Timeout waiting for shutdown
		return fmt.Errorf("timeout waiting for agent to stop")
	case <-time.After(1 * time.Second):
		// Fallback timeout
		break
	}

	a.mu.Lock()
	a.state = StateStopped
	a.mu.Unlock()
	return nil
}

// run is the main agent loop
func (a *Agent) run() {
	defer close(a.done)

	// Print identity and handle on startup
	fmt.Printf("Bee agent started\n")
	fmt.Printf("BID: %s\n", a.BID())
	if a.nickname != "" {
		fmt.Printf("Handle: %s\n", a.Handle(a.nickname))
	}

	// Main agent loop
	for {
		select {
		case <-a.ctx.Done():
			fmt.Printf("Bee agent stopping\n")
			return
		case <-time.After(1 * time.Second):
			// Agent heartbeat - could be used for health checks
			// For now, just continue
		}
	}
}
