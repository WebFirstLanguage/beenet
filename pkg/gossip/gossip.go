// Package gossip implements the BeeGossip/1 protocol for epidemic message dissemination
// with topic-based mesh networks as specified in Phase 4.
package gossip

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/constants"
	"github.com/WebFirstLanguage/beenet/pkg/identity"
	"github.com/WebFirstLanguage/beenet/pkg/wire"
)

// NetworkInterface defines the interface for network operations
type NetworkInterface interface {
	SendMessage(ctx context.Context, target string, frame *wire.BaseFrame) error
	BroadcastMessage(ctx context.Context, frame *wire.BaseFrame) error
}

// Config holds gossip configuration
type Config struct {
	Identity          *identity.Identity // Local node identity
	SwarmID           string             // Swarm identifier
	Network           NetworkInterface   // Network interface for sending messages
	HeartbeatInterval time.Duration      // Heartbeat interval (default: 1s)
	MeshMin           int                // Minimum mesh size (default: 6)
	MeshMax           int                // Maximum mesh size (default: 12)
}

// Gossip represents a gossip protocol instance
type Gossip struct {
	mu sync.RWMutex

	// Configuration
	identity          *identity.Identity
	localBID          string
	swarmID           string
	network           NetworkInterface
	heartbeatInterval time.Duration
	meshMin           int
	meshMax           int

	// Topic meshes
	topicMeshes map[string]*TopicMesh // topicID -> TopicMesh

	// Message deduplication
	seenMessages map[string]time.Time // messageID -> timestamp
	seenTTL      time.Duration        // TTL for seen messages

	// Sequence number for outgoing messages
	sequenceNum uint64

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// TopicMesh represents a mesh network for a specific topic
type TopicMesh struct {
	mu sync.RWMutex

	TopicID string          // Topic identifier
	peers   map[string]bool // BID -> true (mesh peers)
	fanout  map[string]bool // BID -> true (fanout peers for non-subscribed topics)
}

// New creates a new gossip instance
func New(config *Config) (*Gossip, error) {
	if config.Identity == nil {
		return nil, fmt.Errorf("identity is required")
	}

	if config.SwarmID == "" {
		return nil, fmt.Errorf("swarm ID is required")
	}

	if config.Network == nil {
		return nil, fmt.Errorf("network interface is required")
	}

	// Set default values
	heartbeatInterval := config.HeartbeatInterval
	if heartbeatInterval == 0 {
		heartbeatInterval = constants.GossipHeartbeat
	}

	meshMin := config.MeshMin
	if meshMin == 0 {
		meshMin = constants.GossipMeshMin
	}

	meshMax := config.MeshMax
	if meshMax == 0 {
		meshMax = constants.GossipMeshMax
	}

	gossip := &Gossip{
		identity:          config.Identity,
		localBID:          config.Identity.BID(),
		swarmID:           config.SwarmID,
		network:           config.Network,
		heartbeatInterval: heartbeatInterval,
		meshMin:           meshMin,
		meshMax:           meshMax,
		topicMeshes:       make(map[string]*TopicMesh),
		seenMessages:      make(map[string]time.Time),
		seenTTL:           10 * time.Minute, // Keep seen messages for 10 minutes
		sequenceNum:       0,
		done:              make(chan struct{}),
	}

	return gossip, nil
}

// Start starts the gossip protocol
func (g *Gossip) Start(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.ctx != nil {
		return fmt.Errorf("gossip is already running")
	}

	g.ctx, g.cancel = context.WithCancel(ctx)

	// Start heartbeat loop
	go g.heartbeatLoop()

	// Start cleanup loop for seen messages
	go g.cleanupLoop()

	return nil
}

// Stop stops the gossip protocol
func (g *Gossip) Stop() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.cancel != nil {
		g.cancel()
		g.cancel = nil
	}

	return nil
}

