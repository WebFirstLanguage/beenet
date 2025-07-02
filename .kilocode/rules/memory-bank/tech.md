# beenet Technology Stack

## Core Technologies

### Programming Language
- **Python 3.11+**: Modern Python with asyncio for asynchronous programming

### Cryptography
- **NoiseProtocol**: Implementation of the Noise Protocol Framework
- **PyNaCl**: Python binding to libsodium for cryptographic operations
- **Cryptography**: Python library for cryptographic primitives
- **Ed25519**: For identity keys and signatures
- **X25519**: For key exchange
- **ChaCha20-Poly1305**: For authenticated encryption
- **BLAKE2b**: For cryptographic hashing

### Networking
- **asyncio**: Core async framework for non-blocking I/O
- **Kademlia**: Distributed Hash Table for global peer discovery
- **asyncio-dgram**: Async UDP datagram support
- **PySTUN3**: STUN client for NAT traversal
- **aioice**: ICE protocol implementation for NAT traversal

### Data Processing
- **Reed-Solomon**: Error correction coding for reliable transfers

### Observability
- **structlog**: Structured logging
- **prometheus-client**: Metrics collection and exposure

## Development Tools

### Build & Dependency Management
- **Poetry**: Dependency management and packaging

### Code Quality
- **Black**: Code formatting
- **isort**: Import sorting
- **flake8**: Linting
- **mypy**: Static type checking
- **pre-commit**: Git hooks for code quality checks

### Testing
- **pytest**: Test framework
- **pytest-cov**: Coverage reporting
- **pytest-asyncio**: Async test support
- **pytest-mypy**: Type checking in tests
- **hypothesis**: Property-based testing

### Security
- **safety**: Dependency vulnerability checking
- **bandit**: Security linting

### Documentation
- **Sphinx**: Documentation generation
- **furo**: Sphinx theme
- **interrogate**: Docstring coverage checking

## Development Environment

### Requirements
- Python 3.11 or higher
- Poetry for dependency management
- Git for version control

### Setup
```bash
# Install dependencies
poetry install

# Run the demo
poetry run python scripts/beenet_demo.py
```

## Technical Constraints

- **Cross-platform compatibility**: Must work on Linux, macOS, and Windows
- **Minimal dependencies**: Core functionality should have minimal external dependencies
- **Async-first design**: All operations should be non-blocking
- **Security by default**: No insecure fallback modes
- **Bandwidth efficiency**: Optimized for various network conditions