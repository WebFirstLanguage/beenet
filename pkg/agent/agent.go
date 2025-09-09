// Package agent implements the Beenet agent lifecycle and state management as specified in §2.
package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/WebFirstLanguage/beenet/internal/dht"
	"github.com/WebFirstLanguage/beenet/pkg/gossip"
	"github.com/WebFirstLanguage/beenet/pkg/identity"
	"github.com/WebFirstLanguage/beenet/pkg/swim"
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

	// DHT and networking
	dht             *dht.DHT
	presenceManager *dht.PresenceManager
	bootstrap       *dht.Bootstrap
	swarmID         string

	// SWIM and Gossip protocols
	swim           *swim.SWIM
	gossip         *gossip.Gossip
	networkAdapter *NetworkAdapter
	messageRouter  *MessageRouter

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

// SetSwarmID sets the swarm ID for the agent
func (a *Agent) SetSwarmID(swarmID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.state == StateRunning {
		return fmt.Errorf("cannot change swarm ID while agent is running")
	}

	a.swarmID = swarmID
	return nil
}

// GetSwarmID returns the current swarm ID
func (a *Agent) GetSwarmID() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.swarmID
}

// InitializeDHT initializes the DHT components
func (a *Agent) InitializeDHT() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.swarmID == "" {
		return fmt.Errorf("swarm ID must be set before initializing DHT")
	}

	// Create DHT
	dhtConfig := &dht.Config{
		SwarmID:  a.swarmID,
		Identity: a.identity,
		Network:  nil, // Will be set when network layer is implemented
	}

	var err error
	a.dht, err = dht.New(dhtConfig)
	if err != nil {
		return fmt.Errorf("failed to create DHT: %w", err)
	}

	// Create presence manager
	addresses := []string{"/ip4/127.0.0.1/tcp/0"} // Default address for testing
	presenceConfig := &dht.PresenceConfig{
		SwarmID:      a.swarmID,
		Identity:     a.identity,
		Addresses:    addresses,
		Capabilities: []string{"presence", "dht"},
		Nickname:     a.nickname,
	}

	a.presenceManager, err = dht.NewPresenceManager(a.dht, presenceConfig)
	if err != nil {
		return fmt.Errorf("failed to create presence manager: %w", err)
	}

	// Create bootstrap manager
	bootstrapConfig := &dht.BootstrapConfig{
		DHT: a.dht,
	}

	a.bootstrap, err = dht.NewBootstrap(bootstrapConfig)
	if err != nil {
		return fmt.Errorf("failed to create bootstrap manager: %w", err)
	}

	return nil
}

// InitializeSWIMAndGossip initializes the SWIM and gossip protocols
func (a *Agent) InitializeSWIMAndGossip() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.swarmID == "" {
		return fmt.Errorf("swarm ID must be set before initializing SWIM and gossip")
	}

	if a.dht == nil {
		return fmt.Errorf("DHT must be initialized before SWIM and gossip")
	}

	// Create network adapter
	a.networkAdapter = NewNetworkAdapter(a.dht.GetNetworkInterface())

	// Create message router
	a.messageRouter = NewMessageRouter()

	// Initialize SWIM protocol
	swimConfig := &swim.Config{
		Identity:         a.identity,
		SwarmID:          a.swarmID,
		Network:          NewSWIMNetworkAdapter(a.networkAdapter),
		BindAddr:         "/ip4/0.0.0.0/tcp/0", // Will be updated when transport is ready
		ProbeInterval:    0,                    // Use defaults
		PingTimeout:      0,                    // Use defaults
		IndirectTimeout:  0,                    // Use defaults
		SuspicionTimeout: 0,                    // Use defaults
	}

	var err error
	a.swim, err = swim.New(swimConfig)
	if err != nil {
		return fmt.Errorf("failed to create SWIM instance: %w", err)
	}

	// Initialize gossip protocol
	gossipConfig := &gossip.Config{
		Identity:          a.identity,
		SwarmID:           a.swarmID,
		Network:           NewGossipNetworkAdapter(a.networkAdapter),
		HeartbeatInterval: 0, // Use defaults
		MeshMin:           0, // Use defaults
		MeshMax:           0, // Use defaults
	}

	a.gossip, err = gossip.New(gossipConfig)
	if err != nil {
		return fmt.Errorf("failed to create gossip instance: %w", err)
	}

	// Set up message routing
	a.messageRouter.SetSWIMHandler(a.swim)
	a.messageRouter.SetGossipHandler(a.gossip)
	a.messageRouter.SetDHTHandler(a.dht)

	return nil
}

// GetDHT returns the DHT instance (for testing/debugging)
func (a *Agent) GetDHT() *dht.DHT {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.dht
}

// GetBootstrap returns the bootstrap manager
func (a *Agent) GetBootstrap() *dht.Bootstrap {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.bootstrap
}

// GetSWIM returns the SWIM instance (for testing/debugging)
func (a *Agent) GetSWIM() *swim.SWIM {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.swim
}

// GetGossip returns the gossip instance (for testing/debugging)
func (a *Agent) GetGossip() *gossip.Gossip {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.gossip
}

// GetMessageRouter returns the message router (for testing/debugging)
func (a *Agent) GetMessageRouter() *MessageRouter {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.messageRouter
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

	// Initialize DHT if not already done
	if a.dht == nil && a.swarmID != "" {
		if err := a.InitializeDHT(); err != nil {
			a.cancel()
			return fmt.Errorf("failed to initialize DHT: %w", err)
		}
	}

	// Initialize SWIM and gossip if not already done
	if a.swim == nil && a.gossip == nil && a.dht != nil {
		if err := a.InitializeSWIMAndGossip(); err != nil {
			a.cancel()
			return fmt.Errorf("failed to initialize SWIM and gossip: %w", err)
		}
	}

	// Start DHT components if available
	if a.dht != nil {
		if err := a.dht.Start(a.ctx); err != nil {
			a.cancel()
			return fmt.Errorf("failed to start DHT: %w", err)
		}
	}

	if a.presenceManager != nil {
		if err := a.presenceManager.Start(a.ctx); err != nil {
			a.cancel()
			return fmt.Errorf("failed to start presence manager: %w", err)
		}
	}

	// Start SWIM and gossip protocols if available
	if a.swim != nil {
		if err := a.swim.Start(a.ctx); err != nil {
			a.cancel()
			return fmt.Errorf("failed to start SWIM: %w", err)
		}
	}

	if a.gossip != nil {
		if err := a.gossip.Start(a.ctx); err != nil {
			a.cancel()
			return fmt.Errorf("failed to start gossip: %w", err)
		}
	}

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

	// Stop DHT components
	if a.presenceManager != nil {
		if err := a.presenceManager.Stop(); err != nil {
			fmt.Printf("Error stopping presence manager: %v\n", err)
		}
	}

	if a.dht != nil {
		if err := a.dht.Stop(); err != nil {
			fmt.Printf("Error stopping DHT: %v\n", err)
		}
	}

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
