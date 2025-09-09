package quic

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/constants"
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
			Organization: []string{"BeeNet Test"},
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

func TestQUICTransport_Name(t *testing.T) {
	transport := New()
	if transport.Name() != "quic" {
		t.Errorf("Expected transport name 'quic', got '%s'", transport.Name())
	}
}

func TestQUICTransport_DefaultPort(t *testing.T) {
	transport := New()
	if transport.DefaultPort() != constants.DefaultQUICPort {
		t.Errorf("Expected default port %d, got %d", constants.DefaultQUICPort, transport.DefaultPort())
	}
}

func TestQUICTransport_Listen(t *testing.T) {
	transport := New()
	ctx := context.Background()
	tlsConfig := generateTestTLSConfig()

	// Test listening on localhost
	listener, err := transport.Listen(ctx, "127.0.0.1:0", tlsConfig)
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer listener.Close()

	// Verify listener address
	addr := listener.Addr()
	if addr == nil {
		t.Error("Expected listener address to be set")
	}

	// Verify it's a UDP address (QUIC uses UDP)
	if _, ok := addr.(*net.UDPAddr); !ok {
		t.Errorf("Expected UDP address, got %T", addr)
	}
}

func TestQUICTransport_Dial(t *testing.T) {
	transport := New()
	ctx := context.Background()
	tlsConfig := generateTestTLSConfig()

	// Start a listener first
	listener, err := transport.Listen(ctx, "127.0.0.1:0", tlsConfig)
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer listener.Close()

	// Get the actual address
	addr := listener.Addr().String()

	// Test dialing
	clientTLSConfig := &tls.Config{
		NextProtos:         []string{"beenet/1"},
		InsecureSkipVerify: true, // For testing only
	}

	conn, err := transport.Dial(ctx, addr, clientTLSConfig)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	// Verify connection addresses
	if conn.LocalAddr() == nil {
		t.Error("Expected local address to be set")
	}
	if conn.RemoteAddr() == nil {
		t.Error("Expected remote address to be set")
	}

	// Verify TLS connection state
	state := conn.ConnectionState()
	if !state.HandshakeComplete {
		t.Error("Expected TLS handshake to be complete")
	}
	if state.NegotiatedProtocol != "beenet/1" {
		t.Errorf("Expected negotiated protocol 'beenet/1', got '%s'", state.NegotiatedProtocol)
	}
}

func TestQUICTransport_AcceptAndCommunicate(t *testing.T) {
	t.Skip("QUIC stream communication test - requires more complex stream handling, will be implemented in integration tests")
}

func TestQUICTransport_ContextCancellation(t *testing.T) {
	transport := New()
	tlsConfig := generateTestTLSConfig()

	// Test context cancellation during listen
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := transport.Listen(ctx, "127.0.0.1:0", tlsConfig)
	if err == nil {
		t.Error("Expected listen to fail with cancelled context")
	}

	// Test context cancellation during dial
	ctx, cancel = context.WithCancel(context.Background())
	cancel()

	_, err = transport.Dial(ctx, "127.0.0.1:12345", tlsConfig)
	if err == nil {
		t.Error("Expected dial to fail with cancelled context")
	}
}

func TestQUICTransport_InvalidAddress(t *testing.T) {
	transport := New()
	ctx := context.Background()
	tlsConfig := generateTestTLSConfig()

	// Test invalid listen address
	_, err := transport.Listen(ctx, "invalid:address", tlsConfig)
	if err == nil {
		t.Error("Expected listen to fail with invalid address")
	}

	// Test invalid dial address
	_, err = transport.Dial(ctx, "invalid:address", tlsConfig)
	if err == nil {
		t.Error("Expected dial to fail with invalid address")
	}
}
