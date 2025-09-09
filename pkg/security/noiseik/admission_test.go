package noiseik

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/codec/cborcanon"
	"github.com/WebFirstLanguage/beenet/pkg/identity"
)

func TestPSKConfig_NewPSKConfig(t *testing.T) {
	psk := make([]byte, 32)
	rand.Read(psk)

	config := NewPSKConfig(psk, "test-hint")

	if len(config.PSK) != 32 {
		t.Errorf("Expected PSK length 32, got %d", len(config.PSK))
	}

	if config.Hint != "test-hint" {
		t.Errorf("Expected hint 'test-hint', got '%s'", config.Hint)
	}
}

func TestPSKConfig_GenerateProof(t *testing.T) {
	psk := make([]byte, 32)
	rand.Read(psk)

	config := NewPSKConfig(psk, "test-hint")
	message := []byte("test message for PSK proof")

	proof := config.GenerateProof(message)

	if len(proof) == 0 {
		t.Error("PSK proof should not be empty")
	}

	// Verify the proof
	if !config.VerifyProof(message, proof) {
		t.Error("PSK proof verification should succeed")
	}

	// Verify with wrong message should fail
	wrongMessage := []byte("wrong message")
	if config.VerifyProof(wrongMessage, proof) {
		t.Error("PSK proof verification with wrong message should fail")
	}
}

func TestAdmissionConfig_NewAdmissionConfig(t *testing.T) {
	config := NewAdmissionConfig()

	if config.RequireToken {
		t.Error("Should not require token by default")
	}

	if config.ValidTokens == nil {
		t.Error("ValidTokens map should be initialized")
	}
}

func TestAdmissionConfig_AddToken(t *testing.T) {
	config := NewAdmissionConfig()
	config.RequireToken = true

	// Generate a signing key for tokens
	_, signingKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate signing key: %v", err)
	}

	token := "test-token-123"
	expiry := uint64(time.Now().Add(time.Hour).Unix())

	err = config.AddToken(token, expiry, signingKey)
	if err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	// Verify token exists
	tokenInfo, exists := config.ValidTokens[token]
	if !exists {
		t.Error("Token should exist in ValidTokens")
	}

	if tokenInfo.Expiry != expiry {
		t.Errorf("Expected expiry %d, got %d", expiry, tokenInfo.Expiry)
	}
}

func TestAdmissionConfig_ValidateToken(t *testing.T) {
	config := NewAdmissionConfig()
	config.RequireToken = true

	// Generate a signing key for tokens
	publicKey, signingKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate signing key: %v", err)
	}

	token := "test-token-456"
	expiry := uint64(time.Now().Add(time.Hour).Unix())
	swarmID := "test-swarm"

	// Add token
	err = config.AddToken(token, expiry, signingKey)
	if err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	// Generate proof
	proof := config.GenerateTokenProof(token, swarmID, signingKey)

	// Validate token
	if !config.ValidateToken(token, swarmID, proof, publicKey) {
		t.Error("Token validation should succeed")
	}

	// Test with wrong swarm ID
	if config.ValidateToken(token, "wrong-swarm", proof, publicKey) {
		t.Error("Token validation with wrong swarm should fail")
	}

	// Test with wrong proof
	wrongProof := make([]byte, len(proof))
	copy(wrongProof, proof)
	wrongProof[0] ^= 0xFF // Corrupt the proof

	if config.ValidateToken(token, swarmID, wrongProof, publicKey) {
		t.Error("Token validation with wrong proof should fail")
	}
}

func TestAdmissionConfig_ExpiredToken(t *testing.T) {
	config := NewAdmissionConfig()
	config.RequireToken = true

	// Generate a signing key for tokens
	publicKey, signingKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate signing key: %v", err)
	}

	token := "expired-token"
	expiry := uint64(time.Now().Add(-time.Hour).Unix()) // Expired 1 hour ago
	swarmID := "test-swarm"

	// Add expired token
	err = config.AddToken(token, expiry, signingKey)
	if err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	// Generate proof
	proof := config.GenerateTokenProof(token, swarmID, signingKey)

	// Validate expired token - should fail
	if config.ValidateToken(token, swarmID, proof, publicKey) {
		t.Error("Expired token validation should fail")
	}
}

func TestPSKProofDebug(t *testing.T) {
	// Create a simple test message
	psk := make([]byte, 32)
	rand.Read(psk)
	pskConfig := NewPSKConfig(psk, "test-psk")

	message := []byte("test message")

	// Generate proof
	proof := pskConfig.GenerateProof(message)

	// Verify proof
	if !pskConfig.VerifyProof(message, proof) {
		t.Error("PSK proof verification should succeed")
	}

	// Test with different message
	wrongMessage := []byte("wrong message")
	if pskConfig.VerifyProof(wrongMessage, proof) {
		t.Error("PSK proof verification with wrong message should fail")
	}
}

