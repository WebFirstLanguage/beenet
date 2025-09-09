// Package honeytag implements the Honeytag/1 name resolution system as specified in §12
package honeytag

import (
	"crypto/ed25519"
	"fmt"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/codec/cborcanon"
	"github.com/WebFirstLanguage/beenet/pkg/constants"
	"github.com/WebFirstLanguage/beenet/pkg/identity"
	"lukechampine.com/blake3"
)

// NameRecord represents bare-name ownership as specified in §12.3
type NameRecord struct {
	V     uint16 `cbor:"v"`     // Version (always 1)
	Swarm string `cbor:"swarm"` // SwarmID
	Name  string `cbor:"name"`  // Normalized nickname (bare)
	Owner string `cbor:"owner"` // Owner BID
	Ver   uint64 `cbor:"ver"`   // Monotonic version by owner
	TS    uint64 `cbor:"ts"`    // Timestamp (ms since Unix epoch)
	Lease uint64 `cbor:"lease"` // Absolute ms epoch; <= ts + T_lease_max
	Sig   []byte `cbor:"sig"`   // Ed25519 signature over canonical(...)
}

// HandleIndex represents handle → BID binding as specified in §12.3
type HandleIndex struct {
	V      uint16 `cbor:"v"`      // Version (always 1)
	Swarm  string `cbor:"swarm"`  // SwarmID
	Handle string `cbor:"handle"` // "nickname~honeytag"
	BID    string `cbor:"bid"`    // BID
	TS     uint64 `cbor:"ts"`     // Timestamp (ms since Unix epoch)
	Expire uint64 `cbor:"expire"` // Expiration timestamp (~10-30 min)
	Sig    []byte `cbor:"sig"`    // Ed25519 signature over canonical(...)
}

// DelegationRecord allows multiple devices to represent a bare name as specified in §12.3
type DelegationRecord struct {
	V        uint16   `cbor:"v"`         // Version (always 1)
	Swarm    string   `cbor:"swarm"`     // SwarmID
	Owner    string   `cbor:"owner"`     // Owner BID
	Device   string   `cbor:"device"`    // Device BID
	Caps     []string `cbor:"caps"`      // Capabilities ["presence", "handle-index"]
	Ver      uint64   `cbor:"ver"`       // Monotonic version by owner
	TS       uint64   `cbor:"ts"`        // Timestamp (ms since Unix epoch)
	Expire   uint64   `cbor:"expire"`    // Expiration timestamp
	SigOwner []byte   `cbor:"sig_owner"` // Ed25519 signature by owner
}

// NewNameRecord creates a new NameRecord for claiming a bare name
func NewNameRecord(swarmID, name, ownerBID string, ver uint64, leaseDuration time.Duration) *NameRecord {
	now := uint64(time.Now().UnixMilli())
	return &NameRecord{
		V:     1,
		Swarm: swarmID,
		Name:  name,
		Owner: ownerBID,
		Ver:   ver,
		TS:    now,
		Lease: now + uint64(leaseDuration.Milliseconds()),
	}
}

// NewHandleIndex creates a new HandleIndex for handle → BID binding
func NewHandleIndex(swarmID, handle, bid string) *HandleIndex {
	now := uint64(time.Now().UnixMilli())
	return &HandleIndex{
		V:      1,
		Swarm:  swarmID,
		Handle: handle,
		BID:    bid,
		TS:     now,
		Expire: now + uint64(constants.HandleIndexExpire.Milliseconds()),
	}
}

// NewDelegationRecord creates a new DelegationRecord for owner→device delegation
func NewDelegationRecord(swarmID, ownerBID, deviceBID string, caps []string, ver uint64, expireDuration time.Duration) *DelegationRecord {
	now := uint64(time.Now().UnixMilli())
	return &DelegationRecord{
		V:      1,
		Swarm:  swarmID,
		Owner:  ownerBID,
		Device: deviceBID,
		Caps:   caps,
		Ver:    ver,
		TS:     now,
		Expire: now + uint64(expireDuration.Milliseconds()),
	}
}

// Sign signs the NameRecord with the owner's private key
func (nr *NameRecord) Sign(privateKey ed25519.PrivateKey) error {
	sigData, err := cborcanon.EncodeForSigning(nr, "sig")
	if err != nil {
		return fmt.Errorf("failed to encode NameRecord for signing: %w", err)
	}
	nr.Sig = ed25519.Sign(privateKey, sigData)
	return nil
}

