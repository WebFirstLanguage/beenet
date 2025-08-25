Below are **normative appendices** for Beenet v0.1 that define (1) the **HELLO** and **ID‑BEACON** management PDUs and their **TLV** fields, and (2) the **NameClaim** DHT record schema. These are written so an AI can implement tests *first* with no ambiguity.

> **Notation:**
>
> * **MUST/SHOULD/MAY** are as per RFC 2119.
> * All integers are **unsigned big‑endian** unless stated.
> * **ASCII** means 7‑bit printable characters (0x20–0x7E) unless stricter rules are specified.
> * **TLV** = **T**ype (1 byte), **L**ength (2 bytes), **V**alue (L bytes). Multiple TLVs concatenate to form a **TLV Block**. Types 0x00 and 0xFF are reserved.
> * “Profile: Radio (Part 97)” refers to operation on amateur bands; encryption is disabled; callsign fields are required; identification intervals are enforced.
> * PDU headers are defined elsewhere; this appendix standardizes **payload TLVs** for the two management PDUs and the DHT record schema.

---

## Appendix A — BeeNet Management TLV Registry (initial assignments)

This registry governs TLV **Type** codes used inside HELLO and ID‑BEACON PDUs. Length is always a 16‑bit byte count; a TLV MAY appear at most once unless marked **repeating**.

| Type | Name               | Value format                                                                                  | Presence                                                                                    |
| ---: | ------------------ | --------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| 0x01 | **NodeID**         | 32 bytes (SHA‑256 of the node’s public key)                                                   | **MUST** (HELLO)                                                                            |
| 0x02 | **BeeName**        | ASCII lowercase `[a‑z0‑9-]{3,32}`                                                             | **MUST** (HELLO), **SHOULD** (ID‑BEACON)                                                    |
| 0x03 | **SwarmID**        | 8‑byte opaque identifier                                                                      | **MUST** (HELLO, ID‑BEACON)                                                                 |
| 0x04 | **Callsign**       | ASCII uppercase `[A‑Z0-9\/\-]{2,16}` (region‑agnostic)                                        | **MUST** (HELLO in Radio/Part 97), **MUST** (ID‑BEACON in Radio/Part 97), **MAY** otherwise |
| 0x05 | **HelloSeq**       | 8 bytes: `boot_id` (u32 random) + `seq` (u32 monotonic)                                       | **MUST** (HELLO)                                                                            |
| 0x06 | **Capabilities**   | 4‑byte bitset (see Table A‑1)                                                                 | **SHOULD** (HELLO)                                                                          |
| 0x07 | **LinkProfile**    | 10 bytes: `profile_id`(u16), `mtu`(u16), `bitrate_bps`(u32), `duty_centi%`(u16)               | **SHOULD** (HELLO on radio)                                                                 |
| 0x08 | **LinkMetrics**    | 5 bytes: `etx_q8_8`(u16), `rssi_dbm`(i8), `snr_db`(i8), `lq%`(u8)                             | **SHOULD** (HELLO)                                                                          |
| 0x09 | **Timestamp**      | 8‑byte ms since Unix epoch                                                                    | **MUST** (HELLO, ID‑BEACON)                                                                 |
| 0x0A | **PublicKey**      | 32 bytes (Ed25519)                                                                            | **MUST** (HELLO), **SHOULD** (ID‑BEACON)                                                    |
| 0x0B | **Signature**      | 64 bytes (Ed25519 over canonical SigInput; see §A.4)                                          | **SHOULD** (HELLO, ID‑BEACON)                                                               |
| 0x0C | **RegulatoryMode** | 1‑byte bitset; bit0=Part97 ON(1)/OFF(0); other bits reserved(0)                               | **MUST** (HELLO)                                                                            |
| 0x10 | **IPv4UDP**        | 6 bytes: IPv4(4) + UDP port(u16) — **repeating**                                              | **MAY** (HELLO)                                                                             |
| 0x11 | **IPv6UDP**        | 18 bytes: IPv6(16) + UDP port(u16) — **repeating**                                            | **MAY** (HELLO)                                                                             |
| 0x12 | **AX25Path**       | ASCII up to 64 bytes (e.g., `N0CALL-10,WIDE1-1,WIDE2-1`) — **repeating**                      | **MAY** (HELLO)                                                                             |
| 0x13 | **LoRaParams**     | 12 bytes: `freq_hz`(u32), `bw_hz`(u24), `sf`(u8), `cr4/x`(u8), `tx_dBm`(i8), `rx_gain_dB`(i8) | **MAY** (HELLO)                                                                             |
| 0x14 | **PowerStatus**    | 2 bytes: `battery_%`(u8), `external_power`(u8: 0/1/2=off/on/charging)                         | **MAY** (HELLO)                                                                             |
| 0x15 | **Roles**          | 2‑byte bitset: bit0=DHT server, 1=gateway(APRS), 2=internet bridge, etc.                      | **MAY** (HELLO)                                                                             |
| 0x20 | **IDReason**       | 1 byte enum: 0=Periodic(≤10 min), 1=End‑of‑contact, 2=Operator, 3=Join                        | **MUST** (ID‑BEACON)                                                                        |
| 0x21 | **IDText**         | ASCII free text 1–160 bytes, plain language                                                   | **SHOULD** (ID‑BEACON in Radio/Part 97)                                                     |
| 0x22 | **RegionHint**     | 3 bytes: `country` ISO‑3166‑1 alpha‑2 (2 ASCII) + `service_class`(u8)                         | **MAY** (ID‑BEACON)                                                                         |

