// Package dht implements DHT records as specified in §14
package dht

import (
	"crypto/ed25519"
	"fmt"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/codec/cborcanon"
	"github.com/WebFirstLanguage/beenet/pkg/constants"
	"github.com/WebFirstLanguage/beenet/pkg/identity"
	"lukechampine.com/blake3"
)

// PresenceRecord represents a signed presence record as specified in §14
type PresenceRecord struct {
	V      uint16   `cbor:"v"`      // Version (always 1)
	Swarm  string   `cbor:"swarm"`  // SwarmID
	Bee    string   `cbor:"bee"`    // BID (Bee ID)
	Handle string   `cbor:"handle"` // Handle (must match honeytag(bid))
	Addrs  []string `cbor:"addrs"`  // Multiaddresses
	Caps   []string `cbor:"caps"`   // Capabilities
	Expire uint64   `cbor:"expire"` // Expiration timestamp (ms since Unix epoch)
	Sig    []byte   `cbor:"sig"`    // Ed25519 signature
}

// ProvideRecord represents a content provider record as specified in §14
type ProvideRecord struct {
	V        uint16   `cbor:"v"`        // Version (always 1)
	Swarm    string   `cbor:"swarm"`    // SwarmID
	CID      string   `cbor:"cid"`      // Content ID
	Provider string   `cbor:"provider"` // Provider BID
	Addrs    []string `cbor:"addrs"`    // Multiaddresses
	Expire   uint64   `cbor:"expire"`   // Expiration timestamp (ms since Unix epoch)
	Sig      []byte   `cbor:"sig"`      // Ed25519 signature
}

// NewPresenceRecord creates a new presence record
func NewPresenceRecord(swarmID string, identity *identity.Identity, addrs []string, caps []string) (*PresenceRecord, error) {
	if identity == nil {
		return nil, fmt.Errorf("identity is required")
	}
	
	// Generate handle from identity
	nickname := "bee" // Default nickname, should be configurable
	handle := identity.Handle(nickname)
	
	// Calculate expiration time
	expire := time.Now().Add(constants.PresenceTTL).UnixMilli()
	
	record := &PresenceRecord{
		V:      1,
		Swarm:  swarmID,
		Bee:    identity.BID(),
		Handle: handle,
		Addrs:  addrs,
		Caps:   caps,
		Expire: uint64(expire),
	}
	
	// Sign the record
	if err := record.Sign(identity.SigningPrivateKey); err != nil {
		return nil, fmt.Errorf("failed to sign presence record: %w", err)
	}
	
	return record, nil
}

// Sign signs the presence record with the given private key
func (pr *PresenceRecord) Sign(privateKey ed25519.PrivateKey) error {
	// Create a copy without signature for signing
	unsigned := &PresenceRecord{
		V:      pr.V,
		Swarm:  pr.Swarm,
		Bee:    pr.Bee,
		Handle: pr.Handle,
		Addrs:  pr.Addrs,
		Caps:   pr.Caps,
		Expire: pr.Expire,
	}
	
	// Canonicalize the unsigned record
	canonical, err := cborcanon.Marshal(unsigned)
	if err != nil {
		return fmt.Errorf("failed to canonicalize record: %w", err)
	}
	
	// Sign the canonical representation
	pr.Sig = ed25519.Sign(privateKey, canonical)
	
	return nil
}

// Verify verifies the signature of the presence record
func (pr *PresenceRecord) Verify(publicKey ed25519.PublicKey) error {
	if len(pr.Sig) == 0 {
		return fmt.Errorf("record is not signed")
	}
	
	// Create a copy without signature for verification
	unsigned := &PresenceRecord{
		V:      pr.V,
		Swarm:  pr.Swarm,
		Bee:    pr.Bee,
		Handle: pr.Handle,
		Addrs:  pr.Addrs,
		Caps:   pr.Caps,
		Expire: pr.Expire,
	}
	
	// Canonicalize the unsigned record
	canonical, err := cborcanon.Marshal(unsigned)
	if err != nil {
		return fmt.Errorf("failed to canonicalize record: %w", err)
	}
	
	// Verify the signature
	if !ed25519.Verify(publicKey, canonical, pr.Sig) {
		return fmt.Errorf("invalid signature")
	}
	
	return nil
}

// IsExpired checks if the presence record has expired
func (pr *PresenceRecord) IsExpired() bool {
	return time.Now().UnixMilli() > int64(pr.Expire)
}

