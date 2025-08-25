Below is a **phased, test‑driven development (TDD) plan** to build **Beenet** in Rust. It treats the spec as the source of truth, assumes **an AI will write the code**, and uses tests as contracts at every boundary. No code here—only design, artifacts, and acceptance criteria.

---

# Beenet – Phased TDD Plan (Rust)

**Guiding principles**

* **Spec → Tests → Code**: Write/lock tests before the AI produces code.
* **Determinism first**: Use simulated time and deterministically replayable network models so failures are reproducible.
* **Small interfaces, strong properties**: Prefer property‑based tests and invariants over example‑only tests.
* **Regulatory mode**: “Part 97 mode” is a first‑class configuration with its own acceptance tests; no mDNS anywhere.
* **IMP‑style API**: Treat the local API like the ARPANET IMP interface—simple datagrams + control—fully contract‑tested.

---

## Phase 0 — Foundational Test Harness & Project Scaffolding

**Objective:** Establish the test infrastructure that every later phase depends on.

**Work Products**

* **Repository layout** with Rust workspaces:

  * `bee-core` (envelope, identity, clocks, utils)
  * `bee-sim` (deterministic network simulator)
  * `bee-dht`, `bee-route`, `bee-transport`
  * `bee-api` (IMP‑style local API)
  * `bee-cli` (operator tools; used only in black‑box tests)
* **Deterministic Time & RNG**: Virtual clock and seeded PRNG exposed via traits; production code depends on these abstractions.
* **Network Simulator** (`bee-sim`):

  * Link types with pluggable MTU/latency/loss/duplication, bandwidth, duty‑cycle, and on/off patterns.
  * Topology builder (line, ring, grid, random geometric).
* **Test Types available from Day 1**

  * Unit tests (per crate)
  * Property‑based tests (quickcheck/proptest style)
  * Black‑box scenario tests (spawn multi‑node systems in‑process)
  * **Fuzzers** (message envelope, parser, DHT record)
  * **Mutation testing** (to ensure tests are meaningful)
* **CI/CD gates**: Must run unit/property/black‑box tests; deny merge on any failure. Lints and `unsafe` audit on.

**TDD Contracts (examples; write these tests now)**

* `CoreClock_advances_only_when_ticked`
* `SimNet_delivery_follows_latency_and_loss_profile`
* `No_mDNS_calls_present` (static scan: ensure no 224.0.0.251:5353, `_services._dns-sd._udp`, etc.)
* `Fuzz_BeeEnvelope_never_panics` (100k cases, limited runtime)

**Definition of Done (DoD)**

* All above tests exist and pass against stub implementations.
* Single‑command CI pipeline green.
* Reproducible builds; SBOM generated.
* No `unsafe` in core crates.

---

## Phase 1 — Identity, Names & Integrity (No Networking Yet)

**Objective:** Lock the primitives used everywhere: NodeID, BeeName, Callsign binding, integrity digests, signatures.

**Capabilities**

* **NodeID** = hash(public key).
* **BeeName** rules (3–32 chars `[a–z0–9-]`, case‑insensitive).
* **Regulatory binding**: `(Callsign, BeeName, NodeID)` association object.
* **Envelope**: Versioned header + SHA‑256/BLAKE3 digest + optional Ed25519 signature.

**TDD Contracts**

* `BeeName_validation_rejects_uppercase_and_unicode`
* `NodeID_is_stable_for_public_key_and_collision_resistant` (property)
* `Signature_verifies_and_refuses_mismatch`
* `Envelope_roundtrip_preserves_plaintext_payload` (no encoding that obscures meaning)

**DoD**

* Canonical serialization spec + golden vectors.
* Property tests: 10k random name cases; 100k signature verify inputs.
* Fuzzer for envelope parsing runs clean.

---

## Phase 2 — IMP‑Style Local API (Host Interface) **without routing**

**Objective:** Expose a local, stable contract for applications (the “IMP interface”).

**Capabilities**

* **Local datagram send/receive** with queuing & status lifecycle (`accepted → queued → sent → delivered|expired|failed`).
* **Resolve** `BeeName → NodeID` (temporary in‑proc registry; DHT comes later).
* **Admin endpoints**: set Callsign/BeeName bindings; toggle Part 97 mode.

