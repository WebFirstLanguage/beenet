// Package dht implements the main DHT interface and operations
package dht

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"sync"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/constants"
	"github.com/WebFirstLanguage/beenet/pkg/identity"
	"github.com/WebFirstLanguage/beenet/pkg/wire"
	"lukechampine.com/blake3"
)

// DHT represents a Kademlia-compatible Distributed Hash Table
type DHT struct {
	mu           sync.RWMutex
	localNode    *Node
	routingTable *RoutingTable
	identity     *identity.Identity
	swarmID      string

	// Storage for DHT records
	storage map[string]*DHTRecord

	// Network layer (to be injected)
	network NetworkInterface

	// Security
	security *SecurityManager

	// Configuration
	alpha int // Concurrency parameter for iterative operations

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// DHTRecord represents a stored record in the DHT
type DHTRecord struct {
	Key       []byte
	Value     []byte
	Signature []byte
	Timestamp time.Time
	TTL       time.Duration
}

// NetworkInterface defines the interface for network operations
type NetworkInterface interface {
	SendMessage(ctx context.Context, target *Node, frame *wire.BaseFrame) error
	BroadcastMessage(ctx context.Context, frame *wire.BaseFrame) error
}

// Config holds DHT configuration
type Config struct {
	SwarmID  string
	Identity *identity.Identity
	Network  NetworkInterface
	Alpha    int // Concurrency parameter (default: 3)
}

// New creates a new DHT instance
func New(config *Config) (*DHT, error) {
	if config.Identity == nil {
		return nil, fmt.Errorf("identity is required")
	}

	if config.SwarmID == "" {
		return nil, fmt.Errorf("swarm ID is required")
	}

	alpha := config.Alpha
	if alpha <= 0 {
		alpha = constants.DHTAlpha
	}

	// Create local node
	localNode := NewNode(config.Identity.BID(), []string{})

	// Create security manager
	securityConfig := &SecurityConfig{}
	security := NewSecurityManager(securityConfig)

	dht := &DHT{
		localNode:    localNode,
		routingTable: NewRoutingTable(localNode.ID),
		identity:     config.Identity,
		swarmID:      config.SwarmID,
		storage:      make(map[string]*DHTRecord),
		network:      config.Network,
		security:     security,
		alpha:        alpha,
		done:         make(chan struct{}),
	}

	return dht, nil
}

// Start starts the DHT
func (d *DHT) Start(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.ctx != nil {
		return fmt.Errorf("DHT is already running")
	}

	d.ctx, d.cancel = context.WithCancel(ctx)

	// Start background maintenance
	go d.maintenanceLoop()

	return nil
}

// Stop stops the DHT
func (d *DHT) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.cancel != nil {
		d.cancel()
		d.cancel = nil
	}

	// Wait for maintenance loop to finish
	select {
	case <-d.done:
	case <-time.After(5 * time.Second):
		// Timeout waiting for shutdown
	}

	return nil
}

// AddNode adds a node to the routing table
func (d *DHT) AddNode(node *Node) bool {
	return d.routingTable.Add(node)
}

// RemoveNode removes a node from the routing table
func (d *DHT) RemoveNode(nodeID NodeID) bool {
	return d.routingTable.Remove(nodeID)
}

// GetClosestNodes returns the k closest nodes to the target ID
func (d *DHT) GetClosestNodes(target NodeID, k int) []*Node {
	return d.routingTable.GetClosest(target, k)
}

