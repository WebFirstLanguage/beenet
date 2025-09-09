// Package noiseik implements PSK and token-based admission control for BeeNet handshakes
package noiseik

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"time"
)

// PSKConfig holds Pre-Shared Key configuration
type PSKConfig struct {
	PSK  []byte // The pre-shared key (should be at least 32 bytes)
	Hint string // Optional hint to identify which PSK to use
}

// NewPSKConfig creates a new PSK configuration
func NewPSKConfig(psk []byte, hint string) *PSKConfig {
	if len(psk) < 32 {
		// Pad PSK to 32 bytes if it's shorter
		paddedPSK := make([]byte, 32)
		copy(paddedPSK, psk)
		psk = paddedPSK
	}

	return &PSKConfig{
		PSK:  psk,
		Hint: hint,
	}
}

// GenerateProof generates an HMAC-SHA256 proof using the PSK
func (pc *PSKConfig) GenerateProof(message []byte) []byte {
	h := hmac.New(sha256.New, pc.PSK)
	h.Write(message)
	return h.Sum(nil)
}

// VerifyProof verifies an HMAC-SHA256 proof using the PSK
func (pc *PSKConfig) VerifyProof(message []byte, proof []byte) bool {
	expected := pc.GenerateProof(message)
	return hmac.Equal(expected, proof)
}

// TokenInfo holds information about an admission token
type TokenInfo struct {
	Token  string // The token string
	Expiry uint64 // Unix timestamp when the token expires
	Proof  []byte // Ed25519 signature proof
}

// AdmissionConfig holds token-based admission control configuration
type AdmissionConfig struct {
	RequireToken bool                 // Whether tokens are required
	ValidTokens  map[string]TokenInfo // Map of valid tokens
}

// NewAdmissionConfig creates a new admission control configuration
func NewAdmissionConfig() *AdmissionConfig {
	return &AdmissionConfig{
		RequireToken: false,
		ValidTokens:  make(map[string]TokenInfo),
	}
}

// AddToken adds a valid token to the admission configuration
func (ac *AdmissionConfig) AddToken(token string, expiry uint64, signingKey ed25519.PrivateKey) error {
	if len(token) == 0 {
		return fmt.Errorf("token cannot be empty")
	}

	// Create token info
	tokenInfo := TokenInfo{
		Token:  token,
		Expiry: expiry,
	}

	ac.ValidTokens[token] = tokenInfo
	return nil
}

// GenerateTokenProof generates an Ed25519 signature proof for a token
func (ac *AdmissionConfig) GenerateTokenProof(token, swarmID string, signingKey ed25519.PrivateKey) []byte {
	// Create message to sign: token + swarm_id + expiry
	tokenInfo, exists := ac.ValidTokens[token]
	if !exists {
		return nil
	}

	message := fmt.Sprintf("%s:%s:%d", token, swarmID, tokenInfo.Expiry)
	return ed25519.Sign(signingKey, []byte(message))
}

// ValidateToken validates a token and its proof
func (ac *AdmissionConfig) ValidateToken(token, swarmID string, proof []byte, publicKey ed25519.PublicKey) bool {
	// Check if token exists
	tokenInfo, exists := ac.ValidTokens[token]
	if !exists {
		return false
	}

	// Check if token is expired
	if uint64(time.Now().Unix()) > tokenInfo.Expiry {
		return false
	}

	// Verify the proof
	message := fmt.Sprintf("%s:%s:%d", token, swarmID, tokenInfo.Expiry)
	return ed25519.Verify(publicKey, []byte(message), proof)
}

// RemoveExpiredTokens removes expired tokens from the configuration
func (ac *AdmissionConfig) RemoveExpiredTokens() {
	now := uint64(time.Now().Unix())
	for token, info := range ac.ValidTokens {
		if now > info.Expiry {
			delete(ac.ValidTokens, token)
		}
	}
}

