# Beenet – Phased TDD Plan (Rust)

**Guiding principles**

* **Spec → Tests → Code**: Write/lock tests before the AI produces code.
* **Determinism first**: Use simulated time and deterministically replayable network models so failures are reproducible.
* **Small interfaces, strong properties**: Prefer property‑based tests and invariants over example‑only tests.
* **Regulatory mode**: “Part 97 mode” is a first‑class configuration with its own acceptance tests; no mDNS anywhere.
* **IMP‑style API**: Treat the local API like the ARPANET IMP interface—simple datagrams + control—fully contract‑tested.