func TestCBOREncodingConsistency(t *testing.T) {
	// Create a ClientHello
	hello := &ClientHello{
		Version:  1,
		SwarmID:  "test-swarm",
		From:     "test-from",
		Nonce:    12345,
		Caps:     []string{"test"},
		NoiseKey: make([]byte, 32),
	}

	// Encode twice and verify they're the same
	data1, err := cborcanon.EncodeForSigning(hello, "proof", "psk_proof")
	if err != nil {
		t.Fatalf("First encoding failed: %v", err)
	}

	data2, err := cborcanon.EncodeForSigning(hello, "proof", "psk_proof")
	if err != nil {
		t.Fatalf("Second encoding failed: %v", err)
	}

	if string(data1) != string(data2) {
		t.Error("CBOR encoding should be deterministic")
		t.Logf("First:  %x", data1)
		t.Logf("Second: %x", data2)
	}
}

func TestHandshakeWithPSK(t *testing.T) {
	// Generate client and server identities
	clientIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate client identity: %v", err)
	}

	serverIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate server identity: %v", err)
	}

	swarmID := "test-swarm-psk"

	// Create PSK configuration
	psk := make([]byte, 32)
	rand.Read(psk)
	pskConfig := NewPSKConfig(psk, "test-psk")

	// Create handshakes with PSK (use same config instance)
	clientHandshake := NewHandshakeWithPSK(clientIdentity, swarmID, pskConfig)
	serverHandshake := NewHandshakeWithPSK(serverIdentity, swarmID, pskConfig)

	// Create ClientHello with PSK
	clientHello, err := clientHandshake.CreateClientHello()
	if err != nil {
		t.Fatalf("Failed to create ClientHello with PSK: %v", err)
	}

	// Verify PSK fields are present
	if clientHello.PSKHint == nil || *clientHello.PSKHint != "test-psk" {
		t.Error("ClientHello should contain PSK hint")
	}

	if len(clientHello.PSKProof) == 0 {
		t.Error("ClientHello should contain PSK proof")
	}

	// Debug: manually verify PSK proof
	sigData, err := cborcanon.EncodeForSigning(clientHello, "proof", "psk_proof")
	if err != nil {
		t.Fatalf("Failed to encode for PSK verification: %v", err)
	}

	// Generate expected proof for comparison
	expectedProof := pskConfig.GenerateProof(sigData)

	if !pskConfig.VerifyProof(sigData, clientHello.PSKProof) {
		t.Errorf("Manual PSK proof verification failed")
		t.Logf("PSK: %x", pskConfig.PSK)
		t.Logf("Message: %x", sigData)
		t.Logf("Expected proof: %x", expectedProof)
		t.Logf("Actual proof:   %x", clientHello.PSKProof)
	}

	// Server processes ClientHello
	serverHello, err := serverHandshake.ProcessClientHello(clientHello)
	if err != nil {
		t.Fatalf("Server failed to process ClientHello with PSK: %v", err)
	}

	// Verify ServerHello has PSK proof
	if len(serverHello.PSKProof) == 0 {
		t.Error("ServerHello should contain PSK proof")
	}

	// Client processes ServerHello
	err = clientHandshake.ProcessServerHello(serverHello)
	if err != nil {
		t.Fatalf("Client failed to process ServerHello with PSK: %v", err)
	}

	// Both handshakes should be complete
	if !clientHandshake.IsComplete() {
		t.Error("Client handshake should be complete")
	}

	if !serverHandshake.IsComplete() {
		t.Error("Server handshake should be complete")
	}
}

func TestHandshakeWithInvalidPSK(t *testing.T) {
	// Generate identities
	clientIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate client identity: %v", err)
	}

	serverIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate server identity: %v", err)
	}

	swarmID := "test-swarm-invalid-psk"

	// Create different PSK configurations for client and server
	clientPSK := make([]byte, 32)
	serverPSK := make([]byte, 32)
	rand.Read(clientPSK)
	rand.Read(serverPSK)

	clientPSKConfig := NewPSKConfig(clientPSK, "client-psk")
	serverPSKConfig := NewPSKConfig(serverPSK, "server-psk")

	// Create handshakes with different PSKs
	clientHandshake := NewHandshakeWithPSK(clientIdentity, swarmID, clientPSKConfig)
	serverHandshake := NewHandshakeWithPSK(serverIdentity, swarmID, serverPSKConfig)

	// Create ClientHello
	clientHello, err := clientHandshake.CreateClientHello()
	if err != nil {
		t.Fatalf("Failed to create ClientHello: %v", err)
	}

	// Server should reject ClientHello due to invalid PSK
	_, err = serverHandshake.ProcessClientHello(clientHello)
	if err == nil {
		t.Error("Server should reject ClientHello with invalid PSK")
	}
}

