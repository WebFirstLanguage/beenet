// Package dht implements bootstrap and seed node management
package dht

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/constants"
	"github.com/WebFirstLanguage/beenet/pkg/wire"
)

// SeedNode represents a bootstrap seed node
type SeedNode struct {
	BID   string   `json:"bid"`   // Bee ID of the seed node
	Addrs []string `json:"addrs"` // Multiaddresses to connect to the seed
	Name  string   `json:"name"`  // Human-readable name (optional)
}

// Bootstrap manages seed nodes and bootstrap process
type Bootstrap struct {
	mu        sync.RWMutex
	dht       *DHT
	seedNodes []*SeedNode

	// Configuration
	seedFile string

	// Bootstrap state
	bootstrapped  bool
	lastBootstrap time.Time
}

// BootstrapConfig holds bootstrap configuration
type BootstrapConfig struct {
	DHT      *DHT
	SeedFile string // Path to seed nodes file
}

// NewBootstrap creates a new bootstrap manager
func NewBootstrap(config *BootstrapConfig) (*Bootstrap, error) {
	if config.DHT == nil {
		return nil, fmt.Errorf("DHT is required")
	}

	seedFile := config.SeedFile
	if seedFile == "" {
		// Default seed file location
		homeDir, err := os.UserHomeDir()
		if err != nil {
			seedFile = "bee-seeds.json"
		} else {
			seedFile = filepath.Join(homeDir, ".bee", "seeds.json")
		}
	}

	b := &Bootstrap{
		dht:      config.DHT,
		seedFile: seedFile,
	}

	// Load existing seed nodes
	if err := b.loadSeedNodes(); err != nil {
		// If file doesn't exist, start with empty list
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load seed nodes: %w", err)
		}
	}

	return b, nil
}

// AddSeedNode adds a new seed node
func (b *Bootstrap) AddSeedNode(seed *SeedNode) error {
	if seed == nil {
		return fmt.Errorf("seed node is required")
	}

	if seed.BID == "" {
		return fmt.Errorf("seed node BID is required")
	}

	if len(seed.Addrs) == 0 {
		return fmt.Errorf("seed node must have at least one address")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Check if seed already exists
	for i, existing := range b.seedNodes {
		if existing.BID == seed.BID {
			// Update existing seed
			b.seedNodes[i] = seed
			return b.saveSeedNodes()
		}
	}

	// Add new seed
	b.seedNodes = append(b.seedNodes, seed)
	return b.saveSeedNodes()
}

// RemoveSeedNode removes a seed node by BID
func (b *Bootstrap) RemoveSeedNode(bid string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i, seed := range b.seedNodes {
		if seed.BID == bid {
			// Remove seed
			b.seedNodes = append(b.seedNodes[:i], b.seedNodes[i+1:]...)
			return b.saveSeedNodes()
		}
	}

	return fmt.Errorf("seed node not found: %s", bid)
}

// GetSeedNodes returns a copy of all seed nodes
func (b *Bootstrap) GetSeedNodes() []*SeedNode {
	b.mu.RLock()
	defer b.mu.RUnlock()

	seeds := make([]*SeedNode, len(b.seedNodes))
	for i, seed := range b.seedNodes {
		seeds[i] = &SeedNode{
			BID:   seed.BID,
			Addrs: append([]string{}, seed.Addrs...),
			Name:  seed.Name,
		}
	}
	return seeds
}

// Bootstrap performs the bootstrap process
func (b *Bootstrap) Bootstrap(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.seedNodes) == 0 {
		return fmt.Errorf("no seed nodes configured")
	}

	fmt.Printf("Starting bootstrap with %d seed nodes...\n", len(b.seedNodes))

	// Connect to seed nodes
	connected := 0
	for _, seed := range b.seedNodes {
		if err := b.connectToSeed(ctx, seed); err != nil {
			fmt.Printf("Failed to connect to seed %s (%s): %v\n", seed.Name, seed.BID, err)
			continue
		}
		connected++
	}

	if connected == 0 {
		return fmt.Errorf("failed to connect to any seed nodes")
	}

	fmt.Printf("Connected to %d seed nodes\n", connected)

	// Perform initial peer discovery
	if err := b.performPeerDiscovery(ctx); err != nil {
		fmt.Printf("Peer discovery failed: %v\n", err)
		// Don't fail bootstrap if peer discovery fails
	}

	b.bootstrapped = true
	b.lastBootstrap = time.Now()

	fmt.Println("Bootstrap completed successfully")
	return nil
}

