package noiseik

import (
	"crypto/rand"
	"testing"

	"github.com/WebFirstLanguage/beenet/pkg/identity"
)

func TestClientHello_MarshalUnmarshal(t *testing.T) {
	// Generate test identity
	testIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate test identity: %v", err)
	}

	// Create ClientHello
	hello := &ClientHello{
		Version:  1,
		SwarmID:  "test-swarm-id",
		From:     testIdentity.BID(),
		Nonce:    12345,
		Caps:     []string{"pubsub/1", "dht/1", "chunks/1", "honeytag/1"},
		NoiseKey: make([]byte, 32), // X25519 public key
	}

	// Fill noise key with random data
	if _, err := rand.Read(hello.NoiseKey); err != nil {
		t.Fatalf("Failed to generate noise key: %v", err)
	}

	// Sign the ClientHello
	if err := hello.Sign(testIdentity.SigningPrivateKey); err != nil {
		t.Fatalf("Failed to sign ClientHello: %v", err)
	}

	// Marshal to CBOR
	data, err := hello.Marshal()
	if err != nil {
		t.Fatalf("Failed to marshal ClientHello: %v", err)
	}

	// Unmarshal from CBOR
	var decoded ClientHello
	if err := decoded.Unmarshal(data); err != nil {
		t.Fatalf("Failed to unmarshal ClientHello: %v", err)
	}

	// Verify fields
	if decoded.Version != hello.Version {
		t.Errorf("Expected version %d, got %d", hello.Version, decoded.Version)
	}
	if decoded.SwarmID != hello.SwarmID {
		t.Errorf("Expected swarm ID %s, got %s", hello.SwarmID, decoded.SwarmID)
	}
	if decoded.From != hello.From {
		t.Errorf("Expected from %s, got %s", hello.From, decoded.From)
	}
	if decoded.Nonce != hello.Nonce {
		t.Errorf("Expected nonce %d, got %d", hello.Nonce, decoded.Nonce)
	}
	if len(decoded.Caps) != len(hello.Caps) {
		t.Errorf("Expected %d capabilities, got %d", len(hello.Caps), len(decoded.Caps))
	}
	if len(decoded.NoiseKey) != len(hello.NoiseKey) {
		t.Errorf("Expected noise key length %d, got %d", len(hello.NoiseKey), len(decoded.NoiseKey))
	}

	// Verify signature
	if err := decoded.Verify(testIdentity.SigningPublicKey); err != nil {
		t.Errorf("Failed to verify ClientHello signature: %v", err)
	}
}

func TestServerHello_MarshalUnmarshal(t *testing.T) {
	// Generate test identity
	testIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate test identity: %v", err)
	}

	// Create ServerHello
	hello := &ServerHello{
		Version:  1,
		SwarmID:  "test-swarm-id",
		From:     testIdentity.BID(),
		Nonce:    67890,
		Caps:     []string{"pubsub/1", "dht/1", "chunks/1", "honeytag/1"},
		NoiseKey: make([]byte, 32), // X25519 public key
	}

	// Fill noise key with random data
	if _, err := rand.Read(hello.NoiseKey); err != nil {
		t.Fatalf("Failed to generate noise key: %v", err)
	}

	// Sign the ServerHello
	if err := hello.Sign(testIdentity.SigningPrivateKey); err != nil {
		t.Fatalf("Failed to sign ServerHello: %v", err)
	}

	// Marshal to CBOR
	data, err := hello.Marshal()
	if err != nil {
		t.Fatalf("Failed to marshal ServerHello: %v", err)
	}

	// Unmarshal from CBOR
	var decoded ServerHello
	if err := decoded.Unmarshal(data); err != nil {
		t.Fatalf("Failed to unmarshal ServerHello: %v", err)
	}

	// Verify fields
	if decoded.Version != hello.Version {
		t.Errorf("Expected version %d, got %d", hello.Version, decoded.Version)
	}
	if decoded.SwarmID != hello.SwarmID {
		t.Errorf("Expected swarm ID %s, got %s", hello.SwarmID, decoded.SwarmID)
	}
	if decoded.From != hello.From {
		t.Errorf("Expected from %s, got %s", hello.From, decoded.From)
	}
	if decoded.Nonce != hello.Nonce {
		t.Errorf("Expected nonce %d, got %d", hello.Nonce, decoded.Nonce)
	}

	// Verify signature
	if err := decoded.Verify(testIdentity.SigningPublicKey); err != nil {
		t.Errorf("Failed to verify ServerHello signature: %v", err)
	}
}

