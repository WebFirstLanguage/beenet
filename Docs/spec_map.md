# Beenet Specification to Package Mapping

This document maps clauses from the Beenet v0.1 specification to their corresponding Go packages and implementation status.

## Phase 0 - Project Bootstrap (Infrastructure) ✅

| Spec Section | Description | Package/File | Status | Notes |
|--------------|-------------|--------------|--------|-------|
| §18 | Wire & Encoding Details | `pkg/constants/defaults.go` | ✅ | Cross-cutting constants |
| §21 | Configuration Defaults | `pkg/constants/defaults.go` | ✅ | DHT, timing, data config |
| §11 | Base Framing, Canonicalization & Signing | `pkg/wire/frame.go` | ✅ | BaseFrame type |
| §17 | Error Handling | `pkg/wire/errors.go` | ✅ | Error codes and handling |
| §15 | Control Message Kinds | `pkg/constants/defaults.go` | ✅ | Message kind constants |
| §3.1 | Bee Identity | `pkg/identity/identity.go` | ✅ | Ed25519/X25519 keygen |
| §4.1 | Handle & BeeQuint-32 | `pkg/identity/identity.go` | ✅ | Honeytag token generator |
| §24.4 | Honeytag Token Test Vectors | `pkg/identity/identity_test.go` | ✅ | Golden test vectors |
| - | Canonical CBOR | `pkg/codec/cborcanon/` | ✅ | CTAP2-style determinism |
| - | Build System | `Makefile`, `cmd/beenet/` | ✅ | Cross-compilation ready |
| - | CI/CD Pipeline | `.github/workflows/` | ✅ | Format, lint, test, race, fuzz |

## Phase 1 - Core Protocol (Planned)

| Spec Section | Description | Package/File | Status | Notes |
|--------------|-------------|--------------|--------|-------|
| §2 | Runtime Model — Bee Agent | `internal/agent/` | 🔄 | Long-running daemon |
| §5 | Addresses & Multiaddrs | `pkg/multiaddr/` | 🔄 | QUIC/TCP/WebRTC support |
| §8 | Transport & Session Handshake | `internal/transport/` | 🔄 | Noise IK over TLS |
| §9 | Overlay Routing | `internal/routing/` | 🔄 | Kademlia DHT |
| §6 | Discovery (No mDNS) | `internal/discovery/` | 🔄 | Bootstrap + DHT rendezvous |
| §7 | NAT Traversal | `internal/nat/` | 🔄 | ICE with STUN/TURN |

## Phase 2 - DHT & Presence (Planned)

| Spec Section | Description | Package/File | Status | Notes |
|--------------|-------------|--------------|--------|-------|
| §14 | DHT Records (Presence, Provide) | `internal/dht/records.go` | 🔄 | Signed DHT records |
| §6.2 | DHT Rendezvous | `internal/dht/kademlia.go` | 🔄 | K=20, alpha=3 |
| §6.3 | LAN Buzz | `internal/discovery/buzz.go` | 🔄 | UDP broadcast discovery |

## Phase 3 - PubSub & Messaging (Planned)

| Spec Section | Description | Package/File | Status | Notes |
|--------------|-------------|--------------|--------|-------|
| §10 | PubSub (BeeGossip/1) | `internal/pubsub/` | 🔄 | Epidemic gossip |
| §13 | Data Integrity & Content Addressing | `pkg/content/` | 🔄 | CID, chunking, manifests |

## Phase 4 - Honeytag Naming Service (Planned)

| Spec Section | Description | Package/File | Status | Notes |
|--------------|-------------|--------------|--------|-------|
| §12 | Honeytag/1 — Swarm-Scoped Naming | `internal/honeytag/` | 🔄 | BNS implementation |
| §12.3 | Records & Keys | `internal/honeytag/records.go` | 🔄 | NameRecord, HandleIndex, etc. |
| §12.4 | Operations (wire API) | `internal/honeytag/ops.go` | 🔄 | claim, refresh, resolve, etc. |
| §12.5 | Resolution Algorithm | `internal/honeytag/resolver.go` | 🔄 | Deterministic resolution |

## Phase 5 - Security & Advanced Features (Planned)

| Spec Section | Description | Package/File | Status | Notes |
|--------------|-------------|--------------|--------|-------|
| §16 | Security Model | `internal/security/` | 🔄 | AuthN, replay protection |
| §19 | Bee Agent Lifecycle | `internal/agent/lifecycle.go` | 🔄 | Boot, connect, create flows |
| §20 | Invites & URI Formats | `pkg/invite/` | 🔄 | beenet:// URI parsing |
| §25 | Example Flows | `examples/` | 🔄 | Usage examples |

## Implementation Status Legend

- ✅ **Complete**: Fully implemented and tested
- 🔄 **Planned**: Scheduled for future phases
- ⚠️ **Partial**: Partially implemented
- ❌ **Blocked**: Blocked by dependencies

## Package Structure Overview

```
beenet/
├── cmd/beenet/                 # CLI application
├── pkg/                        # Public API packages
│   ├── codec/cborcanon/       # Canonical CBOR encoding
│   ├── constants/             # Cross-cutting constants
│   ├── identity/              # Identity management
│   └── wire/                  # Base framing and errors
├── internal/                   # Private implementation packages
│   ├── agent/                 # Bee agent runtime (planned)
│   ├── dht/                   # DHT implementation (planned)
│   ├── honeytag/              # Honeytag service (planned)
│   ├── network/               # Network layer (planned)
│   └── pubsub/                # PubSub implementation (planned)
├── docs/                      # Documentation
├── .github/workflows/         # CI/CD pipelines
└── Makefile                   # Build system
```

## Testing Strategy

### Golden Tests (Phase 0) ✅

- **Canonical CBOR determinism**: `pkg/codec/cborcanon/canonical_test.go`
- **Ed25519 signatures**: `pkg/wire/frame_test.go`
- **Honeytag token vectors**: `pkg/identity/identity_test.go`

### Integration Tests (Future Phases)

- **End-to-end swarm creation and joining**
- **DHT record storage and retrieval**
- **PubSub message propagation**
- **Honeytag name resolution**

### Fuzz Tests

- **CBOR canonicalization**: Ensures deterministic encoding
- **Frame parsing**: Tests wire protocol robustness
- **Identity generation**: Validates key generation

## Build Targets

### Supported Platforms

- **Linux**: amd64, arm64
- **macOS**: amd64, arm64 (Apple Silicon)
- **Windows**: amd64, arm64

### Build Commands

```bash
# Development build
make dev

# Cross-compile for all platforms
make cross-compile

# Create release archives
make release

# Run all CI checks
make ci
```

## Compliance Checklist (§22)

Phase 0 establishes the foundation for compliance:

- ✅ Canonical CBOR encoding implemented
- ✅ Every envelope signed with Ed25519 (BaseFrame)
- ✅ Bee handle `<nickname>~<honeytag>` format implemented
- ✅ Content addressed (BLAKE3-256 CIDs) - foundation ready
- ✅ DHT records structure defined for signing
- ✅ Cross-platform reproducible builds

Future phases will complete the remaining compliance requirements.

## Development Workflow

1. **Phase 0** (Current): Infrastructure and core types ✅
2. **Phase 1**: Transport and session establishment
3. **Phase 2**: DHT and presence management
4. **Phase 3**: PubSub and content distribution
5. **Phase 4**: Honeytag naming service
6. **Phase 5**: Security hardening and optimization

Each phase builds upon the previous, ensuring a solid foundation for the complete Beenet implementation.
