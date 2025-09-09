# Beenet v0.1 (Draft) — Harmonized Spec (with Honeytag/1)

**Status:** Draft
**Intended Audience:** Implementers of Beenet nodes (“bees”) in desktop/server and browser runtimes.
**Normative Language:** RFC 2119 (MUST/SHOULD/MAY).

---

## 1) Scope & Goals

**Goals**

* Decentralized, self‑healing P2P mesh (“**swarm**”) of endpoints (“**bees**”).
* **No mDNS** at any stage.
* Human‑friendly unique names bound to cryptographic identities (**Honeytag**).
* End‑to‑end authenticated sessions; strong content integrity at rest/in‑flight.
* NAT traversal; LAN/WAN operation; browser compatibility.
* Minimal viable primitives; evolvable.

**Non‑goals (v0.1)**

* Global name uniqueness across unrelated swarms.
* Byzantine consensus / blockchain.
* Perfect anonymity.

---

## 2) Runtime Model — Bee Agent

A **bee** is a long‑running agent program (daemon/service) on a device.

**Behavioral summary**

* On start, the agent **attempts to connect to a configured swarm** (join mode).
  If the operator **explicitly requests a new Beenet**, the agent creates a new swarm (create mode).
* The agent can join multiple swarms concurrently (multi‑tenant runtime).
* The agent exposes a local control API/CLI to:

  * import invites, set nickname, claim/transfer bare names, list peers,
  * publish/resolve names and addresses (Honeytag), and
  * publish/fetch content by CID.

**State & persistence**

* Identity keys (Ed25519 + X25519) and profile (nickname, handle).
* Known swarms (`SwarmID`, optional `SwarmName`, bootstrap seeds, PSK/token).
* Honeytag records it owns (bare‑name lease metadata).
* Cache (DHT entries, presence/handle indexes, manifests).

**Service integration (informative)**

* Linux `systemd` unit: `beenet.service`; macOS launchd; Windows Service.
* Control socket (local): `unix:///var/run/bee.sock` or `tcp://127.0.0.1:27777` (TLS optional).

**CLI sketch (informative)**

```
# Join mode (default)
bee start --swarm <swarm-id> --seed <multiaddr> [--seed <...>] [--psk <hex> | --token <jwt>]

# Create mode (explicit)
bee create --name teamnet --seed-self --listen /ip4/0.0.0.0/udp/27487/quic

# Import invite
bee join beenet:swarm/<b32id>?seed=/ip4/203.0.113.5/udp/27487/quic&psk=<b32>

# Name ops
bee name claim brad
bee name transfer brad --to bee:key:z6Mk...
bee name delegate brad --device bee:key:z6Ms...
```

---

## 3) Core Entities & Identifiers

### 3.1 Bee Identity

* Long‑term **identity key pair** per bee:

  * **Signing:** Ed25519 (public key = `BID`).
  * **Key agreement:** X25519 (may be derived or separate).
* Canonical ID encoding: **multibase + multicodec** (e.g., base32).

  * Example: `bee:key:z6Mkj...` (Ed25519‑pub).

### 3.2 Swarm

* **SwarmID:** 128‑bit random (`base32`: `swarm:…`).
* Optional **SwarmName** label; uniqueness scoped to the admin domain that assigns it.
* A bee MAY join multiple swarms concurrently.

---

## 4) Human‑Friendly Names & Handles

### 4.1 Handle (`nickname~honeytag[@SwarmName]`)

```
handle := <nickname> "~" <honeytag> [ "@" <SwarmName> ]
```

* **nickname:** user‑chosen, NFKC‑normalized, `[a-z0-9-]`, length 3–32.
* **honeytag (token):** short pronounceable fingerprint derived from `BID`.

**BeeQuint‑32 (token) algorithm**

1. `fp32 = first 32 bits of BLAKE3(BID-bytes)`
2. Encode `fp32` as **two proquints** (`CVCVC` each), joined by `-`.

   * Consonants: `b d f g h j k l m n p r s t v z`
   * Vowels: `a e i o u`

* Example token: `mapiq-lunov`
* **Uniqueness rule:** `<nickname>~<honeytag>` MUST be unique within a swarm.
  (The honeytag ensures collision‑free, human‑memorable IDs without coordination.)

### 4.2 Bare names