func TestHandshakeFlow(t *testing.T) {
	// Generate client and server identities
	clientIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate client identity: %v", err)
	}

	serverIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate server identity: %v", err)
	}

	swarmID := "test-swarm-id"

	// Create handshake
	handshake := NewHandshake(clientIdentity, swarmID)

	// Generate ClientHello
	clientHello, err := handshake.CreateClientHello()
	if err != nil {
		t.Fatalf("Failed to create ClientHello: %v", err)
	}

	// Verify ClientHello
	if err := clientHello.Verify(clientIdentity.SigningPublicKey); err != nil {
		t.Errorf("Failed to verify ClientHello: %v", err)
	}

	// Server processes ClientHello and creates ServerHello
	serverHandshake := NewHandshake(serverIdentity, swarmID)
	serverHello, err := serverHandshake.ProcessClientHello(clientHello)
	if err != nil {
		t.Fatalf("Failed to process ClientHello: %v", err)
	}

	// Verify ServerHello
	if err := serverHello.Verify(serverIdentity.SigningPublicKey); err != nil {
		t.Errorf("Failed to verify ServerHello: %v", err)
	}

	// Client processes ServerHello
	if err := handshake.ProcessServerHello(serverHello); err != nil {
		t.Fatalf("Failed to process ServerHello: %v", err)
	}

	// Both sides should now have established session keys
	if !handshake.IsComplete() {
		t.Error("Expected handshake to be complete")
	}
	if !serverHandshake.IsComplete() {
		t.Error("Expected server handshake to be complete")
	}
}

func TestInvalidSignature(t *testing.T) {
	// Generate test identities
	testIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate test identity: %v", err)
	}

	wrongIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate wrong identity: %v", err)
	}

	// Create ClientHello
	hello := &ClientHello{
		Version:  1,
		SwarmID:  "test-swarm-id",
		From:     testIdentity.BID(),
		Nonce:    12345,
		Caps:     []string{"pubsub/1"},
		NoiseKey: make([]byte, 32),
	}

	// Sign with correct key
	if err := hello.Sign(testIdentity.SigningPrivateKey); err != nil {
		t.Fatalf("Failed to sign ClientHello: %v", err)
	}

	// Try to verify with wrong key - should fail
	if err := hello.Verify(wrongIdentity.SigningPublicKey); err == nil {
		t.Error("Expected verification to fail with wrong public key")
	}
}

func TestReplayProtection(t *testing.T) {
	// Generate test identity
	testIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate test identity: %v", err)
	}

	swarmID := "test-swarm-id"

	// Create two separate handshake instances
	handshake1 := NewHandshake(testIdentity, swarmID)
	handshake2 := NewHandshake(testIdentity, swarmID)

	// Create ClientHellos from different handshake instances
	hello1, err := handshake1.CreateClientHello()
	if err != nil {
		t.Fatalf("Failed to create first ClientHello: %v", err)
	}

	hello2, err := handshake2.CreateClientHello()
	if err != nil {
		t.Fatalf("Failed to create second ClientHello: %v", err)
	}

	// The nonces should be different (replay protection)
	if hello1.Nonce == hello2.Nonce {
		t.Error("Expected different nonces for replay protection")
	}
}

