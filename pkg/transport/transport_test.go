package transport

import (
	"context"
	"crypto/tls"
	"net"
	"testing"
	"time"
)

// MockTransport implements Transport for testing
type MockTransport struct {
	name        string
	defaultPort int
}

func (m *MockTransport) Listen(ctx context.Context, addr string, tlsConfig *tls.Config) (Listener, error) {
	return &MockListener{addr: addr}, nil
}

func (m *MockTransport) Dial(ctx context.Context, addr string, tlsConfig *tls.Config) (Conn, error) {
	return &MockConn{addr: addr}, nil
}

func (m *MockTransport) Name() string {
	return m.name
}

func (m *MockTransport) DefaultPort() int {
	return m.defaultPort
}

// MockListener implements Listener for testing
type MockListener struct {
	addr   string
	closed bool
}

func (m *MockListener) Accept(ctx context.Context) (Conn, error) {
	if m.closed {
		return nil, net.ErrClosed
	}
	return &MockConn{addr: m.addr}, nil
}

func (m *MockListener) Close() error {
	m.closed = true
	return nil
}

func (m *MockListener) Addr() net.Addr {
	addr, _ := net.ResolveTCPAddr("tcp", m.addr)
	return addr
}

// MockConn implements Conn for testing
type MockConn struct {
	addr   string
	closed bool
}

func (m *MockConn) Read(b []byte) (n int, err error) {
	if m.closed {
		return 0, net.ErrClosed
	}
	return 0, nil
}

func (m *MockConn) Write(b []byte) (n int, err error) {
	if m.closed {
		return 0, net.ErrClosed
	}
	return len(b), nil
}

func (m *MockConn) Close() error {
	m.closed = true
	return nil
}

func (m *MockConn) LocalAddr() net.Addr {
	addr, _ := net.ResolveTCPAddr("tcp", m.addr)
	return addr
}

func (m *MockConn) RemoteAddr() net.Addr {
	addr, _ := net.ResolveTCPAddr("tcp", m.addr)
	return addr
}

func (m *MockConn) SetDeadline(t time.Time) error {
	return nil
}

func (m *MockConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *MockConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func (m *MockConn) ConnectionState() tls.ConnectionState {
	return tls.ConnectionState{}
}

func TestRegistry(t *testing.T) {
	registry := NewRegistry()

	// Test empty registry
	if len(registry.List()) != 0 {
		t.Error("Expected empty registry")
	}

	// Test registration
	mockTransport := &MockTransport{name: "mock", defaultPort: 1234}
	registry.Register("mock", mockTransport)

	// Test retrieval
	transport, ok := registry.Get("mock")
	if !ok {
		t.Error("Expected to find registered transport")
	}
	if transport.Name() != "mock" {
		t.Errorf("Expected transport name 'mock', got '%s'", transport.Name())
	}
	if transport.DefaultPort() != 1234 {
		t.Errorf("Expected default port 1234, got %d", transport.DefaultPort())
	}

	// Test list
	names := registry.List()
	if len(names) != 1 || names[0] != "mock" {
		t.Errorf("Expected list ['mock'], got %v", names)
	}

	// Test non-existent transport
	_, ok = registry.Get("nonexistent")
	if ok {
		t.Error("Expected not to find non-existent transport")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if len(config.ALPNProtocols) == 0 {
		t.Error("Expected ALPN protocols to be set")
	}
	if config.ALPNProtocols[0] != "beenet/1" {
		t.Errorf("Expected ALPN protocol 'beenet/1', got '%s'", config.ALPNProtocols[0])
	}
	if config.ConnectTimeout == 0 {
		t.Error("Expected connect timeout to be set")
	}
	if config.KeepAlive == 0 {
		t.Error("Expected keep-alive to be set")
	}
	if config.MaxIdleTimeout == 0 {
		t.Error("Expected max idle timeout to be set")
	}
}

func TestTransportInterface(t *testing.T) {
	transport := &MockTransport{name: "test", defaultPort: 8080}
	ctx := context.Background()

	// Test Listen
	listener, err := transport.Listen(ctx, "localhost:8080", nil)
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer listener.Close()

	// Test Dial
	conn, err := transport.Dial(ctx, "localhost:8080", nil)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	// Test connection operations
	data := []byte("test data")
	n, err := conn.Write(data)
	if err != nil {
		t.Fatalf("Failed to write: %v", err)
	}
	if n != len(data) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
	}

	// Test listener operations
	if listener.Addr() == nil {
		t.Error("Expected listener address to be set")
	}
}

func TestConnectionLifecycle(t *testing.T) {
	conn := &MockConn{addr: "localhost:8080"}

	// Test initial state
	if conn.LocalAddr() == nil {
		t.Error("Expected local address to be set")
	}
	if conn.RemoteAddr() == nil {
		t.Error("Expected remote address to be set")
	}

	// Test deadline operations
	deadline := time.Now().Add(time.Second)
	if err := conn.SetDeadline(deadline); err != nil {
		t.Errorf("Failed to set deadline: %v", err)
	}
	if err := conn.SetReadDeadline(deadline); err != nil {
		t.Errorf("Failed to set read deadline: %v", err)
	}
	if err := conn.SetWriteDeadline(deadline); err != nil {
		t.Errorf("Failed to set write deadline: %v", err)
	}

	// Test close
	if err := conn.Close(); err != nil {
		t.Errorf("Failed to close connection: %v", err)
	}

	// Test operations after close
	_, err := conn.Write([]byte("test"))
	if err == nil {
		t.Error("Expected write to fail after close")
	}
}