* `nickname` without the honeytag is optionally **managed** by **Honeytag/1** (swarm‑scoped registry; §12).

---

## 5) Addresses & Multiaddrs

Bees advertise one or more **multiaddrs**:

* QUIC/UDP (recommended): `/ip4/203.0.113.5/udp/27487/quic`
* TCP/TLS fallback: `/ip4/203.0.113.5/tcp/27487/tls`
* WebRTC DataChannel (browser): `/webrtc-direct/certhash/<…>`
* (LAN optional) UDP “buzz” beacons: `/udp/27488/beenet-buzz`

Default ports: **27487** (QUIC+TLS), **27488** (buzz).

---

## 6) Discovery (No mDNS)

### 6.1 Bootstrap (required)

Implementations MUST accept at least one **bootstrap endpoint** (seed bee or rendezvous relay) via config or invite. Multiple seeds are RECOMMENDED.

### 6.2 DHT Rendezvous (wide‑area)

A Kademlia‑style DHT is used for rendezvous and address records.

* **Presence key:** `K_presence = H("presence" | SwarmID | BID)`
* **Topic key (pubsub/groups):** `K_topic = H("topic" | SwarmID | topic)`
* Bees **PUT** signed presence records (§14) with their multiaddrs and TTL.
* Lookup via iterative DHT **GET**. Alpha=3, bucket size K=20, 256‑bit keyspace (BLAKE3).

### 6.3 LAN Buzz (optional; still no mDNS)

Same‑L2 discovery without mDNS:

* Periodic, small **UDP broadcast** to 255.255.255.255:27488 (or IPv6 ff02::1 scope).
* Frame: `{magic:"BZZ1", swarmID, BID-hash16, epoch, sig}` (signed per §11).
* Responders unicast their signed presence.

---

## 7) NAT Traversal

* **ICE** with **STUN**; **TURN** only if policy allows.
* UDP hole‑punching preferred; QUIC over UDP.
* For browsers: standard WebRTC ICE.

---

## 8) Transport & Session Handshake

### 8.1 Transports

* **Primary:** QUIC + TLS 1.3 (X.509 optional; identity binding at app layer).
* **Fallback:** TCP + TLS 1.3.
* **Browser:** WebRTC DataChannel (DTLS/SRTP).

### 8.2 Beenet Session Handshake (Noise IK over established transport)

Performed at the application layer to bind the session to `BID` and scope to `SwarmID`.

* **Pattern:** Noise **IK** with X25519 + ChaCha20‑Poly1305 + BLAKE3.

* **ClientHello (C→S)** (CBOR):

  ```
  {
    v: 1,
    swarm: SwarmID,
    from: BID,
    nonce: u64,
    caps: [ "pubsub/1", "dht/1", "chunks/1", "honeytag/1" ],
    proof: ed25519_sig( H(v|swarm|from|nonce|caps) )
  }
  ```

* **ServerHello (S→C)** mirrors fields, includes server nonce and proof.
  On success both sides derive session keys; subsequent frames are AEAD‑sealed (double encryption over TLS is acceptable and simplifies uniform semantics across WebRTC/TLS).

* Replay prevention: nonces + session transcripts; implementations maintain last‑seen window.

---

## 9) Overlay Routing

* **Lookup & addressing:** Kademlia DHT over secure sessions.
* **Liveness:** SWIM‑style ping/indirect‑ping for failure detection (over secured channel).
* **Gossip:** Epidemic gossip for membership and topic meshes (§10).

---

## 10) PubSub (BeeGossip/1)

* Topic ID: `TopicID = base32( H("topic"|SwarmID|topic_name) )`

* **Message envelope** (CBOR):

  ```
  {
    mid: multihash(blake3-256, payload || from || seq),
    from: BID,
    seq: u64,
    ts: u64,  // ms since epoch
    topic: TopicID,
    payload: bytes,
    sig: ed25519_sig( canonical(envelope without sig) )
  }
  ```

* De‑duplication via `mid`; mesh maintained with gossip (GossipSub‑like: heartbeat, IHAVE/IWANT).

* Integrity: signature + `mid` hash.

---

## 11) Base Framing, Canonicalization & Signing

All Beenet control/app envelopes share a canonical CBOR structure and are individually signed with the sender’s Ed25519 key.

**Base frame (before transport AEAD):**

