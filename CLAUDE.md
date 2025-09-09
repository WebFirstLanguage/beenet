# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

BeeNet is a decentralized peer-to-peer mesh network with cryptographic identities and human-readable handles. This is a Go codebase implementing secure transport layers, content addressing, and distributed hash table functionality.

## Essential Development Commands

### Primary Development Workflow
```bash
make dev          # Clean, deps, check, test, build (recommended for development)
make build        # Build bee binary
make test         # Run all tests
make test-coverage # Test with coverage report
```

### Code Quality & CI
```bash
make ci           # Full CI pipeline (check, test, race)
make fmt          # Format code
make lint         # golangci-lint
make vet          # Go vet
make race         # Race condition detection
make security     # gosec security scan
```

### Testing
```bash
make fuzz         # Fuzz testing (codec, wire, identity)
make golden       # Run golden test vectors
make bench        # Benchmark tests
```

### CLI Usage
```bash
go run cmd/bee/main.go keygen   # Generate identity
go run cmd/bee/main.go start    # Start agent daemon
go run cmd/bee/main.go status   # Check agent status
```

## Architecture Overview

### Core Components

- **Identity System** (`pkg/identity/`): Ed25519/X25519 cryptographic identities with BID format (`bee:key:z6Mk<base58>`) and human-readable handles (`<nickname>~<honeytag>`)
- **Agent System** (`pkg/agent/`): Daemon lifecycle management with supervisor pattern and state management
- **Transport Layer** (`pkg/transport/`): Dual QUIC/TCP transport with TLS 1.3 and ALPN negotiation
- **Security Layer** (`pkg/security/noiseik/`): Noise IK protocol with ChaCha20-Poly1305, replay protection, and identity binding
- **Content Addressing** (`pkg/content/`): CID system with BLAKE3-256, chunking, and integrity verification
- **DHT** (`internal/dht/`): Kademlia DHT implementation for peer discovery

### Package Structure

- `pkg/`: Public APIs and core functionality
- `internal/`: Private implementation details
- `cmd/bee/`: CLI interface and main entry point

## Development Requirements

### Test-Driven Development
This codebase follows strict TDD (see `.augment/rules/TDD.md`). Always write failing tests first, then implement functionality.

### Code Quality Standards
- Go 1.24.0 required
- 120-character line limit
- Comprehensive linting via `.golangci.yml`
- Function complexity limits (100 lines, 50 statements)
- Security scanning with gosec

### Testing Strategy
- Golden tests for canonical formats (CBOR, Ed25519, honeytag vectors)
- Integration tests for cross-component functionality  
- Fuzz testing for critical packages (codec, wire, identity)
- Race condition detection required for concurrency

## Key Implementation Patterns

### Error Handling
Centralized error types in `pkg/wire/errors.go` with comprehensive error classification.

### Constants & Configuration  
Centralized constants in `pkg/constants/defaults.go` for system-wide defaults.

### Cryptographic Operations
- Ed25519 for signing, X25519 for key agreement
- BLAKE3 for hashing and content addressing
- Noise IK protocol for application-layer security
- TLS 1.3 for transport-layer security

### Network Protocols
- QUIC primary transport, TCP fallback
- Double encryption: TLS transport + Noise application layer
- ALPN negotiation for protocol selection
- Kademlia DHT with 256 buckets, K=20, alpha=3

## Platform Support

Cross-platform builds supported for:
- Linux (amd64, arm64)
- macOS (amd64, arm64)  
- Windows (amd64, arm64)

Use `make cross-compile` for building all platforms.

## CI/CD Integration

GitHub Actions workflow (`.github/workflows/ci.yml`) runs:
- Code formatting and linting checks
- Unit tests and race detection
- Cross-platform builds
- Security scanning
- Coverage reporting via Codecov

Always run `make ci` before committing to ensure all checks pass.