func TestHandshakeWithAdmissionToken(t *testing.T) {
	// Generate identities
	clientIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate client identity: %v", err)
	}

	serverIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate server identity: %v", err)
	}

	swarmID := "test-swarm-token"

	// Create admission configuration
	admissionConfig := NewAdmissionConfig()
	admissionConfig.RequireToken = true

	// Generate token signing key
	tokenPublicKey, tokenSigningKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate token signing key: %v", err)
	}

	// Add a valid token
	token := "valid-admission-token"
	expiry := uint64(time.Now().Add(time.Hour).Unix())
	err = admissionConfig.AddToken(token, expiry, tokenSigningKey)
	if err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	// Create handshakes with admission control
	clientHandshake := NewHandshakeWithAdmission(clientIdentity, swarmID, admissionConfig, token, tokenSigningKey)
	serverHandshake := NewHandshakeWithAdmission(serverIdentity, swarmID, admissionConfig, "", nil)
	serverHandshake.SetTokenValidator(tokenPublicKey)

	// Create ClientHello with token
	clientHello, err := clientHandshake.CreateClientHello()
	if err != nil {
		t.Fatalf("Failed to create ClientHello with token: %v", err)
	}

	// Verify token fields are present
	if clientHello.AdmissionToken == nil || *clientHello.AdmissionToken != token {
		t.Error("ClientHello should contain admission token")
	}

	if len(clientHello.TokenProof) == 0 {
		t.Error("ClientHello should contain token proof")
	}

	// Server processes ClientHello
	serverHello, err := serverHandshake.ProcessClientHello(clientHello)
	if err != nil {
		t.Fatalf("Server failed to process ClientHello with token: %v", err)
	}

	// Complete handshake
	err = clientHandshake.ProcessServerHello(serverHello)
	if err != nil {
		t.Fatalf("Client failed to process ServerHello: %v", err)
	}

	// Both handshakes should be complete
	if !clientHandshake.IsComplete() {
		t.Error("Client handshake should be complete")
	}

	if !serverHandshake.IsComplete() {
		t.Error("Server handshake should be complete")
	}
}

func TestErrorConditions(t *testing.T) {
	// Test PSK with empty key
	emptyPSK := make([]byte, 0)
	pskConfig := NewPSKConfig(emptyPSK, "empty")
	if len(pskConfig.PSK) != 32 {
		t.Error("PSK should be padded to 32 bytes")
	}

	// Test admission config with empty token
	admissionConfig := NewAdmissionConfig()
	err := admissionConfig.AddToken("", 12345, nil)
	if err == nil {
		t.Error("Should reject empty token")
	}

	// Test token validation with non-existent token
	publicKey := make([]byte, 32)
	if admissionConfig.ValidateToken("nonexistent", "swarm", []byte("proof"), publicKey) {
		t.Error("Should reject non-existent token")
	}
}

func TestBackwardCompatibility(t *testing.T) {
	// Generate identities
	clientIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate client identity: %v", err)
	}

	serverIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate server identity: %v", err)
	}

	swarmID := "test-swarm-compat"

	// Create handshakes without PSK or admission control (backward compatibility)
	clientHandshake := NewHandshake(clientIdentity, swarmID)
	serverHandshake := NewHandshake(serverIdentity, swarmID)

	// Create ClientHello
	clientHello, err := clientHandshake.CreateClientHello()
	if err != nil {
		t.Fatalf("Failed to create ClientHello: %v", err)
	}

	// Verify no PSK or token fields
	if clientHello.PSKHint != nil {
		t.Error("ClientHello should not have PSK hint in backward compatibility mode")
	}
	if len(clientHello.PSKProof) > 0 {
		t.Error("ClientHello should not have PSK proof in backward compatibility mode")
	}
	if clientHello.AdmissionToken != nil {
		t.Error("ClientHello should not have admission token in backward compatibility mode")
	}

	// Server should accept it
	serverHello, err := serverHandshake.ProcessClientHello(clientHello)
	if err != nil {
		t.Fatalf("Server should accept ClientHello without PSK/tokens: %v", err)
	}

	// Complete handshake
	err = clientHandshake.ProcessServerHello(serverHello)
	if err != nil {
		t.Fatalf("Client should accept ServerHello: %v", err)
	}

	// Both should be complete
	if !clientHandshake.IsComplete() || !serverHandshake.IsComplete() {
		t.Error("Handshakes should complete without PSK/tokens")
	}
}