**TDD Contracts**

* `API_send_rejects_if_name_unresolved`
* `API_status_transitions_are_linear_and_total`
* `API_part97_default_is_enabled_on_radio_profiles`
* `Contract_tests`: black‑box harness sends/receives messages using only the API; validates headers/digests are present.

**DoD**

* OpenAPI/JSON schema published; endpoints stable.
* Contract tests pass against stubbed networking.

---

## Phase 3 — Neighbor Discovery (No mDNS) & Beacons

**Objective:** Replace mDNS with **HELLO** beacons and establish neighbor tables.

**Capabilities**

* Periodic **HELLO** frames with `NodeID, BeeName, Callsign, link metrics`.
* Clear‑text **ID‑BEACON** field included per frame in Part 97 mode.
* Neighbor expiry with virtual time.

**TDD Contracts**

* `Neighbors_appear_after_3_hellos_and_persist_until_timeout`
* `IDBeacon_transmits_at_or_before_10min_interval_equivalent` (sim‑time)
* `Hello_deduplication_by_NodeID`
* `No_multicast_DNS_used_anywhere` (static and runtime probes)

**DoD**

* Black‑box 10‑node ring stabilizes neighbor tables under 1 simulated minute.
* Beacons visible in plaintext logs.

---

## Phase 4 — Overlay DHT for Names & Locators

**Objective:** Distributed, churn‑tolerant resolution (BeeName↔NodeID; NodeID↔locators; Callsign↔NodeIDs).

**Capabilities**

* Kademlia‑class DHT over Bee‑DG control channel.
* Signed **NameClaim** records with TTL & renewal; conflict policy (earliest valid claim; callsign‑tiebreaker).

**TDD Contracts**

* `DHT_put_get_roundtrip_signed_records`
* `Name_conflict_resolves_deterministically` (property: earliest/CS tie‑break)
* `DHT_survives_40_percent_node_churn_without_data_loss_beyond_TTL` (scenario)
* `Partition_brain_splits_and_heals_with_eventual_consistency`

**DoD**

* 100‑node sim with churn passes availability SLO (define % retrieval within N hops).
* All DHT records are short, signed, and validated.

---

## Phase 5 — Routing: BeeRoute (distance‑vector with feasibility)

**Objective:** Multi‑bearer, loop‑free routing suited for lossy links.

**Capabilities**

* Composite metric (ETX, airtime cost, duty‑cycle penalty, SNR).
* Sequence numbers + feasibility condition; multi‑path allowed.
* Policy hooks (rate caps; duty‑cycle; per‑bearer cost).

**TDD Contracts**

* `Convergence_no_forwarding_loops_after_failover` (inject link down; assert no packet visits same node twice)
* `Metric_prefers_lower_airtime_over_hops_when_configured`
* `Rate_caps_prevent_exceeding_duty_cycle_in_sim`
* `Blackhole_detection_triggers_recompute_within_k_intervals`

**DoD**

* Grids/rings/random topologies converge under defined sim limits; no persistent loops.
* Route changes preserve in‑flight fragment delivery guarantees (see Phase 6).

---

## Phase 6 — Transport & Fragmentation (Bee‑DG; optional Bee‑ST)

**Objective:** Reliable-enough datagrams across variable MTUs; optional stream.

**Capabilities**

* Fragmentation/reassembly with replay windows.
* Selective repeat ARQ **profile‑dependent** (conservative on radio).
* Optional FEC (outer) that doesn’t obscure meaning.

**TDD Contracts**

* `Fragments_reassemble_in_order_and_tolerate_reordering_loss_duplication`
* `Replay_window_rejects_old_or_replayed_fragments` (property over timestamps)
* `ARQ_disabled_by_default_on_radio_profiles`
* `FEC_does_not_modify_plaintext_payload_semantics`

**DoD**

* Lossy links (10–30% loss) deliver >X% of datagrams within Y retries (set SLOs).
* Fuzzers for fragment state machines run clean.

---

## Phase 7 — Part 97 Mode: Compliance Controls

**Objective:** Make compliance testable and default‑on for radio bearers.

**Capabilities**