**Table A‑1 — Capabilities bitset (0x06)**
`bit0=Bee‑DG`, `bit1=Bee‑ST`, `bit2=DHT`, `bit3=Routing`, `bit4=FEC`, `bit5=ARQ`, `bit6=Gateway(APRS)`, `bit7=Store‑and‑Forward`; bits 8–31 reserved 0.

### A.1 TLV Encoding and Ordering

* TLVs **MUST** be encoded as `(Type:u8, Length:u16, Value:Length bytes)`.
* Within a TLV block, fields **MUST** be ordered by **Type ascending**. Receivers **MUST** ignore unknown Types and continue.
* A TLV marked **repeating** MAY appear multiple times; all others MUST appear at most once.

### A.2 String Rules

* **BeeName (0x02):** lowercase; regex `^[a-z0-9-]{3,32}$`; comparisons are case‑insensitive (normalize to lowercase).
* **Callsign (0x04):** upper‑case; regex `^[A-Z0-9/-]{2,16}$`; receivers MUST NOT alter case for comparison.
* **IDText (0x21):** plain language, printable ASCII; MUST NOT be encrypted or base64‑encoded on radio profiles.

### A.3 Regulatory Mode (0x0C)

* In Radio/Part 97 operation, `RegulatoryMode.bit0` **MUST** be `1`. When set:

  * Payload encryption **MUST** be disabled at all layers.
  * **Callsign (0x04)** **MUST** be present in both HELLO and ID‑BEACON.
  * **ID‑BEACON** frames **MUST** be emitted at ≤10‑minute intervals and at end of communications.
* In non‑Part 97 operation, implementers MAY set bit0=0 and enable encryption in other PDUs (not defined here).

### A.4 Signature (0x0B) Coverage

* Algorithm: **Ed25519** over **SigInput**.
* **SigInput** = `ContextString || 0x00 || SwarmID(8) || NodeID(32 if present) || CanonicalTLVs`, where:

  * `ContextString` = `"BeeNet-HELLO-v1"` for HELLO PDUs and `"BeeNet-IDBEACON-v1"` for ID‑BEACON PDUs (ASCII).
  * `CanonicalTLVs` = concatenation of all TLVs in ascending **Type** order **excluding** 0x0B (**Signature**) itself.
* **PublicKey (0x0A)** MUST be the verifying key for the signature.
* Receivers **MUST** reject signatures if **PublicKey** is absent or length ≠ 32.

---

## Appendix B — HELLO PDU (Type: Management/HELLO)

**Purpose:** Neighbor discovery, capability advertisement, on‑air identification assistance, and link metrics exchange without mDNS.

### B.1 Required TLVs (HELLO)

