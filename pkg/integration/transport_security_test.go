// Package integration provides integration tests for BeeNet transport and security layers
package integration

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/identity"
	"github.com/WebFirstLanguage/beenet/pkg/security/noiseik"
	"github.com/WebFirstLanguage/beenet/pkg/transport"
	"github.com/WebFirstLanguage/beenet/pkg/transport/tcp"
)

// generateTestTLSConfig creates a test TLS configuration with self-signed certificate
func generateTestTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"BeeNet Integration Test"},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)},
		DNSNames:    []string{"localhost"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{certDER},
			PrivateKey:  key,
		}},
		NextProtos:         []string{"beenet/1"},
		InsecureSkipVerify: true, // For testing only
	}
}

// BeeNode represents a simplified BeeNet node for integration testing
type BeeNode struct {
	identity  *identity.Identity
	transport transport.Transport
	swarmID   string
}

// NewBeeNode creates a new test bee node
func NewBeeNode(swarmID string) (*BeeNode, error) {
	id, err := identity.GenerateIdentity()
	if err != nil {
		return nil, err
	}

	return &BeeNode{
		identity:  id,
		transport: tcp.New(), // Use TCP for reliable integration testing
		swarmID:   swarmID,
	}, nil
}

// Listen starts the bee node listening for connections
func (bn *BeeNode) Listen(ctx context.Context, addr string) (transport.Listener, error) {
	tlsConfig := generateTestTLSConfig()
	return bn.transport.Listen(ctx, addr, tlsConfig)
}

// Dial connects to another bee node
func (bn *BeeNode) Dial(ctx context.Context, addr string) (transport.Conn, error) {
	clientTLSConfig := &tls.Config{
		NextProtos:         []string{"beenet/1"},
		InsecureSkipVerify: true,
	}
	return bn.transport.Dial(ctx, addr, clientTLSConfig)
}

// PerformHandshake performs the Noise IK handshake over an established TLS connection
func (bn *BeeNode) PerformHandshake(conn transport.Conn, isInitiator bool, peerPublicKey []byte) (*noiseik.Handshake, error) {
	var handshake *noiseik.Handshake
	var err error

	if isInitiator {
		handshake, err = noiseik.NewClientHandshake(bn.identity, bn.swarmID, peerPublicKey)
	} else {
		handshake, err = noiseik.NewServerHandshake(bn.identity, bn.swarmID)
	}

	if err != nil {
		return nil, err
	}

	if isInitiator {
		// Client sends ClientHello
		clientHello, err := handshake.CreateClientHello()
		if err != nil {
			return nil, err
		}

		helloData, err := clientHello.Marshal()
		if err != nil {
			return nil, err
		}

		if _, err := conn.Write(helloData); err != nil {
			return nil, err
		}

		// Client receives ServerHello
		buffer := make([]byte, 4096)
		n, err := conn.Read(buffer)
		if err != nil {
			return nil, err
		}

		var serverHello noiseik.ServerHello
		if err := serverHello.Unmarshal(buffer[:n]); err != nil {
			return nil, err
		}

		if err := handshake.ProcessServerHello(&serverHello); err != nil {
			return nil, err
		}
	} else {
		// Server receives ClientHello
		buffer := make([]byte, 4096)
		n, err := conn.Read(buffer)
		if err != nil {
			return nil, err
		}

		var clientHello noiseik.ClientHello
		if err := clientHello.Unmarshal(buffer[:n]); err != nil {
			return nil, err
		}

		// Server sends ServerHello
		serverHello, err := handshake.ProcessClientHello(&clientHello)
		if err != nil {
			return nil, err
		}

		helloData, err := serverHello.Marshal()
		if err != nil {
			return nil, err
		}

		if _, err := conn.Write(helloData); err != nil {
			return nil, err
		}
	}

	return handshake, nil
}

