package gossip

import (
	"context"
	"testing"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/constants"
	"github.com/WebFirstLanguage/beenet/pkg/identity"
	"github.com/WebFirstLanguage/beenet/pkg/wire"
)

// MockNetworkInterface implements NetworkInterface for testing
type MockNetworkInterface struct {
	sentMessages []MockMessage
	responses    map[string]*wire.BaseFrame
}

type MockMessage struct {
	Target string // BID of target, empty for broadcast
	Frame  *wire.BaseFrame
}

func NewMockNetworkInterface() *MockNetworkInterface {
	return &MockNetworkInterface{
		sentMessages: make([]MockMessage, 0),
		responses:    make(map[string]*wire.BaseFrame),
	}
}

func (m *MockNetworkInterface) SendMessage(ctx context.Context, target string, frame *wire.BaseFrame) error {
	m.sentMessages = append(m.sentMessages, MockMessage{
		Target: target,
		Frame:  frame,
	})
	return nil
}

func (m *MockNetworkInterface) BroadcastMessage(ctx context.Context, frame *wire.BaseFrame) error {
	m.sentMessages = append(m.sentMessages, MockMessage{
		Target: "", // Empty indicates broadcast
		Frame:  frame,
	})
	return nil
}

func (m *MockNetworkInterface) GetSentMessages() []MockMessage {
	return m.sentMessages
}

func (m *MockNetworkInterface) ClearMessages() {
	m.sentMessages = make([]MockMessage, 0)
}

func TestNewGossip(t *testing.T) {
	identity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}
	
	network := NewMockNetworkInterface()
	
	config := &Config{
		Identity:        identity,
		SwarmID:         "test-swarm",
		Network:         network,
		HeartbeatInterval: 1 * time.Second,
		MeshMin:         6,
		MeshMax:         12,
	}
	
	gossip, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create gossip instance: %v", err)
	}
	
	if gossip.localBID != identity.BID() {
		t.Errorf("Expected local BID %s, got %s", identity.BID(), gossip.localBID)
	}
	
	if gossip.swarmID != "test-swarm" {
		t.Errorf("Expected swarm ID 'test-swarm', got %s", gossip.swarmID)
	}
}

func TestGossipSubscribe(t *testing.T) {
	identity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}
	
	network := NewMockNetworkInterface()
	
	config := &Config{
		Identity: identity,
		SwarmID:  "test-swarm",
		Network:  network,
	}
	
	gossip, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create gossip instance: %v", err)
	}
	
	// Subscribe to a topic
	topicID := "test-topic"
	err = gossip.Subscribe(topicID)
	if err != nil {
		t.Fatalf("Failed to subscribe to topic: %v", err)
	}
	
	// Check that topic mesh was created
	mesh := gossip.GetTopicMesh(topicID)
	if mesh == nil {
		t.Fatal("Topic mesh was not created")
	}
	
	if mesh.TopicID != topicID {
		t.Errorf("Expected topic ID %s, got %s", topicID, mesh.TopicID)
	}
}

func TestGossipPublish(t *testing.T) {
	identity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}
	
	network := NewMockNetworkInterface()
	
	config := &Config{
		Identity: identity,
		SwarmID:  "test-swarm",
		Network:  network,
	}
	
	gossip, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create gossip instance: %v", err)
	}
	
	// Subscribe to a topic first
	topicID := "test-topic"
	err = gossip.Subscribe(topicID)
	if err != nil {
		t.Fatalf("Failed to subscribe to topic: %v", err)
	}
	
	// Publish a message
	payload := []byte("Hello, gossip!")
	err = gossip.Publish(topicID, payload)
	if err != nil {
		t.Fatalf("Failed to publish message: %v", err)
	}
	
	// Check that message was sent
	messages := network.GetSentMessages()
	if len(messages) == 0 {
		t.Fatal("No messages were sent")
	}
	
	// Should be a broadcast message
	msg := messages[0]
	if msg.Target != "" {
		t.Errorf("Expected broadcast message, got target %s", msg.Target)
	}
	
	if msg.Frame.Kind != constants.KindPubSubMsg {
		t.Errorf("Expected message kind %d, got %d", constants.KindPubSubMsg, msg.Frame.Kind)
	}
}

func TestGossipHandleMessage(t *testing.T) {
	identity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}
	
	network := NewMockNetworkInterface()
	
	config := &Config{
		Identity: identity,
		SwarmID:  "test-swarm",
		Network:  network,
	}
	
	gossip, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create gossip instance: %v", err)
	}
	
	// Subscribe to a topic
	topicID := "test-topic"
	err = gossip.Subscribe(topicID)
	if err != nil {
		t.Fatalf("Failed to subscribe to topic: %v", err)
	}
	
	// Create an IHAVE message
	senderBID := "sender-bid"
	messageIDs := []string{"msg1", "msg2", "msg3"}
	ihaveFrame := wire.NewGossipIHaveFrame(senderBID, 1, topicID, messageIDs)
	
	// Handle the message
	err = gossip.HandleMessage(context.Background(), ihaveFrame)
	if err != nil {
		t.Fatalf("Failed to handle IHAVE message: %v", err)
	}
	
	// Check that an IWANT message was sent back (if we don't have those messages)
	messages := network.GetSentMessages()
	if len(messages) > 0 {
		msg := messages[0]
		if msg.Frame.Kind != constants.KindGossipIWant {
			t.Errorf("Expected IWANT response, got kind %d", msg.Frame.Kind)
		}
	}
}

func TestMessageDeduplication(t *testing.T) {
	identity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}
	
	network := NewMockNetworkInterface()
	
	config := &Config{
		Identity: identity,
		SwarmID:  "test-swarm",
		Network:  network,
	}
	
	gossip, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create gossip instance: %v", err)
	}
	
	// Test message deduplication
	messageID := "test-message-id"
	
	// First time should return false (not seen before)
	if gossip.HasSeen(messageID) {
		t.Error("Message should not be seen initially")
	}
	
	// Mark as seen
	gossip.MarkSeen(messageID)
	
	// Second time should return true (already seen)
	if !gossip.HasSeen(messageID) {
		t.Error("Message should be marked as seen")
	}
}

func TestTopicMeshManagement(t *testing.T) {
	identity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}
	
	network := NewMockNetworkInterface()
	
	config := &Config{
		Identity: identity,
		SwarmID:  "test-swarm",
		Network:  network,
		MeshMin:  3,
		MeshMax:  6,
	}
	
	gossip, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create gossip instance: %v", err)
	}
	
	topicID := "test-topic"
	err = gossip.Subscribe(topicID)
	if err != nil {
		t.Fatalf("Failed to subscribe to topic: %v", err)
	}
	
	mesh := gossip.GetTopicMesh(topicID)
	if mesh == nil {
		t.Fatal("Topic mesh not found")
	}
	
	// Add peers to mesh
	peers := []string{"peer1", "peer2", "peer3", "peer4"}
	for _, peer := range peers {
		mesh.AddPeer(peer)
	}
	
	// Check mesh size
	meshPeers := mesh.GetPeers()
	if len(meshPeers) != len(peers) {
		t.Errorf("Expected %d peers in mesh, got %d", len(peers), len(meshPeers))
	}
	
	// Remove a peer
	mesh.RemovePeer("peer2")
	meshPeers = mesh.GetPeers()
	if len(meshPeers) != len(peers)-1 {
		t.Errorf("Expected %d peers after removal, got %d", len(peers)-1, len(meshPeers))
	}
}
