# Phase 2: Transport Layer + Application-Layer Session Security

This document describes the implementation of Phase 2 of the BeeNet project, which provides secure transport connections using QUIC/TLS and TCP/TLS, followed by application-layer Noise IK protocol to bind sessions to Bee ID (BID) and Swarm ID.

## Overview

Phase 2 implements a dual-layer security architecture:

1. **Transport Layer Security**: TLS 1.3 over QUIC (primary) or TCP (fallback)
2. **Application Layer Security**: Noise IK protocol with Ed25519 identity binding

This provides defense-in-depth with double encryption and strong identity verification.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Application Layer                        │
├─────────────────────────────────────────────────────────────┤
│  Noise IK Protocol (X25519 + ChaCha20-Poly1305 + BLAKE3)   │
│  - Identity binding (Ed25519)                               │
│  - PSK authentication (optional)                            │
│  - Token-based admission control (optional)                 │
│  - Replay protection with sequence tracking                 │
├─────────────────────────────────────────────────────────────┤
│                  Transport Layer                            │
│  TLS 1.3 over QUIC (primary) or TCP (fallback)            │
│  - ALPN: "beenet/1"                                        │
│  - Perfect Forward Secrecy                                 │
│  - Certificate-based authentication                         │
└─────────────────────────────────────────────────────────────┘
```

## Transport Layer

### QUIC Transport (`pkg/transport/quic/`)

Primary transport protocol providing:
- Built-in multiplexing and flow control
- 0-RTT connection establishment (after initial handshake)
- Automatic congestion control
- Connection migration support
- TLS 1.3 integration

**Key Features:**
- ALPN negotiation with "beenet/1"
- Configurable timeouts and keep-alive
- Context-based cancellation
- Automatic retry and error recovery

### TCP Transport (`pkg/transport/tcp/`)

Fallback transport protocol providing:
- Reliable, ordered delivery
- TLS 1.3 encryption
- Wide compatibility
- Simple connection model

**Key Features:**
- TLS 1.3 with ALPN "beenet/1"
- Connection pooling support
- Graceful shutdown handling
- Error recovery mechanisms

### Transport Interface

Both transports implement the common `transport.Transport` interface:

```go
type Transport interface {
    Name() string
    DefaultPort() int
    Listen(ctx context.Context, addr string, tlsConfig *tls.Config) (Listener, error)
    Dial(ctx context.Context, addr string, tlsConfig *tls.Config) (Conn, error)
}
```

## Application Layer Security

### Noise IK Protocol (`pkg/security/noiseik/`)

The Noise IK (Interactive Key exchange) protocol provides:

- **Key Exchange**: X25519 Elliptic Curve Diffie-Hellman
- **Encryption**: ChaCha20-Poly1305 AEAD
- **Hashing**: BLAKE3 for key derivation and authentication
- **Identity**: Ed25519 signatures for identity binding

### Protocol Flow

1. **ClientHello**: Client sends identity, capabilities, and Noise ephemeral key
2. **ServerHello**: Server responds with identity and completes key exchange
3. **Session Keys**: Both parties derive shared encryption keys
4. **Secure Communication**: All subsequent messages use derived keys

### Message Structures

#### ClientHello
```go
type ClientHello struct {
    Version         int      `cbor:"v"`
    SwarmID         string   `cbor:"swarm"`
    From            string   `cbor:"from"`           // Client BID
    Nonce           uint64   `cbor:"nonce"`
    Caps            []string `cbor:"caps"`           // Capabilities
    NoiseKey        []byte   `cbor:"noisekey"`       // X25519 public key
    Proof           []byte   `cbor:"proof"`          // Ed25519 signature
    
    // Optional PSK fields
    PSKHint         *string  `cbor:"psk_hint,omitempty"`
    PSKProof        []byte   `cbor:"psk_proof,omitempty"`
    
    // Optional admission control fields
    AdmissionToken  *string  `cbor:"admission_token,omitempty"`
    TokenProof      []byte   `cbor:"token_proof,omitempty"`
    TokenExpiry     *uint64  `cbor:"token_expiry,omitempty"`
}
```

#### ServerHello
```go
type ServerHello struct {
    Version         int      `cbor:"v"`
    SwarmID         string   `cbor:"swarm"`
    From            string   `cbor:"from"`           // Server BID
    Nonce           uint64   `cbor:"nonce"`
    NoiseKey        []byte   `cbor:"noisekey"`       // X25519 public key
    Proof           []byte   `cbor:"proof"`          // Ed25519 signature
    
    // Optional PSK fields
    PSKProof        []byte   `cbor:"psk_proof,omitempty"`
}
```

### Security Features

#### 1. Pre-Shared Key (PSK) Authentication

Optional shared secret authentication using HMAC-SHA256:

```go
// Create PSK configuration
pskConfig := noiseik.NewPSKConfig(psk, "swarm-psk-hint")

