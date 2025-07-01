# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Beenet is a secure peer-to-peer networking library for Python that provides:
- Secure channels using Noise XX protocol for mutual authentication
- Hybrid peer discovery (Kademlia DHT + BeeQuiet LAN discovery)
- Integrity-verified data transfer using Merkle trees
- Ed25519 identity keys with rotating session keys

## Development Commands

```bash
# Installation
poetry install

# Code Quality
poetry run black beenet/ tests/              # Format code
poetry run isort beenet/ tests/              # Sort imports
poetry run flake8 beenet/ tests/             # Lint code
poetry run mypy --strict beenet/             # Type checking

# Testing
poetry run pytest tests/                      # Run all tests
poetry run pytest tests/ --cov=beenet        # Run with coverage
poetry run pytest -k "test_specific_name"    # Run specific test
poetry run pytest -m "not slow"              # Skip slow tests
poetry run pytest tests/unit/                # Unit tests only
poetry run pytest tests/integration/         # Integration tests only
poetry run pytest tests/fuzz/                # Fuzz tests only
poetry run pytest tests/property/            # Property-based tests only

# Security
poetry run safety check -i 51457             # Check dependencies
poetry run bandit -r beenet/                 # Security linting

# Documentation
poetry run interrogate beenet/ --fail-under=90   # Check docstring coverage
poetry run sphinx-build -W -b html docs/ docs/_build  # Build documentation

# Pre-commit
poetry run pre-commit run --all-files        # Run all checks

# CLI and Demo
poetry run beenet                             # Main CLI command
poetry run python scripts/beenet_demo.py     # Demo application
```

## Architecture

### Module Structure
- **`beenet/core/`**: Main orchestration (Peer class), connections, events
- **`beenet/crypto/`**: Identity management, Noise protocol wrapper, keystore
- **`beenet/discovery/`**: Kademlia DHT and BeeQuiet LAN discovery
- **`beenet/transfer/`**: File chunking, Merkle tree verification, streaming

### Key Implementation Details

**Security Model**:
- Noise XX protocol (Noise_XX_25519_ChaChaPoly_BLAKE2b) for secure channels
- Ed25519 for peer identities, ephemeral keys for sessions
- BeeQuiet uses HKDF-derived ChaCha20-Poly1305 session keys

**BeeQuiet Protocol**:
- Multicast on 239.255.7.7:7777 with magic number 0xBEEC
- Message types: WHO_IS_HERE → I_AM_HERE → HEARTBEAT/GOODBYE
- Challenge-response authentication with AEAD encryption

**Data Transfer**:
- BLAKE2b Merkle trees for integrity verification
- Resumable transfers with state persistence
- Concurrent transfer support

**Performance Requirements**:
- Tests verify 10 MiB transfers complete within 60 seconds
- 30-second heartbeat interval, 90-second peer timeout

## Testing Approach

The codebase uses multiple testing strategies:
- Unit tests for individual components
- Integration tests for peer-to-peer scenarios
- Property-based testing for Merkle tree invariants
- Fuzz testing for protocol robustness (see `tests/fuzz/`)

## Important Notes

- Always run `poetry run mypy --strict beenet/` before committing
- The CI requires 90% docstring coverage (use `interrogate --fail-under=90`)
- Security vulnerability CVE-2022-42969 is ignored (disputed, see `-i 51457`)
- Use async/await consistently throughout the codebase
- Follow the existing error hierarchy in `beenet/core/errors.py`
- Test markers available: `slow`, `integration`, `fuzz` (use `-m` to filter)
- Line length is 100 characters (Black configuration)
- New dependencies: `pystun3`, `aioice`, `reedsolo`, `structlog`, `prometheus-client`, `pytest-mypy`
- NAT traversal now supports STUN/TURN/ICE with configurable policies
- Flow control includes adaptive windowing and BDP-based throttling  
- Forward error correction uses Reed-Solomon codes with enhanced Merkle trees
- Self-healing peer reconnection with exponential backoff and scoring
- Structured logging and Prometheus metrics for comprehensive observability
- Type safety enforced with pytest-mypy in CI pipeline