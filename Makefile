.PHONY: all build test clean fmt lint coverage fuzz static-analysis ci

# Default target
all: fmt lint build test

# Build the project
build:
	cargo build --all

# Build release version
release:
	cargo build --all --release

# Run all tests
test:
	cargo test --all

# Run unit tests only
test-unit:
	cargo test --all --lib

# Run integration tests
test-integration:
	cargo test --all --test '*'

# Run property tests with more cases
test-property:
	PROPTEST_CASES=10000 cargo test --all

# Run fuzz tests
fuzz:
	cargo test --package bee-core --test fuzz_envelope

# Run static analysis checks
static-analysis:
	cargo test --test static_analysis

# Clean build artifacts
clean:
	cargo clean

# Format code
fmt:
	cargo fmt --all

# Check formatting
fmt-check:
	cargo fmt --all -- --check

# Run clippy linter
lint:
	cargo clippy --all --all-targets -- -D warnings

# Generate code coverage
coverage:
	cargo tarpaulin --all --out Html --output-dir target/coverage

# Run all CI checks locally
ci: fmt-check lint build test static-analysis fuzz
	@echo "All CI checks passed!"

# Install development tools
install-tools:
	cargo install cargo-tarpaulin
	cargo install cargo-audit
	cargo install cargo-sbom
	cargo install cargo-mutants

# Run mutation testing
mutants:
	cargo mutants --all

# Audit dependencies for vulnerabilities
audit:
	cargo audit

# Generate SBOM
sbom:
	cargo sbom > sbom.json

# Help target
help:
	@echo "Available targets:"
	@echo "  all              - Format, lint, build, and test"
	@echo "  build            - Build debug version"
	@echo "  release          - Build release version"
	@echo "  test             - Run all tests"
	@echo "  test-unit        - Run unit tests only"
	@echo "  test-integration - Run integration tests"
	@echo "  test-property    - Run property tests with more cases"
	@echo "  fuzz             - Run fuzz tests"
	@echo "  static-analysis  - Run static analysis checks"
	@echo "  clean            - Clean build artifacts"
	@echo "  fmt              - Format code"
	@echo "  fmt-check        - Check code formatting"
	@echo "  lint             - Run clippy linter"
	@echo "  coverage         - Generate code coverage report"
	@echo "  ci               - Run all CI checks locally"
	@echo "  install-tools    - Install development tools"
	@echo "  mutants          - Run mutation testing"
	@echo "  audit            - Audit dependencies"
	@echo "  sbom             - Generate SBOM"
	@echo "  help             - Show this help message"