// Create handshake with PSK
handshake := noiseik.NewHandshakeWithPSK(identity, swarmID, pskConfig)
```

#### 2. Token-Based Admission Control

Ed25519 signature-based tokens for swarm access control:

```go
// Create admission configuration
admissionConfig := noiseik.NewAdmissionConfig()
admissionConfig.RequireToken = true

// Add valid token
err := admissionConfig.AddToken("token-id", expiry, signingKey)

// Create handshake with admission control
handshake := noiseik.NewHandshakeWithAdmission(identity, swarmID, 
    admissionConfig, "token-id", signingKey)
```

#### 3. Replay Protection

Sliding window mechanism with sequence number tracking:

- 64-bit sequence numbers
- Configurable window size (default: 1000)
- Bitmap-based duplicate detection
- Automatic window advancement

### API Usage Examples

#### Basic Handshake

```go
// Client side
clientHandshake := noiseik.NewHandshake(clientIdentity, swarmID)
clientHello, err := clientHandshake.CreateClientHello()

// Server side  
serverHandshake := noiseik.NewHandshake(serverIdentity, swarmID)
serverHello, err := serverHandshake.ProcessClientHello(clientHello)

// Complete handshake
err = clientHandshake.ProcessServerHello(serverHello)

// Get session keys
sendKey, recvKey, err := clientHandshake.GetSessionKeys()
```

#### Secure Swarm with PSK and Tokens

```go
// Server setup
pskConfig := noiseik.NewPSKConfig(sharedSecret, "swarm-psk")
admissionConfig := noiseik.NewAdmissionConfig()
admissionConfig.RequireToken = true

serverHandshake := noiseik.NewHandshakeWithAdmission(
    serverIdentity, swarmID, admissionConfig, "", nil)
serverHandshake.SetTokenValidator(tokenPublicKey)

// Client setup
clientHandshake := noiseik.NewHandshakeWithAdmission(
    clientIdentity, swarmID, admissionConfig, "valid-token", tokenSigningKey)
```

## Integration Testing

The implementation includes comprehensive integration tests demonstrating:

1. **Basic TLS + Noise IK**: Two nodes establishing secure connection
2. **Token-based Admission**: Secure swarm with admission control
3. **Error Conditions**: Proper rejection of invalid clients

### Running Integration Tests

```bash
go test -v ./pkg/integration
```

## Performance Characteristics

### QUIC Transport
- **Connection Establishment**: ~1 RTT after initial handshake
- **Multiplexing**: Multiple streams per connection
- **Head-of-line Blocking**: Eliminated at transport layer

### TCP Transport  
- **Connection Establishment**: ~3 RTT (TCP + TLS handshake)
- **Reliability**: Guaranteed ordered delivery
- **Compatibility**: Works with all network configurations

### Noise IK Protocol
- **Handshake**: 2 messages (ClientHello + ServerHello)
- **Key Derivation**: Single BLAKE3 operation
- **Session Keys**: 32 bytes send + 32 bytes receive

## Security Considerations

### Threat Model

The implementation protects against:

1. **Eavesdropping**: Double encryption (TLS + Noise)
2. **Man-in-the-Middle**: Ed25519 identity verification
3. **Replay Attacks**: Sequence number tracking
4. **Unauthorized Access**: PSK and token-based admission control

### Cryptographic Primitives

- **Key Exchange**: X25519 (Curve25519)
- **Signatures**: Ed25519
- **Encryption**: ChaCha20-Poly1305
- **Hashing**: BLAKE3
- **PSK Authentication**: HMAC-SHA256

### Forward Secrecy

Both TLS 1.3 and Noise IK provide perfect forward secrecy:
- TLS 1.3: Ephemeral key exchange
- Noise IK: X25519 ephemeral keys

## Error Handling

The implementation provides comprehensive error handling for:

- **Protocol Version Mismatches**: Rejected with version error
- **Invalid Signatures**: Ed25519 verification failures
- **Replay Attacks**: Sequence number validation
- **PSK Mismatches**: HMAC verification failures  
- **Token Validation**: Signature and expiry checks
- **Network Errors**: Connection failures and timeouts

## Future Enhancements

Phase 2 provides the foundation for:

1. **Connection Pooling**: Reuse of established connections
2. **Load Balancing**: Multiple transport endpoints
3. **Advanced Admission Control**: Role-based access control
4. **Metrics and Monitoring**: Connection and security metrics
5. **Protocol Upgrades**: Backward-compatible protocol evolution

## Conclusion

Phase 2 successfully implements a robust, secure transport and session layer for BeeNet, providing:

- ✅ Dual-layer security (TLS + Noise IK)
- ✅ Strong identity binding (Ed25519)
- ✅ Optional PSK authentication
- ✅ Token-based admission control
- ✅ Replay protection
- ✅ QUIC and TCP transport support
- ✅ Comprehensive testing
- ✅ Production-ready error handling

The implementation is ready for Phase 3 development and production deployment.