* **NodeID (0x01)** — **MUST**
* **BeeName (0x02)** — **MUST**
* **SwarmID (0x03)** — **MUST**
* **HelloSeq (0x05)** — **MUST**
* **Timestamp (0x09)** — **MUST**
* **PublicKey (0x0A)** — **MUST**
* **RegulatoryMode (0x0C)** — **MUST**
* **Callsign (0x04)** — **MUST** when `RegulatoryMode.bit0==1`; **MAY** otherwise

### B.2 Recommended TLVs (HELLO)

* **Capabilities (0x06), LinkProfile (0x07), LinkMetrics (0x08)** — **SHOULD**
* One or more **locator** TLVs (**IPv4UDP 0x10**, **IPv6UDP 0x11**, **AX25Path 0x12**, **LoRaParams 0x13**) — **SHOULD** as applicable
* **Signature (0x0B)** — **SHOULD** to authenticate origin without obscuring meaning
* **Roles (0x15), PowerStatus (0x14)** — **MAY**

### B.3 Semantics

* **Emission cadence:** Each Bee **SHOULD** emit HELLO every 15–60 s per bearer; receivers expire neighbors after 3× the advertised cadence or 180 s, whichever is smaller (profile may override).
* **Deduplication:** A HELLO is considered a duplicate if the tuple `(SwarmID, NodeID, HelloSeq.boot_id, HelloSeq.seq)` has been seen.
* **Neighbor table:** Upon valid HELLO, update/insert neighbor with the most recent **HelloSeq.seq** and **Timestamp**.
* **On‑air identification:** When in Radio/Part 97 mode, HELLO **MUST** include a plain‑text **Callsign** field; this may count toward station ID frequency if emitted at or under the required interval (the **ID‑BEACON** remains the canonical mechanism).

### B.4 Validation Rules (testable)

* Reject HELLO if **BeeName** violates regex or if **NodeID** ≠ `SHA‑256(PublicKey)`.
* If `RegulatoryMode.bit0==1`, reject HELLO lacking **Callsign**.
* Ignore TLVs with unknown types; do not fail the frame.
* If **Signature (0x0B)** present, it **MUST** verify; otherwise ignore frame. (Profile option: accept unsigned HELLO but mark “unauthenticated”.)

---

## Appendix C — ID‑BEACON PDU (Type: Management/ID‑BEACON)

**Purpose:** Satisfy on‑air identification requirements by transmitting plain‑language station information at or below the prescribed interval and at the end of a contact. It is intentionally small and human‑intelligible.

### C.1 Required TLVs (ID‑BEACON)

* **SwarmID (0x03)** — **MUST**
* **Timestamp (0x09)** — **MUST**
* **IDReason (0x20)** — **MUST**
* **Callsign (0x04)** — **MUST** when `RegulatoryMode.bit0==1`; **MAY** otherwise

### C.2 Recommended TLVs (ID‑BEACON)

* **BeeName (0x02)** — **SHOULD**
* **PublicKey (0x0A)** and **Signature (0x0B)** — **SHOULD** (origin authentication)
* **IDText (0x21)** — **SHOULD** in Radio/Part 97 to include plain language message (e.g., `"ID de K7XYZ Beenet hello"`).
* **RegionHint (0x22)** — **MAY** (country/service class)

### C.3 Emission Rules (testable)

* In Radio/Part 97 mode, each active transmitter **MUST** emit an ID‑BEACON **at least every 10 minutes** during communications and **at the end** of communications.
* **IDText** MUST be **plain language**; implementers **MUST NOT** encrypt or encode it in a way that obscures meaning (e.g., no base64 ciphertext).
* Receivers **MUST** accept multiple ID‑BEACONs and update the last‑seen‑ID timer per neighbor.

### C.4 Validation Rules

* If `RegulatoryMode.bit0==1`, reject ID‑BEACON lacking **Callsign** or containing non‑printable characters in **IDText**.
* If **Signature** present, it **MUST** verify as per §A.4.

---

## Appendix D — NameClaim DHT Record (BeeName ↔ NodeID binding)

