package agent

import (
	"context"
	"testing"
	"time"

	"github.com/WebFirstLanguage/beenet/internal/dht"
	"github.com/WebFirstLanguage/beenet/pkg/identity"
	"github.com/WebFirstLanguage/beenet/pkg/wire"
)

func TestAgentSWIMGossipIntegration(t *testing.T) {
	// Generate identity
	identity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}

	// Create agent
	agent := New(identity)
	agent.SetSwarmID("test-swarm")
	agent.SetNickname("test-agent")

	// Initialize DHT first
	err = agent.InitializeDHT()
	if err != nil {
		t.Fatalf("Failed to initialize DHT: %v", err)
	}

	// Initialize SWIM and gossip
	err = agent.InitializeSWIMAndGossip()
	if err != nil {
		t.Fatalf("Failed to initialize SWIM and gossip: %v", err)
	}

	// Verify components are created
	if agent.GetSWIM() == nil {
		t.Fatal("SWIM instance not created")
	}

	if agent.GetGossip() == nil {
		t.Fatal("Gossip instance not created")
	}

	if agent.GetMessageRouter() == nil {
		t.Fatal("Message router not created")
	}

	// Start the agent
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = agent.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}

	// Verify agent is running
	if agent.State() != StateRunning {
		t.Errorf("Expected agent state to be running, got %s", agent.State())
	}

	// Stop the agent
	err = agent.Stop(ctx)
	if err != nil {
		t.Fatalf("Failed to stop agent: %v", err)
	}

	// Verify agent is stopped
	if agent.State() != StateStopped {
		t.Errorf("Expected agent state to be stopped, got %s", agent.State())
	}
}

func TestNetworkAdapterCreation(t *testing.T) {
	// Create a mock DHT network interface
	mockNetwork := &MockDHTNetwork{}

	// Create network adapter
	adapter := NewNetworkAdapter(mockNetwork)
	if adapter == nil {
		t.Fatal("Network adapter not created")
	}

	// Create SWIM adapter
	swimAdapter := NewSWIMNetworkAdapter(adapter)
	if swimAdapter == nil {
		t.Fatal("SWIM network adapter not created")
	}

	// Create gossip adapter
	gossipAdapter := NewGossipNetworkAdapter(adapter)
	if gossipAdapter == nil {
		t.Fatal("Gossip network adapter not created")
	}
}

func TestMessageRouterCreation(t *testing.T) {
	// Create message router
	router := NewMessageRouter()
	if router == nil {
		t.Fatal("Message router not created")
	}

	// Test setting handlers
	router.SetSWIMHandler(nil)
	router.SetGossipHandler(nil)
	router.SetDHTHandler(nil)

	// Router should handle nil handlers gracefully
	// (actual routing tests would require more complex setup)
}

// MockDHTNetwork implements dht.NetworkInterface for testing
type MockDHTNetwork struct {
	sentMessages []MockDHTMessage
}

type MockDHTMessage struct {
	Target *dht.Node
	Frame  *wire.BaseFrame
}

func (m *MockDHTNetwork) SendMessage(ctx context.Context, target *dht.Node, frame *wire.BaseFrame) error {
	m.sentMessages = append(m.sentMessages, MockDHTMessage{
		Target: target,
		Frame:  frame,
	})
	return nil
}

func (m *MockDHTNetwork) BroadcastMessage(ctx context.Context, frame *wire.BaseFrame) error {
	m.sentMessages = append(m.sentMessages, MockDHTMessage{
		Target: nil, // nil indicates broadcast
		Frame:  frame,
	})
	return nil
}

func (m *MockDHTNetwork) GetSentMessages() []MockDHTMessage {
	return m.sentMessages
}

func (m *MockDHTNetwork) ClearMessages() {
	m.sentMessages = make([]MockDHTMessage, 0)
}

func TestAgentLifecycleWithProtocols(t *testing.T) {
	// Generate identity
	identity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}

	// Create agent
	agent := New(identity)
	agent.SetSwarmID("test-swarm")

	// Test that agent can be created without protocols
	if agent.State() != StateStopped {
		t.Errorf("Expected initial state to be stopped, got %s", agent.State())
	}

	// Initialize DHT
	err = agent.InitializeDHT()
	if err != nil {
		t.Fatalf("Failed to initialize DHT: %v", err)
	}

	// Initialize protocols
	err = agent.InitializeSWIMAndGossip()
	if err != nil {
		t.Fatalf("Failed to initialize protocols: %v", err)
	}

	// Start agent
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = agent.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	// Stop agent
	err = agent.Stop(ctx)
	if err != nil {
		t.Fatalf("Failed to stop agent: %v", err)
	}

	// Verify final state
	if agent.State() != StateStopped {
		t.Errorf("Expected final state to be stopped, got %s", agent.State())
	}
}

func TestAgentProtocolAccess(t *testing.T) {
	// Generate identity
	identity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}

	// Create agent
	agent := New(identity)
	agent.SetSwarmID("test-swarm")

	// Before initialization, protocols should be nil
	if agent.GetSWIM() != nil {
		t.Error("SWIM should be nil before initialization")
	}

	if agent.GetGossip() != nil {
		t.Error("Gossip should be nil before initialization")
	}

	if agent.GetMessageRouter() != nil {
		t.Error("Message router should be nil before initialization")
	}

	// Initialize DHT and protocols
	err = agent.InitializeDHT()
	if err != nil {
		t.Fatalf("Failed to initialize DHT: %v", err)
	}

	err = agent.InitializeSWIMAndGossip()
	if err != nil {
		t.Fatalf("Failed to initialize protocols: %v", err)
	}

	// After initialization, protocols should be available
	if agent.GetSWIM() == nil {
		t.Error("SWIM should not be nil after initialization")
	}

	if agent.GetGossip() == nil {
		t.Error("Gossip should not be nil after initialization")
	}

	if agent.GetMessageRouter() == nil {
		t.Error("Message router should not be nil after initialization")
	}
}
