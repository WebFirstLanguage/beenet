// Package dht integration tests
package dht

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/identity"
	"github.com/WebFirstLanguage/beenet/pkg/wire"
)

// MockNetwork implements NetworkInterface for testing
type MockNetwork struct {
	nodes map[string]*DHT // BID -> DHT instance
}

func NewMockNetwork() *MockNetwork {
	return &MockNetwork{
		nodes: make(map[string]*DHT),
	}
}

func (mn *MockNetwork) RegisterNode(bid string, dht *DHT) {
	mn.nodes[bid] = dht
}

func (mn *MockNetwork) SendMessage(ctx context.Context, target *Node, frame *wire.BaseFrame) error {
	// Find the target DHT instance
	targetDHT, exists := mn.nodes[target.BID]
	if !exists {
		return fmt.Errorf("target node %s not found in mock network", target.BID)
	}
	
	// Simulate network delay
	go func() {
		time.Sleep(10 * time.Millisecond)
		targetDHT.HandleDHTMessage(frame)
	}()
	
	return nil
}

func (mn *MockNetwork) BroadcastMessage(ctx context.Context, frame *wire.BaseFrame) error {
	// Broadcast to all nodes except sender
	for bid, dht := range mn.nodes {
		if bid != frame.From {
			go func(d *DHT) {
				time.Sleep(10 * time.Millisecond)
				d.HandleDHTMessage(frame)
			}(dht)
		}
	}
	return nil
}

// TestDHTBasicOperations tests basic DHT GET/PUT operations
func TestDHTBasicOperations(t *testing.T) {
	// Create test identity
	identity1, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}
	
	// Create DHT
	config := &Config{
		SwarmID:  "test-swarm",
		Identity: identity1,
		Network:  nil, // No network for basic test
	}
	
	dht, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create DHT: %v", err)
	}
	
	// Start DHT
	ctx := context.Background()
	if err := dht.Start(ctx); err != nil {
		t.Fatalf("Failed to start DHT: %v", err)
	}
	defer dht.Stop()
	
	// Test PUT operation
	key := make([]byte, 32)
	copy(key, "test-key-12345678901234567890123")
	value := []byte("test-value")
	
	if err := dht.Put(ctx, key, value); err != nil {
		t.Fatalf("Failed to PUT: %v", err)
	}
	
	// Test GET operation
	retrievedValue, err := dht.Get(ctx, key)
	if err != nil {
		t.Fatalf("Failed to GET: %v", err)
	}
	
	if string(retrievedValue) != string(value) {
		t.Errorf("Retrieved value mismatch: expected %s, got %s", value, retrievedValue)
	}
}

// TestPresenceRecordSigning tests presence record signing and verification
func TestPresenceRecordSigning(t *testing.T) {
	// Create test identity
	identity1, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}
	
	// Create presence record
	swarmID := "test-swarm"
	addrs := []string{"/ip4/127.0.0.1/udp/27487/quic"}
	caps := []string{"presence", "dht"}
	
	record, err := NewPresenceRecord(swarmID, identity1, addrs, caps)
	if err != nil {
		t.Fatalf("Failed to create presence record: %v", err)
	}
	
	// Verify the record is valid
	if err := record.IsValid(); err != nil {
		t.Errorf("Presence record validation failed: %v", err)
	}
	
	// Verify the signature
	if err := record.Verify(identity1.SigningPublicKey); err != nil {
		t.Errorf("Presence record signature verification failed: %v", err)
	}
	
	// Test that tampering breaks verification
	originalHandle := record.Handle
	record.Handle = "tampered-handle"
	
	if err := record.Verify(identity1.SigningPublicKey); err == nil {
		t.Error("Expected signature verification to fail after tampering")
	}
	
	// Restore original handle
	record.Handle = originalHandle
}