```
{
  v: 1,
  kind: u16,   // e.g., 1=PING, 2=PONG, 10=DHT_GET, 50=HONEYTAG_OP, ...
  from: BID,
  seq: u64,
  ts: u64,     // ms epoch
  body: any,   // kind-specific CBOR
  sig: ed25519_sig( canonical(v|kind|from|seq|ts|body) )
}
```

**Canonicalization:** Deterministic key order, no floating types, integer timestamps; signature over canonical bytes.

---

## 12) Honeytag/1 — Swarm‑Scoped Naming & Addressing

(*Integrated BNS; first‑class service. Replaces prior §10.*)

**Purpose**

* Human‑friendly names resolve to cryptographic identities and live addresses.
* Works over WAN/LAN via **DHT + gossip** (no mDNS).
* Eventual consistency; integrity via signatures.
* Backwards‑compatible with handles `nickname~honeytag`.

### 12.1 Terminology

* **BID** — Bee Ed25519 public key.
* **honeytag (token)** — pronounceable fingerprint from `BID` (e.g., `mapiq-lunov`).
* **handle** — `nickname~honeytag` (always resolvable).
* **bare name** — `nickname` without token (managed by Honeytag/1).
* **Honeytag** — the naming + addressing service.

### 12.2 Syntax accepted by resolvers

ABNF (lowercase, NFKC):

```
handle     = nickname "~" tag [ "@" swarmname ]
bare       = nickname [ "@" swarmname ]
nickname   = 3*32( ALPHA / DIGIT / "-" )
tag        = 5*13( ALPHA / "-" ) ; two CVCVC proquints with "-"
swarmname  = 1*32( ALPHA / DIGIT / "-" )
bid        = "bee:key:" 1*BASE32SYM
```

Resolvers MUST accept: **handle**, **bare**, or **bid**.

### 12.3 Records & Keys

All Honeytag objects are canonical CBOR, Ed25519‑signed. DHT keys are BLAKE3 of namespaced tuples to avoid cross‑swarm leakage.

**NameRecord (bare‑name ownership)**

```
NameRecord {
  v: 1,
  swarm: SwarmID,
  name: string,      // normalized nickname (bare)
  owner: BID,        // owner of the bare name
  ver: u64,          // monotonic by owner
  ts: u64,
  lease: u64,        // absolute ms epoch; <= ts + T_lease_max
  sig: ed25519_sig(owner, canonical(...))
}
```

* **Key:** `K_name = H("name" | SwarmID | name)`
* **Lease:** default `T_lease = 90d`. Refresh at ≤ 60% of lease.
* **Conflict resolution (CRDT LWW‑Register):** higher `ver`, else older `ts`, else lexicographically smaller `owner` BID.

**HandleIndex (handle → BID binding)**

```
HandleIndex {
  v: 1,
  swarm: SwarmID,
  handle: string,    // "nickname~honeytag"
  bid: BID,
  ts: u64,
  expire: u64,       // ~10–30 min
  sig: ed25519_sig(bid, canonical(...))
}
```

* **Key:** `K_handle = H("handle" | SwarmID | handle)`
* Resolvers MUST recompute honeytag(bid) and reject mismatches.

**PresenceRecord (addresses)** — see §14 (also indexed by `K_handle` for fast handle→addr).

**DelegationRecord (optional owner→device)**
Allows multiple devices to represent a bare name.

```
DelegationRecord {
  v: 1, swarm: SwarmID,
  owner: BID, device: BID,
  caps: [ "presence", "handle-index" ],
  ver: u64, ts: u64, expire: u64,
  sig_owner: ed25519_sig(owner, canonical(...))
}
```

* **Key:** `K_owner = H("owner" | SwarmID | owner)`

### 12.4 Operations (wire API)

Embedded in Base frame with `kind: 50 HONEYTAG_OP`.

```
HONEYTAG_OP {
  op: "claim" | "refresh" | "release" | "transfer" | "delegate" | "revoke" | "resolve",
  body: any
}
```

