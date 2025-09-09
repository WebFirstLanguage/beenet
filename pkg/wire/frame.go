// Package wire implements the Beenet base framing protocol as specified in §11.
// All Beenet control/app envelopes share a canonical CBOR structure and are
// individually signed with the sender's Ed25519 key.
package wire

import (
	"crypto/ed25519"
	"fmt"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/codec/cborcanon"
	"github.com/WebFirstLanguage/beenet/pkg/constants"
)

// BaseFrame represents the common structure for all Beenet protocol messages (§11)
type BaseFrame struct {
	V    uint16      `cbor:"v"`    // Protocol version
	Kind uint16      `cbor:"kind"` // Message kind (e.g., 1=PING, 2=PONG, 10=DHT_GET, etc.)
	From string      `cbor:"from"` // Sender BID (multibase/multicodec Ed25519-pub)
	Seq  uint64      `cbor:"seq"`  // Sequence number
	TS   uint64      `cbor:"ts"`   // Timestamp (ms since Unix epoch)
	Body interface{} `cbor:"body"` // Kind-specific CBOR payload
	Sig  []byte      `cbor:"sig"`  // Ed25519 signature over canonical(v|kind|from|seq|ts|body)
}

// NewBaseFrame creates a new BaseFrame with the current timestamp
func NewBaseFrame(kind uint16, from string, seq uint64, body interface{}) *BaseFrame {
	return &BaseFrame{
		V:    constants.ProtocolVersion,
		Kind: kind,
		From: from,
		Seq:  seq,
		TS:   uint64(time.Now().UnixMilli()),
		Body: body,
	}
}

// Sign signs the frame with the provided Ed25519 private key
func (f *BaseFrame) Sign(privateKey ed25519.PrivateKey) error {
	// Encode frame without signature for signing
	sigData, err := cborcanon.EncodeForSigning(f, "sig")
	if err != nil {
		return fmt.Errorf("failed to encode frame for signing: %w", err)
	}

	// Sign the canonical bytes
	f.Sig = ed25519.Sign(privateKey, sigData)
	return nil
}

// Verify verifies the frame signature using the provided Ed25519 public key
func (f *BaseFrame) Verify(publicKey ed25519.PublicKey) error {
	if len(f.Sig) == 0 {
		return fmt.Errorf("frame has no signature")
	}

	// Encode frame without signature for verification
	sigData, err := cborcanon.EncodeForSigning(f, "sig")
	if err != nil {
		return fmt.Errorf("failed to encode frame for verification: %w", err)
	}

	// Verify the signature
	if !ed25519.Verify(publicKey, sigData, f.Sig) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}

// Marshal encodes the frame to canonical CBOR
func (f *BaseFrame) Marshal() ([]byte, error) {
	return cborcanon.Marshal(f)
}

// Unmarshal decodes canonical CBOR data into the frame
func (f *BaseFrame) Unmarshal(data []byte) error {
	return cborcanon.Unmarshal(data, f)
}

// Validate performs basic validation on the frame
func (f *BaseFrame) Validate() error {
	if f.V != constants.ProtocolVersion {
		return NewError(constants.ErrorVersionMismatch, 
			fmt.Sprintf("unsupported protocol version: %d", f.V))
	}

	if f.From == "" {
		return NewError(constants.ErrorInvalidSig, "missing sender BID")
	}

	if len(f.Sig) == 0 {
		return NewError(constants.ErrorInvalidSig, "missing signature")
	}

	// Check timestamp is reasonable (within max clock skew)
	now := uint64(time.Now().UnixMilli())
	maxSkew := uint64(constants.MaxClockSkew.Milliseconds())
	
	if f.TS > now+maxSkew {
		return NewError(constants.ErrorVersionMismatch, "timestamp too far in future")
	}
	
	if now > f.TS+maxSkew {
		return NewError(constants.ErrorVersionMismatch, "timestamp too far in past")
	}

	return nil
}

// IsKind checks if the frame is of the specified kind
func (f *BaseFrame) IsKind(kind uint16) bool {
	return f.Kind == kind
}

// GetTimestamp returns the frame timestamp as a time.Time
func (f *BaseFrame) GetTimestamp() time.Time {
	return time.UnixMilli(int64(f.TS))
}

// Common frame body types for specific message kinds

// PingBody represents the body of a PING message (§15)
type PingBody struct {
	Token []byte `cbor:"token"` // 8-byte random token
}

// PongBody represents the body of a PONG message (§15)
type PongBody struct {
	Token []byte `cbor:"token"` // Echo of PING token
}

// DHTGetBody represents the body of a DHT_GET message (§15)
type DHTGetBody struct {
	Key []byte `cbor:"key"` // 32-byte key
}

// DHTPutBody represents the body of a DHT_PUT message (§15)
type DHTPutBody struct {
	Key   []byte `cbor:"key"`   // 32-byte key
	Value []byte `cbor:"value"` // CBOR-encoded value
	Sig   []byte `cbor:"sig"`   // Signature over key|value
}

// FetchChunkBody represents the body of a FETCH_CHUNK message (§15)
type FetchChunkBody struct {
	CID    string  `cbor:"cid"`            // Content ID
	Offset *uint64 `cbor:"offset,omitempty"` // Optional byte offset
}

// ChunkDataBody represents the body of a CHUNK_DATA message (§15)
type ChunkDataBody struct {
	CID  string `cbor:"cid"`  // Content ID
	Off  uint64 `cbor:"off"`  // Byte offset
	Data []byte `cbor:"data"` // Chunk data
}

// Helper functions for creating common frame types

// NewPingFrame creates a new PING frame
func NewPingFrame(from string, seq uint64, token []byte) *BaseFrame {
	return NewBaseFrame(constants.KindPing, from, seq, &PingBody{Token: token})
}

// NewPongFrame creates a new PONG frame
func NewPongFrame(from string, seq uint64, token []byte) *BaseFrame {
	return NewBaseFrame(constants.KindPong, from, seq, &PongBody{Token: token})
}

// NewDHTGetFrame creates a new DHT_GET frame
func NewDHTGetFrame(from string, seq uint64, key []byte) *BaseFrame {
	return NewBaseFrame(constants.KindDHTGet, from, seq, &DHTGetBody{Key: key})
}

// NewDHTPutFrame creates a new DHT_PUT frame
func NewDHTPutFrame(from string, seq uint64, key, value, sig []byte) *BaseFrame {
	return NewBaseFrame(constants.KindDHTPut, from, seq, &DHTPutBody{
		Key:   key,
		Value: value,
		Sig:   sig,
	})
}
