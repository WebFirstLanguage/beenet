// Package quic implements QUIC transport for BeeNet as specified in §8.1.
// It provides QUIC + TLS 1.3 transport with proper ALPN negotiation.
package quic

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/WebFirstLanguage/beenet/pkg/constants"
	"github.com/WebFirstLanguage/beenet/pkg/transport"
	"github.com/quic-go/quic-go"
)

// Transport implements the QUIC transport
type Transport struct{}

// New creates a new QUIC transport
func New() transport.Transport {
	return &Transport{}
}

// Name returns the transport name
func (t *Transport) Name() string {
	return "quic"
}

// DefaultPort returns the default QUIC port
func (t *Transport) DefaultPort() int {
	return constants.DefaultQUICPort
}

// Listen starts listening for QUIC connections
func (t *Transport) Listen(ctx context.Context, addr string, tlsConfig *tls.Config) (transport.Listener, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Parse the address
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve UDP address: %w", err)
	}

	// Configure TLS for QUIC
	quicTLSConfig := tlsConfig.Clone()
	if quicTLSConfig == nil {
		quicTLSConfig = &tls.Config{}
	}

	// Ensure ALPN protocols are set
	if len(quicTLSConfig.NextProtos) == 0 {
		quicTLSConfig.NextProtos = []string{"beenet/1"}
	}

	// Create QUIC listener
	listener, err := quic.ListenAddr(udpAddr.String(), quicTLSConfig, &quic.Config{
		MaxIdleTimeout:  5 * time.Minute,
		KeepAlivePeriod: 30 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create QUIC listener: %w", err)
	}

	return &Listener{
		listener: listener,
	}, nil
}

// Dial establishes a QUIC connection
func (t *Transport) Dial(ctx context.Context, addr string, tlsConfig *tls.Config) (transport.Conn, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Configure TLS for QUIC
	quicTLSConfig := tlsConfig.Clone()
	if quicTLSConfig == nil {
		quicTLSConfig = &tls.Config{}
	}

	// Ensure ALPN protocols are set
	if len(quicTLSConfig.NextProtos) == 0 {
		quicTLSConfig.NextProtos = []string{"beenet/1"}
	}

	// Dial QUIC connection
	connection, err := quic.DialAddr(ctx, addr, quicTLSConfig, &quic.Config{
		MaxIdleTimeout:  5 * time.Minute,
		KeepAlivePeriod: 30 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to dial QUIC connection: %w", err)
	}

	// Open a stream for communication
	stream, err := connection.OpenStreamSync(ctx)
	if err != nil {
		connection.CloseWithError(0, "failed to open stream")
		return nil, fmt.Errorf("failed to open stream: %w", err)
	}

	return &Conn{
		connection: connection,
		stream:     stream,
	}, nil
}

// Listener wraps a QUIC listener
type Listener struct {
	listener *quic.Listener
}

// Accept waits for and returns the next connection
func (l *Listener) Accept(ctx context.Context) (transport.Conn, error) {
	connection, err := l.listener.Accept(ctx)
	if err != nil {
		return nil, err
	}

	// Accept a stream from the connection
	stream, err := connection.AcceptStream(ctx)
	if err != nil {
		connection.CloseWithError(0, "failed to accept stream")
		return nil, fmt.Errorf("failed to accept stream: %w", err)
	}

	return &Conn{
		connection: connection,
		stream:     stream,
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

// Conn wraps a QUIC connection and stream
type Conn struct {
	connection *quic.Conn
	stream     *quic.Stream
}

// Read reads data from the stream
func (c *Conn) Read(b []byte) (n int, err error) {
	return c.stream.Read(b)
}

// Write writes data to the stream
func (c *Conn) Write(b []byte) (n int, err error) {
	return c.stream.Write(b)
}

// Close closes the connection
func (c *Conn) Close() error {
	// Close the stream first
	if err := c.stream.Close(); err != nil {
		// Still try to close the connection
		c.connection.CloseWithError(0, "stream close error")
		return err
	}

	// Close the connection
	return c.connection.CloseWithError(0, "normal close")
}

// LocalAddr returns the local network address
func (c *Conn) LocalAddr() net.Addr {
	return c.connection.LocalAddr()
}

// RemoteAddr returns the remote network address
func (c *Conn) RemoteAddr() net.Addr {
	return c.connection.RemoteAddr()
}

// SetDeadline sets the read and write deadlines
func (c *Conn) SetDeadline(t time.Time) error {
	return c.stream.SetDeadline(t)
}

// SetReadDeadline sets the read deadline
func (c *Conn) SetReadDeadline(t time.Time) error {
	return c.stream.SetReadDeadline(t)
}

// SetWriteDeadline sets the write deadline
func (c *Conn) SetWriteDeadline(t time.Time) error {
	return c.stream.SetWriteDeadline(t)
}

// ConnectionState returns the TLS connection state
func (c *Conn) ConnectionState() tls.ConnectionState {
	return c.connection.ConnectionState().TLS
}
