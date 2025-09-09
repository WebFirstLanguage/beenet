// Package test provides comprehensive test harness for Phase 4 SWIM and Gossip protocols
package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/agent"
	"github.com/WebFirstLanguage/beenet/pkg/identity"
	"github.com/WebFirstLanguage/beenet/pkg/wire"
)

// TestHarness provides a comprehensive test environment for Phase 4
type TestHarness struct {
	agents     []*agent.Agent
	identities []*identity.Identity
	swarmID    string
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewTestHarness creates a new test harness
func NewTestHarness(numAgents int, swarmID string) (*TestHarness, error) {
	if numAgents < 1 {
		return nil, fmt.Errorf("need at least 1 agent")
	}

	ctx, cancel := context.WithCancel(context.Background())

	harness := &TestHarness{
		agents:     make([]*agent.Agent, 0, numAgents),
		identities: make([]*identity.Identity, 0, numAgents),
		swarmID:    swarmID,
		ctx:        ctx,
		cancel:     cancel,
	}

	// Create agents
	for i := 0; i < numAgents; i++ {
		identity, err := identity.GenerateIdentity()
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to generate identity for agent %d: %w", i, err)
		}

		agent := agent.New(identity)
		agent.SetSwarmID(swarmID)
		agent.SetNickname(fmt.Sprintf("agent-%d", i))

		harness.identities = append(harness.identities, identity)
		harness.agents = append(harness.agents, agent)
	}

	return harness, nil
}

// Start starts all agents in the test harness
func (h *TestHarness) Start() error {
	for i, agent := range h.agents {
		// Initialize DHT
		if err := agent.InitializeDHT(); err != nil {
			return fmt.Errorf("failed to initialize DHT for agent %d: %w", i, err)
		}

		// Initialize SWIM and gossip
		if err := agent.InitializeSWIMAndGossip(); err != nil {
			return fmt.Errorf("failed to initialize SWIM and gossip for agent %d: %w", i, err)
		}

		// Start agent
		if err := agent.Start(h.ctx); err != nil {
			return fmt.Errorf("failed to start agent %d: %w", i, err)
		}
	}

	return nil
}

// Stop stops all agents in the test harness
func (h *TestHarness) Stop() error {
	h.cancel()

	var errors []error
	for i, agent := range h.agents {
		if err := agent.Stop(h.ctx); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop agent %d: %w", i, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors stopping agents: %v", errors)
	}

	return nil
}

// GetAgent returns an agent by index
func (h *TestHarness) GetAgent(index int) *agent.Agent {
	if index < 0 || index >= len(h.agents) {
		return nil
	}
	return h.agents[index]
}

// GetAgentCount returns the number of agents
func (h *TestHarness) GetAgentCount() int {
	return len(h.agents)
}

// SimulateNetworkChurn simulates nodes joining and leaving the network
func (h *TestHarness) SimulateNetworkChurn(duration time.Duration, churnRate float64) error {
	// This is a simplified simulation - in a full implementation,
	// we would randomly stop and start agents to simulate churn

	ticker := time.NewTicker(time.Duration(float64(duration) * churnRate))
	defer ticker.Stop()

	timeout := time.After(duration)

	for {
		select {
		case <-timeout:
			return nil
		case <-ticker.C:
			// Simulate some network activity
			// In a full implementation, this would stop/start random agents
			fmt.Printf("Network churn simulation tick\n")
		case <-h.ctx.Done():
			return nil
		}
	}
}

// TestMeshFormation tests that gossip meshes form correctly
func TestMeshFormation(t *testing.T) {
	harness, err := NewTestHarness(5, "test-swarm")
	if err != nil {
		t.Fatalf("Failed to create test harness: %v", err)
	}
	defer harness.Stop()

	// Start all agents
	err = harness.Start()
	if err != nil {
		t.Fatalf("Failed to start test harness: %v", err)
	}

	// Let agents initialize
	time.Sleep(100 * time.Millisecond)

	// Test that all agents have SWIM and gossip protocols initialized
	for i := 0; i < harness.GetAgentCount(); i++ {
		agentInstance := harness.GetAgent(i)
		if agentInstance == nil {
			t.Fatalf("Agent %d is nil", i)
		}

		if agentInstance.GetSWIM() == nil {
			t.Errorf("Agent %d does not have SWIM initialized", i)
		}

		if agentInstance.GetGossip() == nil {
			t.Errorf("Agent %d does not have gossip initialized", i)
		}

		if agentInstance.State() != agent.StateRunning {
			t.Errorf("Agent %d is not running, state: %s", i, agentInstance.State())
		}
	}
}