// IsValid performs basic validation of the presence record
func (pr *PresenceRecord) IsValid() error {
	if pr.V != 1 {
		return fmt.Errorf("invalid version: %d", pr.V)
	}
	
	if pr.Swarm == "" {
		return fmt.Errorf("swarm ID is required")
	}
	
	if pr.Bee == "" {
		return fmt.Errorf("bee ID is required")
	}
	
	if pr.Handle == "" {
		return fmt.Errorf("handle is required")
	}
	
	if len(pr.Addrs) == 0 {
		return fmt.Errorf("at least one address is required")
	}
	
	if pr.Expire == 0 {
		return fmt.Errorf("expiration time is required")
	}
	
	if len(pr.Sig) == 0 {
		return fmt.Errorf("signature is required")
	}
	
	return nil
}

// GetPresenceKey generates the DHT key for a presence record
func GetPresenceKey(swarmID, bid string) []byte {
	// K_presence = H("presence" | SwarmID | BID)
	data := []byte("presence")
	data = append(data, []byte(swarmID)...)
	data = append(data, []byte(bid)...)
	hash := blake3.Sum256(data)
	return hash[:]
}

// GetHandleKey generates the DHT key for a handle lookup
func GetHandleKey(swarmID, handle string) []byte {
	// K_handle = H("handle" | SwarmID | handle)
	data := []byte("handle")
	data = append(data, []byte(swarmID)...)
	data = append(data, []byte(handle)...)
	hash := blake3.Sum256(data)
	return hash[:]
}

// NewProvideRecord creates a new provide record
func NewProvideRecord(swarmID, cid, providerBID string, addrs []string, privateKey ed25519.PrivateKey) (*ProvideRecord, error) {
	// Calculate expiration time
	expire := time.Now().Add(constants.PresenceTTL).UnixMilli()
	
	record := &ProvideRecord{
		V:        1,
		Swarm:    swarmID,
		CID:      cid,
		Provider: providerBID,
		Addrs:    addrs,
		Expire:   uint64(expire),
	}
	
	// Sign the record
	if err := record.Sign(privateKey); err != nil {
		return nil, fmt.Errorf("failed to sign provide record: %w", err)
	}
	
	return record, nil
}

// Sign signs the provide record with the given private key
func (pr *ProvideRecord) Sign(privateKey ed25519.PrivateKey) error {
	// Create a copy without signature for signing
	unsigned := &ProvideRecord{
		V:        pr.V,
		Swarm:    pr.Swarm,
		CID:      pr.CID,
		Provider: pr.Provider,
		Addrs:    pr.Addrs,
		Expire:   pr.Expire,
	}
	
	// Canonicalize the unsigned record
	canonical, err := cborcanon.Marshal(unsigned)
	if err != nil {
		return fmt.Errorf("failed to canonicalize record: %w", err)
	}
	
	// Sign the canonical representation
	pr.Sig = ed25519.Sign(privateKey, canonical)
	
	return nil
}

// Verify verifies the signature of the provide record
func (pr *ProvideRecord) Verify(publicKey ed25519.PublicKey) error {
	if len(pr.Sig) == 0 {
		return fmt.Errorf("record is not signed")
	}
	
	// Create a copy without signature for verification
	unsigned := &ProvideRecord{
		V:        pr.V,
		Swarm:    pr.Swarm,
		CID:      pr.CID,
		Provider: pr.Provider,
		Addrs:    pr.Addrs,
		Expire:   pr.Expire,
	}
	
	// Canonicalize the unsigned record
	canonical, err := cborcanon.Marshal(unsigned)
	if err != nil {
		return fmt.Errorf("failed to canonicalize record: %w", err)
	}
	
	// Verify the signature
	if !ed25519.Verify(publicKey, canonical, pr.Sig) {
		return fmt.Errorf("invalid signature")
	}
	
	return nil
}

// IsExpired checks if the provide record has expired
func (pr *ProvideRecord) IsExpired() bool {
	return time.Now().UnixMilli() > int64(pr.Expire)
}

// IsValid performs basic validation of the provide record
func (pr *ProvideRecord) IsValid() error {
	if pr.V != 1 {
		return fmt.Errorf("invalid version: %d", pr.V)
	}
	
	if pr.Swarm == "" {
		return fmt.Errorf("swarm ID is required")
	}
	
	if pr.CID == "" {
		return fmt.Errorf("CID is required")
	}
	
	if pr.Provider == "" {
		return fmt.Errorf("provider BID is required")
	}
	
	if len(pr.Addrs) == 0 {
		return fmt.Errorf("at least one address is required")
	}
	
	if pr.Expire == 0 {
		return fmt.Errorf("expiration time is required")
	}
	
	if len(pr.Sig) == 0 {
		return fmt.Errorf("signature is required")
	}
	
	return nil
}

// GetProvideKey generates the DHT key for a provide record
func GetProvideKey(swarmID, cid string) []byte {
	// K_provide = H("provide" | SwarmID | CID)
	data := []byte("provide")
	data = append(data, []byte(swarmID)...)
	data = append(data, []byte(cid)...)
	hash := blake3.Sum256(data)
	return hash[:]
}