// Put stores a value in the DHT
func (d *DHT) Put(ctx context.Context, key []byte, value []byte) error {
	if len(key) != 32 {
		return fmt.Errorf("key must be exactly 32 bytes")
	}

	// Sign the key|value pair
	signData := append(key, value...)
	signature := ed25519.Sign(d.identity.SigningPrivateKey, signData)

	// Store locally
	keyStr := string(key)
	d.mu.Lock()
	d.storage[keyStr] = &DHTRecord{
		Key:       key,
		Value:     value,
		Signature: signature,
		Timestamp: time.Now(),
		TTL:       constants.PresenceTTL, // Default TTL
	}
	d.mu.Unlock()

	// Find closest nodes to the key
	targetID := NodeID(blake3.Sum256(key))
	closestNodes := d.GetClosestNodes(targetID, constants.DHTBucketSize)

	// Send PUT messages to closest nodes
	frame := wire.NewDHTPutFrame(d.identity.BID(), d.getNextSeq(), key, value, signature)

	for _, node := range closestNodes {
		if d.network != nil {
			go func(n *Node) {
				if err := d.network.SendMessage(ctx, n, frame); err != nil {
					// Log error but don't fail the operation
					fmt.Printf("Failed to send PUT to node %s: %v\n", n.BID, err)
				}
			}(node)
		}
	}

	return nil
}

// Get retrieves a value from the DHT
func (d *DHT) Get(ctx context.Context, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be exactly 32 bytes")
	}

	// Check local storage first
	keyStr := string(key)
	d.mu.RLock()
	if record, exists := d.storage[keyStr]; exists && !d.isExpired(record) {
		d.mu.RUnlock()
		return record.Value, nil
	}
	d.mu.RUnlock()

	// Perform iterative lookup
	targetID := NodeID(blake3.Sum256(key))
	return d.iterativeGet(ctx, targetID, key)
}

// GetAllNodes returns all nodes in the routing table
func (d *DHT) GetAllNodes() []*Node {
	return d.routingTable.GetAllNodes()
}

// GetRoutingTableSize returns the number of nodes in the routing table
func (d *DHT) GetRoutingTableSize() int {
	return d.routingTable.Size()
}

// maintenanceLoop runs periodic maintenance tasks
func (d *DHT) maintenanceLoop() {
	defer close(d.done)

	ticker := time.NewTicker(30 * time.Second) // Run maintenance every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.performMaintenance()
		}
	}
}

// performMaintenance performs periodic maintenance tasks
func (d *DHT) performMaintenance() {
	// Remove stale nodes
	staleTimeout := 10 * time.Minute
	removed := d.routingTable.RemoveStale(staleTimeout)
	if removed > 0 {
		fmt.Printf("Removed %d stale nodes from routing table\n", removed)
	}

	// Clean up expired records
	d.cleanupExpiredRecords()
}

// cleanupExpiredRecords removes expired records from local storage
func (d *DHT) cleanupExpiredRecords() {
	d.mu.Lock()
	defer d.mu.Unlock()

	for key, record := range d.storage {
		if d.isExpired(record) {
			delete(d.storage, key)
		}
	}
}

// isExpired checks if a record has expired
func (d *DHT) isExpired(record *DHTRecord) bool {
	return time.Since(record.Timestamp) > record.TTL
}

// iterativeGet performs an iterative lookup for a key
func (d *DHT) iterativeGet(ctx context.Context, targetID NodeID, key []byte) ([]byte, error) {
	// This is a simplified implementation
	// In a full implementation, this would perform iterative closest-node lookup

	closestNodes := d.GetClosestNodes(targetID, d.alpha)
	if len(closestNodes) == 0 {
		return nil, fmt.Errorf("no nodes available for lookup")
	}

	// Send GET requests to closest nodes
	frame := wire.NewDHTGetFrame(d.identity.BID(), d.getNextSeq(), key)

	for _, node := range closestNodes {
		if d.network != nil {
			if err := d.network.SendMessage(ctx, node, frame); err != nil {
				fmt.Printf("Failed to send GET to node %s: %v\n", node.BID, err)
			}
		}
	}

	// For now, return not found
	// In a full implementation, this would wait for responses and return the value
	return nil, fmt.Errorf("key not found")
}

