package noiseik

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/identity"
)

// TestProtocolVersionMismatch tests handling of protocol version mismatches
func TestProtocolVersionMismatch(t *testing.T) {
	// Generate identities
	clientIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate client identity: %v", err)
	}

	serverIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate server identity: %v", err)
	}

	swarmID := "version-test-swarm"

	// Register the client's public key so the server can verify signatures
	RegisterTestKey(clientIdentity.BID(), clientIdentity.SigningPublicKey)

	// Create handshakes
	clientHandshake := NewHandshake(clientIdentity, swarmID)
	serverHandshake := NewHandshake(serverIdentity, swarmID)

	// Create ClientHello with correct version
	clientHello, err := clientHandshake.CreateClientHello()
	if err != nil {
		t.Fatalf("Failed to create ClientHello: %v", err)
	}

	// Modify version to simulate version mismatch
	originalVersion := clientHello.Version
	clientHello.Version = 999 // Invalid version

	// Re-sign with invalid version
	if err := clientHello.Sign(clientIdentity.SigningPrivateKey); err != nil {
		t.Fatalf("Failed to re-sign ClientHello: %v", err)
	}

	// Server should reject due to version mismatch
	_, err = serverHandshake.ProcessClientHello(clientHello)
	if err == nil {
		t.Error("Server should reject ClientHello with invalid version")
	}

	// Restore original version for comparison
	clientHello.Version = originalVersion
	if err := clientHello.Sign(clientIdentity.SigningPrivateKey); err != nil {
		t.Fatalf("Failed to restore ClientHello signature: %v", err)
	}

	// Should work with correct version
	_, err = serverHandshake.ProcessClientHello(clientHello)
	if err != nil {
		t.Errorf("Server should accept ClientHello with correct version: %v", err)
	}
}

// TestInvalidEd25519Signatures tests handling of invalid Ed25519 signatures
func TestInvalidEd25519Signatures(t *testing.T) {
	// Generate identities
	clientIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate client identity: %v", err)
	}

	serverIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate server identity: %v", err)
	}

	swarmID := "signature-test-swarm"

	// Register the client's public key so the server can verify signatures
	RegisterTestKey(clientIdentity.BID(), clientIdentity.SigningPublicKey)

	// Create handshakes
	clientHandshake := NewHandshake(clientIdentity, swarmID)
	serverHandshake := NewHandshake(serverIdentity, swarmID)

	// Create ClientHello
	clientHello, err := clientHandshake.CreateClientHello()
	if err != nil {
		t.Fatalf("Failed to create ClientHello: %v", err)
	}

	// Test 1: Corrupt signature
	originalProof := make([]byte, len(clientHello.Proof))
	copy(originalProof, clientHello.Proof)

	// Corrupt the signature
	clientHello.Proof[0] ^= 0xFF

	// Server should reject due to invalid signature
	_, err = serverHandshake.ProcessClientHello(clientHello)
	if err == nil {
		t.Error("Server should reject ClientHello with corrupted signature")
	}

	// Test 2: Wrong signature length
	clientHello.Proof = []byte("invalid-signature")

	// Server should reject due to invalid signature length
	_, err = serverHandshake.ProcessClientHello(clientHello)
	if err == nil {
		t.Error("Server should reject ClientHello with invalid signature length")
	}

	// Test 3: Empty signature
	clientHello.Proof = []byte{}

	// Server should reject due to empty signature
	_, err = serverHandshake.ProcessClientHello(clientHello)
	if err == nil {
		t.Error("Server should reject ClientHello with empty signature")
	}

	// Create a fresh server handshake and ClientHello for the positive test to avoid replay protection
	freshServerHandshake := NewHandshake(serverIdentity, swarmID)

	freshClientHello, err := clientHandshake.CreateClientHello()
	if err != nil {
		t.Fatalf("Failed to create fresh ClientHello: %v", err)
	}

	// Should work with correct signature
	_, err = freshServerHandshake.ProcessClientHello(freshClientHello)
	if err != nil {
		t.Errorf("Server should accept ClientHello with correct signature: %v", err)
	}
}

// TestReplayAttackPrevention tests replay attack prevention
func TestReplayAttackPrevention(t *testing.T) {
	// Generate identities
	clientIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate client identity: %v", err)
	}

	serverIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate server identity: %v", err)
	}

	swarmID := "replay-test-swarm"

	// Register the client's public key so the server can verify signatures
	RegisterTestKey(clientIdentity.BID(), clientIdentity.SigningPublicKey)

	// Create handshakes (sequence tracking is built-in)
	clientHandshake := NewHandshake(clientIdentity, swarmID)
	serverHandshake := NewHandshake(serverIdentity, swarmID)

	// Create ClientHello
	clientHello, err := clientHandshake.CreateClientHello()
	if err != nil {
		t.Fatalf("Failed to create ClientHello: %v", err)
	}

	// First handshake should succeed
	serverHello, err := serverHandshake.ProcessClientHello(clientHello)
	if err != nil {
		t.Fatalf("First handshake should succeed: %v", err)
	}

	// Complete first handshake
	err = clientHandshake.ProcessServerHello(serverHello)
	if err != nil {
		t.Fatalf("Failed to complete first handshake: %v", err)
	}

	// Create new server handshake for replay test
	serverHandshake2 := NewHandshake(serverIdentity, swarmID)

	// Copy sequence tracker state to simulate same server instance
	if serverHandshake.sequenceTracker != nil {
		serverHandshake2.sequenceTracker = serverHandshake.sequenceTracker
	}

	// Replay the same ClientHello - should be rejected
	_, err = serverHandshake2.ProcessClientHello(clientHello)
	if err == nil {
		t.Error("Server should reject replayed ClientHello")
	}

	t.Logf("✓ Replay attack correctly prevented")
}

