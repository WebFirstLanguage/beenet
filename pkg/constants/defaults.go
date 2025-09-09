// Package constants defines cross-cutting constants from §21 defaults and §18 encodings
package constants

import "time"

// DHT Configuration (§21)
const (
	// DHT bucket size K=20, alpha=3
	DHTBucketSize = 20
	DHTAlpha      = 3
)

// Timing Configuration (§21)
const (
	// Presence TTL 10 min, refresh at 5 min
	PresenceTTL     = 10 * time.Minute
	PresenceRefresh = 5 * time.Minute

	// Honeytag HandleIndex expire ≈ 20 min
	HandleIndexExpire = 20 * time.Minute

	// Bare-name lease 90 days, refresh at ≤60%
	BareNameLease        = 90 * 24 * time.Hour
	BareNameRefreshRatio = 0.6

	// Gossip heartbeat 1s, mesh degree 6-12
	GossipHeartbeat = 1 * time.Second
	GossipMeshMin   = 6
	GossipMeshMax   = 12

	// SWIM failure detection timeouts
	SWIMPingTimeout     = 1 * time.Second  // Direct ping timeout
	SWIMIndirectTimeout = 3 * time.Second  // Indirect ping timeout
	SWIMSuspicionTime   = 10 * time.Second // Time to remain in suspect state
	SWIMProbeInterval   = 5 * time.Second  // Interval between probes

	// Max tolerated clock skew ±120s
	MaxClockSkew = 120 * time.Second

	// Buzz interval 5s
	BuzzInterval = 5 * time.Second
)

// Data Configuration (§21)
const (
	// Chunk size 1 MiB, concurrent chunk fetch 4
	ChunkSize            = 1024 * 1024 // 1 MiB
	ConcurrentChunkFetch = 4

	// DHT Configuration (K=20 already defined as DHTBucketSize above)
	// DHTAlpha = 3 already defined above
)

// Protocol Configuration (§18)
const (
	// Protocol version
	ProtocolVersion = 1

	// Default ports
	DefaultQUICPort = 27487
	DefaultBuzzPort = 27488

	// Hash algorithm: BLAKE3-256 by default
	HashAlgorithm = "blake3-256"

	// Text encoding: UTF-8, NFC on input, names normalized to NFKC
	TextEncoding = "utf-8"
)

// BeeQuint-32 Configuration (§4.1)
const (
	// Consonants and vowels for proquint encoding
	Consonants = "bdfghjklmnprstv z"
	Vowels     = "aeiou"
)

// Error Codes (§17)
const (
	ErrorInvalidSig      = 1
	ErrorNotInSwarm      = 2
	ErrorNoProvider      = 3
	ErrorRateLimit       = 4
	ErrorVersionMismatch = 5

	// Honeytag-specific errors
	ErrorNameNotFound      = 20
	ErrorNameLeaseExpired  = 21
	ErrorHandleMismatch    = 22
	ErrorNotOwner          = 23
	ErrorDelegationMissing = 24
)

// Message Kinds (§15)
const (
	KindPing             = 1
	KindPong             = 2
	KindDHTGet           = 10
	KindDHTPut           = 11
	KindAnnouncePresence = 20
	KindPubSubMsg        = 30
	KindFetchChunk       = 40
	KindChunkData        = 41
	KindHoneytagOp       = 50

	// SWIM Protocol Message Kinds
	KindSWIMPing     = 60 // Direct ping for liveness detection
	KindSWIMPingReq  = 61 // Indirect ping request
	KindSWIMPingResp = 62 // Indirect ping response
	KindSWIMAck      = 63 // Acknowledgment of ping
	KindSWIMNack     = 64 // Negative acknowledgment
	KindSWIMSuspect  = 65 // Suspect member announcement
	KindSWIMAlive    = 66 // Alive member announcement
	KindSWIMConfirm  = 67 // Confirm member failure
	KindSWIMLeave    = 68 // Voluntary leave announcement

	// Gossip Protocol Message Kinds
	KindGossipIHave     = 70 // IHAVE message with message IDs
	KindGossipIWant     = 71 // IWANT message requesting specific messages
	KindGossipGraft     = 72 // GRAFT message to join topic mesh
	KindGossipPrune     = 73 // PRUNE message to leave topic mesh
	KindGossipHeartbeat = 74 // Heartbeat to maintain mesh connections
)
