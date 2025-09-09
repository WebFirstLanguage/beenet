package content

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/codec/cborcanon"
	"github.com/WebFirstLanguage/beenet/pkg/constants"
	"github.com/WebFirstLanguage/beenet/pkg/identity"
	"lukechampine.com/blake3"
)

// DHT interface for content provider operations
type DHTInterface interface {
	Put(ctx context.Context, key []byte, value []byte) error
	Get(ctx context.Context, key []byte) ([]byte, error)
}

// ContentProvider manages content provider records in the DHT
type ContentProvider struct {
	dht      DHTInterface
	identity *identity.Identity
	swarmID  string
	addrs    []string
}

// NewContentProvider creates a new content provider
func NewContentProvider(dht DHTInterface, identity *identity.Identity, swarmID string, addrs []string) *ContentProvider {
	return &ContentProvider{
		dht:      dht,
		identity: identity,
		swarmID:  swarmID,
		addrs:    addrs,
	}
}

// PublishContent announces that this node provides the given content
func (cp *ContentProvider) PublishContent(ctx context.Context, cid CID) error {
	// Create provide record
	record := &ProvideRecord{
		CID:       cid,
		Provider:  cp.identity.BID(),
		Addresses: cp.addrs,
		Timestamp: uint64(time.Now().UnixMilli()),
		TTL:       uint32(constants.PresenceTTL.Seconds()), // Convert to seconds
	}

	// Sign the record
	if err := cp.signProvideRecord(record); err != nil {
		return fmt.Errorf("failed to sign provide record: %w", err)
	}

	// Serialize the record
	recordData, err := cborcanon.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to serialize provide record: %w", err)
	}

	// Generate DHT key for this content
	key := generateProvideKey(cp.swarmID, cid.String)

	// Store in DHT
	if err := cp.dht.Put(ctx, key, recordData); err != nil {
		return fmt.Errorf("failed to store provide record in DHT: %w", err)
	}

	return nil
}

// LookupProviders finds providers for the given content
func (cp *ContentProvider) LookupProviders(ctx context.Context, cid CID) ([]*ProvideRecord, error) {
	// Generate DHT key for this content
	key := generateProvideKey(cp.swarmID, cid.String)

	// Lookup in DHT
	data, err := cp.dht.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup providers in DHT: %w", err)
	}

	if data == nil {
		return []*ProvideRecord{}, nil // No providers found
	}

	// Try to deserialize as a single record first
	var record ProvideRecord
	if err := cborcanon.Unmarshal(data, &record); err == nil {
		// Single record
		if cp.isProvideRecordValid(&record) {
			return []*ProvideRecord{&record}, nil
		}
		return []*ProvideRecord{}, nil
	}

	// Try to deserialize as an array of records
	var records []*ProvideRecord
	if err := cborcanon.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("failed to deserialize provide records: %w", err)
	}

	// Filter valid records
	validRecords := make([]*ProvideRecord, 0, len(records))
	for _, record := range records {
		if cp.isProvideRecordValid(record) {
			validRecords = append(validRecords, record)
		}
	}

	return validRecords, nil
}

// UnpublishContent removes the provide record for the given content
func (cp *ContentProvider) UnpublishContent(ctx context.Context, cid CID) error {
	// For now, we'll implement this by publishing an expired record
	// In a full implementation, we might want a DELETE operation

	record := &ProvideRecord{
		CID:       cid,
		Provider:  cp.identity.BID(),
		Addresses: cp.addrs,
		Timestamp: uint64(time.Now().Add(-time.Hour).UnixMilli()), // Set to past
		TTL:       1,                                              // 1 second TTL, but timestamp is in the past
	}

	// Sign the record
	if err := cp.signProvideRecord(record); err != nil {
		return fmt.Errorf("failed to sign unpublish record: %w", err)
	}

	// Serialize the record
	recordData, err := cborcanon.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to serialize unpublish record: %w", err)
	}

	// Generate DHT key for this content
	key := generateProvideKey(cp.swarmID, cid.String)

	// Store in DHT
	if err := cp.dht.Put(ctx, key, recordData); err != nil {
		return fmt.Errorf("failed to store unpublish record in DHT: %w", err)
	}

	return nil
}

