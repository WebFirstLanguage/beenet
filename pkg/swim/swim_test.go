package swim

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
	responses    map[string]*wire.BaseFrame // Map from target BID to response frame
}

type MockMessage struct {
	Target *Member
	Frame  *wire.BaseFrame
}

func NewMockNetworkInterface() *MockNetworkInterface {
	return &MockNetworkInterface{
		sentMessages: make([]MockMessage, 0),
		responses:    make(map[string]*wire.BaseFrame),
	}
}

func (m *MockNetworkInterface) SendMessage(ctx context.Context, target *Member, frame *wire.BaseFrame) error {
	m.sentMessages = append(m.sentMessages, MockMessage{
		Target: target,
		Frame:  frame,
	})

	// If there's a pre-configured response, simulate receiving it
	if response, exists := m.responses[target.BID]; exists {
		// In a real implementation, this would be handled by the network layer
		// For testing, we'll just store it for verification
		_ = response
	}

	return nil
}

func (m *MockNetworkInterface) BroadcastMessage(ctx context.Context, frame *wire.BaseFrame) error {
	// For testing, we'll just record that a broadcast was attempted
	m.sentMessages = append(m.sentMessages, MockMessage{
		Target: nil, // nil indicates broadcast
		Frame:  frame,
	})
	return nil
}

func (m *MockNetworkInterface) GetSentMessages() []MockMessage {
	return m.sentMessages
}

func (m *MockNetworkInterface) SetResponse(targetBID string, response *wire.BaseFrame) {
	m.responses[targetBID] = response
}

func (m *MockNetworkInterface) ClearMessages() {
	m.sentMessages = make([]MockMessage, 0)
}

func TestNewSWIM(t *testing.T) {
	identity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}

	network := NewMockNetworkInterface()

	config := &Config{
		Identity:         identity,
		SwarmID:          "test-swarm",
		Network:          network,
		BindAddr:         "/ip4/127.0.0.1/tcp/27487",
		ProbeInterval:    1 * time.Second,
		PingTimeout:      500 * time.Millisecond,
		IndirectTimeout:  1 * time.Second,
		SuspicionTimeout: 5 * time.Second,
	}

	swim, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create SWIM instance: %v", err)
	}

	if swim.localMember.BID != identity.BID() {
		t.Errorf("Expected local member BID %s, got %s", identity.BID(), swim.localMember.BID)
	}

	if swim.swarmID != "test-swarm" {
		t.Errorf("Expected swarm ID 'test-swarm', got %s", swim.swarmID)
	}
}

func TestSWIMAddMember(t *testing.T) {
	identity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}

	network := NewMockNetworkInterface()

	config := &Config{
		Identity: identity,
		SwarmID:  "test-swarm",
		Network:  network,
		BindAddr: "/ip4/127.0.0.1/tcp/27487",
	}

	swim, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create SWIM instance: %v", err)
	}

	// Add a member
	memberBID := "test-member-bid"
	memberAddrs := []string{"/ip4/192.168.1.100/tcp/27487"}

	err = swim.AddMember(memberBID, memberAddrs)
	if err != nil {
		t.Fatalf("Failed to add member: %v", err)
	}

	// Check that member was added
	member := swim.GetMember(memberBID)
	if member == nil {
		t.Fatal("Member was not added to the membership list")
	}

	if member.BID != memberBID {
		t.Errorf("Expected member BID %s, got %s", memberBID, member.BID)
	}

	if !member.IsAlive() {
		t.Error("New member should be alive")
	}
}

func TestSWIMPingMember(t *testing.T) {
	identity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}

	network := NewMockNetworkInterface()

	config := &Config{
		Identity:    identity,
		SwarmID:     "test-swarm",
		Network:     network,
		BindAddr:    "/ip4/127.0.0.1/tcp/27487",
		PingTimeout: 500 * time.Millisecond,
	}

	swim, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create SWIM instance: %v", err)
	}

	// Add a member
	memberBID := "test-member-bid"
	memberAddrs := []string{"/ip4/192.168.1.100/tcp/27487"}

	err = swim.AddMember(memberBID, memberAddrs)
	if err != nil {
		t.Fatalf("Failed to add member: %v", err)
	}

	member := swim.GetMember(memberBID)
	if member == nil {
		t.Fatal("Member not found")
	}

	// Ping the member
	ctx := context.Background()
	err = swim.PingMember(ctx, member)
	if err != nil {
		t.Fatalf("Failed to ping member: %v", err)
	}

	// Check that a SWIM_PING message was sent
	messages := network.GetSentMessages()
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}

	msg := messages[0]
	if msg.Target.BID != memberBID {
		t.Errorf("Expected message target %s, got %s", memberBID, msg.Target.BID)
	}

	if msg.Frame.Kind != constants.KindSWIMPing {
		t.Errorf("Expected message kind %d, got %d", constants.KindSWIMPing, msg.Frame.Kind)
	}
}

func TestSWIMHandleMessage(t *testing.T) {
	identity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}

	network := NewMockNetworkInterface()

	config := &Config{
		Identity: identity,
		SwarmID:  "test-swarm",
		Network:  network,
		BindAddr: "/ip4/127.0.0.1/tcp/27487",
	}

	swim, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create SWIM instance: %v", err)
	}

	// Add the sender as a member first
	senderBID := "sender-bid"
	senderAddrs := []string{"/ip4/192.168.1.200/tcp/27487"}
	err = swim.AddMember(senderBID, senderAddrs)
	if err != nil {
		t.Fatalf("Failed to add sender member: %v", err)
	}

	// Create a SWIM_PING message
	pingFrame := wire.NewSWIMPingFrame(senderBID, 1, identity.BID(), 12345)

	// Handle the message
	err = swim.HandleMessage(context.Background(), pingFrame)
	if err != nil {
		t.Fatalf("Failed to handle SWIM_PING message: %v", err)
	}

	// Check that a SWIM_ACK was sent back
	messages := network.GetSentMessages()
	if len(messages) != 1 {
		t.Fatalf("Expected 1 response message, got %d", len(messages))
	}

	msg := messages[0]
	if msg.Frame.Kind != constants.KindSWIMAck {
		t.Errorf("Expected response kind %d, got %d", constants.KindSWIMAck, msg.Frame.Kind)
	}
}
