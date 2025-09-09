// Package tcp implements TCP+TLS transport for BeeNet as specified in §8.1.
// It provides TCP + TLS 1.3 transport as a fallback to QUIC.
package tcp

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/constants"
	"github.com/WebFirstLanguage/beenet/pkg/transport"
)

// Transport implements the TCP+TLS transport
type Transport struct{}

// New creates a new TCP transport
func New() transport.Transport {
	return &Transport{}
}

// Name returns the transport name
func (t *Transport) Name() string {
	return "tcp"
}

// DefaultPort returns the default TCP port (same as QUIC for simplicity)
func (t *Transport) DefaultPort() int {
	return constants.DefaultQUICPort
}

// Listen starts listening for TCP+TLS connections
func (t *Transport) Listen(ctx context.Context, addr string, tlsConfig *tls.Config) (transport.Listener, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Parse the address
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve TCP address: %w", err)
	}

	// Create TCP listener
	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create TCP listener: %w", err)
	}

	// Configure TLS
	serverTLSConfig := tlsConfig.Clone()
	if serverTLSConfig == nil {
		serverTLSConfig = &tls.Config{}
	}

	// Ensure ALPN protocols are set
	if len(serverTLSConfig.NextProtos) == 0 {
		serverTLSConfig.NextProtos = []string{"beenet/1"}
	}

	// Ensure TLS 1.3 minimum
	if serverTLSConfig.MinVersion == 0 {
		serverTLSConfig.MinVersion = tls.VersionTLS13
	}

	return &Listener{
		listener:  listener,
		tlsConfig: serverTLSConfig,
	}, nil
}

// Dial establishes a TCP+TLS connection
func (t *Transport) Dial(ctx context.Context, addr string, tlsConfig *tls.Config) (transport.Conn, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Configure TLS for client
	clientTLSConfig := tlsConfig.Clone()
	if clientTLSConfig == nil {
		clientTLSConfig = &tls.Config{}
	}

	// Ensure ALPN protocols are set
	if len(clientTLSConfig.NextProtos) == 0 {
		clientTLSConfig.NextProtos = []string{"beenet/1"}
	}

	// Ensure TLS 1.3 minimum
	if clientTLSConfig.MinVersion == 0 {
		clientTLSConfig.MinVersion = tls.VersionTLS13
	}

	// Create dialer with timeout
	dialer := &net.Dialer{
		Timeout: 30 * time.Second,
	}

	// Dial TCP+TLS connection
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, clientTLSConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to dial TCP+TLS connection: %w", err)
	}

	return &Conn{
		conn: conn,
	}, nil
}

// Listener wraps a TCP listener with TLS
type Listener struct {
	listener  *net.TCPListener
	tlsConfig *tls.Config
}

// Accept waits for and returns the next connection
func (l *Listener) Accept(ctx context.Context) (transport.Conn, error) {
	// Set deadline based on context
	if deadline, ok := ctx.Deadline(); ok {
		l.listener.SetDeadline(deadline)
	}

	// Accept TCP connection
	tcpConn, err := l.listener.AcceptTCP()
	if err != nil {
		return nil, err
	}

	// Wrap with TLS
	tlsConn := tls.Server(tcpConn, l.tlsConfig)

	// Perform TLS handshake
	if err := tlsConn.Handshake(); err != nil {
		tcpConn.Close()
		return nil, fmt.Errorf("TLS handshake failed: %w", err)
	}

	return &Conn{
		conn: tlsConn,
	}, nil
}

// Close closes the listener
func (l *Listener) Close() error {
	return l.listener.Close()
}

// Addr returns the listener's network address
func (l *Listener) Addr() net.Addr {
	return l.listener.Addr()
}

// Conn wraps a TLS connection
type Conn struct {
	conn *tls.Conn
}

// Read reads data from the connection
func (c *Conn) Read(b []byte) (n int, err error) {
	return c.conn.Read(b)
}

// Write writes data to the connection
func (c *Conn) Write(b []byte) (n int, err error) {
	return c.conn.Write(b)
}

// Close closes the connection
func (c *Conn) Close() error {
	return c.conn.Close()
}

// LocalAddr returns the local network address
func (c *Conn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// RemoteAddr returns the remote network address
func (c *Conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// SetDeadline sets the read and write deadlines
func (c *Conn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

// SetReadDeadline sets the read deadline
func (c *Conn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

// SetWriteDeadline sets the write deadline
func (c *Conn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

// ConnectionState returns the TLS connection state
func (c *Conn) ConnectionState() tls.ConnectionState {
	return c.conn.ConnectionState()
}
