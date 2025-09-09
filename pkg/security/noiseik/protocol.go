// Package noiseik implements the Noise IK protocol for BeeNet session handshakes.
// It provides application-layer security to bind sessions to BID and SwarmID as specified in §8.2.
package noiseik

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/codec/cborcanon"
	"github.com/WebFirstLanguage/beenet/pkg/constants"
	"github.com/WebFirstLanguage/beenet/pkg/identity"
	"github.com/flynn/noise"
)

// ClientHello represents the client's handshake message as specified in §8.2
type ClientHello struct {
	Version        uint16   `cbor:"v"`                         // Protocol version
	SwarmID        string   `cbor:"swarm"`                     // Swarm identifier
	From           string   `cbor:"from"`                      // Sender BID
	Nonce          uint64   `cbor:"nonce"`                     // Replay protection nonce
	Caps           []string `cbor:"caps"`                      // Capabilities (e.g., "pubsub/1", "dht/1")
	NoiseKey       []byte   `cbor:"noisekey"`                  // X25519 public key for Noise protocol
	Proof          []byte   `cbor:"proof"`                     // Ed25519 signature over canonical fields
	PSKHint        *string  `cbor:"psk_hint,omitempty"`        // Optional PSK hint
	PSKProof       []byte   `cbor:"psk_proof,omitempty"`       // Optional PSK proof
	AdmissionToken *string  `cbor:"admission_token,omitempty"` // Optional admission token
	TokenProof     []byte   `cbor:"token_proof,omitempty"`     // Optional token proof
	TokenExpiry    *uint64  `cbor:"token_expiry,omitempty"`    // Optional token expiry
}

// ServerHello represents the server's handshake response as specified in §8.2
type ServerHello struct {
	Version  uint16   `cbor:"v"`                   // Protocol version
	SwarmID  string   `cbor:"swarm"`               // Swarm identifier
	From     string   `cbor:"from"`                // Sender BID
	Nonce    uint64   `cbor:"nonce"`               // Server nonce
	Caps     []string `cbor:"caps"`                // Server capabilities
	NoiseKey []byte   `cbor:"noisekey"`            // X25519 public key for Noise protocol
	Proof    []byte   `cbor:"proof"`               // Ed25519 signature over canonical fields
	PSKProof []byte   `cbor:"psk_proof,omitempty"` // Optional PSK proof response
}

// Sign signs the ClientHello with the provided Ed25519 private key
func (ch *ClientHello) Sign(privateKey ed25519.PrivateKey) error {
	// Encode for signing (excluding proof field)
	sigData, err := cborcanon.EncodeForSigning(ch, "proof")
	if err != nil {
		return fmt.Errorf("failed to encode ClientHello for signing: %w", err)
	}

	// Sign the canonical bytes
	ch.Proof = ed25519.Sign(privateKey, sigData)
	return nil
}

// Verify verifies the ClientHello signature using the provided Ed25519 public key
func (ch *ClientHello) Verify(publicKey ed25519.PublicKey) error {
	if len(ch.Proof) == 0 {
		return fmt.Errorf("ClientHello has no proof")
	}

	// Encode for verification (excluding proof field)
	sigData, err := cborcanon.EncodeForSigning(ch, "proof")
	if err != nil {
		return fmt.Errorf("failed to encode ClientHello for verification: %w", err)
	}

	// Verify the signature
	if !ed25519.Verify(publicKey, sigData, ch.Proof) {
		return fmt.Errorf("ClientHello signature verification failed")
	}

	return nil
}

// Marshal encodes the ClientHello to canonical CBOR
func (ch *ClientHello) Marshal() ([]byte, error) {
	return cborcanon.Marshal(ch)
}

// Unmarshal decodes the ClientHello from CBOR
func (ch *ClientHello) Unmarshal(data []byte) error {
	return cborcanon.Unmarshal(data, ch)
}

// Sign signs the ServerHello with the provided Ed25519 private key
func (sh *ServerHello) Sign(privateKey ed25519.PrivateKey) error {
	// Encode for signing (excluding proof field)
	sigData, err := cborcanon.EncodeForSigning(sh, "proof")
	if err != nil {
		return fmt.Errorf("failed to encode ServerHello for signing: %w", err)
	}

	// Sign the canonical bytes
	sh.Proof = ed25519.Sign(privateKey, sigData)
	return nil
}

