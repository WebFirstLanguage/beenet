package content

import (
	"context"
	"testing"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/identity"
)

func TestNewContentProvider(t *testing.T) {
	// Create test identity
	id, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}

	// Create mock DHT
	mockDHT := NewMockDHT()

	// Create content provider
	swarmID := "test-swarm"
	addrs := []string{"/ip4/127.0.0.1/tcp/8080", "/ip6/::1/tcp/8080"}

	provider := NewContentProvider(mockDHT, id, swarmID, addrs)

	// Verify provider properties
	if provider.identity != id {
		t.Error("Provider identity mismatch")
	}

	if provider.swarmID != swarmID {
		t.Errorf("Provider swarm ID mismatch: got %s, want %s", provider.swarmID, swarmID)
	}

	if len(provider.addrs) != len(addrs) {
		t.Errorf("Provider addresses length mismatch: got %d, want %d", len(provider.addrs), len(addrs))
	}
}

func TestPublishContent(t *testing.T) {
	// Create test identity
	id, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}

	// Create mock DHT
	mockDHT := NewMockDHT()

	// Create content provider
	swarmID := "test-swarm"
	addrs := []string{"/ip4/127.0.0.1/tcp/8080"}
	provider := NewContentProvider(mockDHT, id, swarmID, addrs)

	// Create test CID
	testData := []byte("test content")
	cid := NewCID(testData)

	// Publish content
	ctx := context.Background()
	err = provider.PublishContent(ctx, cid)
	if err != nil {
		t.Fatalf("Failed to publish content: %v", err)
	}

	// Verify record was stored in DHT
	if mockDHT.Size() != 1 {
		t.Errorf("Expected 1 record in DHT, got %d", mockDHT.Size())
	}
}

func TestLookupProviders(t *testing.T) {
	// Create test identity
	id, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}

	// Create mock DHT
	mockDHT := NewMockDHT()

	// Create content provider
	swarmID := "test-swarm"
	addrs := []string{"/ip4/127.0.0.1/tcp/8080"}
	provider := NewContentProvider(mockDHT, id, swarmID, addrs)

	// Create test CID
	testData := []byte("test content")
	cid := NewCID(testData)

	// Test lookup with no providers
	ctx := context.Background()
	providers, err := provider.LookupProviders(ctx, cid)
	if err != nil {
		t.Fatalf("Failed to lookup providers: %v", err)
	}

	if len(providers) != 0 {
		t.Errorf("Expected 0 providers, got %d", len(providers))
	}

	// Publish content
	err = provider.PublishContent(ctx, cid)
	if err != nil {
		t.Fatalf("Failed to publish content: %v", err)
	}

	// Lookup providers again
	providers, err = provider.LookupProviders(ctx, cid)
	if err != nil {
		t.Fatalf("Failed to lookup providers after publish: %v", err)
	}

	if len(providers) != 1 {
		t.Errorf("Expected 1 provider, got %d", len(providers))
	}

	// Verify provider details
	if len(providers) > 0 {
		record := providers[0]

		if !record.CID.Equals(cid) {
			t.Error("Provider record CID mismatch")
		}

		if record.Provider != id.BID() {
			t.Errorf("Provider BID mismatch: got %s, want %s", record.Provider, id.BID())
		}

		if len(record.Addresses) != len(addrs) {
			t.Errorf("Provider addresses length mismatch: got %d, want %d",
				len(record.Addresses), len(addrs))
		}
	}
}

func TestUnpublishContent(t *testing.T) {
	// Create test identity
	id, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}

	// Create mock DHT
	mockDHT := NewMockDHT()

	// Create content provider
	swarmID := "test-swarm"
	addrs := []string{"/ip4/127.0.0.1/tcp/8080"}
	provider := NewContentProvider(mockDHT, id, swarmID, addrs)

	// Create test CID
	testData := []byte("test content")
	cid := NewCID(testData)

	// Publish content
	ctx := context.Background()
	err = provider.PublishContent(ctx, cid)
	if err != nil {
		t.Fatalf("Failed to publish content: %v", err)
	}

	// Verify content is published
	providers, err := provider.LookupProviders(ctx, cid)
	if err != nil {
		t.Fatalf("Failed to lookup providers: %v", err)
	}

	if len(providers) != 1 {
		t.Errorf("Expected 1 provider before unpublish, got %d", len(providers))
	}

	// Unpublish content
	err = provider.UnpublishContent(ctx, cid)
	if err != nil {
		t.Fatalf("Failed to unpublish content: %v", err)
	}

	// Verify content is no longer available (expired record)
	providers, err = provider.LookupProviders(ctx, cid)
	if err != nil {
		t.Fatalf("Failed to lookup providers after unpublish: %v", err)
	}

	if len(providers) != 0 {
		t.Errorf("Expected 0 providers after unpublish, got %d", len(providers))
	}
}

func TestProvideRecordExpiration(t *testing.T) {
	// Create test identity
	id, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}

	// Create content provider
	swarmID := "test-swarm"
	addrs := []string{"/ip4/127.0.0.1/tcp/8080"}
	provider := NewContentProvider(nil, id, swarmID, addrs)

	// Create test CID
	testData := []byte("test content")
	cid := NewCID(testData)

	// Test expired record
	expiredRecord := &ProvideRecord{
		CID:       cid,
		Provider:  id.BID(),
		Addresses: addrs,
		Timestamp: uint64(time.Now().Add(-2 * time.Hour).UnixMilli()), // 2 hours ago
		TTL:       3600,                                               // 1 hour TTL (expired)
		Signature: []byte("test-signature"),
	}

	if provider.isProvideRecordValid(expiredRecord) {
		t.Error("Expired record should not be valid")
	}

	// Test valid record
	validRecord := &ProvideRecord{
		CID:       cid,
		Provider:  id.BID(),
		Addresses: addrs,
		Timestamp: uint64(time.Now().UnixMilli()),
		TTL:       3600, // 1 hour TTL
		Signature: []byte("test-signature"),
	}

	if !provider.isProvideRecordValid(validRecord) {
		t.Error("Valid record should be valid")
	}
}