// TestMalformedMessages tests handling of malformed protocol messages
func TestMalformedMessages(t *testing.T) {
	// Generate identities
	serverIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate server identity: %v", err)
	}

	swarmID := "malformed-test-swarm"
	serverHandshake := NewHandshake(serverIdentity, swarmID)

	// Test 1: ClientHello with missing required fields
	malformedHello := &ClientHello{
		Version: 1,
		// Missing SwarmID, From, Nonce, etc.
	}

	_, err = serverHandshake.ProcessClientHello(malformedHello)
	if err == nil {
		t.Error("Server should reject ClientHello with missing required fields")
	}

	// Test 2: ClientHello with invalid BID format
	malformedHello2 := &ClientHello{
		Version:  1,
		SwarmID:  swarmID,
		From:     "invalid-bid-format",
		Nonce:    12345,
		Caps:     []string{"test"},
		NoiseKey: make([]byte, 32),
	}

	if err := malformedHello2.Sign(serverIdentity.SigningPrivateKey); err != nil {
		t.Fatalf("Failed to sign malformed hello: %v", err)
	}

	_, err = serverHandshake.ProcessClientHello(malformedHello2)
	if err == nil {
		t.Error("Server should reject ClientHello with invalid BID format")
	}

	// Test 3: ClientHello with invalid NoiseKey length
	malformedHello3 := &ClientHello{
		Version:  1,
		SwarmID:  swarmID,
		From:     "bee:key:z6MkTest",
		Nonce:    12345,
		Caps:     []string{"test"},
		NoiseKey: make([]byte, 16), // Wrong length
	}

	if err := malformedHello3.Sign(serverIdentity.SigningPrivateKey); err != nil {
		t.Fatalf("Failed to sign malformed hello: %v", err)
	}

	_, err = serverHandshake.ProcessClientHello(malformedHello3)
	if err == nil {
		t.Error("Server should reject ClientHello with invalid NoiseKey length")
	}
}

// TestPSKValidationErrors tests PSK validation error conditions
func TestPSKValidationErrors(t *testing.T) {
	// Generate identities
	clientIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate client identity: %v", err)
	}

	serverIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate server identity: %v", err)
	}

	swarmID := "psk-error-test-swarm"

	// Create PSK configs
	clientPSK := make([]byte, 32)
	rand.Read(clientPSK)
	clientPSKConfig := NewPSKConfig(clientPSK, "client-psk")

	serverPSK := make([]byte, 32)
	rand.Read(serverPSK)
	serverPSKConfig := NewPSKConfig(serverPSK, "server-psk") // Different PSK

	// Create handshakes with different PSKs
	clientHandshake := NewHandshakeWithPSK(clientIdentity, swarmID, clientPSKConfig)
	serverHandshake := NewHandshakeWithPSK(serverIdentity, swarmID, serverPSKConfig)

	// Create ClientHello with client PSK
	clientHello, err := clientHandshake.CreateClientHello()
	if err != nil {
		t.Fatalf("Failed to create ClientHello: %v", err)
	}

	// Server with different PSK should reject
	_, err = serverHandshake.ProcessClientHello(clientHello)
	if err == nil {
		t.Error("Server should reject ClientHello with mismatched PSK")
	}

	t.Logf("✓ PSK mismatch correctly detected and rejected")
}

// TestTokenValidationErrors tests token validation error conditions
func TestTokenValidationErrors(t *testing.T) {
	// Generate identities
	clientIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate client identity: %v", err)
	}

	serverIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate server identity: %v", err)
	}

	swarmID := "token-error-test-swarm"

	// Create admission control
	admissionConfig := NewAdmissionConfig()
	admissionConfig.RequireToken = true

	// Generate token signing keys
	tokenPublicKey, tokenSigningKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate token signing key: %v", err)
	}

	// Add expired token
	expiredToken := "expired-token"
	expiredTime := uint64(time.Now().Add(-time.Hour).Unix()) // Expired 1 hour ago
	err = admissionConfig.AddToken(expiredToken, expiredTime, tokenSigningKey)
	if err != nil {
		t.Fatalf("Failed to add expired token: %v", err)
	}

	// Create handshakes
	clientHandshake := NewHandshakeWithAdmission(clientIdentity, swarmID, admissionConfig, expiredToken, tokenSigningKey)
	serverHandshake := NewHandshakeWithAdmission(serverIdentity, swarmID, admissionConfig, "", nil)
	serverHandshake.SetTokenValidator(tokenPublicKey)

	// Create ClientHello with expired token
	clientHello, err := clientHandshake.CreateClientHello()
	if err != nil {
		t.Fatalf("Failed to create ClientHello: %v", err)
	}

	// Server should reject expired token
	_, err = serverHandshake.ProcessClientHello(clientHello)
	if err == nil {
		t.Error("Server should reject ClientHello with expired token")
	}

	t.Logf("✓ Expired token correctly rejected")
}
