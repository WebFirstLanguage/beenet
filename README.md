# beenet
[![Coverage Status](https://codecov.io/gh/WebFirstLanguage/beenet/branch/main/graph/badge.svg)](https://codecov.io/gh/WebFirstLanguage/beenet)
[![Docstring Coverage](https://img.shields.io/badge/docstring--coverage-100%25-brightgreen.svg)](https://github.com/WebFirstLanguage/beenet)

The beenet P2P python lib with Noise XX secure channels and Merkle tree data transfer.

## Quick Start

### Installation

```bash
# Install dependencies
poetry install
```

### Running the Demo

To test the P2P file transfer capabilities:

```bash
# Run the CLI demo (must use poetry to access dependencies)
poetry run python scripts/beenet_demo.py
```

**Important**: Always use `poetry run` to execute the demo script. Running with plain `python` or `python3` will fail with dependency errors because PyNaCl and other cryptographic libraries are only available in the Poetry virtual environment.

The demo will:
- Create two peers and establish a secure connection
- Transfer a 10 MiB test file using chunked streaming
- Verify data integrity using Merkle tree cryptographic proofs
- Display transfer progress and performance metrics

## Documentation

- **[API Reference](docs/api.md)** - Complete API documentation with all classes and methods
- **[Usage Guide](docs/usage.md)** - Practical examples and production deployment patterns
- **[Development Guide](CLAUDE.md)** - Development workflow including testing, linting, and security checks

## Features

- **🔐 Secure Channels**: Noise XX protocol with mutual authentication
- **🌐 Hybrid Discovery**: Kademlia DHT for global + BeeQuiet for LAN discovery
- **🔍 Data Integrity**: BLAKE2b Merkle trees verify all transfers
- **⚡ Resumable Transfers**: State persistence for interrupted transfers
- **📡 Event-Driven**: Async event system for reactive applications
- **🔑 Key Management**: Ed25519 identity + X25519 session keys
