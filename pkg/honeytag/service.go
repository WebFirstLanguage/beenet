// Package honeytag implements the Honeytag/1 service for name management
package honeytag

import (
	"context"
	"fmt"
	"time"

	"github.com/WebFirstLanguage/beenet/internal/dht"
	"github.com/WebFirstLanguage/beenet/pkg/codec/cborcanon"
	"github.com/WebFirstLanguage/beenet/pkg/constants"
	"github.com/WebFirstLanguage/beenet/pkg/identity"
)

// Service implements the Honeytag/1 name management service
type Service struct {
	dht      *dht.DHT
	identity *identity.Identity
	swarmID  string
	resolver *Resolver

	// Owned names for lease management
	ownedNames map[string]*OwnedName
}

// OwnedName represents a name owned by this node
type OwnedName struct {
	Name         string
	Record       *NameRecord
	LastRefresh  time.Time
	NextRefresh  time.Time
	RefreshTimer *time.Timer
}

// NewService creates a new honeytag service
func NewService(dht *dht.DHT, identity *identity.Identity, swarmID string) *Service {
	return &Service{
		dht:        dht,
		identity:   identity,
		swarmID:    swarmID,
		resolver:   NewResolver(dht, swarmID),
		ownedNames: make(map[string]*OwnedName),
	}
}

// ClaimName claims a new bare name
func (s *Service) ClaimName(ctx context.Context, name string) error {
	// Validate name format
	if !isValidNickname(name) {
		return fmt.Errorf("invalid name format: %s", name)
	}

	// Check if name is already claimed
	key := K_name(s.swarmID, name)
	existingData, err := s.dht.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to check existing name: %w", err)
	}

	if existingData != nil {
		var existing NameRecord
		if err := cborcanon.Unmarshal(existingData, &existing); err == nil && !existing.IsExpired() {
			return fmt.Errorf("name already claimed by %s", existing.Owner)
		}
	}

	// Create new NameRecord
	record := NewNameRecord(s.swarmID, name, s.identity.BID(), 1, constants.BareNameLease)
	if err := record.Sign(s.identity.SigningPrivateKey); err != nil {
		return fmt.Errorf("failed to sign name record: %w", err)
	}

	// Store in DHT
	recordData, err := cborcanon.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal name record: %w", err)
	}

	if err := s.dht.Put(ctx, key, recordData); err != nil {
		return fmt.Errorf("failed to store name record: %w", err)
	}

	// Add to owned names for lease management
	s.addOwnedName(name, record)

	return nil
}

// RefreshName refreshes the lease on an owned name
func (s *Service) RefreshName(ctx context.Context, name string) error {
	owned, exists := s.ownedNames[name]
	if !exists {
		return fmt.Errorf("name not owned: %s", name)
	}

	// Create refreshed record with incremented version
	record := NewNameRecord(s.swarmID, name, s.identity.BID(), owned.Record.Ver+1, constants.BareNameLease)
	if err := record.Sign(s.identity.SigningPrivateKey); err != nil {
		return fmt.Errorf("failed to sign name record: %w", err)
	}

	// Store in DHT
	key := K_name(s.swarmID, name)
	recordData, err := cborcanon.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal name record: %w", err)
	}

	if err := s.dht.Put(ctx, key, recordData); err != nil {
		return fmt.Errorf("failed to store name record: %w", err)
	}

	// Update owned name
	owned.Record = record
	owned.LastRefresh = time.Now()
	s.scheduleNextRefresh(owned)

	return nil
}

// ReleaseName releases ownership of a name
func (s *Service) ReleaseName(ctx context.Context, name string) error {
	owned, exists := s.ownedNames[name]
	if !exists {
		return fmt.Errorf("name not owned: %s", name)
	}

	// Create release record with lease set to current time (immediate expiry)
	now := uint64(time.Now().UnixMilli())
	record := &NameRecord{
		V:     1,
		Swarm: s.swarmID,
		Name:  name,
		Owner: s.identity.BID(),
		Ver:   owned.Record.Ver + 1,
		TS:    now,
		Lease: now, // Immediate expiry
	}

	if err := record.Sign(s.identity.SigningPrivateKey); err != nil {
		return fmt.Errorf("failed to sign release record: %w", err)
	}

	// Store in DHT
	key := K_name(s.swarmID, name)
	recordData, err := cborcanon.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal release record: %w", err)
	}

	if err := s.dht.Put(ctx, key, recordData); err != nil {
		return fmt.Errorf("failed to store release record: %w", err)
	}

	// Remove from owned names
	s.removeOwnedName(name)

	return nil
}