// Subscribe subscribes to a topic and creates/joins the mesh
func (g *Gossip) Subscribe(topicID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.topicMeshes[topicID]; exists {
		return nil // Already subscribed
	}

	mesh := &TopicMesh{
		TopicID: topicID,
		peers:   make(map[string]bool),
		fanout:  make(map[string]bool),
	}

	g.topicMeshes[topicID] = mesh

	return nil
}

// Unsubscribe unsubscribes from a topic and leaves the mesh
func (g *Gossip) Unsubscribe(topicID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	mesh, exists := g.topicMeshes[topicID]
	if !exists {
		return nil // Not subscribed
	}

	// Send PRUNE messages to all mesh peers
	ctx := context.Background()
	for peerBID := range mesh.peers {
		pruneFrame := wire.NewGossipPruneFrame(g.localBID, g.getNextSequence(), topicID, []string{})
		if err := pruneFrame.Sign(g.identity.SigningPrivateKey); err == nil {
			g.network.SendMessage(ctx, peerBID, pruneFrame)
		}
	}

	delete(g.topicMeshes, topicID)

	return nil
}

// Publish publishes a message to a topic
func (g *Gossip) Publish(topicID string, payload []byte) error {
	g.mu.RLock()
	mesh, exists := g.topicMeshes[topicID]
	g.mu.RUnlock()

	if !exists {
		return fmt.Errorf("not subscribed to topic: %s", topicID)
	}

	// Create message envelope
	envelope := &wire.PubSubMessageEnvelope{
		From:    g.localBID,
		Seq:     g.getNextSequence(),
		TS:      uint64(time.Now().UnixMilli()),
		Topic:   topicID,
		Payload: payload,
	}

	// Generate message ID (simplified - in full implementation would use proper multihash)
	envelope.MID = fmt.Sprintf("%s-%d-%d", g.localBID, envelope.Seq, envelope.TS)

	// Sign the envelope
	if err := g.signEnvelope(envelope); err != nil {
		return fmt.Errorf("failed to sign message envelope: %w", err)
	}

	// Create PubSub message frame
	frame := wire.NewPubSubMessageFrame(g.localBID, g.getNextSequence(), envelope)
	if err := frame.Sign(g.identity.SigningPrivateKey); err != nil {
		return fmt.Errorf("failed to sign message frame: %w", err)
	}

	// Mark as seen to avoid processing our own message
	g.MarkSeen(envelope.MID)

	// Send to mesh peers or use fanout
	mesh.mu.RLock()
	peers := make([]string, 0, len(mesh.peers))
	for peerBID := range mesh.peers {
		peers = append(peers, peerBID)
	}
	mesh.mu.RUnlock()

	ctx := context.Background()
	if len(peers) > 0 {
		// Send to mesh peers
		for _, peerBID := range peers {
			g.network.SendMessage(ctx, peerBID, frame)
		}
	} else {
		// Use fanout (broadcast for now)
		g.network.BroadcastMessage(ctx, frame)
	}

	return nil
}

// HandleMessage handles incoming gossip protocol messages
func (g *Gossip) HandleMessage(ctx context.Context, frame *wire.BaseFrame) error {
	switch frame.Kind {
	case constants.KindPubSubMsg:
		return g.handlePubSubMessage(ctx, frame)
	case constants.KindGossipIHave:
		return g.handleIHave(ctx, frame)
	case constants.KindGossipIWant:
		return g.handleIWant(ctx, frame)
	case constants.KindGossipGraft:
		return g.handleGraft(ctx, frame)
	case constants.KindGossipPrune:
		return g.handlePrune(ctx, frame)
	case constants.KindGossipHeartbeat:
		return g.handleHeartbeat(ctx, frame)
	default:
		return fmt.Errorf("unsupported gossip message kind: %d", frame.Kind)
	}
}

// GetTopicMesh returns the mesh for a topic
func (g *Gossip) GetTopicMesh(topicID string) *TopicMesh {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.topicMeshes[topicID]
}