// HandleDHTMessage handles incoming DHT messages with security checks
func (d *DHT) HandleDHTMessage(frame *wire.BaseFrame) error {
	// Security check: rate limiting and blacklist
	if !d.security.AllowRequest(frame.From) {
		return fmt.Errorf("request from %s denied by security policy", frame.From)
	}

	// Verify signature (simplified - in full implementation would extract public key from BID)
	// For now, we'll skip signature verification but keep the security structure
	// if err := frame.Verify(publicKey); err != nil {
	//     d.security.BlacklistBID(frame.From, 10*time.Minute)
	//     return fmt.Errorf("invalid signature from %s: %w", frame.From, err)
	// }

	switch frame.Kind {
	case constants.KindDHTGet:
		return d.handleDHTGet(frame)
	case constants.KindDHTPut:
		return d.handleDHTPut(frame)
	case constants.KindAnnouncePresence:
		return d.handleAnnouncePresence(frame)
	default:
		return fmt.Errorf("unsupported DHT message kind: %d", frame.Kind)
	}
}

// handleDHTGet handles DHT GET requests
func (d *DHT) handleDHTGet(frame *wire.BaseFrame) error {
	body, ok := frame.Body.(*wire.DHTGetBody)
	if !ok {
		return fmt.Errorf("invalid DHT GET body")
	}

	// Look up the key in local storage
	keyStr := string(body.Key)
	d.mu.RLock()
	record, exists := d.storage[keyStr]
	d.mu.RUnlock()

	if exists && !d.isExpired(record) {
		// Send response with the value
		// In a full implementation, this would send a DHT_GET_RESPONSE message
		fmt.Printf("DHT GET: Found key %x for %s\n", body.Key, frame.From)
	} else {
		// Key not found or expired
		fmt.Printf("DHT GET: Key %x not found for %s\n", body.Key, frame.From)
	}

	return nil
}

// handleDHTPut handles DHT PUT requests
func (d *DHT) handleDHTPut(frame *wire.BaseFrame) error {
	body, ok := frame.Body.(*wire.DHTPutBody)
	if !ok {
		return fmt.Errorf("invalid DHT PUT body")
	}

	// Verify the signature on the key|value pair
	// signData := append(body.Key, body.Value...)

	// Extract public key from BID (simplified - in full implementation would parse BID properly)
	// For now, we'll skip signature verification of the PUT data

	// Store the record
	keyStr := string(body.Key)
	d.mu.Lock()
	d.storage[keyStr] = &DHTRecord{
		Key:       body.Key,
		Value:     body.Value,
		Signature: body.Sig,
		Timestamp: time.Now(),
		TTL:       constants.PresenceTTL,
	}
	d.mu.Unlock()

	fmt.Printf("DHT PUT: Stored key %x from %s\n", body.Key, frame.From)
	return nil
}

// handleAnnouncePresence handles ANNOUNCE_PRESENCE messages
func (d *DHT) handleAnnouncePresence(frame *wire.BaseFrame) error {
	presence, ok := frame.Body.(*PresenceRecord)
	if !ok {
		return fmt.Errorf("invalid presence record body")
	}

	// Validate the presence record
	if err := presence.IsValid(); err != nil {
		return fmt.Errorf("invalid presence record: %w", err)
	}

	// Verify the presence record signature
	// In a full implementation, we would extract the public key from the BID
	// For now, we'll skip signature verification

	// Add the announcing node to our routing table
	node := NewNode(frame.From, presence.Addrs)
	d.AddNode(node)

	fmt.Printf("ANNOUNCE_PRESENCE: Added node %s with handle %s\n", frame.From, presence.Handle)
	return nil
}

// GetSecurityStats returns security-related statistics
func (d *DHT) GetSecurityStats() map[string]interface{} {
	return d.security.GetStats()
}

// GetNetworkInterface returns the network interface
func (d *DHT) GetNetworkInterface() NetworkInterface {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.network
}

// HandleMessage is a wrapper for HandleDHTMessage for compatibility
func (d *DHT) HandleMessage(frame *wire.BaseFrame) error {
	return d.HandleDHTMessage(frame)
}

// getNextSeq returns the next sequence number for messages
func (d *DHT) getNextSeq() uint64 {
	// Simple implementation - in production, this should be properly managed
	return uint64(time.Now().UnixNano())
}