* **Encryption OFF** in Part 97 mode; **signatures ON** for integrity only.
* **ID beaconing** enforcement (≤10 minutes and on termination).
* **Content policy hooks**: prevent business‑use tags (application‑level), third‑party routing allowlists, automatic‑control constraints.

**TDD Contracts**

* `Part97_mode_forbids_ciphertext_payloads` (attempt to enable encryption → test fails)
* `Plaintext_headers_and_callsign_always_present_in_on_air_frames`
* `Third_party_routing_respects_allowlist_rules`
* `Automatic_control_can_be_disabled_immediately` (cease‑TX switch honored)

**DoD**

* Compliance test suite is comprehensive and **blocks release** if any test fails.
* Operator checklist generated from tests (what is enforced vs. operator’s responsibility).

---

## Phase 8 — IMP‑Style API: Full Contract, Telemetry, Backpressure

**Objective:** Stabilize the application surface for real users.

**Capabilities**

* Subscriptions by `service`.
* Backpressure & fair queuing; TTL expiry; priorities.
* Observability: per‑message route trace, airtime estimate, reason codes.

**TDD Contracts**

* `Backpressure_drops_low_priority_under_sustained_overload`
* `TTL_expiry_never_delivers_after_deadline`
* `Route_trace_contains_all_forwarding_hops` (without sensitive data)

**DoD**

* Contract tests (API‑only) pass on every supported profile; OpenAPI locked and versioned.

---

## Phase 9 — Persistence, Store‑and‑Forward, and Resilience

**Objective:** Make intermittent links first‑class.

**Capabilities**

* Durable queue for delayed delivery; policy limits (bytes/time).
* Node restart persistence for identities & DHT cache.
* Opportunistic delivery over multiple bearers.

**TDD Contracts**

* `Persistent_queue_survives_crash_and_reboots_with_no_dup_delivery`
* `Opportunistic_delivery_chooses_available_bearer_without_mdns`
* `Disk_quota_limits_are_enforced_and_backpressure_signaled`

**DoD**

* Long‑haul soak tests (simulated days) complete without memory leaks or integrity failures.

---

## Phase 10 — Link Profiles & Gateways

**Objective:** Concrete profiles with realistic constraints; optional gateways.

**Profiles**

* IP/UDP (loopback, LAN/Wi‑Fi)
* Serial/TNC (AX.25 adapter)
* LoRa‑class (low MTU, strict duty‑cycle)
* (Optional) APRS/AX.25 gateway—ensuring plain‑language ID insertion

**TDD Contracts**

* `Profile_MTU_enforced_and_fragmentation_behaves` (per profile)
* `LoRa_duty_cycle_enforcement_blocks_excess_tx`
* `Gateway_inserts_callsign_and_preserves_meaning`

**DoD**

* Each profile has a numeric defaults table; profile conformance tests pass.

---

## Phase 11 — Hardening: Fuzz, Race, Chaos

**Objective:** Prove robustness.

**Activities**

* **Coverage‑guided fuzzing** of envelope/DHT/fragmentation.
* **Concurrency testing** (deterministic scheduler; Loom‑class approach).
* **Chaos scenarios** (burst loss, partitions, sequence number wrap, key rotation).

**TDD Contracts**

* `No_panics_under_fuzzer_inputs_overnight_budget`
* `Race_detector_finds_no_deadlocks_in_core_paths`
* `Key_rotation_does_not_break_existing_NameClaims`

**DoD**

* Coverage thresholds met; panics/crashes at 0 over defined fuzz budget.

---

## Phase 12 — Documentation, Operator UX, and Release Governance

**Objective:** Make the system operable and auditable.

**Artifacts**

* **Spec book** (wire formats, API, profiles, compliance appendix).
* **Operator guide** (Part 97 checklist, do/don’t, examples).
* **Threat model & safety notes** (what signatures do/don’t cover).

**TDD‑Adjacent Checks**

* **Documentation tests**: embedded examples validated by tests.
* **CLI usability tests**: black‑box “golden screen” snapshots.
* **License/attribution checks** and SBOM validation.

**Release Gates**

* All compliance tests pass.
* No mDNS import or use.
* Reproducible release artifacts signed and verified.

---

# How the AI Will Write Code (and Stay Inside the Guardrails)