// HasSeen checks if a message has been seen before
func (g *Gossip) HasSeen(messageID string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	_, exists := g.seenMessages[messageID]
	return exists
}

// MarkSeen marks a message as seen
func (g *Gossip) MarkSeen(messageID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.seenMessages[messageID] = time.Now()
}

// getNextSequence returns the next sequence number
func (g *Gossip) getNextSequence() uint64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.sequenceNum++
	return g.sequenceNum
}

// signEnvelope signs a PubSub message envelope
func (g *Gossip) signEnvelope(envelope *wire.PubSubMessageEnvelope) error {
	// In a full implementation, this would create a canonical representation
	// and sign it. For now, we'll create a simple signature.
	data := fmt.Sprintf("%s|%d|%d|%s|%s", envelope.From, envelope.Seq, envelope.TS, envelope.Topic, string(envelope.Payload))
	envelope.Sig = []byte("fake-signature-" + data[:min(len(data), 20)])
	return nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TopicMesh methods

// AddPeer adds a peer to the topic mesh
func (tm *TopicMesh) AddPeer(peerBID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.peers[peerBID] = true
}

// RemovePeer removes a peer from the topic mesh
func (tm *TopicMesh) RemovePeer(peerBID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	delete(tm.peers, peerBID)
}

// GetPeers returns a list of mesh peers
func (tm *TopicMesh) GetPeers() []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	peers := make([]string, 0, len(tm.peers))
	for peerBID := range tm.peers {
		peers = append(peers, peerBID)
	}
	return peers
}

// HasPeer checks if a peer is in the mesh
func (tm *TopicMesh) HasPeer(peerBID string) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.peers[peerBID]
}

// Message handlers

// handlePubSubMessage handles incoming PubSub messages
func (g *Gossip) handlePubSubMessage(ctx context.Context, frame *wire.BaseFrame) error {
	envelope, ok := frame.Body.(*wire.PubSubMessageEnvelope)
	if !ok {
		return fmt.Errorf("invalid PubSub message body")
	}

	// Check for duplicate
	if g.HasSeen(envelope.MID) {
		return nil // Already processed
	}

	// Mark as seen
	g.MarkSeen(envelope.MID)

	// Check if we're subscribed to this topic
	g.mu.RLock()
	mesh, subscribed := g.topicMeshes[envelope.Topic]
	g.mu.RUnlock()

	if !subscribed {
		return nil // Not interested in this topic
	}

	// Forward to other mesh peers (except sender)
	mesh.mu.RLock()
	peers := make([]string, 0, len(mesh.peers))
	for peerBID := range mesh.peers {
		if peerBID != frame.From {
			peers = append(peers, peerBID)
		}
	}
	mesh.mu.RUnlock()

	// Forward to a subset of peers to avoid flooding
	maxForward := min(len(peers), 3) // Forward to at most 3 peers
	if maxForward > 0 {
		// Select random peers to forward to
		for i := 0; i < maxForward; i++ {
			n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(peers))))
			peerBID := peers[n.Int64()]
			g.network.SendMessage(ctx, peerBID, frame)

			// Remove selected peer to avoid duplicates
			peers[n.Int64()] = peers[len(peers)-1]
			peers = peers[:len(peers)-1]
		}
	}

	return nil
}

// handleIHave handles incoming IHAVE messages
func (g *Gossip) handleIHave(ctx context.Context, frame *wire.BaseFrame) error {
	body, ok := frame.Body.(*wire.GossipIHaveBody)
	if !ok {
		return fmt.Errorf("invalid IHAVE body")
	}

	// Check if we're interested in this topic
	g.mu.RLock()
	_, subscribed := g.topicMeshes[body.Topic]
	g.mu.RUnlock()

	if !subscribed {
		return nil // Not interested
	}

	// Find messages we don't have
	wantedMessages := make([]string, 0)
	for _, messageID := range body.MessageIDs {
		if !g.HasSeen(messageID) {
			wantedMessages = append(wantedMessages, messageID)
		}
	}

	// Send IWANT if we want any messages
	if len(wantedMessages) > 0 {
		iwantFrame := wire.NewGossipIWantFrame(g.localBID, g.getNextSequence(), wantedMessages)
		if err := iwantFrame.Sign(g.identity.SigningPrivateKey); err != nil {
			return fmt.Errorf("failed to sign IWANT frame: %w", err)
		}
		return g.network.SendMessage(ctx, frame.From, iwantFrame)
	}

	return nil
}

