# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial beenet P2P networking library implementation
- Noise XX secure channels with mutual authentication and forward secrecy
- Hybrid discovery system with Kademlia DHT and BeeQuiet LAN protocol
- AEAD-wrapped BeeQuiet payloads with ChaCha20-Poly1305 encryption
- Merkle tree-based data transfer with integrity verification
- Ed25519 identity keys with rotating X25519 session keys
- Secure keystore abstraction with file-based and OS keyring support
- Async API design for all network-facing operations
- Comprehensive test suite with unit, property-based, fuzz, and integration tests
- Pre-commit hooks for code quality (black, isort, flake8, mypy)
- CI/CD workflow with coverage reporting and documentation builds
- NAT traversal feature flag stub for future STUN/TURN support

### Security
- All peer traffic encrypted with Noise XX protocol
- BeeQuiet discovery messages authenticated with AEAD
- Secure key rotation with proper revocation handling
- Fuzz testing for all untrusted input parsers

## [0.1.0] - TBD

### Added
- First working prototype of beenet P2P library
- Core cryptographic components with Noise XX implementation
- Discovery layer with Kademlia DHT and BeeQuiet protocol
- Data transfer layer with chunking and Merkle verification
- Comprehensive documentation and API reference
- CLI demo for file transfer between peers

[Unreleased]: https://github.com/WebFirstLanguage/beenet/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/WebFirstLanguage/beenet/releases/tag/v0.1.0
