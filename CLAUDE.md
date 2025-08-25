# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Beenet is a deterministic, test-driven mesh networking protocol implementation in Rust, designed for amateur radio and other resilient communication scenarios. The project strictly follows TDD principles with spec-first development.

## Common Development Commands

```bash
# Build
cargo build
cargo build --release

# Test
cargo test                    # Run all tests
cargo test test_name          # Run specific test
cargo test --lib              # Run only library tests
cargo test --doc              # Run documentation tests

# Code Quality
cargo fmt                     # Format code
cargo fmt --check            # Check formatting without changes
cargo clippy                 # Run linter
cargo clippy -- -D warnings  # Treat warnings as errors
cargo clippy --all --all-targets -- -D warnings  # ALWAYS RUN THIS COMMAND. it will find stuff you missed.
```

## Architecture & Development Approach

### Core Principles

1. **Spec → Tests → Code**: Always write tests before implementation
2. **Determinism First**: Use virtual clocks and deterministic network simulation
3. **No mDNS**: Never use multicast DNS (224.0.0.251:5353) - use HELLO beacons instead
4. **Part 97 Compliance**: Amateur radio operation is first-class with mandatory plain-text beaconing

### Planned Workspace Structure

The project will be organized as Rust workspaces:
- `bee-core`: Identity, envelopes, clocks, utilities
- `bee-sim`: Deterministic network simulator
- `bee-dht`: Distributed hash table for name resolution
- `bee-route`: Routing protocol (distance-vector)
- `bee-transport`: Fragmentation and transport
- `bee-api`: IMP-style local API
- `bee-cli`: Command-line tools

### Key Specifications

**Protocol Components** (defined in `Docs/normative_appendices.md`):
- **HELLO PDU**: Neighbor discovery with TLV fields (NodeID, BeeName, Callsign, etc.)
- **ID-BEACON PDU**: Regulatory compliance for amateur radio identification
- **NameClaim DHT Record**: Distributed name-to-NodeID binding

**Critical Constraints**:
- NodeID = SHA-256(PublicKey)
- BeeName: lowercase `[a-z0-9-]{3,32}`
- Callsign: uppercase `[A-Z0-9/-]{2,16}`
- Part 97 mode: No encryption, mandatory callsign fields, ≤10 minute ID beacons

### Testing Strategy

**Test Types Required**:
- Unit tests per module
- Property-based tests (quickcheck/proptest)
- Black-box scenario tests
- Fuzz testing for parsers
- Mutation testing for test quality

**Key Test Contracts** (from Phase 0):
- `CoreClock_advances_only_when_ticked`
- `SimNet_delivery_follows_latency_and_loss_profile`
- `No_mDNS_calls_present`
- `Fuzz_BeeEnvelope_never_panics`

## Implementation Notes

- **NEVER** import or use mDNS libraries
- **ALWAYS** use the project's virtual clock abstraction, not system time
- **STRICT** validation: Reject malformed BeeNames, invalid NodeIDs, bad signatures
- **TLV Parsing**: Treat as untrusted input with bounds checking
- **Regulatory Mode**: When bit0=1, encryption MUST be disabled, callsigns MUST be present

## Phase-Based Development

Currently in Phase 0: Foundational test harness and project scaffolding. See `Docs/phase.md` for the complete 12-phase TDD plan.