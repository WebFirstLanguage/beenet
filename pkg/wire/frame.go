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
	CID    string  `cbor:"cid"`              // Content ID
	Offset *uint64 `cbor:"offset,omitempty"` // Optional byte offset
}

// ChunkDataBody represents the body of a CHUNK_DATA message (§15)
type ChunkDataBody struct {
	CID  string `cbor:"cid"`  // Content ID
	Off  uint64 `cbor:"off"`  // Byte offset
	Data []byte `cbor:"data"` // Chunk data
}

// SWIM Protocol Message Bodies

// SWIMPingBody represents the body of a SWIM_PING message
type SWIMPingBody struct {
	Target string `cbor:"target"` // Target BID to ping
	SeqNo  uint64 `cbor:"seq_no"` // Sequence number for this ping
}

// SWIMPingReqBody represents the body of a SWIM_PING_REQ message (indirect ping)
type SWIMPingReqBody struct {
	Target    string `cbor:"target"`    // Target BID to ping
	SeqNo     uint64 `cbor:"seq_no"`    // Sequence number for this ping
	Requestor string `cbor:"requestor"` // BID of the original requestor
	Timeout   uint64 `cbor:"timeout"`   // Timeout in milliseconds
}

// SWIMPingRespBody represents the body of a SWIM_PING_RESP message
type SWIMPingRespBody struct {
	Target    string `cbor:"target"`    // Target BID that was pinged
	SeqNo     uint64 `cbor:"seq_no"`    // Sequence number from the original ping
	Requestor string `cbor:"requestor"` // BID of the original requestor
	Success   bool   `cbor:"success"`   // Whether the ping was successful
}

// SWIMAckBody represents the body of a SWIM_ACK message
type SWIMAckBody struct {
	SeqNo uint64 `cbor:"seq_no"` // Sequence number being acknowledged
}

// SWIMNackBody represents the body of a SWIM_NACK message
type SWIMNackBody struct {
	SeqNo uint64 `cbor:"seq_no"` // Sequence number being negatively acknowledged
}

// SWIMSuspectBody represents the body of a SWIM_SUSPECT message
type SWIMSuspectBody struct {
	Target      string `cbor:"target"`      // BID of the suspected member
	Incarnation uint64 `cbor:"incarnation"` // Incarnation number of the suspected member
}

// SWIMAliveBody represents the body of a SWIM_ALIVE message
type SWIMAliveBody struct {
	Target      string   `cbor:"target"`      // BID of the alive member
	Incarnation uint64   `cbor:"incarnation"` // Incarnation number of the alive member
	Addrs       []string `cbor:"addrs"`       // Current addresses of the member
}

// SWIMConfirmBody represents the body of a SWIM_CONFIRM message
type SWIMConfirmBody struct {
	Target      string `cbor:"target"`      // BID of the confirmed failed member
	Incarnation uint64 `cbor:"incarnation"` // Incarnation number of the failed member
}

// SWIMLeaveBody represents the body of a SWIM_LEAVE message
type SWIMLeaveBody struct {
	Incarnation uint64 `cbor:"incarnation"` // Incarnation number of the leaving member
}

// Gossip Protocol Message Bodies

// GossipIHaveBody represents the body of a GOSSIP_IHAVE message
type GossipIHaveBody struct {
	Topic      string   `cbor:"topic"`       // Topic ID
	MessageIDs []string `cbor:"message_ids"` // List of message IDs we have
}

// GossipIWantBody represents the body of a GOSSIP_IWANT message
type GossipIWantBody struct {
	MessageIDs []string `cbor:"message_ids"` // List of message IDs we want
}

// GossipGraftBody represents the body of a GOSSIP_GRAFT message
type GossipGraftBody struct {
	Topic string `cbor:"topic"` // Topic ID to join
}

// GossipPruneBody represents the body of a GOSSIP_PRUNE message
type GossipPruneBody struct {
	Topic string   `cbor:"topic"` // Topic ID to leave
	Peers []string `cbor:"peers"` // Alternative peers for the topic
}

// GossipHeartbeatBody represents the body of a GOSSIP_HEARTBEAT message
type GossipHeartbeatBody struct {
	Topics []string `cbor:"topics"` // List of topics we're subscribed to
}