// signProvideRecord signs a provide record
func (cp *ContentProvider) signProvideRecord(record *ProvideRecord) error {
	// Create canonical data to sign
	signData := make([]byte, 0, 256)

	// Add CID hash
	signData = append(signData, record.CID.Hash...)

	// Add provider BID
	signData = append(signData, []byte(record.Provider)...)

	// Add timestamp
	signData = append(signData,
		byte(record.Timestamp>>56), byte(record.Timestamp>>48),
		byte(record.Timestamp>>40), byte(record.Timestamp>>32),
		byte(record.Timestamp>>24), byte(record.Timestamp>>16),
		byte(record.Timestamp>>8), byte(record.Timestamp))

	// Add TTL
	signData = append(signData,
		byte(record.TTL>>24), byte(record.TTL>>16),
		byte(record.TTL>>8), byte(record.TTL))

	// Add addresses
	for _, addr := range record.Addresses {
		signData = append(signData, []byte(addr)...)
	}

	// Sign the data
	signature := ed25519.Sign(cp.identity.SigningPrivateKey, signData)
	record.Signature = signature

	return nil
}

// isProvideRecordValid checks if a provide record is valid and not expired
func (cp *ContentProvider) isProvideRecordValid(record *ProvideRecord) bool {
	if record == nil {
		return false
	}

	// Check if expired
	now := uint64(time.Now().UnixMilli())
	expiryTime := record.Timestamp + uint64(record.TTL)*1000 // Convert TTL to milliseconds
	if now > expiryTime {
		return false
	}

	// Check required fields
	if !record.CID.IsValid() || record.Provider == "" || len(record.Addresses) == 0 {
		return false
	}

	// TODO: Verify signature
	// In a full implementation, we would extract the public key from the provider BID
	// and verify the signature

	return true
}

// generateProvideKey generates a DHT key for content provider records
func generateProvideKey(swarmID, cidString string) []byte {
	// K_provide = H("provide" | SwarmID | CID)
	data := []byte("provide")
	data = append(data, []byte(swarmID)...)
	data = append(data, []byte(cidString)...)
	hash := blake3.Sum256(data)
	return hash[:]
}

// GetProviderStats returns statistics about provider operations
func (cp *ContentProvider) GetProviderStats() map[string]interface{} {
	return map[string]interface{}{
		"provider_bid": cp.identity.BID(),
		"swarm_id":     cp.swarmID,
		"addresses":    cp.addrs,
	}
}

// MockDHT implements DHTInterface for testing
type MockDHT struct {
	storage map[string][]byte
}

// NewMockDHT creates a new mock DHT for testing
func NewMockDHT() *MockDHT {
	return &MockDHT{
		storage: make(map[string][]byte),
	}
}

// Put stores a value in the mock DHT
func (m *MockDHT) Put(ctx context.Context, key []byte, value []byte) error {
	keyStr := string(key)
	m.storage[keyStr] = make([]byte, len(value))
	copy(m.storage[keyStr], value)
	return nil
}

// Get retrieves a value from the mock DHT
func (m *MockDHT) Get(ctx context.Context, key []byte) ([]byte, error) {
	keyStr := string(key)
	if value, exists := m.storage[keyStr]; exists {
		result := make([]byte, len(value))
		copy(result, value)
		return result, nil
	}
	return nil, nil // Not found
}

// Clear clears all data from the mock DHT
func (m *MockDHT) Clear() {
	m.storage = make(map[string][]byte)
}

// Size returns the number of entries in the mock DHT
func (m *MockDHT) Size() int {
	return len(m.storage)
}