// TransferName transfers ownership of a name to another owner
func (s *Service) TransferName(ctx context.Context, name, newOwner string) error {
	owned, exists := s.ownedNames[name]
	if !exists {
		return fmt.Errorf("name not owned: %s", name)
	}

	// For now, implement a simplified transfer (in full implementation would require new owner signature)
	record := NewNameRecord(s.swarmID, name, newOwner, owned.Record.Ver+1, constants.BareNameLease)
	if err := record.Sign(s.identity.SigningPrivateKey); err != nil {
		return fmt.Errorf("failed to sign transfer record: %w", err)
	}

	// Store in DHT
	key := K_name(s.swarmID, name)
	recordData, err := cborcanon.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal transfer record: %w", err)
	}

	if err := s.dht.Put(ctx, key, recordData); err != nil {
		return fmt.Errorf("failed to store transfer record: %w", err)
	}

	// Remove from owned names
	s.removeOwnedName(name)

	return nil
}

// Resolve resolves a query to addresses and proofs
func (s *Service) Resolve(ctx context.Context, query string, preferredCaps []string) (*ResolveResult, error) {
	return s.resolver.Resolve(ctx, query, preferredCaps)
}

// addOwnedName adds a name to the owned names map and schedules refresh
func (s *Service) addOwnedName(name string, record *NameRecord) {
	owned := &OwnedName{
		Name:        name,
		Record:      record,
		LastRefresh: time.Now(),
	}

	s.scheduleNextRefresh(owned)
	s.ownedNames[name] = owned
}

// removeOwnedName removes a name from the owned names map
func (s *Service) removeOwnedName(name string) {
	if owned, exists := s.ownedNames[name]; exists {
		if owned.RefreshTimer != nil {
			owned.RefreshTimer.Stop()
		}
		delete(s.ownedNames, name)
	}
}

// scheduleNextRefresh schedules the next refresh for an owned name
func (s *Service) scheduleNextRefresh(owned *OwnedName) {
	// Calculate next refresh time (60% of lease duration)
	leaseDuration := time.Duration(owned.Record.Lease-owned.Record.TS) * time.Millisecond
	refreshInterval := time.Duration(float64(leaseDuration) * constants.BareNameRefreshRatio)
	owned.NextRefresh = owned.LastRefresh.Add(refreshInterval)

	// Cancel existing timer
	if owned.RefreshTimer != nil {
		owned.RefreshTimer.Stop()
	}

	// Schedule new timer
	timeUntilRefresh := time.Until(owned.NextRefresh)
	if timeUntilRefresh < 0 {
		timeUntilRefresh = time.Second // Refresh immediately if overdue
	}

	owned.RefreshTimer = time.AfterFunc(timeUntilRefresh, func() {
		ctx := context.Background()
		if err := s.RefreshName(ctx, owned.Name); err != nil {
			fmt.Printf("Failed to auto-refresh name %s: %v\n", owned.Name, err)
		}
	})
}

// ValidatePresenceHoneytag validates that a presence record's honeytag matches its BID
func (s *Service) ValidatePresenceHoneytag(presence *dht.PresenceRecord) error {
	if presence == nil {
		return nil
	}

	// Extract honeytag from handle
	handleParts := parseHandle(presence.Handle)
	if handleParts == nil {
		return fmt.Errorf("invalid handle format: %s", presence.Handle)
	}

	// Validate honeytag matches BID
	return identity.ValidateHoneytag(presence.Bee, handleParts.Honeytag)
}

// GetOwnedNames returns a list of names owned by this node
func (s *Service) GetOwnedNames() []string {
	names := make([]string, 0, len(s.ownedNames))
	for name := range s.ownedNames {
		names = append(names, name)
	}
	return names
}