**Workflow loop per task**

1. **Task card**: human (or PM agent) writes a concise goal tied to an acceptance test list.
2. **Tests first**: AI generates/updates tests only; CI runs and **confirms red**.
3. **Minimal implementation**: AI writes the smallest code to satisfy tests.
4. **Refactor with safety**: AI simplifies code while keeping tests green.
5. **Property/fuzz augmentation**: AI proposes additional properties; reviewer approves; CI runs.
6. **Review gate**: Human reviewer (or separate verifier agent) checks diffs against the spec, especially Part 97 toggles and any changes near the radio profiles.
7. **Merge only if**: All gates pass; new tests added where behavior changed.

**Automations**

* **Static scanners**: ban lists (`mdns`, multicast addresses), forbidden cfg flags in Part 97 builds.
* **Content compliance hooks**: Any attempt to add encryption in Part 97 mode triggers CI failure.
* **Prompt templates** for the AI that always include: spec excerpt, test checklist, non‑goals, and “do not use mDNS”.

---

# Cross‑Phase Test Matrix (abbreviated)

| Dimension     | Values                                                             |
| ------------- | ------------------------------------------------------------------ |
| Topology      | single, line(5), ring(10), mesh(25), random(100), partitioned      |
| Link Profiles | IP/UDP, Serial/TNC, LoRa‑class                                     |
| Conditions    | 0–30% loss, variable latency/jitter, MTU 64–1500, duty‑cycle 1–10% |
| Modes         | Part 97 ON, Part 97 OFF                                            |
| Traffic       | short control, large fragmented, mixed priority, bursty            |
| Scale         | 1–5k DHT keys, 100–1k messages/min                                 |

---

# Example Acceptance Criteria by Milestone (extract)

* **M1 (Phase 1 complete):** Given a public key, the computed NodeID is deterministic; a signed envelope verifies; 100k random malformed envelopes neither crash nor validate.
* **M3 (Discovery):** In a 10‑node ring, all nodes list 2 neighbors within 30 simulated seconds; ID beacons observed at or before the 10‑minute simulated boundary.
* **M5 (Routing):** After a single link failure, no packet revisits any node (loop) and >90% of in‑flight datagrams still deliver within the configured retry budget.
* **M7 (Part 97):** With Part 97 ON, any attempt to enable encryption fails the build; all on‑air frames include plaintext Callsign; ID beacon timer never exceeds the limit.
* **M10 (Profiles):** On LoRa profile (MTU ≤ 160), 10‑KB payloads deliver via fragmentation with \<N retransmissions on a 10% loss link while respecting duty‑cycle caps.

---

# Risk Register (with test mitigations)

* **Accidental mDNS usage** → *Mitigation:* static bans + runtime probes; test `No_multicast_traffic_emitted`.
* **Fragmentation reassembly DOS** → *Mitigation:* bounded buffers & eviction policy; fuzz + property tests on size limits.
* **Clock skew/replay issues** → *Mitigation:* virtual clocks in tests; replay window properties.
* **Part 97 edge cases** → *Mitigation:* explicit compliance tests and human review of any changes to `part97` module.
* **DHT split‑brain** → *Mitigation:* partition tests with eventual consistency assertions.
* **AI overfitting to tests** → *Mitigation:* mutation testing; periodically add adversarial tests authored by a different reviewer.

---

# What You’ll Get After Each Phase

* **Executable test suites** (unit/property/black‑box/fuzzers).
* **Traceable requirements**: every spec line references one or more tests.
* **Playbooks**: how to run the sim to reproduce a finding; operator checklists generated from compliance tests.
* **Signed artifacts**: binaries and SBOM when applicable.

---

## Immediate Next Actions

1. Approve Phase 0/1 contracts above (we’ll freeze the initial envelope & naming rules as testable artifacts).
2. I’ll draft the **HELLO** and **ID‑BEACON** TLV field lists and the **NameClaim** DHT record schema as normative appendices (so the AI has unambiguous targets for tests).
3. Stand up the CI skeleton with “red” placeholder tests so the AI can begin the **tests‑first** loop right away.

If you want, I’ll produce those normative appendices (field maps + validation rules + example golden vectors) next.