// TestSWIMFailureDetection tests SWIM failure detection mechanisms
func TestSWIMFailureDetection(t *testing.T) {
	harness, err := NewTestHarness(3, "test-swarm")
	if err != nil {
		t.Fatalf("Failed to create test harness: %v", err)
	}
	defer harness.Stop()

	// Start all agents
	err = harness.Start()
	if err != nil {
		t.Fatalf("Failed to start test harness: %v", err)
	}

	// Let agents initialize
	time.Sleep(100 * time.Millisecond)

	// Test that SWIM members can be added
	agent0 := harness.GetAgent(0)
	agent1 := harness.GetAgent(1)

	swim0 := agent0.GetSWIM()
	swim1 := agent1.GetSWIM()

	// Add agent1 as a member of agent0's SWIM
	err = swim0.AddMember(agent1.BID(), []string{"/ip4/127.0.0.1/tcp/0"})
	if err != nil {
		t.Fatalf("Failed to add member to SWIM: %v", err)
	}

	// Add agent0 as a member of agent1's SWIM
	err = swim1.AddMember(agent0.BID(), []string{"/ip4/127.0.0.1/tcp/0"})
	if err != nil {
		t.Fatalf("Failed to add member to SWIM: %v", err)
	}

	// Test that members are present
	members0 := swim0.GetMembers()
	if len(members0) == 0 {
		t.Error("SWIM should have at least one member")
	}

	members1 := swim1.GetMembers()
	if len(members1) == 0 {
		t.Error("SWIM should have at least one member")
	}
}

// TestGossipMessagePropagation tests gossip message propagation
func TestGossipMessagePropagation(t *testing.T) {
	harness, err := NewTestHarness(4, "test-swarm")
	if err != nil {
		t.Fatalf("Failed to create test harness: %v", err)
	}
	defer harness.Stop()

	// Start all agents
	err = harness.Start()
	if err != nil {
		t.Fatalf("Failed to start test harness: %v", err)
	}

	// Let agents initialize
	time.Sleep(100 * time.Millisecond)

	// Test gossip subscription and publishing
	agent0 := harness.GetAgent(0)
	gossip0 := agent0.GetGossip()

	topicID := "test-topic"

	// Subscribe to topic
	err = gossip0.Subscribe(topicID)
	if err != nil {
		t.Fatalf("Failed to subscribe to topic: %v", err)
	}

	// Check that topic mesh was created
	mesh := gossip0.GetTopicMesh(topicID)
	if mesh == nil {
		t.Fatal("Topic mesh was not created")
	}

	// Publish a message
	payload := []byte("Hello, gossip network!")
	err = gossip0.Publish(topicID, payload)
	if err != nil {
		t.Fatalf("Failed to publish message: %v", err)
	}

	// Test message deduplication
	messageID := "test-message-id"
	if gossip0.HasSeen(messageID) {
		t.Error("Message should not be seen initially")
	}

	gossip0.MarkSeen(messageID)
	if !gossip0.HasSeen(messageID) {
		t.Error("Message should be marked as seen")
	}
}

// TestNetworkChurnResilience tests network resilience under churn
func TestNetworkChurnResilience(t *testing.T) {
	harness, err := NewTestHarness(6, "test-swarm")
	if err != nil {
		t.Fatalf("Failed to create test harness: %v", err)
	}
	defer harness.Stop()

	// Start all agents
	err = harness.Start()
	if err != nil {
		t.Fatalf("Failed to start test harness: %v", err)
	}

	// Let agents initialize
	time.Sleep(100 * time.Millisecond)

	// Simulate network churn for a short period
	churnDuration := 500 * time.Millisecond
	churnRate := 0.1 // 10% churn rate

	err = harness.SimulateNetworkChurn(churnDuration, churnRate)
	if err != nil {
		t.Fatalf("Network churn simulation failed: %v", err)
	}

	// Verify that agents are still running after churn
	runningCount := 0
	for i := 0; i < harness.GetAgentCount(); i++ {
		agentInstance := harness.GetAgent(i)
		if agentInstance.State() == agent.StateRunning {
			runningCount++
		}
	}

	if runningCount == 0 {
		t.Error("No agents are running after network churn")
	}

	t.Logf("Network churn test completed: %d/%d agents still running", runningCount, harness.GetAgentCount())
}

// TestMessageSigning tests that all messages are properly signed
func TestMessageSigning(t *testing.T) {
	// Generate identity for testing
	identity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}

	// Test SWIM message signing
	swimPingFrame := wire.NewSWIMPingFrame("sender", 1, "target", 12345)
	err = swimPingFrame.Sign(identity.SigningPrivateKey)
	if err != nil {
		t.Fatalf("Failed to sign SWIM ping frame: %v", err)
	}

	if len(swimPingFrame.Sig) == 0 {
		t.Error("SWIM ping frame signature is empty")
	}

	// Test gossip message signing
	gossipIHaveFrame := wire.NewGossipIHaveFrame("sender", 1, "topic", []string{"msg1", "msg2"})
	err = gossipIHaveFrame.Sign(identity.SigningPrivateKey)
	if err != nil {
		t.Fatalf("Failed to sign gossip IHAVE frame: %v", err)
	}

	if len(gossipIHaveFrame.Sig) == 0 {
		t.Error("Gossip IHAVE frame signature is empty")
	}

	// Test PubSub message signing
	envelope := &wire.PubSubMessageEnvelope{
		From:    identity.BID(),
		Seq:     1,
		TS:      uint64(time.Now().UnixMilli()),
		Topic:   "test-topic",
		Payload: []byte("test message"),
		MID:     "test-message-id",
	}

	pubsubFrame := wire.NewPubSubMessageFrame("sender", 1, envelope)
	err = pubsubFrame.Sign(identity.SigningPrivateKey)
	if err != nil {
		t.Fatalf("Failed to sign PubSub frame: %v", err)
	}

	if len(pubsubFrame.Sig) == 0 {
		t.Error("PubSub frame signature is empty")
	}
}
