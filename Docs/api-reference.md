# BeeNet Phase 2 API Reference

This document provides detailed API reference for the BeeNet Phase 2 transport and security components.

## Transport Layer APIs

### `pkg/transport`

#### Transport Interface

```go
type Transport interface {
    // Name returns the transport protocol name
    Name() string
    
    // DefaultPort returns the default port for this transport
    DefaultPort() int
    
    // Listen starts listening for connections on the given address
    Listen(ctx context.Context, addr string, tlsConfig *tls.Config) (Listener, error)
    
    // Dial establishes a connection to the given address
    Dial(ctx context.Context, addr string, tlsConfig *tls.Config) (Conn, error)
}
```

#### Listener Interface

```go
type Listener interface {
    // Accept waits for and returns the next connection
    Accept(ctx context.Context) (Conn, error)
    
    // Close closes the listener
    Close() error
    
    // Addr returns the listener's network address
    Addr() net.Addr
}
```

#### Connection Interface

```go
type Conn interface {
    // Read reads data from the connection
    Read([]byte) (int, error)
    
    // Write writes data to the connection
    Write([]byte) (int, error)
    
    // Close closes the connection
    Close() error
    
    // ConnectionState returns the TLS connection state
    ConnectionState() tls.ConnectionState
}
```

### `pkg/transport/quic`

#### QUIC Transport

```go
// New creates a new QUIC transport
func New() transport.Transport

// QUICTransport implements the Transport interface for QUIC
type QUICTransport struct{}

func (t *QUICTransport) Name() string
func (t *QUICTransport) DefaultPort() int
func (t *QUICTransport) Listen(ctx context.Context, addr string, tlsConfig *tls.Config) (transport.Listener, error)
func (t *QUICTransport) Dial(ctx context.Context, addr string, tlsConfig *tls.Config) (transport.Conn, error)
```

### `pkg/transport/tcp`

#### TCP Transport

```go
// New creates a new TCP transport
func New() transport.Transport

// TCPTransport implements the Transport interface for TCP
type TCPTransport struct{}

func (t *TCPTransport) Name() string
func (t *TCPTransport) DefaultPort() int
func (t *TCPTransport) Listen(ctx context.Context, addr string, tlsConfig *tls.Config) (transport.Listener, error)
func (t *TCPTransport) Dial(ctx context.Context, addr string, tlsConfig *tls.Config) (transport.Conn, error)
```

## Security Layer APIs

### `pkg/security/noiseik`

#### Core Handshake

```go
// Handshake represents a Noise IK handshake instance
type Handshake struct {
    // Private fields
}

// NewHandshake creates a new handshake instance
func NewHandshake(id *identity.Identity, swarmID string) *Handshake

// CreateClientHello creates a ClientHello message
func (h *Handshake) CreateClientHello() (*ClientHello, error)

// ProcessClientHello processes a ClientHello and returns ServerHello
func (h *Handshake) ProcessClientHello(hello *ClientHello) (*ServerHello, error)

// ProcessServerHello processes a ServerHello message
func (h *Handshake) ProcessServerHello(hello *ServerHello) error

// IsComplete returns true if the handshake is complete
func (h *Handshake) IsComplete() bool

// GetSessionKeys returns the derived session keys
func (h *Handshake) GetSessionKeys() (sendKey, recvKey []byte, err error)
```

#### PSK Authentication

```go
// PSKConfig represents Pre-Shared Key configuration
type PSKConfig struct {
    PSK  []byte // The pre-shared key (should be at least 32 bytes)
    Hint string // Optional hint to identify which PSK to use
}

// NewPSKConfig creates a new PSK configuration
func NewPSKConfig(psk []byte, hint string) *PSKConfig

// GenerateProof generates HMAC proof for a message
func (psk *PSKConfig) GenerateProof(message []byte) []byte

// VerifyProof verifies HMAC proof for a message
func (psk *PSKConfig) VerifyProof(message []byte, proof []byte) bool

// NewHandshakeWithPSK creates a handshake with PSK authentication
func NewHandshakeWithPSK(id *identity.Identity, swarmID string, pskConfig *PSKConfig) *Handshake
```