* **claim:** `{ name, ver, ts, lease }` signed by `owner`. PUT NameRecord at `K_name`.
* **refresh:** `{ name, ver:=prev+1, ts, lease }` signed by `owner`.
* **release:** `{ name, ver:=prev+1, ts, lease:=ts }` signed by `owner`.
* **transfer:** `{ name, new_owner, ver:=prev+1, ts, sig_owner, sig_new }`
* **delegate:** `{ owner, device, caps, ver, ts, expire, sig_owner }`
* **revoke:** `{ owner, device, ver, ts, sig_owner }` (treat as `expire=ts`)
* **resolve:** `{ query: string }` → returns `ResolveResult` (below).

**ResolveResult**

```
{
  kind: "handle"|"bare"|"bid",
  owner: BID,                // if known
  device?: BID,
  handle?: string,
  addrs?: [ multiaddr ],
  proof: {
    name?: NameRecord,
    handle_index?: HandleIndex,
    presence?: PresenceRecord,
    delegation?: DelegationRecord
  }
}
```

### 12.5 Resolution algorithm (deterministic)

1. **Normalize** `q` (trim, NFKC; lowercase nickname).
2. If `q` is a **BID** → fetch `PresenceRecord` at `K_presence`; return kind `"bid"`.
3. If `q` is a **handle**:

   * GET `K_handle`; verify honeytag(bid) equals suffix in handle.
   * Fetch `PresenceRecord` for that `bid`. If valid, return with addresses.
   * If missing, return device bid with empty `addrs` (offline).
4. Else treat `q` as **bare**:

   * GET `K_name`; select NameRecord via CRDT rule and lease.
   * Let `owner := record.owner`.
   * Select a **device**:

     * If a DelegationRecord exists and a delegated device has fresh presence, prefer it (optionally by `preferred_caps`); otherwise use `owner`.
   * Fetch `PresenceRecord` for chosen device. Return with proofs.
5. **Guards:** Reject presence whose `handle` token doesn’t match honeytag(BID). If conflicts exist, prefer the latest valid NameRecord; include conflicting proofs so UIs can warn.

**Caching:**
`HandleIndex`/`PresenceRecord` cached ≤ `expire`. `NameRecord` MAY be cached up to `lease` but MUST be revalidated near expiry (≤10% remaining).

---

## 13) Data Integrity & Content Addressing

### 13.1 Content IDs

* **CID:** `multihash(blake3-256, bytes)` with multicodec.
* Large data is chunked (default **1 MiB**). Each chunk has its own CID.

### 13.2 Merkle Manifests

```
Manifest {
  version: 1,
  size: u64,
  chunk_size: u32,
  chunks: [ CID ... ],
  meta: { mime?: str, name?: str }
}
```

* Manifest CID is the immutable handle for the asset.
* Receivers verify every chunk hash and recompute manifest CID.

### 13.3 Stream Integrity

* Each pubsub message is signed; optional `prev_mid` forms a per‑sender/topic hash chain to detect omission/tamper.

---

## 14) DHT Records (Presence, Provide)

**PresenceRecord**

```
PresenceRecord {
  v: 1,
  swarm: SwarmID,
  bee: BID,
  handle: string,          // MUST match honeytag(bid)
  addrs: [ multiaddr ],
  caps: [ string ],
  expire: u64,             // ms epoch
  sig: ed25519_sig( canonical(...) )
}
```

* **Keys:**
  `K_presence = H("presence" | SwarmID | BID)`
  `K_handle` (from §12.3) MAY also be provided for quicker handle lookups.

**ProvideRecord**

```
ProvideRecord {
  v: 1,
  swarm: SwarmID,
  cid: CID,
  provider: BID,
  addrs: [ multiaddr ],
  expire: u64,
  sig: ...
}
```

---

## 15) Control Message Kinds (non‑exhaustive)

* `1  PING` → `{ token: bytes(8) }`
* `2  PONG` → `{ token: bytes(8) }`
* `10 DHT_GET` → `{ key: bytes32 }`
* `11 DHT_PUT` → `{ key: bytes32, value: bytes, sig: bytes }`
* `20 ANNOUNCE_PRESENCE` → `PresenceRecord`
* `30 PUBSUB_MSG` → envelope (§10)
* `40 FETCH_CHUNK` → `{ cid: CID, offset?: u64 }`
* `41 CHUNK_DATA` → `{ cid: CID, off: u64, data: bytes }`
* `50 HONEYTAG_OP` → §12 operations

All embedded in the **Base frame** (§11).

---

## 16) Security Model

