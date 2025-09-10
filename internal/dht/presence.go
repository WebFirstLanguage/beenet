// Package dht implements presence management functionality
package dht

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/codec/cborcanon"
	"github.com/WebFirstLanguage/beenet/pkg/constants"
	"github.com/WebFirstLanguage/beenet/pkg/identity"
	"github.com/WebFirstLanguage/beenet/pkg/wire"
)

// PresenceManager manages presence records and refresh cycles
type PresenceManager struct {
	mu       sync.RWMutex
	dht      *DHT
	identity *identity.Identity
	swarmID  string

	// Current presence record
	currentRecord *PresenceRecord

	// Refresh management
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}

	// Configuration
	addresses    []string
	capabilities []string
	nickname     string
}

// PresenceConfig holds configuration for presence management
type PresenceConfig struct {
	SwarmID      string
	Identity     *identity.Identity
	Addresses    []string
	Capabilities []string
	Nickname     string
}

// NewPresenceManager creates a new presence manager
func NewPresenceManager(dht *DHT, config *PresenceConfig) (*PresenceManager, error) {
	if dht == nil {
		return nil, fmt.Errorf("DHT is required")
	}

	if config.Identity == nil {
		return nil, fmt.Errorf("identity is required")
	}

	if config.SwarmID == "" {
		return nil, fmt.Errorf("swarm ID is required")
	}

	nickname := config.Nickname
	if nickname == "" {
		nickname = "bee" // Default nickname
	}

	capabilities := config.Capabilities
	if capabilities == nil {
		capabilities = []string{"presence", "dht"} // Default capabilities
	}

	pm := &PresenceManager{
		dht:          dht,
		identity:     config.Identity,
		swarmID:      config.SwarmID,
		addresses:    config.Addresses,
		capabilities: capabilities,
		nickname:     nickname,
		done:         make(chan struct{}),
	}

	return pm, nil
}

// Start starts the presence manager and begins refresh cycles
func (pm *PresenceManager) Start(ctx context.Context) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.ctx != nil {
		return fmt.Errorf("presence manager is already running")
	}

	pm.ctx, pm.cancel = context.WithCancel(ctx)

	// Create and publish initial presence record
	if err := pm.publishPresence(); err != nil {
		pm.cancel()
		return fmt.Errorf("failed to publish initial presence: %w", err)
	}

	// Start refresh cycle
	go pm.refreshLoop()

	return nil
}

// Stop stops the presence manager
func (pm *PresenceManager) Stop() error {
	pm.mu.Lock()
	if pm.cancel != nil {
		pm.cancel()
		pm.cancel = nil
	}
	pm.mu.Unlock()

	// Wait for refresh loop to finish
	select {
	case <-pm.done:
	case <-time.After(5 * time.Second):
		// Timeout waiting for shutdown
	}

	return nil
}

// GetCurrentRecord returns the current presence record
func (pm *PresenceManager) GetCurrentRecord() *PresenceRecord {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.currentRecord == nil {
		return nil
	}

	// Return a copy to prevent external modification
	record := *pm.currentRecord
	return &record
}

// UpdateAddresses updates the addresses in the presence record
func (pm *PresenceManager) UpdateAddresses(addresses []string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.addresses = addresses

	// If we're running, immediately publish updated presence
	if pm.ctx != nil {
		return pm.publishPresence()
	}

	return nil
}

// UpdateCapabilities updates the capabilities in the presence record
func (pm *PresenceManager) UpdateCapabilities(capabilities []string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.capabilities = capabilities

	// If we're running, immediately publish updated presence
	if pm.ctx != nil {
		return pm.publishPresence()
	}

	return nil
}

// UpdateNickname updates the nickname (which affects the handle)
func (pm *PresenceManager) UpdateNickname(nickname string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.nickname = nickname

	// If we're running, immediately publish updated presence
	if pm.ctx != nil {
		return pm.publishPresence()
	}

	return nil
}