func TestProvideRecordValidation(t *testing.T) {
	// Create test identity
	id, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}

	// Create content provider
	swarmID := "test-swarm"
	addrs := []string{"/ip4/127.0.0.1/tcp/8080"}
	provider := NewContentProvider(nil, id, swarmID, addrs)

	// Create test CID
	testData := []byte("test content")
	cid := NewCID(testData)

	testCases := []struct {
		name   string
		record *ProvideRecord
		valid  bool
	}{
		{
			"nil record",
			nil,
			false,
		},
		{
			"invalid CID",
			&ProvideRecord{
				CID:       CID{Hash: []byte("invalid"), String: "invalid"},
				Provider:  id.BID(),
				Addresses: addrs,
				Timestamp: uint64(time.Now().UnixMilli()),
				TTL:       3600,
			},
			false,
		},
		{
			"empty provider",
			&ProvideRecord{
				CID:       cid,
				Provider:  "",
				Addresses: addrs,
				Timestamp: uint64(time.Now().UnixMilli()),
				TTL:       3600,
			},
			false,
		},
		{
			"no addresses",
			&ProvideRecord{
				CID:       cid,
				Provider:  id.BID(),
				Addresses: []string{},
				Timestamp: uint64(time.Now().UnixMilli()),
				TTL:       3600,
			},
			false,
		},
		{
			"valid record",
			&ProvideRecord{
				CID:       cid,
				Provider:  id.BID(),
				Addresses: addrs,
				Timestamp: uint64(time.Now().UnixMilli()),
				TTL:       3600,
			},
			true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			valid := provider.isProvideRecordValid(tc.record)
			if valid != tc.valid {
				t.Errorf("Expected validity %v, got %v", tc.valid, valid)
			}
		})
	}
}

func TestGenerateProvideKey(t *testing.T) {
	swarmID := "test-swarm"
	cidString := "bee:abcdef123456"

	key1 := generateProvideKey(swarmID, cidString)
	key2 := generateProvideKey(swarmID, cidString)

	// Same inputs should produce same key
	if len(key1) != len(key2) {
		t.Error("Keys have different lengths")
	}

	for i, b := range key1 {
		if key2[i] != b {
			t.Error("Keys don't match")
			break
		}
	}

	// Different inputs should produce different keys
	key3 := generateProvideKey("different-swarm", cidString)
	if len(key1) == len(key3) {
		same := true
		for i, b := range key1 {
			if key3[i] != b {
				same = false
				break
			}
		}
		if same {
			t.Error("Different swarm IDs should produce different keys")
		}
	}
}

func TestGetProviderStats(t *testing.T) {
	// Create test identity
	id, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}

	// Create content provider
	swarmID := "test-swarm"
	addrs := []string{"/ip4/127.0.0.1/tcp/8080"}
	provider := NewContentProvider(nil, id, swarmID, addrs)

	stats := provider.GetProviderStats()

	// Check expected fields
	expectedFields := []string{"provider_bid", "swarm_id", "addresses"}
	for _, field := range expectedFields {
		if _, exists := stats[field]; !exists {
			t.Errorf("Missing field in stats: %s", field)
		}
	}

	// Check values
	if stats["provider_bid"] != id.BID() {
		t.Errorf("Wrong provider BID in stats: got %v, want %s", stats["provider_bid"], id.BID())
	}

	if stats["swarm_id"] != swarmID {
		t.Errorf("Wrong swarm ID in stats: got %v, want %s", stats["swarm_id"], swarmID)
	}
}

func TestMockDHT(t *testing.T) {
	mockDHT := NewMockDHT()
	ctx := context.Background()

	// Test empty DHT
	if mockDHT.Size() != 0 {
		t.Errorf("New DHT should be empty, got size %d", mockDHT.Size())
	}

	// Test Put and Get
	key := []byte("test-key")
	value := []byte("test-value")

	err := mockDHT.Put(ctx, key, value)
	if err != nil {
		t.Fatalf("Failed to put value: %v", err)
	}

	if mockDHT.Size() != 1 {
		t.Errorf("Expected size 1 after put, got %d", mockDHT.Size())
	}

	retrievedValue, err := mockDHT.Get(ctx, key)
	if err != nil {
		t.Fatalf("Failed to get value: %v", err)
	}

	if string(retrievedValue) != string(value) {
		t.Errorf("Retrieved value mismatch: got %s, want %s", string(retrievedValue), string(value))
	}

	// Test Get non-existent key
	nonExistentValue, err := mockDHT.Get(ctx, []byte("non-existent"))
	if err != nil {
		t.Fatalf("Get should not error for non-existent key: %v", err)
	}

	if nonExistentValue != nil {
		t.Error("Non-existent key should return nil")
	}

	// Test Clear
	mockDHT.Clear()
	if mockDHT.Size() != 0 {
		t.Errorf("DHT should be empty after clear, got size %d", mockDHT.Size())
	}
}