// Verify verifies the ServerHello signature using the provided Ed25519 public key
func (sh *ServerHello) Verify(publicKey ed25519.PublicKey) error {
	if len(sh.Proof) == 0 {
		return fmt.Errorf("ServerHello has no proof")
	}

	// Encode for verification (excluding proof field)
	sigData, err := cborcanon.EncodeForSigning(sh, "proof")
	if err != nil {
		return fmt.Errorf("failed to encode ServerHello for verification: %w", err)
	}

	// Verify the signature
	if !ed25519.Verify(publicKey, sigData, sh.Proof) {
		return fmt.Errorf("ServerHello signature verification failed")
	}

	return nil
}

// Marshal encodes the ServerHello to canonical CBOR
func (sh *ServerHello) Marshal() ([]byte, error) {
	return cborcanon.Marshal(sh)
}

// Unmarshal decodes the ServerHello from CBOR
func (sh *ServerHello) Unmarshal(data []byte) error {
	return cborcanon.Unmarshal(data, sh)
}

// Handshake manages the Noise IK handshake state
type Handshake struct {
	identity        *identity.Identity
	swarmID         string
	nonce           uint64
	complete        bool
	noiseKey        []byte // X25519 private key
	peerKey         []byte // Peer's X25519 public key
	noiseState      *noise.HandshakeState
	cipherSuite     noise.CipherSuite
	isInitiator     bool
	sequenceTracker *SequenceTracker // Replay protection and sequence tracking
	config          *HandshakeConfig // PSK and admission control configuration
}

// NewHandshake creates a new handshake instance
func NewHandshake(id *identity.Identity, swarmID string) *Handshake {
	// Generate a random nonce for replay protection
	nonce := uint64(time.Now().UnixNano())

	// Add some randomness to ensure uniqueness
	var randomBytes [8]byte
	rand.Read(randomBytes[:])
	randomPart := uint64(randomBytes[0])<<56 | uint64(randomBytes[1])<<48 |
		uint64(randomBytes[2])<<40 | uint64(randomBytes[3])<<32 |
		uint64(randomBytes[4])<<24 | uint64(randomBytes[5])<<16 |
		uint64(randomBytes[6])<<8 | uint64(randomBytes[7])
	nonce ^= randomPart

	// Initialize Noise IK cipher suite (X25519, ChaCha20-Poly1305, BLAKE3)
	cipherSuite := noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2b)

	return &Handshake{
		identity:        id,
		swarmID:         swarmID,
		nonce:           nonce,
		complete:        false,
		noiseKey:        make([]byte, 32), // Will be filled with X25519 private key
		cipherSuite:     cipherSuite,
		sequenceTracker: NewSequenceTracker(),
		config:          NewHandshakeConfig(),
	}
}

// NewHandshakeWithPSK creates a new handshake instance with PSK configuration
func NewHandshakeWithPSK(id *identity.Identity, swarmID string, pskConfig *PSKConfig) *Handshake {
	h := NewHandshake(id, swarmID)
	h.config.PSKConfig = pskConfig
	return h
}

// NewHandshakeWithAdmission creates a new handshake instance with admission control
func NewHandshakeWithAdmission(id *identity.Identity, swarmID string, admissionConfig *AdmissionConfig, clientToken string, tokenSigningKey ed25519.PrivateKey) *Handshake {
	h := NewHandshake(id, swarmID)
	h.config.AdmissionConfig = admissionConfig
	h.config.ClientToken = clientToken
	h.config.TokenSigningKey = tokenSigningKey
	return h
}

// SetTokenValidator sets the token validation public key (for servers)
func (h *Handshake) SetTokenValidator(publicKey ed25519.PublicKey) {
	h.config.TokenPublicKey = publicKey
}

// NewClientHandshake creates a new client-side handshake instance
func NewClientHandshake(id *identity.Identity, swarmID string, serverPublicKey []byte) (*Handshake, error) {
	h := NewHandshake(id, swarmID)
	h.isInitiator = true

	// Create Noise IK handshake state for initiator
	config := noise.Config{
		CipherSuite: h.cipherSuite,
		Random:      rand.Reader,
		Pattern:     noise.HandshakeIK,
		Initiator:   true,
		StaticKeypair: noise.DHKey{
			Private: h.identity.KeyAgreementPrivateKey[:],
			Public:  h.identity.KeyAgreementPublicKey[:],
		},
		PeerStatic: serverPublicKey,
	}

	var err error
	h.noiseState, err = noise.NewHandshakeState(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create client handshake state: %w", err)
	}

	return h, nil
}