// publishPresence creates and publishes a new presence record
func (pm *PresenceManager) publishPresence() error {
	// Create new presence record
	record, err := NewPresenceRecord(pm.swarmID, pm.identity, pm.addresses, pm.capabilities)
	if err != nil {
		return fmt.Errorf("failed to create presence record: %w", err)
	}

	// Update the handle with current nickname
	record.Handle = pm.identity.Handle(pm.nickname)

	// Re-sign the record after updating the handle
	if err := record.Sign(pm.identity.SigningPrivateKey); err != nil {
		return fmt.Errorf("failed to sign updated presence record: %w", err)
	}

	// Validate the record
	if err := record.IsValid(); err != nil {
		return fmt.Errorf("invalid presence record: %w", err)
	}

	// Store the record in the DHT
	presenceKey := GetPresenceKey(pm.swarmID, pm.identity.BID())

	// Serialize the record for storage
	recordBytes, err := pm.serializeRecord(record)
	if err != nil {
		return fmt.Errorf("failed to serialize presence record: %w", err)
	}

	// Store in DHT
	if err := pm.dht.Put(pm.ctx, presenceKey, recordBytes); err != nil {
		return fmt.Errorf("failed to store presence record in DHT: %w", err)
	}

	// Also store handle index for quick lookups
	handleKey := GetHandleKey(pm.swarmID, record.Handle)
	handleIndexBytes, err := pm.serializeHandleIndex(record)
	if err != nil {
		return fmt.Errorf("failed to serialize handle index: %w", err)
	}

	if err := pm.dht.Put(pm.ctx, handleKey, handleIndexBytes); err != nil {
		return fmt.Errorf("failed to store handle index in DHT: %w", err)
	}

	// Send ANNOUNCE_PRESENCE message to connected peers
	if pm.dht.network != nil {
		frame := &wire.BaseFrame{
			V:    constants.ProtocolVersion,
			Kind: constants.KindAnnouncePresence,
			From: pm.identity.BID(),
			Seq:  pm.dht.getNextSeq(),
			TS:   uint64(time.Now().UnixMilli()),
			Body: record,
		}

		// Sign the frame
		if err := frame.Sign(pm.identity.SigningPrivateKey); err != nil {
			return fmt.Errorf("failed to sign announce frame: %w", err)
		}

		// Broadcast to connected peers
		if err := pm.dht.network.BroadcastMessage(pm.ctx, frame); err != nil {
			// Log error but don't fail the operation
			fmt.Printf("Failed to broadcast presence announcement: %v\n", err)
		}
	}

	// Update current record
	pm.currentRecord = record

	fmt.Printf("Published presence record for %s (expires: %v)\n",
		record.Handle, time.UnixMilli(int64(record.Expire)).Format(time.RFC3339))

	return nil
}

// refreshLoop runs the periodic presence refresh
func (pm *PresenceManager) refreshLoop() {
	defer close(pm.done)

	// Create local ticker to avoid race conditions
	ticker := time.NewTicker(constants.PresenceRefresh)
	defer ticker.Stop()

	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-ticker.C:
			pm.mu.Lock()
			if err := pm.publishPresence(); err != nil {
				fmt.Printf("Failed to refresh presence: %v\n", err)
			}
			pm.mu.Unlock()
		}
	}
}

// serializeRecord serializes a presence record for DHT storage
func (pm *PresenceManager) serializeRecord(record *PresenceRecord) ([]byte, error) {
	return cborcanon.Marshal(record)
}

// serializeHandleIndex serializes a handle index for DHT storage
func (pm *PresenceManager) serializeHandleIndex(record *PresenceRecord) ([]byte, error) {
	// Create a proper HandleIndex record
	handleIndex, err := NewHandleIndex(pm.swarmID, record.Handle, record.Bee)
	if err != nil {
		return nil, fmt.Errorf("failed to create handle index: %w", err)
	}

	// Sign the HandleIndex with the identity's private key
	if err := handleIndex.Sign(pm.identity.SigningPrivateKey); err != nil {
		return nil, fmt.Errorf("failed to sign handle index: %w", err)
	}

	return cborcanon.Marshal(handleIndex)
}
