// Package transport provides transport layer abstractions for BeeNet.
// It supports both QUIC and TCP transports with TLS, as specified in §8.1.
package transport

import (
	"context"
	"crypto/tls"
	"net"
	"time"
)

// Transport represents a transport protocol (QUIC or TCP)
type Transport interface {
	// Listen starts listening for incoming connections on the given address
	Listen(ctx context.Context, addr string, tlsConfig *tls.Config) (Listener, error)

	// Dial establishes a connection to the given address
	Dial(ctx context.Context, addr string, tlsConfig *tls.Config) (Conn, error)

	// Name returns the transport name (e.g., "quic", "tcp")
	Name() string

	// DefaultPort returns the default port for this transport
	DefaultPort() int
}

// Listener represents a transport listener
type Listener interface {
	// Accept waits for and returns the next connection
	Accept(ctx context.Context) (Conn, error)

	// Close closes the listener
	Close() error

	// Addr returns the listener's network address
	Addr() net.Addr
}

// Conn represents a transport connection
type Conn interface {
	// Read reads data from the connection
	Read(b []byte) (n int, err error)

	// Write writes data to the connection
	Write(b []byte) (n int, err error)

	// Close closes the connection
	Close() error

	// LocalAddr returns the local network address
	LocalAddr() net.Addr

	// RemoteAddr returns the remote network address
	RemoteAddr() net.Addr

	// SetDeadline sets the read and write deadlines
	SetDeadline(t time.Time) error

	// SetReadDeadline sets the read deadline
	SetReadDeadline(t time.Time) error

	// SetWriteDeadline sets the write deadline
	SetWriteDeadline(t time.Time) error

	// ConnectionState returns the TLS connection state
	ConnectionState() tls.ConnectionState
}

// Config holds transport configuration
type Config struct {
	// TLS configuration
	TLSConfig *tls.Config

	// ALPN protocols to negotiate
	ALPNProtocols []string

	// Connection timeout
	ConnectTimeout time.Duration

	// Keep-alive settings
	KeepAlive time.Duration

	// Maximum idle timeout
	MaxIdleTimeout time.Duration
}

// DefaultConfig returns a default transport configuration
func DefaultConfig() *Config {
	return &Config{
		ALPNProtocols:  []string{"beenet/1"},
		ConnectTimeout: 30 * time.Second,
		KeepAlive:      30 * time.Second,
		MaxIdleTimeout: 5 * time.Minute,
	}
}

// Registry manages available transports
type Registry struct {
	transports map[string]Transport
}

// NewRegistry creates a new transport registry
func NewRegistry() *Registry {
	return &Registry{
		transports: make(map[string]Transport),
	}
}

// Register registers a transport with the given name
func (r *Registry) Register(name string, transport Transport) {
	r.transports[name] = transport
}

// Get returns the transport with the given name
func (r *Registry) Get(name string) (Transport, bool) {
	t, ok := r.transports[name]
	return t, ok
}

// List returns all registered transport names
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.transports))
	for name := range r.transports {
		names = append(names, name)
	}
	return names
}

// Default registry instance
var DefaultRegistry = NewRegistry()
