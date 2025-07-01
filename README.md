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

### Development Commands

See [CLAUDE.md](CLAUDE.md) for complete development workflow including testing, linting, and security checks.
