// Package agent provides network adapter for integrating SWIM and gossip protocols
package agent

import (
	"context"
	"fmt"

	"github.com/WebFirstLanguage/beenet/internal/dht"
	"github.com/WebFirstLanguage/beenet/pkg/gossip"
	"github.com/WebFirstLanguage/beenet/pkg/swim"
	"github.com/WebFirstLanguage/beenet/pkg/wire"
)

// NetworkAdapter adapts between different network interface types
type NetworkAdapter struct {
	dhtNetwork dht.NetworkInterface // DHT network interface
}

// NewNetworkAdapter creates a new network adapter
func NewNetworkAdapter(dhtNetwork dht.NetworkInterface) *NetworkAdapter {
	return &NetworkAdapter{
		dhtNetwork: dhtNetwork,
	}
}

// SWIM Network Interface Implementation

// SendMessage sends a message to a SWIM member (adapts Member to Node)
func (na *NetworkAdapter) SendMessage(ctx context.Context, target *swim.Member, frame *wire.BaseFrame) error {
	if na.dhtNetwork == nil {
		return fmt.Errorf("DHT network interface not available")
	}

	// Convert SWIM Member to DHT Node
	node := dht.NewNode(target.BID, target.GetAddresses())

	return na.dhtNetwork.SendMessage(ctx, node, frame)
}

// BroadcastMessage broadcasts a message to all connected peers
func (na *NetworkAdapter) BroadcastMessage(ctx context.Context, frame *wire.BaseFrame) error {
	if na.dhtNetwork == nil {
		return fmt.Errorf("DHT network interface not available")
	}

	return na.dhtNetwork.BroadcastMessage(ctx, frame)
}

// Gossip Network Interface Implementation

// SendMessageToPeer sends a message to a specific peer by BID
func (na *NetworkAdapter) SendMessageToPeer(ctx context.Context, targetBID string, frame *wire.BaseFrame) error {
	if na.dhtNetwork == nil {
		return fmt.Errorf("DHT network interface not available")
	}

	// Create a temporary node for the target
	// In a full implementation, we would look up the addresses from the routing table
	node := dht.NewNode(targetBID, []string{})

	return na.dhtNetwork.SendMessage(ctx, node, frame)
}

// GossipNetworkAdapter adapts the NetworkAdapter for gossip protocol
type GossipNetworkAdapter struct {
	adapter *NetworkAdapter
}

// NewGossipNetworkAdapter creates a gossip network adapter
func NewGossipNetworkAdapter(adapter *NetworkAdapter) *GossipNetworkAdapter {
	return &GossipNetworkAdapter{adapter: adapter}
}

// SendMessage implements gossip.NetworkInterface
func (gna *GossipNetworkAdapter) SendMessage(ctx context.Context, target string, frame *wire.BaseFrame) error {
	return gna.adapter.SendMessageToPeer(ctx, target, frame)
}

// BroadcastMessage implements gossip.NetworkInterface
func (gna *GossipNetworkAdapter) BroadcastMessage(ctx context.Context, frame *wire.BaseFrame) error {
	return gna.adapter.BroadcastMessage(ctx, frame)
}

// SWIMNetworkAdapter adapts the NetworkAdapter for SWIM protocol
type SWIMNetworkAdapter struct {
	adapter *NetworkAdapter
}

// NewSWIMNetworkAdapter creates a SWIM network adapter
func NewSWIMNetworkAdapter(adapter *NetworkAdapter) *SWIMNetworkAdapter {
	return &SWIMNetworkAdapter{adapter: adapter}
}

// SendMessage implements swim.NetworkInterface
func (sna *SWIMNetworkAdapter) SendMessage(ctx context.Context, target *swim.Member, frame *wire.BaseFrame) error {
	return sna.adapter.SendMessage(ctx, target, frame)
}

// BroadcastMessage implements swim.NetworkInterface
func (sna *SWIMNetworkAdapter) BroadcastMessage(ctx context.Context, frame *wire.BaseFrame) error {
	return sna.adapter.BroadcastMessage(ctx, frame)
}

// MessageRouter routes incoming messages to appropriate protocol handlers
type MessageRouter struct {
	swimHandler   *swim.SWIM
	gossipHandler *gossip.Gossip
	dhtHandler    *dht.DHT
}

// NewMessageRouter creates a new message router
func NewMessageRouter() *MessageRouter {
	return &MessageRouter{}
}

// SetSWIMHandler sets the SWIM protocol handler
func (mr *MessageRouter) SetSWIMHandler(handler *swim.SWIM) {
	mr.swimHandler = handler
}

// SetGossipHandler sets the gossip protocol handler
func (mr *MessageRouter) SetGossipHandler(handler *gossip.Gossip) {
	mr.gossipHandler = handler
}

// SetDHTHandler sets the DHT protocol handler
func (mr *MessageRouter) SetDHTHandler(handler *dht.DHT) {
	mr.dhtHandler = handler
}

// RouteMessage routes an incoming message to the appropriate handler
func (mr *MessageRouter) RouteMessage(ctx context.Context, frame *wire.BaseFrame) error {
	switch {
	// SWIM protocol messages (60-68)
	case frame.Kind >= 60 && frame.Kind <= 68:
		if mr.swimHandler != nil {
			return mr.swimHandler.HandleMessage(ctx, frame)
		}
		return fmt.Errorf("SWIM handler not available for message kind %d", frame.Kind)

	// Gossip protocol messages (70-74)
	case frame.Kind >= 70 && frame.Kind <= 74:
		if mr.gossipHandler != nil {
			return mr.gossipHandler.HandleMessage(ctx, frame)
		}
		return fmt.Errorf("gossip handler not available for message kind %d", frame.Kind)

	// PubSub messages (30)
	case frame.Kind == 30:
		if mr.gossipHandler != nil {
			return mr.gossipHandler.HandleMessage(ctx, frame)
		}
		return fmt.Errorf("gossip handler not available for PubSub message")

	// DHT messages (10-11, 20)
	case frame.Kind >= 10 && frame.Kind <= 11, frame.Kind == 20:
		if mr.dhtHandler != nil {
			return mr.dhtHandler.HandleMessage(frame)
		}
		return fmt.Errorf("DHT handler not available for message kind %d", frame.Kind)

	// Basic connectivity (1-2)
	case frame.Kind >= 1 && frame.Kind <= 2:
		if mr.dhtHandler != nil {
			return mr.dhtHandler.HandleMessage(frame)
		}
		return fmt.Errorf("no handler available for basic connectivity message kind %d", frame.Kind)

	default:
		return fmt.Errorf("unknown message kind: %d", frame.Kind)
	}
}