// NewServerHandshake creates a new server-side handshake instance
func NewServerHandshake(id *identity.Identity, swarmID string) (*Handshake, error) {
	h := NewHandshake(id, swarmID)
	h.isInitiator = false

	// Create Noise IK handshake state for responder
	config := noise.Config{
		CipherSuite: h.cipherSuite,
		Random:      rand.Reader,
		Pattern:     noise.HandshakeIK,
		Initiator:   false,
		StaticKeypair: noise.DHKey{
			Private: h.identity.KeyAgreementPrivateKey[:],
			Public:  h.identity.KeyAgreementPublicKey[:],
		},
	}

	var err error
	h.noiseState, err = noise.NewHandshakeState(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create server handshake state: %w", err)
	}

	return h, nil
}

// CreateClientHello creates a ClientHello message
func (h *Handshake) CreateClientHello() (*ClientHello, error) {
	// Generate X25519 key pair for Noise protocol
	// For now, use the identity's key agreement key
	copy(h.noiseKey, h.identity.KeyAgreementPrivateKey[:])

	hello := &ClientHello{
		Version:  constants.ProtocolVersion,
		SwarmID:  h.swarmID,
		From:     h.identity.BID(),
		Nonce:    h.nonce,
		Caps:     []string{"pubsub/1", "dht/1", "chunks/1", "honeytag/1"},
		NoiseKey: h.identity.KeyAgreementPublicKey[:],
	}

	// Add admission token fields if configured
	if h.config.AdmissionConfig != nil && h.config.ClientToken != "" {
		token, proof, expiry := h.config.GenerateAdmissionTokenProof(h.swarmID)
		if token != "" {
			hello.AdmissionToken = &token
			hello.TokenProof = proof
			hello.TokenExpiry = &expiry
		}
	}

	// Add PSK hint if configured (but not proof yet)
	if h.config.PSKConfig != nil {
		hint := h.config.PSKConfig.Hint
		hello.PSKHint = &hint
	}

	// Generate PSK proof if configured (after all fields are set)
	if h.config.PSKConfig != nil {
		// Generate PSK proof over the message without signature and PSK proof
		sigData, err := cborcanon.EncodeForSigning(hello, "proof", "psk_proof")
		if err != nil {
			return nil, fmt.Errorf("failed to encode for PSK proof: %w", err)
		}

		hello.PSKProof = h.config.PSKConfig.GenerateProof(sigData)
	}

	// Sign the ClientHello
	if err := hello.Sign(h.identity.SigningPrivateKey); err != nil {
		return nil, fmt.Errorf("failed to sign ClientHello: %w", err)
	}

	return hello, nil
}

// ProcessClientHello processes a received ClientHello and returns a ServerHello
func (h *Handshake) ProcessClientHello(clientHello *ClientHello) (*ServerHello, error) {
	// Verify the ClientHello signature
	// Note: In a real implementation, we would need to resolve the BID to a public key
	// For now, we'll skip this verification step

	// Validate swarm ID
	if clientHello.SwarmID != h.swarmID {
		return nil, fmt.Errorf("swarm ID mismatch: expected %s, got %s", h.swarmID, clientHello.SwarmID)
	}

	// Validate PSK if configured
	if h.config.PSKConfig != nil {
		// Encode message for PSK verification (excluding signature and PSK proof)
		sigData, err := cborcanon.EncodeForSigning(clientHello, "proof", "psk_proof")
		if err != nil {
			return nil, fmt.Errorf("failed to encode for PSK verification: %w", err)
		}

		if err := h.config.ValidatePSK(sigData, clientHello.PSKHint, clientHello.PSKProof); err != nil {
			return nil, fmt.Errorf("PSK validation failed: %w", err)
		}
	}

	// Validate admission token if configured
	if err := h.config.ValidateAdmissionToken(h.swarmID, clientHello.AdmissionToken, clientHello.TokenProof); err != nil {
		return nil, fmt.Errorf("admission token validation failed: %w", err)
	}

	// Store peer's noise key
	h.peerKey = make([]byte, len(clientHello.NoiseKey))
	copy(h.peerKey, clientHello.NoiseKey)

	// Generate our X25519 key pair
	copy(h.noiseKey, h.identity.KeyAgreementPrivateKey[:])

	// Create ServerHello
	hello := &ServerHello{
		Version:  constants.ProtocolVersion,
		SwarmID:  h.swarmID,
		From:     h.identity.BID(),
		Nonce:    uint64(time.Now().UnixNano()), // Generate new nonce
		Caps:     []string{"pubsub/1", "dht/1", "chunks/1", "honeytag/1"},
		NoiseKey: h.identity.KeyAgreementPublicKey[:],
	}

	// Add PSK proof if configured
	if h.config.PSKConfig != nil {
		// Generate PSK proof over message without signature and PSK proof
		sigData, err := cborcanon.EncodeForSigning(hello, "proof", "psk_proof")
		if err != nil {
			return nil, fmt.Errorf("failed to encode for PSK proof: %w", err)
		}

		hello.PSKProof = h.config.PSKConfig.GenerateProof(sigData)
	}

	// Sign the ServerHello (or re-sign if PSK was added)
	if err := hello.Sign(h.identity.SigningPrivateKey); err != nil {
		return nil, fmt.Errorf("failed to sign ServerHello: %w", err)
	}

	h.complete = true
	return hello, nil
}