* **AuthN:** Every envelope is signed (Ed25519); session bound via Noise IK (mutual auth).
* **Confidentiality:** Transport AEAD (Noise) and/or TLS/DTLS beneath.
* **Replay protection:** Nonces, monotonically increasing `seq`, windowed de‑dupe.
* **Swarm admission (optional):**

  * **PSK:** join requires pre‑shared swarm key; include PSK MAC in handshake body.
  * **Ticket:** JWT‑like token signed by a swarm issuer (a “queen” is organizational only).
* **Rate limiting:** Per‑peer token bucket; drop unauthenticated floods.
* **Sybil resistance (optional):** proof‑of‑work on DHT writes; or stake/vouch policies.
* **Key rotation:** Support secondary keys and migration:
  `KEY_UPDATE { oldBID, newBID, cross-sig }`.

---

## 17) Error Handling

Each request MAY carry `req_id`; responses echo it.

**Error frame body**

```
{ code: u16, reason: string, retry_after?: u32 }
```

**Common codes**

* `1  INVALID_SIG`
* `2  NOT_IN_SWARM`
* `3  NO_PROVIDER`
* `4  RATE_LIMIT`
* `5  VERSION_MISMATCH`
* **Honeytag‑specific:**
  `20 NAME_NOT_FOUND` • `21 NAME_LEASE_EXPIRED` • `22 HANDLE_MISMATCH` •
  `23 NOT_OWNER` • `24 DELEGATION_MISSING`

---

## 18) Wire & Encoding Details

* **Serialization:** Canonical **CBOR** (CTAP2‑style determinism).
* **Text encodings:** UTF‑8; NFC on input; names normalized to NFKC for matching.
* **Time:** milliseconds since Unix epoch (u64).
* **Hash:** BLAKE3‑256 by default (unless otherwise stated).
* **Multiformats:** multibase + multicodec + multihash for IDs.

---

## 19) Bee Agent Lifecycle (Boot, Connect, Create)

**Boot flow (join mode — default)**

1. Load identity (`BID`), swarms, seeds, and policy (PSK/token).
2. For each configured swarm:

   * Start listeners (QUIC/TLS; optional TCP/TLS; WebRTC for browser bees).
   * Run Noise IK handshake to seeds; on first secure channel, **PUT Presence** (§14).
   * Start DHT refresh (presence TTL), gossip (pubsub), and Honeytag publishers:

     * publish `HandleIndex` (expire ≈ 2 × presence TTL),
     * maintain bare‑name lease if owned (NameRecord refresh ≤60% lease).
3. Establish target peer set by DHT rendezvous; maintain via SWIM liveness.

**Create mode (explicit)**

1. Generate `SwarmID` (128‑bit random) and optional `SwarmName`.
2. Start listeners; optionally run a local seed (rendezvous relay).
3. Generate an **invite URI** (§20) including `SwarmID`, seed multiaddrs, optional PSK/token.
4. PUT Presence; begin DHT bootstrap with self/seed.

**Offline & recovery**

* If no seeds reachable, remain **degraded**; keep LAN buzz enabled (if configured); retry back‑off.
* Maintain local operations (content store; enqueue DHT PUTs) until connectivity returns.

---

## 20) Invites & URI Formats

### 20.1 Beenet Invite URI

**Scheme:** `beenet:` (ASCII only; payloads base32/base64url as needed)

**ABNF**

```
beenet-uri = "beenet:" "swarm/" swarm_b32 [ "@" swarmname ]
             [ "?" invite_params ]
swarm_b32  = 26*BASE32 // 128-bit SwarmID
invite_params = param *( "&" param )
param = ( "seed=" maddr_encoded )
      / ( "psk=" b32 )
      / ( "token=" b64url )
      / ( "name=" swarmname )
      / ( "ttl=" 1*DIGIT )  ; seconds
```

**Example**

```
beenet:swarm/ci2dnr4g5sps3t8q8mkv7o3h5k@teamnet?
  seed=%2Fip4%2F203.0.113.5%2Fudp%2F27487%2Fquic&
  psk=fr6t6t4j8r1k5v0r8q2m&
  ttl=604800
```

### 20.2 Handle & Name URIs (optional)

* `bee://<handle>` → resolve via Honeytag/1 within the node’s active swarm.
* `bee://@<swarmname>/<handle-or-bare>`

---

## 21) Configuration Defaults

