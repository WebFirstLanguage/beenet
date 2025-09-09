package tcp

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

func TestTCPTransport_Name(t *testing.T) {
	transport := New()
	if transport.Name() != "tcp" {
		t.Errorf("Expected transport name 'tcp', got '%s'", transport.Name())
	}
}

func TestTCPTransport_DefaultPort(t *testing.T) {
	transport := New()
	if transport.DefaultPort() != constants.DefaultQUICPort {
		t.Errorf("Expected default port %d, got %d", constants.DefaultQUICPort, transport.DefaultPort())
	}
}

func TestTCPTransport_Listen(t *testing.T) {
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

	// Verify it's a TCP address
	if _, ok := addr.(*net.TCPAddr); !ok {
		t.Errorf("Expected TCP address, got %T", addr)
	}
}

func TestTCPTransport_Dial(t *testing.T) {
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

	// Accept connections in a goroutine
	acceptDone := make(chan error, 1)
	go func() {
		_, err := listener.Accept(ctx)
		acceptDone <- err
	}()

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

	// Wait for accept to complete
	if err := <-acceptDone; err != nil {
		t.Fatalf("Failed to accept: %v", err)
	}

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

func TestTCPTransport_AcceptAndCommunicate(t *testing.T) {
	transport := New()
	ctx := context.Background()
	tlsConfig := generateTestTLSConfig()

	// Start listener
	listener, err := transport.Listen(ctx, "127.0.0.1:0", tlsConfig)
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()

	// Accept connections in a goroutine
	acceptDone := make(chan error, 1)
	var serverConn *Conn
	go func() {
		var err error
		conn, err := listener.Accept(ctx)
		if err == nil {
			serverConn = conn.(*Conn)
		}
		acceptDone <- err
	}()

	// Dial from client
	clientTLSConfig := &tls.Config{
		NextProtos:         []string{"beenet/1"},
		InsecureSkipVerify: true,
	}

	clientConn, err := transport.Dial(ctx, addr, clientTLSConfig)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer clientConn.Close()

	// Wait for accept to complete
	if err := <-acceptDone; err != nil {
		t.Fatalf("Failed to accept: %v", err)
	}
	defer serverConn.Close()

	// Test communication
	testData := []byte("Hello, BeeNet!")

	// Client writes, server reads
	n, err := clientConn.Write(testData)
	if err != nil {
		t.Fatalf("Client write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(testData), n)
	}

	readBuf := make([]byte, len(testData))
	n, err = serverConn.Read(readBuf)
	if err != nil {
		t.Fatalf("Server read failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Expected to read %d bytes, read %d", len(testData), n)
	}
	if string(readBuf) != string(testData) {
		t.Errorf("Expected to read '%s', got '%s'", string(testData), string(readBuf))
	}
}

func TestTCPTransport_ContextCancellation(t *testing.T) {
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

func TestTCPTransport_InvalidAddress(t *testing.T) {
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
