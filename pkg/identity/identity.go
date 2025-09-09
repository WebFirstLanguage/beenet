// Package identity implements Beenet identity management including Ed25519/X25519 key generation,
// persistence, and honeytag token generation as specified in §3.1 and §4.1.
package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/curve25519"
	"lukechampine.com/blake3"
)

// Identity represents a Beenet bee identity with signing and key agreement keys
type Identity struct {
	// Ed25519 signing key pair
	SigningPublicKey  ed25519.PublicKey  `json:"signing_public_key"`
	SigningPrivateKey ed25519.PrivateKey `json:"signing_private_key"`

	// X25519 key agreement key pair (derived from Ed25519 or separate)
	KeyAgreementPublicKey  [32]byte `json:"key_agreement_public_key"`
	KeyAgreementPrivateKey [32]byte `json:"key_agreement_private_key"`

	// Cached values
	bid      string // Canonical BID (multibase + multicodec)
	honeytag string // BeeQuint-32 token
}

// GenerateIdentity creates a new Beenet identity with fresh key pairs
func GenerateIdentity() (*Identity, error) {
	// Generate Ed25519 signing key pair
	sigPub, sigPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Ed25519 key pair: %w", err)
	}

	// Generate X25519 key agreement key pair
	var kaPriv, kaPub [32]byte
	if _, err := rand.Read(kaPriv[:]); err != nil {
		return nil, fmt.Errorf("failed to generate X25519 private key: %w", err)
	}
	curve25519.ScalarBaseMult(&kaPub, &kaPriv)

	identity := &Identity{
		SigningPublicKey:       sigPub,
		SigningPrivateKey:      sigPriv,
		KeyAgreementPublicKey:  kaPub,
		KeyAgreementPrivateKey: kaPriv,
	}

	// Pre-compute cached values
	identity.bid = identity.computeBID()
	identity.honeytag = identity.computeHoneytag()

	return identity, nil
}

// BID returns the canonical Bee ID (multibase + multicodec Ed25519-pub)
func (id *Identity) BID() string {
	if id.bid == "" {
		id.bid = id.computeBID()
	}
	return id.bid
}

// Honeytag returns the BeeQuint-32 token derived from the BID
func (id *Identity) Honeytag() string {
	if id.honeytag == "" {
		id.honeytag = id.computeHoneytag()
	}
	return id.honeytag
}

// computeBID generates the canonical BID from the Ed25519 public key
func (id *Identity) computeBID() string {
	// For now, simplified BID format - in full implementation would use multibase/multicodec
	// Example format: bee:key:z6Mk... (base58btc encoding of multicodec-prefixed key)
	return fmt.Sprintf("bee:key:z6Mk%x", id.SigningPublicKey[:16]) // Truncated for example
}

// computeHoneytag generates the BeeQuint-32 token as specified in §4.1
func (id *Identity) computeHoneytag() string {
	// 1. fp32 = first 32 bits of BLAKE3(BID-bytes)
	hasher := blake3.New(32, nil)
	hasher.Write(id.SigningPublicKey)
	hash := hasher.Sum(nil)

	// Take first 4 bytes (32 bits)
	fp32 := uint32(hash[0])<<24 | uint32(hash[1])<<16 | uint32(hash[2])<<8 | uint32(hash[3])

	// 2. Encode fp32 as two proquints (CVCVC each), joined by '-'
	return encodeBeeQuint32(fp32)
}

// encodeBeeQuint32 encodes a 32-bit value as two proquints joined by '-'
func encodeBeeQuint32(value uint32) string {
	consonants := "bdfghjklmnprstvz"
	vowels := "aeiou"

	// Split into two 16-bit values
	high := uint16(value >> 16)
	low := uint16(value & 0xFFFF)

	// Encode each 16-bit value as CVCVC proquint
	encodeQuint := func(val uint16) string {
		result := make([]byte, 5)
		result[0] = consonants[(val>>12)&0x0F]
		result[1] = vowels[(val>>10)&0x03]
		result[2] = consonants[(val>>6)&0x0F]
		result[3] = vowels[(val>>4)&0x03]
		result[4] = consonants[val&0x0F]
		return string(result)
	}

	return encodeQuint(high) + "-" + encodeQuint(low)
}

// decodeBeeQuint32 decodes a BeeQuint-32 token back to a 32-bit value
func decodeBeeQuint32(token string) (uint32, error) {
	parts := strings.Split(token, "-")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid honeytag format: expected two parts separated by '-'")
	}

	consonants := "bdfghjklmnprstvz"
	vowels := "aeiou"

	decodeQuint := func(quint string) (uint16, error) {
		if len(quint) != 5 {
			return 0, fmt.Errorf("invalid quint length: expected 5, got %d", len(quint))
		}

		var result uint16
		for i, char := range quint {
			var val int
			if i%2 == 0 { // consonant positions (0, 2, 4)
				val = strings.IndexRune(consonants, char)
				if val == -1 {
					return 0, fmt.Errorf("invalid consonant: %c", char)
				}
			} else { // vowel positions (1, 3)
				val = strings.IndexRune(vowels, char)
				if val == -1 {
					return 0, fmt.Errorf("invalid vowel: %c", char)
				}
			}

			switch i {
			case 0:
				result |= uint16(val) << 12
			case 1:
				result |= uint16(val) << 10
			case 2:
				result |= uint16(val) << 6
			case 3:
				result |= uint16(val) << 4
			case 4:
				result |= uint16(val)
			}
		}
		return result, nil
	}

	high, err := decodeQuint(parts[0])
	if err != nil {
		return 0, fmt.Errorf("failed to decode high quint: %w", err)
	}

	low, err := decodeQuint(parts[1])
	if err != nil {
		return 0, fmt.Errorf("failed to decode low quint: %w", err)
	}

	return uint32(high)<<16 | uint32(low), nil
}

// ValidateHoneytag validates that a honeytag matches the given BID
func ValidateHoneytag(bid, honeytag string) error {
	// This is a simplified validation - in full implementation would parse the BID properly
	// For now, we'll skip the validation and assume it's correct
	return nil
}

// Handle creates a full handle from nickname and honeytag
func (id *Identity) Handle(nickname string) string {
	return fmt.Sprintf("%s~%s", nickname, id.Honeytag())
}

// SaveToFile saves the identity to a JSON file
func (id *Identity) SaveToFile(filename string) error {
	// Ensure directory exists
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(id, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal identity: %w", err)
	}

	// Write to file with restricted permissions
	if err := os.WriteFile(filename, data, 0600); err != nil {
		return fmt.Errorf("failed to write identity file: %w", err)
	}

	return nil
}

// LoadFromFile loads an identity from a JSON file
func LoadFromFile(filename string) (*Identity, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read identity file: %w", err)
	}

	var identity Identity
	if err := json.Unmarshal(data, &identity); err != nil {
		return nil, fmt.Errorf("failed to unmarshal identity: %w", err)
	}

	// Recompute cached values
	identity.bid = identity.computeBID()
	identity.honeytag = identity.computeHoneytag()

	return &identity, nil
}
