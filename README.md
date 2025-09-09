# BeeNet

A decentralized peer-to-peer mesh network implementation with cryptographic identities and human-readable handles.

## Overview

BeeNet is a P2P mesh network that uses Ed25519/X25519 cryptographic identities with a unique "honeytag" system for human-readable identifiers. Each node has a handle in the format `<nickname>~<honeytag>` where the honeytag is derived from the node's cryptographic identity.

## Current Status: Phase 2 Complete ✅

**Phase 1 - Minimal Agent Kernel** ✅ **COMPLETE**
- ✅ Daemon lifecycle management with start/stop/retry supervisors
- ✅ CLI with `start`, `create`, `status`, `keygen`, `handle` subcommands
- ✅ Local control API (JSON over TCP)
- ✅ Persistent keystore with secure file permissions
- ✅ Nickname normalization and validation per specification
- ✅ Identity generation and handle computation
- ✅ Comprehensive test coverage

**Phase 2 - Transport Layer + Application-Layer Session Security** ✅ **COMPLETE**
- ✅ QUIC transport with TLS 1.3 and ALPN negotiation
- ✅ TCP transport with TLS 1.3 fallback support
- ✅ Noise IK protocol with X25519, ChaCha20-Poly1305, BLAKE3
- ✅ Ed25519 identity binding and signature verification
- ✅ Pre-Shared Key (PSK) authentication (optional)
- ✅ Token-based admission control with Ed25519 signatures
- ✅ Replay protection with sliding window mechanism
- ✅ Double encryption: TLS + Noise IK session security
- ✅ Comprehensive unit and integration testing
- ✅ Production-ready error handling and validation

## Installation

### Prerequisites

- Go 1.21 or later
- Git

### Build from Source

```bash
git clone https://github.com/WebFirstLanguage/beenet.git
cd beenet
make build
```

This creates the `bee` binary in the `build/` directory.

## Quick Start

### 1. Generate Identity

```bash
./build/bee keygen
```

This creates a new cryptographic identity and saves it to `~/.bee/identity.json` with secure permissions.

### 2. Start the Agent

```bash
./build/bee start
```

The agent will:
- Load your identity
- Print your BID (Bee ID) and handle
- Start the control API on `127.0.0.1:27777`
- Run until interrupted (Ctrl+C)

### 3. Check Status

In another terminal:

```bash
./build/bee status
```

### 4. View Your Handle

```bash
./build/bee handle
```

## CLI Commands

- `bee keygen` - Generate a new cryptographic identity
- `bee start` - Start the bee agent daemon
- `bee status` - Check if agent is running and show status
- `bee handle` - Display current BID and handle
- `bee create` - Create new swarm (not yet implemented)
- `bee version` - Show version information
- `bee help` - Show usage information

## Identity & Handles

### BID Format
```
bee:key:z6Mk<base58-encoded-public-key>
```

### Handle Format
```
<nickname>~<honeytag>
```

- **Nickname**: 3-32 characters, `[a-z0-9-]` only, NFKC normalized
- **Honeytag**: 11-character BeeQuint-32 token derived from BID (e.g., `fopeh-dojof`)

### Example
```
BID: bee:key:z6Mkec81c76396c109018ebf134404502ca7
Handle: alice~kapiz-ronit
```

## Control API

The agent exposes a local control API on `127.0.0.1:27777` using JSON over TCP.

### Available Operations

#### GetInfo
Returns current agent information:
```json
{
  "method": "GetInfo",
  "id": "request-id"
}
```

Response:
```json
{
  "id": "request-id",
  "result": {
    "bid": "bee:key:z6Mk...",
    "nickname": "alice",
    "handle": "alice~kapiz-ronit",
    "state": "running"
  }
}
```

#### SetNickname
Sets the agent's nickname:
```json
{
  "method": "SetNickname",
  "id": "request-id",
  "params": {
    "nickname": "alice"
  }
}
```

Response:
```json
{
  "id": "request-id",
  "result": {
    "nickname": "alice",
    "handle": "alice~kapiz-ronit"
  }
}
```

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run specific package tests
go test -v ./pkg/identity
go test -v ./pkg/agent
go test -v ./pkg/control