// PubSubMessageEnvelope represents the envelope for PubSub messages (§10)
type PubSubMessageEnvelope struct {
	MID     string `cbor:"mid"`     // Message ID: multihash(blake3-256, payload || from || seq)
	From    string `cbor:"from"`    // Sender BID
	Seq     uint64 `cbor:"seq"`     // Sequence number
	TS      uint64 `cbor:"ts"`      // Timestamp (ms since epoch)
	Topic   string `cbor:"topic"`   // Topic ID
	Payload []byte `cbor:"payload"` // Message payload
	Sig     []byte `cbor:"sig"`     // Ed25519 signature over canonical envelope
}

// Honeytag Protocol Message Bodies

// HoneytagOpBody represents the body of a HONEYTAG_OP message (§12.4)
type HoneytagOpBody struct {
	Op   string      `cbor:"op"`   // Operation: "claim"|"refresh"|"release"|"transfer"|"delegate"|"revoke"|"resolve"
	Body interface{} `cbor:"body"` // Operation-specific body
}

// HoneytagClaimBody represents the body of a claim operation
type HoneytagClaimBody struct {
	Name  string `cbor:"name"`  // Bare name to claim
	Ver   uint64 `cbor:"ver"`   // Version number
	TS    uint64 `cbor:"ts"`    // Timestamp
	Lease uint64 `cbor:"lease"` // Lease expiration
}

// HoneytagRefreshBody represents the body of a refresh operation
type HoneytagRefreshBody struct {
	Name  string `cbor:"name"`  // Bare name to refresh
	Ver   uint64 `cbor:"ver"`   // Version number (prev+1)
	TS    uint64 `cbor:"ts"`    // Timestamp
	Lease uint64 `cbor:"lease"` // New lease expiration
}

// HoneytagReleaseBody represents the body of a release operation
type HoneytagReleaseBody struct {
	Name  string `cbor:"name"`  // Bare name to release
	Ver   uint64 `cbor:"ver"`   // Version number (prev+1)
	TS    uint64 `cbor:"ts"`    // Timestamp
	Lease uint64 `cbor:"lease"` // Set to ts (immediate expiry)
}

// HoneytagTransferBody represents the body of a transfer operation
type HoneytagTransferBody struct {
	Name     string `cbor:"name"`      // Bare name to transfer
	NewOwner string `cbor:"new_owner"` // New owner BID
	Ver      uint64 `cbor:"ver"`       // Version number (prev+1)
	TS       uint64 `cbor:"ts"`        // Timestamp
	SigOwner []byte `cbor:"sig_owner"` // Signature by current owner
	SigNew   []byte `cbor:"sig_new"`   // Signature by new owner
}

// HoneytagDelegateBody represents the body of a delegate operation
type HoneytagDelegateBody struct {
	Owner    string   `cbor:"owner"`     // Owner BID
	Device   string   `cbor:"device"`    // Device BID
	Caps     []string `cbor:"caps"`      // Capabilities
	Ver      uint64   `cbor:"ver"`       // Version number
	TS       uint64   `cbor:"ts"`        // Timestamp
	Expire   uint64   `cbor:"expire"`    // Expiration timestamp
	SigOwner []byte   `cbor:"sig_owner"` // Signature by owner
}

// HoneytagRevokeBody represents the body of a revoke operation
type HoneytagRevokeBody struct {
	Owner    string `cbor:"owner"`     // Owner BID
	Device   string `cbor:"device"`    // Device BID to revoke
	Ver      uint64 `cbor:"ver"`       // Version number
	TS       uint64 `cbor:"ts"`        // Timestamp
	SigOwner []byte `cbor:"sig_owner"` // Signature by owner
}

// HoneytagResolveBody represents the body of a resolve operation
type HoneytagResolveBody struct {
	Query         string   `cbor:"query"`          // Query string (BID, handle, or bare name)
	PreferredCaps []string `cbor:"preferred_caps"` // Optional preferred capabilities
}