#### Token-Based Admission Control

```go
// AdmissionConfig represents admission control configuration
type AdmissionConfig struct {
    RequireToken bool                    // Whether tokens are required
    ValidTokens  map[string]TokenInfo    // Map of valid tokens
}

// TokenInfo represents information about a valid token
type TokenInfo struct {
    Expiry    uint64            // Unix timestamp when token expires
    PublicKey ed25519.PublicKey // Public key to verify token signatures
}

// NewAdmissionConfig creates a new admission control configuration
func NewAdmissionConfig() *AdmissionConfig

// AddToken adds a valid token to the admission control
func (ac *AdmissionConfig) AddToken(token string, expiry uint64, signingKey ed25519.PrivateKey) error

// ValidateToken validates a token and its proof
func (ac *AdmissionConfig) ValidateToken(token, swarmID string, proof []byte, publicKey ed25519.PublicKey) bool

// NewHandshakeWithAdmission creates a handshake with admission control
func NewHandshakeWithAdmission(id *identity.Identity, swarmID string, 
    admissionConfig *AdmissionConfig, clientToken string, 
    tokenSigningKey ed25519.PrivateKey) *Handshake

// SetTokenValidator sets the token validation public key (for servers)
func (h *Handshake) SetTokenValidator(publicKey ed25519.PublicKey)
```

#### Replay Protection

```go
// NextSendSequence returns the next sequence number for outgoing messages
func (h *Handshake) NextSendSequence() uint64

// ValidateReceiveSequence validates an incoming message sequence number
func (h *Handshake) ValidateReceiveSequence(sequence uint64) bool

// GetSequenceStats returns sequence tracking statistics
func (h *Handshake) GetSequenceStats() (sendSeq uint64, lastRecvSeq uint64)
```

#### Message Structures

```go
// ClientHello represents the initial handshake message from client
type ClientHello struct {
    Version         int      `cbor:"v"`
    SwarmID         string   `cbor:"swarm"`
    From            string   `cbor:"from"`
    Nonce           uint64   `cbor:"nonce"`
    Caps            []string `cbor:"caps"`
    NoiseKey        []byte   `cbor:"noisekey"`
    Proof           []byte   `cbor:"proof"`
    
    // Optional PSK fields
    PSKHint         *string  `cbor:"psk_hint,omitempty"`
    PSKProof        []byte   `cbor:"psk_proof,omitempty"`
    
    // Optional admission control fields
    AdmissionToken  *string  `cbor:"admission_token,omitempty"`
    TokenProof      []byte   `cbor:"token_proof,omitempty"`
    TokenExpiry     *uint64  `cbor:"token_expiry,omitempty"`
}

// Marshal serializes the ClientHello to CBOR
func (ch *ClientHello) Marshal() ([]byte, error)

// Unmarshal deserializes CBOR data to ClientHello
func (ch *ClientHello) Unmarshal(data []byte) error

// Sign signs the ClientHello with the given private key
func (ch *ClientHello) Sign(privateKey ed25519.PrivateKey) error

// Verify verifies the ClientHello signature
func (ch *ClientHello) Verify(publicKey ed25519.PublicKey) error
```

```go
// ServerHello represents the response message from server
type ServerHello struct {
    Version         int      `cbor:"v"`
    SwarmID         string   `cbor:"swarm"`
    From            string   `cbor:"from"`
    Nonce           uint64   `cbor:"nonce"`
    NoiseKey        []byte   `cbor:"noisekey"`
    Proof           []byte   `cbor:"proof"`
    
    // Optional PSK fields
    PSKProof        []byte   `cbor:"psk_proof,omitempty"`
}

// Marshal serializes the ServerHello to CBOR
func (sh *ServerHello) Marshal() ([]byte, error)

// Unmarshal deserializes CBOR data to ServerHello
func (sh *ServerHello) Unmarshal(data []byte) error

// Sign signs the ServerHello with the given private key
func (sh *ServerHello) Sign(privateKey ed25519.PrivateKey) error

// Verify verifies the ServerHello signature
func (sh *ServerHello) Verify(publicKey ed25519.PublicKey) error
```