* DHT bucket size **K=20**; **alpha=3**.
* Presence TTL **10 min**; refresh at **5 min**.
* Honeytag `HandleIndex` expire ≈ **20 min**.
* Bare‑name lease **90 days**; refresh at **≤60%** of lease.
* Gossip heartbeat **1 s**; mesh degree **6–12**.
* Chunk size **1 MiB**; concurrent chunk fetch **4**.
* Max tolerated clock skew **±120 s**.
* Buzz interval **5 s**; rate‑limit responses.

---

## 22) Compliance Checklist

* [ ] No mDNS used anywhere.
* [ ] Every envelope signed with Ed25519.
* [ ] Noise IK app‑layer handshake completed before control/data exchange.
* [ ] Bee handle `<nickname>~<honeytag>` exposed in UI; unique per swarm.
* [ ] Honeytag/1 implemented: NameRecord, HandleIndex, DelegationRecord (optional), resolver (§12).
* [ ] Content addressed (BLAKE3‑256 CIDs); chunk‑level verification.
* [ ] DHT records signed; TTL and leases respected.
* [ ] NAT traversal via ICE/STUN (TURN optional).
* [ ] Canonical CBOR encoding.

---

## 23) Open Extension Points (later versions)

* **Access control lists** for topics/content.
* **Group encryption** (MLS) for multi‑party confidentiality.
* **Resource‑aware routing** (latency/bandwidth metrics).
* **Mobile sleep/awake relaying**.
* **Bridging** to IPFS/libp2p topics.

---

## 24) Developer Starter Kit (Normative Schemas & Pseudocode)

### 24.1 CDDL (wire‑level)

```
; Common
bid        = tstr           ; multibase/multicodec Ed25519-pub
swarmid    = tstr
cid        = tstr
multiaddr  = tstr
ts         = uint
u64        = uint
bytes32    = bytes .size 32

; Base frame
BaseFrame = {
  v: 1,
  kind: uint,
  from: bid,
  seq: u64,
  ts: ts,
  body: any,
  sig: bstr
}

; PresenceRecord
PresenceRecord = {
  v: 1,
  swarm: swarmid,
  bee: bid,
  handle: tstr,
  addrs: [ + multiaddr ],
  caps: [ * tstr ],
  expire: ts,
  sig: bstr
}

; ProvideRecord
ProvideRecord = {
  v: 1,
  swarm: swarmid,
  cid: cid,
  provider: bid,
  addrs: [ * multiaddr ],
  expire: ts,
  sig: bstr
}

; PubSub envelope
PubSubEnv = {
  mid: tstr,       ; multihash
  from: bid,
  seq: u64,
  ts: ts,
  topic: tstr,
  payload: bstr,
  sig: bstr
}

; Honeytag records
NameRecord = {
  v: 1, swarm: swarmid, name: tstr, owner: bid,
  ver: u64, ts: ts, lease: ts, sig: bstr
}

HandleIndex = {
  v: 1, swarm: swarmid, handle: tstr, bid: bid,
  ts: ts, expire: ts, sig: bstr
}

DelegationRecord = {
  v: 1, swarm: swarmid, owner: bid, device: bid,
  caps: [ * tstr ], ver: u64, ts: ts, expire: ts,
  sig_owner: bstr
}

; Honeytag op (body varies by op)
HoneytagOp = {
  op: tstr,  ; "claim"|"refresh"|"release"|"transfer"|"delegate"|"revoke"|"resolve"
  body: any
}
```

### 24.2 Honeytag Resolver — Reference Pseudocode

```pseudo
function resolve(q: string, swarm: SwarmID, preferred_caps=[]):
  norm = normalize(q)  // trim, NFKC, lowercase nickname
  if isBID(norm):
    pres = dht_get(K_presence(swarm, norm))
    return buildResult("bid", owner=norm, device=norm, presence=pres)

  if isHandle(norm):
    hi = dht_get(K_handle(swarm, norm))
    require hi != null && honeytag(hi.bid) == suffix(norm)
    pres = dht_get(K_presence(swarm, hi.bid))
    return buildResult("handle", owner=hi.bid, device=hi.bid, handle=norm, presence=pres, handle_index=hi)

  // bare name
  nr = dht_get(K_name(swarm, norm))
  require nr != null && lease_valid(nr)
  owner = nr.owner
  delset = dht_get_all(K_owner(swarm, owner))   // optional
  candidates = [owner] + [d.device for d in delset if capMatch(d, preferred_caps) && not expired(d)]
  preslist = [(b, dht_get(K_presence(swarm, b))) for b in candidates]
  chosen = freshest(preslist) or owner
  pres = pres_of(chosen)
  handle = pres.handle if pres else nickname~honeytag(chosen) // synthetically form handle if offline
  // Guard: handle token must match BID
  require token(handle) == honeytag(chosen)
  return {
    kind: "bare", owner: owner, device: chosen, handle: handle,
    addrs: pres.addrs if pres else [],
    proof: { name: nr, presence: pres, delegation: delegationFor(chosen, delset) }
  }
```