// HoneytagResolveResponse represents the response to a resolve operation
type HoneytagResolveResponse struct {
	Kind   string               `cbor:"kind"`   // "bid"|"handle"|"bare"
	Owner  string               `cbor:"owner"`  // Owner BID if known
	Device string               `cbor:"device"` // Device BID (may be same as owner)
	Handle string               `cbor:"handle"` // Handle if applicable
	Addrs  []string             `cbor:"addrs"`  // Multiaddresses if available
	Proof  HoneytagResolveProof `cbor:"proof"`  // Cryptographic proofs
}

// HoneytagResolveProof contains cryptographic proofs for resolution
type HoneytagResolveProof struct {
	Name        interface{}   `cbor:"name,omitempty"`         // NameRecord if applicable
	HandleIndex interface{}   `cbor:"handle_index,omitempty"` // HandleIndex if applicable
	Presence    interface{}   `cbor:"presence,omitempty"`     // PresenceRecord if applicable
	Delegation  interface{}   `cbor:"delegation,omitempty"`   // DelegationRecord if applicable
	Conflicts   []interface{} `cbor:"conflicts,omitempty"`    // Conflicting records if any
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

// Content Protocol Helper Functions

// NewFetchChunkFrame creates a new FETCH_CHUNK frame
func NewFetchChunkFrame(from string, seq uint64, cid string, offset *uint64) *BaseFrame {
	return NewBaseFrame(constants.KindFetchChunk, from, seq, &FetchChunkBody{
		CID:    cid,
		Offset: offset,
	})
}

// NewChunkDataFrame creates a new CHUNK_DATA frame
func NewChunkDataFrame(from string, seq uint64, cid string, offset uint64, data []byte) *BaseFrame {
	return NewBaseFrame(constants.KindChunkData, from, seq, &ChunkDataBody{
		CID:  cid,
		Off:  offset,
		Data: data,
	})
}

// SWIM Protocol Helper Functions

// NewSWIMPingFrame creates a new SWIM_PING frame
func NewSWIMPingFrame(from string, seq uint64, target string, seqNo uint64) *BaseFrame {
	return NewBaseFrame(constants.KindSWIMPing, from, seq, &SWIMPingBody{
		Target: target,
		SeqNo:  seqNo,
	})
}

// NewSWIMPingReqFrame creates a new SWIM_PING_REQ frame
func NewSWIMPingReqFrame(from string, seq uint64, target, requestor string, seqNo, timeout uint64) *BaseFrame {
	return NewBaseFrame(constants.KindSWIMPingReq, from, seq, &SWIMPingReqBody{
		Target:    target,
		SeqNo:     seqNo,
		Requestor: requestor,
		Timeout:   timeout,
	})
}

// NewSWIMAckFrame creates a new SWIM_ACK frame
func NewSWIMAckFrame(from string, seq uint64, seqNo uint64) *BaseFrame {
	return NewBaseFrame(constants.KindSWIMAck, from, seq, &SWIMAckBody{
		SeqNo: seqNo,
	})
}

// NewSWIMSuspectFrame creates a new SWIM_SUSPECT frame
func NewSWIMSuspectFrame(from string, seq uint64, target string, incarnation uint64) *BaseFrame {
	return NewBaseFrame(constants.KindSWIMSuspect, from, seq, &SWIMSuspectBody{
		Target:      target,
		Incarnation: incarnation,
	})
}

// Gossip Protocol Helper Functions

// NewGossipIHaveFrame creates a new GOSSIP_IHAVE frame
func NewGossipIHaveFrame(from string, seq uint64, topic string, messageIDs []string) *BaseFrame {
	return NewBaseFrame(constants.KindGossipIHave, from, seq, &GossipIHaveBody{
		Topic:      topic,
		MessageIDs: messageIDs,
	})
}

// NewGossipIWantFrame creates a new GOSSIP_IWANT frame
func NewGossipIWantFrame(from string, seq uint64, messageIDs []string) *BaseFrame {
	return NewBaseFrame(constants.KindGossipIWant, from, seq, &GossipIWantBody{
		MessageIDs: messageIDs,
	})
}

// NewGossipGraftFrame creates a new GOSSIP_GRAFT frame
func NewGossipGraftFrame(from string, seq uint64, topic string) *BaseFrame {
	return NewBaseFrame(constants.KindGossipGraft, from, seq, &GossipGraftBody{
		Topic: topic,
	})
}

// NewGossipPruneFrame creates a new GOSSIP_PRUNE frame
func NewGossipPruneFrame(from string, seq uint64, topic string, peers []string) *BaseFrame {
	return NewBaseFrame(constants.KindGossipPrune, from, seq, &GossipPruneBody{
		Topic: topic,
		Peers: peers,
	})
}

// NewPubSubMessageFrame creates a new PUBSUB_MSG frame
func NewPubSubMessageFrame(from string, seq uint64, envelope *PubSubMessageEnvelope) *BaseFrame {
	return NewBaseFrame(constants.KindPubSubMsg, from, seq, envelope)
}

// Honeytag Protocol Helper Functions

// NewHoneytagOpFrame creates a new HONEYTAG_OP frame
func NewHoneytagOpFrame(from string, seq uint64, op string, body interface{}) *BaseFrame {
	return NewBaseFrame(constants.KindHoneytagOp, from, seq, &HoneytagOpBody{
		Op:   op,
		Body: body,
	})
}

// NewHoneytagClaimFrame creates a new honeytag claim frame
func NewHoneytagClaimFrame(from string, seq uint64, name string, ver uint64, ts uint64, lease uint64) *BaseFrame {
	return NewHoneytagOpFrame(from, seq, "claim", &HoneytagClaimBody{
		Name:  name,
		Ver:   ver,
		TS:    ts,
		Lease: lease,
	})
}

// NewHoneytagRefreshFrame creates a new honeytag refresh frame
func NewHoneytagRefreshFrame(from string, seq uint64, name string, ver uint64, ts uint64, lease uint64) *BaseFrame {
	return NewHoneytagOpFrame(from, seq, "refresh", &HoneytagRefreshBody{
		Name:  name,
		Ver:   ver,
		TS:    ts,
		Lease: lease,
	})
}

// NewHoneytagReleaseFrame creates a new honeytag release frame
func NewHoneytagReleaseFrame(from string, seq uint64, name string, ver uint64, ts uint64) *BaseFrame {
	return NewHoneytagOpFrame(from, seq, "release", &HoneytagReleaseBody{
		Name:  name,
		Ver:   ver,
		TS:    ts,
		Lease: ts, // Set lease to ts for immediate expiry
	})
}

// NewHoneytagTransferFrame creates a new honeytag transfer frame
func NewHoneytagTransferFrame(from string, seq uint64, name string, newOwner string, ver uint64, ts uint64, sigOwner, sigNew []byte) *BaseFrame {
	return NewHoneytagOpFrame(from, seq, "transfer", &HoneytagTransferBody{
		Name:     name,
		NewOwner: newOwner,
		Ver:      ver,
		TS:       ts,
		SigOwner: sigOwner,
		SigNew:   sigNew,
	})
}

// NewHoneytagDelegateFrame creates a new honeytag delegate frame
func NewHoneytagDelegateFrame(from string, seq uint64, owner, device string, caps []string, ver uint64, ts uint64, expire uint64, sigOwner []byte) *BaseFrame {
	return NewHoneytagOpFrame(from, seq, "delegate", &HoneytagDelegateBody{
		Owner:    owner,
		Device:   device,
		Caps:     caps,
		Ver:      ver,
		TS:       ts,
		Expire:   expire,
		SigOwner: sigOwner,
	})
}

// NewHoneytagRevokeFrame creates a new honeytag revoke frame
func NewHoneytagRevokeFrame(from string, seq uint64, owner, device string, ver uint64, ts uint64, sigOwner []byte) *BaseFrame {
	return NewHoneytagOpFrame(from, seq, "revoke", &HoneytagRevokeBody{
		Owner:    owner,
		Device:   device,
		Ver:      ver,
		TS:       ts,
		SigOwner: sigOwner,
	})
}

// NewHoneytagResolveFrame creates a new honeytag resolve frame
func NewHoneytagResolveFrame(from string, seq uint64, query string, preferredCaps []string) *BaseFrame {
	return NewHoneytagOpFrame(from, seq, "resolve", &HoneytagResolveBody{
		Query:         query,
		PreferredCaps: preferredCaps,
	})
}