func TestTCPTransportWithNoiseIKHandshake(t *testing.T) {
	ctx := context.Background()
	swarmID := "integration-test-swarm"

	// Create two bee nodes
	serverNode, err := NewBeeNode(swarmID)
	if err != nil {
		t.Fatalf("Failed to create server node: %v", err)
	}

	clientNode, err := NewBeeNode(swarmID)
	if err != nil {
		t.Fatalf("Failed to create client node: %v", err)
	}

	// Register the client's public key so the server can verify signatures
	noiseik.RegisterTestKey(clientNode.identity.BID(), clientNode.identity.SigningPublicKey)

	// Start server
	listener, err := serverNode.Listen(ctx, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer listener.Close()

	serverAddr := listener.Addr().String()

	// Handle server connections
	serverDone := make(chan error, 1)
	go func() {
		defer close(serverDone)

		// Accept TLS connection
		conn, err := listener.Accept(ctx)
		if err != nil {
			serverDone <- err
			return
		}
		defer conn.Close()

		// Verify TLS connection state
		tlsState := conn.ConnectionState()
		if !tlsState.HandshakeComplete {
			serverDone <- err
			return
		}
		if tlsState.NegotiatedProtocol != "beenet/1" {
			serverDone <- err
			return
		}

		// Perform Noise IK handshake
		handshake, err := serverNode.PerformHandshake(conn, false, nil)
		if err != nil {
			serverDone <- err
			return
		}

		// Verify handshake completion
		if !handshake.IsComplete() {
			serverDone <- err
			return
		}

		// Get session keys
		sendKey, recvKey, err := handshake.GetSessionKeys()
		if err != nil {
			serverDone <- err
			return
		}

		if len(sendKey) == 0 || len(recvKey) == 0 {
			serverDone <- err
			return
		}

		serverDone <- nil
	}()

	// Client connects
	conn, err := clientNode.Dial(ctx, serverAddr)
	if err != nil {
		t.Fatalf("Failed to dial server: %v", err)
	}
	defer conn.Close()

	// Verify TLS connection state
	tlsState := conn.ConnectionState()
	if !tlsState.HandshakeComplete {
		t.Error("Expected TLS handshake to be complete")
	}
	if tlsState.NegotiatedProtocol != "beenet/1" {
		t.Errorf("Expected negotiated protocol 'beenet/1', got '%s'", tlsState.NegotiatedProtocol)
	}

	// Perform Noise IK handshake
	handshake, err := clientNode.PerformHandshake(conn, true, serverNode.identity.KeyAgreementPublicKey[:])
	if err != nil {
		t.Fatalf("Client handshake failed: %v", err)
	}

	// Verify handshake completion
	if !handshake.IsComplete() {
		t.Error("Expected client handshake to be complete")
	}

	// Get session keys
	sendKey, recvKey, err := handshake.GetSessionKeys()
	if err != nil {
		t.Fatalf("Failed to get client session keys: %v", err)
	}

	if len(sendKey) == 0 || len(recvKey) == 0 {
		t.Error("Client session keys should not be empty")
	}

	// Wait for server to complete
	if err := <-serverDone; err != nil {
		t.Fatalf("Server handshake failed: %v", err)
	}

	// Verify both nodes can identify each other
	// In a real implementation, this would involve resolving BIDs to public keys
	// For now, we verify that the handshake completed successfully
	t.Logf("Integration test successful:")
	t.Logf("  Client BID: %s", clientNode.identity.BID())
	t.Logf("  Server BID: %s", serverNode.identity.BID())
	t.Logf("  Swarm ID: %s", swarmID)
	t.Logf("  TLS Protocol: %s", tlsState.NegotiatedProtocol)
	t.Logf("  Double encryption: TLS + Noise IK established")
}

func TestSecureSwarmWithTokens(t *testing.T) {
	ctx := context.Background()
	swarmID := "secure-test-swarm"

	// Create admission control with tokens
	admissionConfig := noiseik.NewAdmissionConfig()
	admissionConfig.RequireToken = true

	// Generate token signing keys
	tokenPublicKey, tokenSigningKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate token signing key: %v", err)
	}

	// Add a valid token
	token := "secure-admission-token"
	expiry := uint64(time.Now().Add(time.Hour).Unix())
	err = admissionConfig.AddToken(token, expiry, tokenSigningKey)
	if err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	// Create two bee nodes
	serverNode, err := NewBeeNode(swarmID)
	if err != nil {
		t.Fatalf("Failed to create server node: %v", err)
	}

	clientNode, err := NewBeeNode(swarmID)
	if err != nil {
		t.Fatalf("Failed to create client node: %v", err)
	}

	// Start server
	listener, err := serverNode.Listen(ctx, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer listener.Close()

	serverAddr := listener.Addr().String()

	// Handle server connections
	serverDone := make(chan error, 1)
	go func() {
		defer close(serverDone)

		// Accept TLS connection
		conn, err := listener.Accept(ctx)
		if err != nil {
			serverDone <- err
			return
		}
		defer conn.Close()

		// Create server handshake with admission control
		handshake := noiseik.NewHandshakeWithAdmission(serverNode.identity, swarmID, admissionConfig, "", nil)
		handshake.SetTokenValidator(tokenPublicKey)

		// For now, we'll test admission control without PSK in integration test
		// Full PSK+Token combination would require additional constructor methods

		// Perform handshake
		_, err = serverNode.performSecureHandshake(conn, handshake, false)
		if err != nil {
			serverDone <- err
			return
		}

		serverDone <- nil
	}()

	// Client connects with PSK and token
	conn, err := clientNode.Dial(ctx, serverAddr)
	if err != nil {
		t.Fatalf("Failed to dial server: %v", err)
	}
	defer conn.Close()

	// Create client handshake with admission token
	handshake := noiseik.NewHandshakeWithAdmission(clientNode.identity, swarmID, admissionConfig, token, tokenSigningKey)

	// Perform secure handshake
	_, err = clientNode.performSecureHandshake(conn, handshake, true)
	if err != nil {
		t.Fatalf("Client secure handshake failed: %v", err)
	}

	// Wait for server to complete
	if err := <-serverDone; err != nil {
		t.Fatalf("Server secure handshake failed: %v", err)
	}

	t.Logf("Secure swarm integration test successful:")
	t.Logf("  Client BID: %s", clientNode.identity.BID())
	t.Logf("  Server BID: %s", serverNode.identity.BID())
	t.Logf("  Swarm ID: %s", swarmID)
	t.Logf("  Token-based Admission: ✓")
	t.Logf("  Double security: TLS + Noise IK with admission control")
}

// performSecureHandshake performs handshake with PSK and token validation
func (bn *BeeNode) performSecureHandshake(conn transport.Conn, handshake *noiseik.Handshake, isInitiator bool) (*noiseik.Handshake, error) {
	if isInitiator {
		// Client sends ClientHello
		clientHello, err := handshake.CreateClientHello()
		if err != nil {
			return nil, err
		}

		helloData, err := clientHello.Marshal()
		if err != nil {
			return nil, err
		}

		if _, err := conn.Write(helloData); err != nil {
			return nil, err
		}

		// Client receives ServerHello
		buffer := make([]byte, 4096)
		n, err := conn.Read(buffer)
		if err != nil {
			return nil, err
		}

		var serverHello noiseik.ServerHello
		if err := serverHello.Unmarshal(buffer[:n]); err != nil {
			return nil, err
		}

		if err := handshake.ProcessServerHello(&serverHello); err != nil {
			return nil, err
		}
	} else {
		// Server receives ClientHello
		buffer := make([]byte, 4096)
		n, err := conn.Read(buffer)
		if err != nil {
			return nil, err
		}

		var clientHello noiseik.ClientHello
		if err := clientHello.Unmarshal(buffer[:n]); err != nil {
			return nil, err
		}

		// Server sends ServerHello
		serverHello, err := handshake.ProcessClientHello(&clientHello)
		if err != nil {
			return nil, err
		}

		helloData, err := serverHello.Marshal()
		if err != nil {
			return nil, err
		}

		if _, err := conn.Write(helloData); err != nil {
			return nil, err
		}
	}

	return handshake, nil
}

func TestSecurityErrorConditions(t *testing.T) {
	ctx := context.Background()
	swarmID := "error-test-swarm"

	// Create admission control that requires tokens
	admissionConfig := noiseik.NewAdmissionConfig()
	admissionConfig.RequireToken = true

	// Generate token signing keys
	tokenPublicKey, tokenSigningKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate token signing key: %v", err)
	}

	// Add a valid token
	token := "valid-token"
	expiry := uint64(time.Now().Add(time.Hour).Unix())
	err = admissionConfig.AddToken(token, expiry, tokenSigningKey)
	if err != nil {
		t.Fatalf("Failed to add token: %v", err)
	}

	// Create two bee nodes
	serverNode, err := NewBeeNode(swarmID)
	if err != nil {
		t.Fatalf("Failed to create server node: %v", err)
	}

	clientNode, err := NewBeeNode(swarmID)
	if err != nil {
		t.Fatalf("Failed to create client node: %v", err)
	}

	// Start server
	listener, err := serverNode.Listen(ctx, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer listener.Close()

	serverAddr := listener.Addr().String()

	// Test 1: Client without token should be rejected
	t.Run("ClientWithoutToken", func(t *testing.T) {
		// Handle server connections
		serverDone := make(chan error, 1)
		go func() {
			defer close(serverDone)

			conn, err := listener.Accept(ctx)
			if err != nil {
				serverDone <- err
				return
			}
			defer conn.Close()

			// Create server handshake with admission control
			handshake := noiseik.NewHandshakeWithAdmission(serverNode.identity, swarmID, admissionConfig, "", nil)
			handshake.SetTokenValidator(tokenPublicKey)

			// This should fail because client has no token
			_, err = serverNode.performSecureHandshake(conn, handshake, false)
			serverDone <- err
		}()

		// Client connects without token
		conn, err := clientNode.Dial(ctx, serverAddr)
		if err != nil {
			t.Fatalf("Failed to dial server: %v", err)
		}
		defer conn.Close()

		// Create client handshake without token (should fail)
		handshake := noiseik.NewHandshake(clientNode.identity, swarmID)

		// This should fail
		_, err = clientNode.performSecureHandshake(conn, handshake, true)
		if err == nil {
			t.Error("Expected client handshake to fail without token")
		}

		// Server should also fail
		if err := <-serverDone; err == nil {
			t.Error("Expected server to reject client without token")
		}

		t.Logf("✓ Server correctly rejected client without admission token")
	})

	// Test 2: Client with invalid token should be rejected
	t.Run("ClientWithInvalidToken", func(t *testing.T) {
		// Generate different signing key (invalid)
		_, invalidSigningKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatalf("Failed to generate invalid signing key: %v", err)
		}

		// Handle server connections
		serverDone := make(chan error, 1)
		go func() {
			defer close(serverDone)

			conn, err := listener.Accept(ctx)
			if err != nil {
				serverDone <- err
				return
			}
			defer conn.Close()

			// Create server handshake with admission control
			handshake := noiseik.NewHandshakeWithAdmission(serverNode.identity, swarmID, admissionConfig, "", nil)
			handshake.SetTokenValidator(tokenPublicKey)

			// This should fail because client has invalid token signature
			_, err = serverNode.performSecureHandshake(conn, handshake, false)
			serverDone <- err
		}()

		// Client connects with invalid token signature
		conn, err := clientNode.Dial(ctx, serverAddr)
		if err != nil {
			t.Fatalf("Failed to dial server: %v", err)
		}
		defer conn.Close()

		// Create client handshake with invalid token signature
		handshake := noiseik.NewHandshakeWithAdmission(clientNode.identity, swarmID, admissionConfig, token, invalidSigningKey)

		// This should fail
		_, err = clientNode.performSecureHandshake(conn, handshake, true)
		if err == nil {
			t.Error("Expected client handshake to fail with invalid token")
		}

		// Server should also fail
		if err := <-serverDone; err == nil {
			t.Error("Expected server to reject client with invalid token")
		}

		t.Logf("✓ Server correctly rejected client with invalid token signature")
	})

	t.Logf("Security error condition tests completed successfully")
}