### 24.3 Invite Parsing — Reference Pseudocode

```pseudo
function parseInvite(uri):
  ensure scheme == "beenet" && path starts with "swarm/"
  swarm = base32decode(pathPart)
  params = parseQuery(uri)
  seeds = params["seed"][]  // multiaddr-decoded
  psk   = base32decode(params["psk"]) if present
  token = b64url(params["token"]) if present
  return { swarm, seeds, psk, token, name: params["name"], ttl: int(params["ttl"]) }
```

### 24.4 Honeytag Token Test Vectors

| BID (hex, first 16 bytes shown) | BLAKE3\[0..3] (hex) | Token (BeeQuint‑32)            |
| ------------------------------- | ------------------: | ------------------------------ |
| `ed01f3a9fc2b9c44…`             |       `a1 5c 3e 92` | `mapiq-lunov` *(example)*      |
| `12aa34bb56cc78dd…`             |       `7f 00 00 01` | `vobad-badba` *(illustrative)* |

*(Note: Implementers MUST use full BID bytes in BLAKE3; table is illustrative only.)*

### 24.5 Bee Agent Boot — Minimal Implementation Plan

1. **Identity**: generate Ed25519; derive X25519; compute honeytag; persist.
2. **Bootstrap**: read seeds; open QUIC+TLS listeners; configure ICE (STUN; TURN optional).
3. **Handshake**: Noise IK; verify `BID ↔ proof`; admit via PSK/token if configured.
4. **Presence**: DHT PUT signed `PresenceRecord`; refresh before TTL.
5. **Honeytag**: publish `HandleIndex`; implement resolver (§12.5).
6. **Discovery**: DHT GET peers; connect to N peers.
7. **Liveness**: SWIM pings; maintain peer set.
8. **PubSub**: implement `PUBSUB_MSG`, gossip mesh; sign/verify envelopes.
9. **Content**: chunking, manifest, fetch by CID; verify hashes.
10. **(Optional)** Honeytag bare names: `claim/refresh/transfer/delegate`.

---

## 25) Example Flows

### 25.1 Provision a Bee

* Generate keys → `BID`.
* Compute honeytag → `mapiq-lunov`.
* Ask user for nickname → `brad`.
* Handle: `brad~mapiq-lunov`.

### 25.2 Join a Swarm (default behavior)

* User imports invite (`SwarmID`, seed multiaddrs, optional PSK/token).
* Connect to seed → Noise IK (include `SwarmID`, PSK MAC if any).
* PUT Presence; publish HandleIndex; start gossip.

### 25.3 Create a New Beenet (explicit)

* `bee create --name teamnet --seed-self` → new `SwarmID`, start listener.
* Produce invite URI; others join with `bee join <invite>`.

### 25.4 Publish & Fetch Data

* Split file into 1 MiB chunks; compute chunk CIDs.
* Build manifest; compute manifest CID.
* Publish `ProvideRecord` for manifest CID.
* Peers fetch manifest; verify; fetch chunks; verify; reassemble.

---

### Quick FAQ

* **Why honeytag in both handle & service?**
  The **token** (short fingerprint) makes handles collision‑proof without coordination; **Honeytag/1** (the service) adds bare‑name registration & resolution with signed proofs.

* **Why double encryption (TLS + Noise)?**
  Uniform app‑layer identity binding across QUIC/TCP/WebRTC and defense‑in‑depth.

* **No mDNS?**
  Correct: discovery uses DHT + gossip + optional UDP buzz; mDNS is never used.

---

**End of Beenet v0.1 (Draft) Harmonized Spec**
