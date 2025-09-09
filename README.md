# BeeNet

A decentralized peer-to-peer mesh network implementation with cryptographic identities and human-readable handles.

## Overview

BeeNet is a P2P mesh network that uses Ed25519/X25519 cryptographic identities with a unique "honeytag" system for human-readable identifiers. Each node has a handle in the format `<nickname>~<honeytag>` where the honeytag is derived from the node's cryptographic identity.

## Current Status: Phase 1 Complete ✅

**Phase 1 - Minimal Agent Kernel** has been fully implemented with:

- ✅ Daemon lifecycle management with start/stop/retry supervisors
- ✅ CLI with `start`, `create`, `status`, `keygen`, `handle` subcommands
- ✅ Local control API (JSON over TCP)
- ✅ Persistent keystore with secure file permissions
- ✅ Nickname normalization and validation per specification
- ✅ Identity generation and handle computation
- ✅ Comprehensive test coverage

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
```

### Project Structure

```
beenet/
├── cmd/bee/           # CLI application
├── pkg/
│   ├── agent/         # Agent lifecycle and state management
│   ├── control/       # Control API server
│   ├── identity/      # Identity generation and management
│   ├── codec/         # CBOR encoding utilities
│   ├── constants/     # Protocol constants
│   └── wire/          # Wire protocol definitions
├── docs/              # Documentation and specifications
├── build/             # Build outputs
└── Makefile          # Build automation
```

### Key Components

- **Agent**: Manages daemon lifecycle with supervisor pattern
- **Identity**: Ed25519 key generation, BID computation, honeytag derivation
- **Control API**: Local JSON-based API for agent interaction
- **CLI**: User-facing command-line interface

## Security

- Identity keys stored in `~/.bee/identity.json` with `0600` permissions
- Identity directory created with `0700` permissions
- No network exposure by default (control API is localhost-only)
- Cryptographic identities use Ed25519 signatures and X25519 key exchange

## Roadmap

### Phase 1: Minimal Agent Kernel ✅ **COMPLETE**
- [x] Daemon lifecycle management
- [x] CLI with core subcommands
- [x] Local control API
- [x] Persistent keystore
- [x] Identity and handle generation

### Phase 2: DHT & Presence (Planned)
- [ ] Distributed Hash Table implementation
- [ ] Peer discovery and presence
- [ ] Network topology management
- [ ] Basic mesh connectivity

### Phase 3: Messaging (Planned)
- [ ] Direct peer messaging
- [ ] Message routing
- [ ] Delivery guarantees
- [ ] Message persistence

### Phase 4: Swarms (Planned)
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

## Specification

For detailed protocol specifications, see the [BeeNet Specification](docs/beenet_spec.md).

## Support

- **Issues**: [GitHub Issues](https://github.com/WebFirstLanguage/beenet/issues)
- **Discussions**: [GitHub Discussions](https://github.com/WebFirstLanguage/beenet/discussions)
- **Documentation**: [docs/](docs/) directory

---

**BeeNet** - Building the decentralized future, one bee at a time. 🐝