func TestNoiseIKHandshake(t *testing.T) {
	// Generate client and server identities
	clientIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate client identity: %v", err)
	}

	serverIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate server identity: %v", err)
	}

	swarmID := "test-swarm-id"

	// Create client handshake (initiator)
	clientHandshake, err := NewClientHandshake(clientIdentity, swarmID, serverIdentity.KeyAgreementPublicKey[:])
	if err != nil {
		t.Fatalf("Failed to create client handshake: %v", err)
	}

	// Create server handshake (responder)
	serverHandshake, err := NewServerHandshake(serverIdentity, swarmID)
	if err != nil {
		t.Fatalf("Failed to create server handshake: %v", err)
	}

	// Client sends first message (-> e, es, s, ss)
	clientMsg1, err := clientHandshake.PerformHandshake(nil)
	if err != nil {
		t.Fatalf("Client handshake step 1 failed: %v", err)
	}

	// Server processes client message and responds (<- e, ee, se)
	_, err = serverHandshake.ReadHandshakeMessage(clientMsg1)
	if err != nil {
		t.Fatalf("Server failed to read client message: %v", err)
	}

	serverMsg1, err := serverHandshake.PerformHandshake(nil)
	if err != nil {
		t.Fatalf("Server handshake step 1 failed: %v", err)
	}

	// Client processes server response
	_, err = clientHandshake.ReadHandshakeMessage(serverMsg1)
	if err != nil {
		t.Fatalf("Client failed to read server message: %v", err)
	}

	// Both sides should now have completed handshakes
	if !clientHandshake.IsComplete() {
		t.Error("Expected client handshake to be complete")
	}
	if !serverHandshake.IsComplete() {
		t.Error("Expected server handshake to be complete")
	}

	// Both sides should be able to derive session keys
	clientSendKey, clientRecvKey, err := clientHandshake.GetSessionKeys()
	if err != nil {
		t.Fatalf("Failed to get client session keys: %v", err)
	}

	serverSendKey, serverRecvKey, err := serverHandshake.GetSessionKeys()
	if err != nil {
		t.Fatalf("Failed to get server session keys: %v", err)
	}

	// Verify keys are not empty
	if len(clientSendKey) == 0 || len(clientRecvKey) == 0 {
		t.Error("Client session keys should not be empty")
	}
	if len(serverSendKey) == 0 || len(serverRecvKey) == 0 {
		t.Error("Server session keys should not be empty")
	}
}

func TestHandshakeWithSequenceTracking(t *testing.T) {
	// Generate client and server identities
	clientIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate client identity: %v", err)
	}

	serverIdentity, err := identity.GenerateIdentity()
	if err != nil {
		t.Fatalf("Failed to generate server identity: %v", err)
	}

	swarmID := "test-swarm-id"

	// Create handshakes
	clientHandshake := NewHandshake(clientIdentity, swarmID)
	serverHandshake := NewHandshake(serverIdentity, swarmID)

	// Test sequence number generation
	seq1 := clientHandshake.NextSendSequence()
	seq2 := clientHandshake.NextSendSequence()
	seq3 := clientHandshake.NextSendSequence()

	if seq1 != 1 || seq2 != 2 || seq3 != 3 {
		t.Errorf("Expected sequences 1,2,3, got %d,%d,%d", seq1, seq2, seq3)
	}

	// Test sequence validation
	if !serverHandshake.ValidateReceiveSequence(1) {
		t.Error("Should accept sequence 1")
	}

	if !serverHandshake.ValidateReceiveSequence(3) {
		t.Error("Should accept sequence 3")
	}

	if !serverHandshake.ValidateReceiveSequence(2) {
		t.Error("Should accept sequence 2 (out of order)")
	}

	// Test replay protection
	if serverHandshake.ValidateReceiveSequence(2) {
		t.Error("Should reject replayed sequence 2")
	}

	if serverHandshake.ValidateReceiveSequence(1) {
		t.Error("Should reject replayed sequence 1")
	}

	// Test sequence stats
	sendSeq, _ := clientHandshake.GetSequenceStats()
	if sendSeq != 3 {
		t.Errorf("Expected client send sequence 3, got %d", sendSeq)
	}

	_, serverLastRecv := serverHandshake.GetSequenceStats()
	if serverLastRecv != 3 {
		t.Errorf("Expected server last received sequence 3, got %d", serverLastRecv)
	}
}