## Usage Examples

### Basic Transport Usage

```go
import (
    "context"
    "crypto/tls"
    "github.com/WebFirstLanguage/beenet/pkg/transport/tcp"
)

// Create transport
transport := tcp.New()

// Server side
tlsConfig := &tls.Config{
    Certificates: []tls.Certificate{cert},
    NextProtos:   []string{"beenet/1"},
}

listener, err := transport.Listen(ctx, ":8080", tlsConfig)
if err != nil {
    return err
}

conn, err := listener.Accept(ctx)
if err != nil {
    return err
}

// Client side
clientTLSConfig := &tls.Config{
    NextProtos: []string{"beenet/1"},
}

conn, err := transport.Dial(ctx, "localhost:8080", clientTLSConfig)
if err != nil {
    return err
}
```

### Basic Noise IK Handshake

```go
import (
    "github.com/WebFirstLanguage/beenet/pkg/security/noiseik"
    "github.com/WebFirstLanguage/beenet/pkg/identity"
)

// Generate identities
clientIdentity, _ := identity.GenerateIdentity()
serverIdentity, _ := identity.GenerateIdentity()

// Client side
clientHandshake := noiseik.NewHandshake(clientIdentity, "my-swarm")
clientHello, err := clientHandshake.CreateClientHello()

// Send clientHello over transport...

// Server side
serverHandshake := noiseik.NewHandshake(serverIdentity, "my-swarm")
serverHello, err := serverHandshake.ProcessClientHello(clientHello)

// Send serverHello back to client...

// Client completes handshake
err = clientHandshake.ProcessServerHello(serverHello)

// Get session keys
sendKey, recvKey, err := clientHandshake.GetSessionKeys()
```

### Secure Swarm with PSK and Tokens

```go
// Create PSK
psk := make([]byte, 32)
rand.Read(psk)
pskConfig := noiseik.NewPSKConfig(psk, "swarm-psk")

// Create admission control
admissionConfig := noiseik.NewAdmissionConfig()
admissionConfig.RequireToken = true

// Generate token keys
tokenPublicKey, tokenSigningKey, _ := ed25519.GenerateKey(rand.Reader)

// Add valid token
expiry := uint64(time.Now().Add(time.Hour).Unix())
err := admissionConfig.AddToken("client-token", expiry, tokenSigningKey)

// Server handshake
serverHandshake := noiseik.NewHandshakeWithAdmission(
    serverIdentity, swarmID, admissionConfig, "", nil)
serverHandshake.SetTokenValidator(tokenPublicKey)

// Client handshake
clientHandshake := noiseik.NewHandshakeWithAdmission(
    clientIdentity, swarmID, admissionConfig, "client-token", tokenSigningKey)
```

## Error Handling

All APIs return Go-style errors. Common error types include:

- **Transport Errors**: Network connectivity, TLS handshake failures
- **Protocol Errors**: Version mismatches, malformed messages
- **Cryptographic Errors**: Signature verification, key derivation failures
- **Authentication Errors**: PSK mismatches, invalid tokens
- **Replay Errors**: Sequence number validation failures

## Thread Safety

- **Transport instances**: Safe for concurrent use
- **Handshake instances**: Not thread-safe, use one per connection
- **Configuration objects**: Safe for read-only concurrent access

## Performance Notes

- **QUIC Transport**: Preferred for performance and features
- **TCP Transport**: Use for compatibility or firewall restrictions
- **Handshake Overhead**: ~2ms for complete Noise IK handshake
- **Session Keys**: Derive once, use for entire session lifetime