// TestMultiNodePeerDiscovery tests peer discovery with multiple nodes
func TestMultiNodePeerDiscovery(t *testing.T) {
	// Create mock network
	network := NewMockNetwork()
	
	// Create multiple test identities
	numNodes := 3
	identities := make([]*identity.Identity, numNodes)
	dhts := make([]*DHT, numNodes)
	presenceManagers := make([]*PresenceManager, numNodes)
	
	swarmID := "test-swarm"
	
	// Initialize nodes
	for i := 0; i < numNodes; i++ {
		identity, err := identity.GenerateIdentity()
		if err != nil {
			t.Fatalf("Failed to generate identity %d: %v", i, err)
		}
		identities[i] = identity
		
		// Create DHT
		config := &Config{
			SwarmID:  swarmID,
			Identity: identity,
			Network:  network,
		}
		
		dht, err := New(config)
		if err != nil {
			t.Fatalf("Failed to create DHT %d: %v", i, err)
		}
		dhts[i] = dht
		
		// Register with mock network
		network.RegisterNode(identity.BID(), dht)
		
		// Create presence manager
		presenceConfig := &PresenceConfig{
			SwarmID:      swarmID,
			Identity:     identity,
			Addresses:    []string{fmt.Sprintf("/ip4/127.0.0.1/udp/%d/quic", 27487+i)},
			Capabilities: []string{"presence", "dht"},
			Nickname:     fmt.Sprintf("node%d", i),
		}
		
		pm, err := NewPresenceManager(dht, presenceConfig)
		if err != nil {
			t.Fatalf("Failed to create presence manager %d: %v", i, err)
		}
		presenceManagers[i] = pm
	}
	
	// Start all nodes
	ctx := context.Background()
	for i, dht := range dhts {
		if err := dht.Start(ctx); err != nil {
			t.Fatalf("Failed to start DHT %d: %v", i, err)
		}
		defer dht.Stop()
		
		if err := presenceManagers[i].Start(ctx); err != nil {
			t.Fatalf("Failed to start presence manager %d: %v", i, err)
		}
		defer presenceManagers[i].Stop()
	}
	
	// Wait for presence announcements to propagate
	time.Sleep(100 * time.Millisecond)
	
	// Verify that nodes have discovered each other
	for i, dht := range dhts {
		peers := dht.GetAllNodes()
		
		// Each node should have discovered the other nodes
		expectedPeers := numNodes - 1 // Excluding self
		if len(peers) < expectedPeers {
			t.Errorf("Node %d discovered %d peers, expected at least %d", i, len(peers), expectedPeers)
		}
		
		t.Logf("Node %d discovered %d peers", i, len(peers))
		for j, peer := range peers {
			t.Logf("  Peer %d: %s", j, peer.BID)
		}
	}
}

// TestRateLimiting tests the rate limiting functionality
func TestRateLimiting(t *testing.T) {
	config := &RateLimiterConfig{
		Capacity: 2,                // 2 requests max
		Refill:   1 * time.Second,  // 1 request per second
		Cleanup:  1 * time.Minute,  // Cleanup every minute
	}
	
	rateLimiter := NewRateLimiter(config)
	
	key := "test-key"
	
	// First two requests should be allowed
	if !rateLimiter.Allow(key) {
		t.Error("First request should be allowed")
	}
	
	if !rateLimiter.Allow(key) {
		t.Error("Second request should be allowed")
	}
	
	// Third request should be denied
	if rateLimiter.Allow(key) {
		t.Error("Third request should be denied")
	}
	
	// Wait for refill and try again
	time.Sleep(1100 * time.Millisecond) // Wait for refill
	
	if !rateLimiter.Allow(key) {
		t.Error("Request after refill should be allowed")
	}
}

// TestBootstrapSeedManagement tests seed node management
func TestBootstrapSeedManagement(t *testing.T) {
	// Create test identity and DHT
	identity1, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}
	
	config := &Config{
		SwarmID:  "test-swarm",
		Identity: identity1,
		Network:  nil,
	}
	
	dht, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create DHT: %v", err)
	}
	
	// Create bootstrap manager
	bootstrapConfig := &BootstrapConfig{
		DHT: dht,
	}
	
	bootstrap, err := NewBootstrap(bootstrapConfig)
	if err != nil {
		t.Fatalf("Failed to create bootstrap: %v", err)
	}
	
	// Test adding seed nodes
	seed1 := &SeedNode{
		BID:   "bee:key:z6MkTest1",
		Addrs: []string{"/ip4/127.0.0.1/udp/27487/quic"},
		Name:  "Test Seed 1",
	}
	
	if err := bootstrap.AddSeedNode(seed1); err != nil {
		t.Fatalf("Failed to add seed node: %v", err)
	}
	
	// Verify seed node was added
	seeds := bootstrap.GetSeedNodes()
	if len(seeds) != 1 {
		t.Errorf("Expected 1 seed node, got %d", len(seeds))
	}
	
	if seeds[0].BID != seed1.BID {
		t.Errorf("Seed BID mismatch: expected %s, got %s", seed1.BID, seeds[0].BID)
	}
	
	// Test removing seed node
	if err := bootstrap.RemoveSeedNode(seed1.BID); err != nil {
		t.Fatalf("Failed to remove seed node: %v", err)
	}
	
	seeds = bootstrap.GetSeedNodes()
	if len(seeds) != 0 {
		t.Errorf("Expected 0 seed nodes after removal, got %d", len(seeds))
	}
}