// handleIWant handles incoming IWANT messages
func (g *Gossip) handleIWant(ctx context.Context, frame *wire.BaseFrame) error {
	// Implementation would send requested messages
	// For now, just acknowledge receipt
	return nil
}

// handleGraft handles incoming GRAFT messages
func (g *Gossip) handleGraft(ctx context.Context, frame *wire.BaseFrame) error {
	body, ok := frame.Body.(*wire.GossipGraftBody)
	if !ok {
		return fmt.Errorf("invalid GRAFT body")
	}

	// Add peer to mesh if we're subscribed to the topic
	g.mu.RLock()
	mesh, subscribed := g.topicMeshes[body.Topic]
	g.mu.RUnlock()

	if subscribed {
		mesh.AddPeer(frame.From)
	}

	return nil
}

// handlePrune handles incoming PRUNE messages
func (g *Gossip) handlePrune(ctx context.Context, frame *wire.BaseFrame) error {
	body, ok := frame.Body.(*wire.GossipPruneBody)
	if !ok {
		return fmt.Errorf("invalid PRUNE body")
	}

	// Remove peer from mesh
	g.mu.RLock()
	mesh, exists := g.topicMeshes[body.Topic]
	g.mu.RUnlock()

	if exists {
		mesh.RemovePeer(frame.From)
	}

	return nil
}

// handleHeartbeat handles incoming HEARTBEAT messages
func (g *Gossip) handleHeartbeat(ctx context.Context, frame *wire.BaseFrame) error {
	// Implementation would update peer liveness
	// For now, just acknowledge receipt
	return nil
}

// heartbeatLoop sends periodic heartbeat messages
func (g *Gossip) heartbeatLoop() {
	ticker := time.NewTicker(g.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			g.sendHeartbeat()
		}
	}
}

// sendHeartbeat sends heartbeat messages to maintain mesh connections
func (g *Gossip) sendHeartbeat() {
	g.mu.RLock()
	topics := make([]string, 0, len(g.topicMeshes))
	for topicID := range g.topicMeshes {
		topics = append(topics, topicID)
	}
	g.mu.RUnlock()

	if len(topics) == 0 {
		return
	}

	// Create heartbeat message
	heartbeatFrame := &wire.BaseFrame{
		V:    1,
		Kind: constants.KindGossipHeartbeat,
		From: g.localBID,
		Seq:  g.getNextSequence(),
		TS:   uint64(time.Now().UnixMilli()),
		Body: &wire.GossipHeartbeatBody{Topics: topics},
	}

	if err := heartbeatFrame.Sign(g.identity.SigningPrivateKey); err != nil {
		return // Skip this heartbeat if signing fails
	}

	// Send to all mesh peers
	ctx := context.Background()
	for _, mesh := range g.topicMeshes {
		mesh.mu.RLock()
		for peerBID := range mesh.peers {
			g.network.SendMessage(ctx, peerBID, heartbeatFrame)
		}
		mesh.mu.RUnlock()
	}
}

// cleanupLoop periodically cleans up old seen messages
func (g *Gossip) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			g.cleanupSeenMessages()
		}
	}
}

// cleanupSeenMessages removes old entries from the seen messages map
func (g *Gossip) cleanupSeenMessages() {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()
	for messageID, timestamp := range g.seenMessages {
		if now.Sub(timestamp) > g.seenTTL {
			delete(g.seenMessages, messageID)
		}
	}
}