**Purpose:** Bind a human‑friendly **BeeName** to a specific **NodeID** (and optionally to a **Callsign**) within a **Swarm**, allowing distributed resolution without mDNS.

### D.1 Record Key (DHT)

* **Primary index (by name):** `key = SHA‑256( lowercase(BeeName) || 0x00 || SwarmID(8) || "NameClaim" )`
* **Secondary index (by callsign, optional):** `key = SHA‑256( uppercase(Callsign) || 0x00 || SwarmID(8) || "CallClaim" )`
* DHT storage is **(key → serialized record bytes)** with TTL.

### D.2 Canonical Serialization

* **Canonical CBOR (CTAP2 deterministic)** **MUST** be used for the record body **before** signature.
* Map keys are the following short ASCII strings; order is lexicographic by key for canonicalization.

### D.3 Fields (all lengths in bytes)

| Key     | Name            | Type            | Constraints                        |
| ------- | --------------- | --------------- | ---------------------------------- |
| `"v"`   | **version**     | u8              | **MUST** be `1`                    |
| `"sid"` | **swarm\_id**   | bytes(8)        | **MUST** match local SwarmID       |
| `"bn"`  | **beename**     | text            | lowercase `[a‑z0-9-]{3,32}`        |
| `"nid"` | **node\_id**    | bytes(32)       | SHA‑256 of public key              |
| `"cs"`  | **callsign**    | text (optional) | uppercase `[A‑Z0-9/-]{2,16}`       |
| `"nb"`  | **not\_before** | u64 (ms epoch)  | start of validity                  |
| `"na"`  | **not\_after**  | u64 (ms epoch)  | end of validity (TTL)              |
| `"sq"`  | **seq**         | u32             | monotonic per (beename, node\_id)  |
| `"pk"`  | **pubkey**      | bytes(32)       | Ed25519 verifying key              |
| `"alg"` | **sig\_alg**    | u16             | `0x01` = Ed25519                   |
| `"sig"` | **signature**   | bytes(64)       | Signature over **SigBase** (below) |

**SigBase** (bytes to sign) = `Context || 0x00 || canonical_cbor_without("sig")`, where `Context = "BeeNet-NameClaim-v1"` (ASCII).
The **signature** covers all fields except `"sig"` itself.

### D.4 Record Semantics

* **Issuer:** The Bee that owns `pubkey` signs the record.
* **Validity:** A record is **valid** if signature verifies, current time ∈ \[`not_before`, `not_after`], and fields meet constraints.
* **TTL guidance:** `not_after - not_before` SHOULD be 60–180 minutes. Renew before expiry with incremented `"sq"`.
* **Conflict resolution (deterministic):** Given two **valid** NameClaims for the **same** `(beename, swarm_id)`:

  1. Prefer the record whose `"cs"` (callsign) is **present** when operating in Radio/Part 97 swarms and the local policy requires callsigns.
  2. Else prefer the one with **earlier `not_before`**.
  3. If equal, prefer **lexicographically smaller `node_id`** (tie‑breaker).
  4. If still equal, prefer lexicographically smaller `pubkey`.
* **Revocation:** To revoke, publish a new NameClaim with the same `"bn"` and `"nid"` where `"na" = nb"` (immediate expiry) and `"sq"` incremented; peers SHOULD purge on receipt.

### D.5 Validation Rules (testable)

* Reject records with invalid regex for `"bn"`/`"cs"`.
* Reject if `"nid"` ≠ `SHA‑256("pk")`.
* Reject if `"v" != 1` or `"alg" != 0x01`.
* Reject if signature fails or time is outside validity.
* When inserting into the DHT cache, apply conflict rules deterministically.

---

## Appendix E — Testable Examples & Golden‑Vector Guidance

> These examples define **exact byte layouts** so the AI can write tests before code. (Hex is uppercase, spaces for readability; **do not** include spaces on wire.)

### E.1 Example HELLO (minimal, unsigned) — TLV block only