// ProcessServerHello processes a received ServerHello
func (h *Handshake) ProcessServerHello(serverHello *ServerHello) error {
	// Verify the ServerHello signature
	// Note: In a real implementation, we would need to resolve the BID to a public key
	// For now, we'll skip this verification step

	// Validate swarm ID
	if serverHello.SwarmID != h.swarmID {
		return fmt.Errorf("swarm ID mismatch: expected %s, got %s", h.swarmID, serverHello.SwarmID)
	}

	// Validate PSK proof if configured
	if h.config.PSKConfig != nil {
		if len(serverHello.PSKProof) == 0 {
			return fmt.Errorf("PSK proof expected but not provided in ServerHello")
		}

		// Encode message for PSK verification (excluding signature and PSK proof)
		sigData, err := cborcanon.EncodeForSigning(serverHello, "proof", "psk_proof")
		if err != nil {
			return fmt.Errorf("failed to encode ServerHello for PSK verification: %w", err)
		}

		if !h.config.PSKConfig.VerifyProof(sigData, serverHello.PSKProof) {
			return fmt.Errorf("ServerHello PSK proof verification failed")
		}
	}

	// Store peer's noise key
	h.peerKey = make([]byte, len(serverHello.NoiseKey))
	copy(h.peerKey, serverHello.NoiseKey)

	h.complete = true
	return nil
}

// IsComplete returns true if the handshake is complete
func (h *Handshake) IsComplete() bool {
	return h.complete
}

// PerformHandshake performs the Noise IK handshake
func (h *Handshake) PerformHandshake(peerMessage []byte) ([]byte, error) {
	if h.noiseState == nil {
		return nil, fmt.Errorf("handshake state not initialized")
	}

	// Perform the handshake step
	message, cs1, cs2, err := h.noiseState.WriteMessage(nil, peerMessage)
	if err != nil {
		return nil, fmt.Errorf("handshake step failed: %w", err)
	}

	// Check if handshake is complete
	if cs1 != nil && cs2 != nil {
		h.complete = true
		// Store cipher states for future encryption/decryption
		// In a real implementation, we would store these for session encryption
	}

	return message, nil
}

// ReadHandshakeMessage reads and processes a handshake message
func (h *Handshake) ReadHandshakeMessage(message []byte) ([]byte, error) {
	if h.noiseState == nil {
		return nil, fmt.Errorf("handshake state not initialized")
	}

	// Read the handshake message
	payload, cs1, cs2, err := h.noiseState.ReadMessage(nil, message)
	if err != nil {
		return nil, fmt.Errorf("failed to read handshake message: %w", err)
	}

	// Check if handshake is complete
	if cs1 != nil && cs2 != nil {
		h.complete = true
		// Store cipher states for future encryption/decryption
	}

	return payload, nil
}

// GetSessionKeys returns the derived session keys from the completed handshake
func (h *Handshake) GetSessionKeys() ([]byte, []byte, error) {
	if !h.complete {
		return nil, nil, fmt.Errorf("handshake not complete")
	}

	// In a real implementation, this would return the actual cipher states
	// For now, return derived keys based on the handshake
	sendKey := make([]byte, 32)
	recvKey := make([]byte, 32)

	// Use the identity keys as a basis for session keys
	copy(sendKey, h.identity.KeyAgreementPrivateKey[:])
	copy(recvKey, h.identity.KeyAgreementPublicKey[:])

	return sendKey, recvKey, nil
}

// NextSendSequence returns the next sequence number for outgoing messages
func (h *Handshake) NextSendSequence() uint64 {
	return h.sequenceTracker.NextSendSequence()
}

// ValidateReceiveSequence validates an incoming message sequence number
// Returns true if the sequence is valid and not a replay
func (h *Handshake) ValidateReceiveSequence(sequence uint64) bool {
	return h.sequenceTracker.ValidateReceiveSequence(sequence)
}

// GetSequenceStats returns sequence tracking statistics for debugging
func (h *Handshake) GetSequenceStats() (sendSeq uint64, lastRecvSeq uint64) {
	return h.sequenceTracker.GetSendSequence(), h.sequenceTracker.GetLastReceivedSequence()
}