# Run transport and security tests
go test -v ./pkg/transport/...
go test -v ./pkg/security/...

# Run integration tests
go test -v ./pkg/integration
```

### Project Structure

```
beenet/
├── cmd/bee/           # CLI application
├── pkg/
│   ├── agent/         # Agent lifecycle and state management
│   ├── control/       # Control API server
│   ├── identity/      # Identity generation and management
│   ├── transport/     # Transport layer (QUIC/TCP with TLS)
│   ├── security/      # Security layer (Noise IK protocol)
│   ├── codec/         # CBOR encoding utilities
│   ├── constants/     # Protocol constants
│   └── wire/          # Wire protocol definitions
├── pkg/integration/   # Integration tests
├── docs/              # Documentation and specifications
├── build/             # Build outputs
└── Makefile          # Build automation
```

### Key Components

- **Agent**: Manages daemon lifecycle with supervisor pattern
- **Identity**: Ed25519 key generation, BID computation, honeytag derivation
- **Transport**: QUIC/TCP transports with TLS 1.3 and ALPN negotiation
- **Security**: Noise IK protocol with PSK and token-based admission control
- **Control API**: Local JSON-based API for agent interaction
- **CLI**: User-facing command-line interface

## Security

- **Identity Protection**: Keys stored in `~/.bee/identity.json` with `0600` permissions
- **Directory Security**: Identity directory created with `0700` permissions
- **Local API Only**: Control API is localhost-only by default
- **Cryptographic Primitives**: Ed25519 signatures, X25519 key exchange
- **Transport Security**: TLS 1.3 with perfect forward secrecy
- **Session Security**: Noise IK protocol with ChaCha20-Poly1305 encryption
- **Authentication**: Optional PSK and Ed25519 token-based admission control
- **Replay Protection**: Sliding window mechanism with sequence tracking
- **Double Encryption**: TLS transport + Noise application layer security

## Roadmap

### Phase 1: Minimal Agent Kernel ✅ **COMPLETE**
- [x] Daemon lifecycle management
- [x] CLI with core subcommands
- [x] Local control API
- [x] Persistent keystore
- [x] Identity and handle generation

### Phase 2: Transport Layer + Application-Layer Session Security ✅ **COMPLETE**
- [x] QUIC transport with TLS 1.3 and ALPN negotiation
- [x] TCP transport with TLS 1.3 fallback support
- [x] Noise IK protocol implementation
- [x] Ed25519 identity binding and verification
- [x] Pre-Shared Key (PSK) authentication
- [x] Token-based admission control
- [x] Replay protection and sequence tracking
- [x] Comprehensive testing and documentation

### Phase 3: DHT & Presence (Planned)
- [ ] Distributed Hash Table implementation
- [ ] Peer discovery and presence
- [ ] Network topology management
- [ ] Basic mesh connectivity

### Phase 4: Messaging (Planned)
- [ ] Direct peer messaging
- [ ] Message routing
- [ ] Delivery guarantees
- [ ] Message persistence

### Phase 5: Swarms (Planned)
- [ ] Swarm creation and management
- [ ] Group messaging
- [ ] Swarm discovery
- [ ] Access control

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Follow TDD practices - write tests first
4. Commit your changes (`git commit -m 'Add amazing feature'`)
5. Push to the branch (`git push origin feature/amazing-feature`)
6. Open a Pull Request

### Development Guidelines

- **Test-Driven Development**: All new features must have tests
- **Go Standards**: Follow standard Go conventions and formatting
- **Documentation**: Update documentation for user-facing changes
- **Commit Messages**: Use clear, descriptive commit messages

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Documentation

- **Protocol Specification**: [BeeNet Specification](docs/beenet_spec.md)
- **Phase 2 Implementation**: [Transport & Security Guide](docs/phase2-transport-security.md)
- **API Reference**: [API Documentation](docs/api-reference.md)

## Support

- **Issues**: [GitHub Issues](https://github.com/WebFirstLanguage/beenet/issues)
- **Discussions**: [GitHub Discussions](https://github.com/WebFirstLanguage/beenet/discussions)
- **Documentation**: [docs/](docs/) directory

---

**BeeNet** - Building the decentralized future, one bee at a time. 🐝