// Sign signs the HandleIndex with the BID's private key
func (hi *HandleIndex) Sign(privateKey ed25519.PrivateKey) error {
	sigData, err := cborcanon.EncodeForSigning(hi, "sig")
	if err != nil {
		return fmt.Errorf("failed to encode HandleIndex for signing: %w", err)
	}
	hi.Sig = ed25519.Sign(privateKey, sigData)
	return nil
}

// Sign signs the DelegationRecord with the owner's private key
func (dr *DelegationRecord) Sign(privateKey ed25519.PrivateKey) error {
	sigData, err := cborcanon.EncodeForSigning(dr, "sig_owner")
	if err != nil {
		return fmt.Errorf("failed to encode DelegationRecord for signing: %w", err)
	}
	dr.SigOwner = ed25519.Sign(privateKey, sigData)
	return nil
}

// IsExpired checks if the NameRecord lease has expired
func (nr *NameRecord) IsExpired() bool {
	return uint64(time.Now().UnixMilli()) > nr.Lease
}

// IsExpired checks if the HandleIndex has expired
func (hi *HandleIndex) IsExpired() bool {
	return uint64(time.Now().UnixMilli()) > hi.Expire
}

// IsExpired checks if the DelegationRecord has expired
func (dr *DelegationRecord) IsExpired() bool {
	return uint64(time.Now().UnixMilli()) > dr.Expire
}

// NeedsRefresh checks if the NameRecord needs to be refreshed (at 60% of lease)
func (nr *NameRecord) NeedsRefresh() bool {
	now := uint64(time.Now().UnixMilli())
	refreshTime := nr.TS + uint64(float64(nr.Lease-nr.TS)*constants.BareNameRefreshRatio)
	return now >= refreshTime
}

// ValidateHoneytag validates that the handle's honeytag matches the BID
func (hi *HandleIndex) ValidateHoneytag() error {
	// Extract honeytag from handle (everything after ~)
	handleParts := parseHandle(hi.Handle)
	if handleParts == nil {
		return fmt.Errorf("invalid handle format: %s", hi.Handle)
	}

	// Validate that the honeytag matches the BID
	return identity.ValidateHoneytag(hi.BID, handleParts.Honeytag)
}

// HandleParts represents the parsed components of a handle
type HandleParts struct {
	Nickname string
	Honeytag string
}

// parseHandle parses a handle into nickname and honeytag components
func parseHandle(handle string) *HandleParts {
	// Find the last ~ character
	for i := len(handle) - 1; i >= 0; i-- {
		if handle[i] == '~' {
			if i == 0 || i == len(handle)-1 {
				return nil // Invalid: ~ at start or end
			}
			return &HandleParts{
				Nickname: handle[:i],
				Honeytag: handle[i+1:],
			}
		}
	}
	return nil // No ~ found
}

// DHT Key Generation Functions as specified in §12.3

// K_name generates the DHT key for a NameRecord
func K_name(swarmID, name string) []byte {
	hasher := blake3.New(32, nil)
	hasher.Write([]byte("name"))
	hasher.Write([]byte(swarmID))
	hasher.Write([]byte(name))
	return hasher.Sum(nil)
}

// K_handle generates the DHT key for a HandleIndex
func K_handle(swarmID, handle string) []byte {
	hasher := blake3.New(32, nil)
	hasher.Write([]byte("handle"))
	hasher.Write([]byte(swarmID))
	hasher.Write([]byte(handle))
	return hasher.Sum(nil)
}

// K_owner generates the DHT key for DelegationRecords
func K_owner(swarmID, ownerBID string) []byte {
	hasher := blake3.New(32, nil)
	hasher.Write([]byte("owner"))
	hasher.Write([]byte(swarmID))
	hasher.Write([]byte(ownerBID))
	return hasher.Sum(nil)
}

// K_presence generates the DHT key for PresenceRecords (from existing DHT implementation)
func K_presence(swarmID, bid string) []byte {
	hasher := blake3.New(32, nil)
	hasher.Write([]byte("presence"))
	hasher.Write([]byte(swarmID))
	hasher.Write([]byte(bid))
	return hasher.Sum(nil)
}