// IsBootstrapped returns whether bootstrap has been completed
func (b *Bootstrap) IsBootstrapped() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.bootstrapped
}

// GetLastBootstrapTime returns the time of the last successful bootstrap
func (b *Bootstrap) GetLastBootstrapTime() time.Time {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.lastBootstrap
}

// connectToSeed attempts to connect to a seed node
func (b *Bootstrap) connectToSeed(ctx context.Context, seed *SeedNode) error {
	// Create a node representation for the seed
	seedNode := NewNode(seed.BID, seed.Addrs)

	// Add to routing table
	if !b.dht.AddNode(seedNode) {
		fmt.Printf("Seed node %s already in routing table\n", seed.BID)
	}

	// Send PING to establish connection
	if b.dht.network != nil {
		pingFrame := wire.NewPingFrame(b.dht.identity.BID(), b.dht.getNextSeq(), []byte("bootstrap"))
		if err := b.dht.network.SendMessage(ctx, seedNode, pingFrame); err != nil {
			return fmt.Errorf("failed to ping seed node: %w", err)
		}
	}

	return nil
}

// performPeerDiscovery performs initial peer discovery through the DHT
func (b *Bootstrap) performPeerDiscovery(ctx context.Context) error {
	// Perform iterative lookups for random keys to populate routing table
	for i := 0; i < constants.DHTAlpha; i++ {
		// Generate a random key
		randomKey := make([]byte, 32)
		for j := range randomKey {
			randomKey[j] = byte(time.Now().UnixNano() % 256)
		}

		// Perform lookup (this will populate our routing table with discovered nodes)
		_, err := b.dht.Get(ctx, randomKey)
		if err != nil {
			// Expected to fail for random keys, but we'll discover nodes in the process
			continue
		}
	}

	// Also look up our own presence to find nearby nodes
	presenceKey := GetPresenceKey(b.dht.swarmID, b.dht.identity.BID())
	_, err := b.dht.Get(ctx, presenceKey)
	if err != nil {
		// This is expected if we haven't published our presence yet
	}

	return nil
}

// loadSeedNodes loads seed nodes from the seed file
func (b *Bootstrap) loadSeedNodes() error {
	data, err := os.ReadFile(b.seedFile)
	if err != nil {
		return err
	}

	var seeds []*SeedNode
	if err := json.Unmarshal(data, &seeds); err != nil {
		return fmt.Errorf("failed to parse seed file: %w", err)
	}

	b.seedNodes = seeds
	return nil
}

// saveSeedNodes saves seed nodes to the seed file
func (b *Bootstrap) saveSeedNodes() error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(b.seedFile), 0700); err != nil {
		return fmt.Errorf("failed to create seed directory: %w", err)
	}

	data, err := json.MarshalIndent(b.seedNodes, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal seed nodes: %w", err)
	}

	if err := os.WriteFile(b.seedFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write seed file: %w", err)
	}

	return nil
}

// GetSeedFile returns the path to the seed file
func (b *Bootstrap) GetSeedFile() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.seedFile
}

// SetSeedFile sets the path to the seed file
func (b *Bootstrap) SetSeedFile(path string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.seedFile = path
	return b.loadSeedNodes()
}

// AddDefaultSeeds adds some default seed nodes for testing
func (b *Bootstrap) AddDefaultSeeds() error {
	// These would be real seed nodes in production
	defaultSeeds := []*SeedNode{
		{
			BID:   "bee:key:z6MkExample1",
			Addrs: []string{"/ip4/127.0.0.1/udp/27487/quic"},
			Name:  "Local Test Seed 1",
		},
		{
			BID:   "bee:key:z6MkExample2",
			Addrs: []string{"/ip4/127.0.0.1/udp/27488/quic"},
			Name:  "Local Test Seed 2",
		},
	}

	for _, seed := range defaultSeeds {
		if err := b.AddSeedNode(seed); err != nil {
			return fmt.Errorf("failed to add default seed %s: %w", seed.Name, err)
		}
	}

	return nil
}