// HandshakeConfig combines PSK and admission control configurations
type HandshakeConfig struct {
	PSKConfig       *PSKConfig         // Optional PSK configuration
	AdmissionConfig *AdmissionConfig   // Optional admission control configuration
	ClientToken     string             // Token to use for client handshakes
	TokenSigningKey ed25519.PrivateKey // Key to sign tokens (for clients)
	TokenPublicKey  ed25519.PublicKey  // Key to verify tokens (for servers)
}

// NewHandshakeConfig creates a new handshake configuration
func NewHandshakeConfig() *HandshakeConfig {
	return &HandshakeConfig{}
}

// WithPSK adds PSK configuration to the handshake config
func (hc *HandshakeConfig) WithPSK(psk []byte, hint string) *HandshakeConfig {
	hc.PSKConfig = NewPSKConfig(psk, hint)
	return hc
}

// WithAdmissionControl adds admission control configuration
func (hc *HandshakeConfig) WithAdmissionControl(requireToken bool) *HandshakeConfig {
	hc.AdmissionConfig = NewAdmissionConfig()
	hc.AdmissionConfig.RequireToken = requireToken
	return hc
}

// WithClientToken sets the token for client handshakes
func (hc *HandshakeConfig) WithClientToken(token string, signingKey ed25519.PrivateKey) *HandshakeConfig {
	hc.ClientToken = token
	hc.TokenSigningKey = signingKey
	return hc
}

// WithTokenValidator sets the public key for token validation (server side)
func (hc *HandshakeConfig) WithTokenValidator(publicKey ed25519.PublicKey) *HandshakeConfig {
	hc.TokenPublicKey = publicKey
	return hc
}

// ValidatePSK validates PSK proof in a message
func (hc *HandshakeConfig) ValidatePSK(message []byte, pskHint *string, pskProof []byte) error {
	if hc.PSKConfig == nil {
		// No PSK configured
		if pskHint != nil || len(pskProof) > 0 {
			return fmt.Errorf("PSK provided but not configured")
		}
		return nil
	}

	// PSK is configured, so it's required
	if pskHint == nil || len(pskProof) == 0 {
		return fmt.Errorf("PSK required but not provided")
	}

	// Check hint matches
	if *pskHint != hc.PSKConfig.Hint {
		return fmt.Errorf("PSK hint mismatch")
	}

	// Verify proof
	if !hc.PSKConfig.VerifyProof(message, pskProof) {
		return fmt.Errorf("PSK proof verification failed")
	}

	return nil
}

// ValidateAdmissionToken validates admission token and proof
func (hc *HandshakeConfig) ValidateAdmissionToken(swarmID string, token *string, tokenProof []byte) error {
	if hc.AdmissionConfig == nil || !hc.AdmissionConfig.RequireToken {
		// No admission control required
		return nil
	}

	// Admission control is required
	if token == nil || len(tokenProof) == 0 {
		return fmt.Errorf("admission token required but not provided")
	}

	// Validate token
	if !hc.AdmissionConfig.ValidateToken(*token, swarmID, tokenProof, hc.TokenPublicKey) {
		return fmt.Errorf("admission token validation failed")
	}

	return nil
}

// GeneratePSKProof generates PSK proof for a message
func (hc *HandshakeConfig) GeneratePSKProof(message []byte) (string, []byte) {
	if hc.PSKConfig == nil {
		return "", nil
	}

	return hc.PSKConfig.Hint, hc.PSKConfig.GenerateProof(message)
}

// GenerateAdmissionTokenProof generates token proof for admission
func (hc *HandshakeConfig) GenerateAdmissionTokenProof(swarmID string) (string, []byte, uint64) {
	if hc.AdmissionConfig == nil || hc.ClientToken == "" {
		return "", nil, 0
	}

	// Find token info
	tokenInfo, exists := hc.AdmissionConfig.ValidTokens[hc.ClientToken]
	if !exists {
		return "", nil, 0
	}

	proof := hc.AdmissionConfig.GenerateTokenProof(hc.ClientToken, swarmID, hc.TokenSigningKey)
	return hc.ClientToken, proof, tokenInfo.Expiry
}