```
# Bee: beename "honey-1", swarm_id 0x1122334455667788, callsign "K7TEST"
# PublicKey: 32 bytes 0x010203...20 (example placeholder; not a real key)
# NodeID = SHA-256(PublicKey) = 32 bytes 0xAA.. (compute in tests)

02 0007 68 6F 6E 65 79 2D 31               # BeeName "honey-1"
03 0008 11 22 33 44 55 66 77 88             # SwarmID
01 0020 AA AA AA AA ... (32 bytes)          # NodeID (hash of pk)
0A 0020 01 02 03 ... 20                     # PublicKey (32 bytes)
05 0008 00 00 12 34  00 00 00 01            # HelloSeq: boot_id=0x1234, seq=1
09 0008 00 00 01 97  6A 2B 3C D0            # Timestamp (ms)
0C 0001 01                                   # RegulatoryMode: Part97=ON
04 0006 4B 37 54 45 53 54                   # Callsign "K7TEST"
```

**Tests:**

* Verify BeeName regex passes; Callsign required since RegMode=ON.
* Verify `NodeID == SHA‑256(PublicKey)`.

### E.2 Example ID‑BEACON (with IDText, unsigned) — TLV block

```
03 0008 11 22 33 44 55 66 77 88             # SwarmID
09 0008 00 00 01 97  6A 2B 3C D0            # Timestamp
20 0001 00                                   # IDReason: Periodic
04 0006 4B 37 54 45 53 54                   # Callsign "K7TEST"
02 0007 68 6F 6E 65 79 2D 31                # BeeName "honey-1"
21 0016 49 44 20 64 65 20 4B 37 54 45 53 54 20 42 65 65 6E 65 74 0A
                                             # "ID de K7TEST Beenet\n"
```

**Tests:**

* With `RegulatoryMode.bit0==1`, ensure Callsign present and IDText plain ASCII.

### E.3 Signature Inputs (for HELLO and ID‑BEACON)

* Build **SigInput** per §A.4 using the **sorted TLVs** (excluding 0x0B Signature).
* Ed25519‑sign and place the 64‑byte signature into TLV **0x0B**.
* **Tests:** verify signature acceptance with the provided **PublicKey (0x0A)**; rejection if any signed TLV mutates.

> *Note:* If you need fully computed golden signatures, anchor on the above layouts and generate keys deterministically from a fixed seed (e.g., RFC 8032 test vectors). Use those in unit tests for signature verification.

### E.4 Example NameClaim (CBOR diagnostic form)

```
{
  "v": 1,
  "sid": h'1122334455667788',
  "bn": "honey-1",
  "nid": h'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA',
  "cs": "K7TEST",
  "nb": 1754000000000,
  "na": 1754003600000,
  "sq": 1,
  "pk": h'010203...20',
  "alg": 1,
  "sig": h'...64 bytes...'
}
```

**Tests:**

* Parse with canonical CBOR; assert `"nid" == SHA‑256("pk")`.
* For conflict resolution, craft two valid records and assert the deterministic winner per §D.4.

---

## Appendix F — Compliance‑Oriented Test Checklist (extract)

1. **HELLO missing Callsign with Part97=ON →** reject frame.
2. **ID‑BEACON periodicity:** in sim‑time, ensure beacons emitted ≤10 min apart and on explicit “session end” event.
3. **No obscuring:** attempts to place non‑ASCII or ciphertext in **IDText** on radio profiles → reject.
4. **Signature admission:** with Signature present but invalid → reject frame; without Signature → accept but mark “unauthenticated.”
5. **DHT NameClaim:** invalid regex / bad signature / expired window → reject; valid conflicts resolve deterministically.

---

### Implementation Notes for the AI (scope/constraints)

* **Do not** implement or import mDNS. Tests **must** fail any attempt to open 224.0.0.251:5353 or `_services._dns-sd._udp`.
* Treat **TLV parsing** as untrusted input: bounds‑check Length; cap total TLV block size to profile MTU.
* **Strict normalization:** Lowercase BeeName at parse time; uppercase Callsign. Reject if normalization changes content (e.g., non‑ASCII).
* **Clocking:** Use the project’s virtual clock; do not call wall‑clock in tests.

---

If you want, I can now produce a **one‑page conformance table** (fields × PDUs × “Required/Optional/Forbidden”) and a **ready‑to‑paste test plan** (names of unit/property/black‑box tests matching the above).
