---
id: R-078
title: What canonical v1 Route wire and conformance boundary permits independent implementations without retaining H3 bytes or allowing a Node-selected downgrade?
status: accepted
owner: Product Owner and Codex
started: 2026-08-23
reviewed: 2026-08-23
---

# R-078 — Interactive Route v1 wire

## Decision this unlocks

Give M8 one canonical, bounded C-5 leg-binding and C-2 sealed-Introduction
wire for `ardents-interactive-route-v1`. It replaces the H3 `AS*` frames,
unsigned plan framing, and nested-TLS Introduction tracer; no byte from those
formats is a v1 predecessor or migration input.

## Current contract

R-076/ADR-0024 select mutually authenticated TCP/TLS 1.3 legs, endpoint-local
Route selection, State/publication authority, C-5 and separate C-2 shapes, and
Service Connection-owned recovery. R-077/ADR-0025 gives Entry only a bounded
State-referenced adjacent authorization. ADR-0006 requires authenticated
generation selection, finite work safety, and no lower-profile fallback.

TLS supplies one protected reliable ordered stream but leaves application
framing and certificate interpretation to the protocol designer. HPKE supplies
an encapsulated key and ciphertext and accepts caller-controlled `info` and
AAD; it does not decide which public header is safe for the Introduction role.

## Hypotheses

- **H1:** two exact typed records, carried only after State-pinned mutual TLS,
  can bind one C-5 leg and one sealed C-2 Introduction without exposing a
  complete Route or Service Target to a Node.
- **H2:** retain the H3 tracer framing, nested TLS envelope, or a generic
  self-described map until every Route behavior has been rebuilt.
- **H0:** no bounded wire can satisfy the information-flow and downgrade
  constraints without more product/cryptographic research.

## Evaluation criteria

The chosen format must have one canonical encoding, finite parser bounds,
no optional/unknown field acceptance, exact authenticated Profile/generation
binding, State-selected peer identity, one-use/replay inputs, and deterministic
positive/mutation vectors. It must carry no complete C-5 selection on a Node
leg, no Service Name/Target/Instance in an Introduction-visible header, no
H3/WebTunnel field, and no peer-selected lower generation.

## Evidence plan

### Primary sources

- R-001, R-004, R-015, R-076, R-077, ADR-0005, ADR-0006, ADR-0024, and
  ADR-0025, inspected 2026-08-23.
- [RFC 8446](https://www.rfc-editor.org/rfc/rfc8446.html), accessed
  2026-08-23: TLS 1.3 protects an ordered reliable stream after a tamper-
  resistant handshake, but leaves higher-level protocol framing and certificate
  interpretation to its users.
- [RFC 9180](https://www.rfc-editor.org/rfc/rfc9180.html), accessed
  2026-08-23: HPKE produces an encapsulated key plus ciphertext; its `info`
  and AAD inputs bind application context while the application transports the
  public values.
- Go 1.26 `crypto/tls` and `crypto/hpke` documentation, inspected
  2026-08-23: the maintained standard library exposes TLS 1.3 controls and
  `hpke.NewSender`/`NewRecipient` for explicit AAD without first-party crypto.

### Experiment

M8 must implement independent encode/decode vectors for both records, mutate
every scalar, length, role, peer, generation, expiry, AAD, encapsulated-key,
and ciphertext boundary, and run mixed-generation refusal plus replay/join
consumption tests. No peer-facing announcement occurs before those tests exist.

### Failure scenarios

- a Node advertises, accepts, or causes selection of another Profile/version;
- a malformed length, duplicate field, alternate integer form, or unknown kind
  is accepted;
- a C-5 Node receives a complete Route or C-2 role receives Target material;
- an expired or replayed Attachment/Introduction joins fresh work; or
- an HPKE header is decrypted under a different State, Rendezvous, or endpoint
  handshake context.

## Findings

- **Sourced fact:** RFC 8446 authenticates the TLS handshake parameters and
  protects subsequent records, but does not define Ardents message grammar.
- **Sourced fact:** RFC 9180 lets an application bind its own public context as
  AAD; its public encapsulation and ciphertext require an application format.
- **Inspection:** every current Route `AS*`/nested-TLS byte is explicitly an H3
  tracer field and has no accepted observer. `routeplan` duplicates complete
  Route knowledge into command JSON.
- **Inference:** a short closed binary grammar is less likely to preserve H3
  compatibility accidentally than a generic map/JSON or an implicit TLS-only
  context. Exact State Profile and reciprocal peer fields make an unauthenticated
  downgrade unrepresentable at the Route boundary.

## Selected v1 format

All v1 Route application records are sent only after mutually authenticated TLS
1.3 with `MinVersion == MaxVersion == TLS1.3`, session tickets disabled, an
exact State public-key pin, and ALPN exactly `ardents-interactive-route-v1`.
The carrier accepts no absent, alternate, or legacy ALPN. TLS certificate and
State checks authenticate the adjacent peer; Route records do not introduce a
second Node identity authority.

The common envelope is:

```text
"ardents-interactive-route-v1\x00" || uint16(body-length) || body
```

`body-length` is big-endian, is at most 4096, and must consume exactly the
record. Integers are unsigned big-endian. A decoder rejects zero required
identifiers, non-ASCII Profile bytes, any unknown kind/version/role, surplus
bytes, and all alternate field orderings.

`kind = 1` is the fixed-size **LegBinding** body:

```text
uint16(version=1) || uint8(kind=1) || profile(short ASCII, exact v1)
network-id[32] || epoch[8] || epoch-digest[32] || attachment-id[32]
sender-role[1] || sender-node-id[32]
peer-role[1] || peer-node-id[32]
not-after-unix[8]
```

Role ordinals are `Initiator=1`, `Introduction=2`, `Rendezvous=3`, and
`Responder=4`. Each side writes and reads exactly one reciprocal Binding: the
network, epoch, digest, Profile, attachment ID, and expiry are identical, while
the sender/peer roles and Node IDs reverse. The expiry is no later than all
applicable State, duty, credential, protocol/build, and Work Safety bounds.
The receiver checks its expected adjacent State candidate and its own local
Role Domain before allocating a forward attachment. The record reveals only
the two adjacent identities and roles; it contains neither a complete Route nor
Service material.

`kind = 2` is a **SealedIntroduction** body:

```text
uint16(version=1) || uint8(kind=2) || profile(short ASCII, exact v1)
network-id[32] || epoch[8] || epoch-digest[32]
introduction-node-id[32] || rendezvous-node-id[32]
rendezvous-reachability-digest[32] || not-after-unix[8]
join-handle[32] || endpoint-handshake-context[32]
uint16(enc-length=32) || enc[32] || uint16(ciphertext-length) || ciphertext
```

`ciphertext-length` is 16 through 4096 bytes. `enc` and `ciphertext` come from
one single-use base-mode HPKE context using `DHKEM(X25519, HKDF-SHA256)`,
`HKDF-SHA256`, and `AES-128-GCM` from Go `crypto/hpke`; no suite identifier or
negotiation field exists in v1. `info` is the exact profile/version literal;
the complete visible SealedIntroduction prefix through
`endpoint-handshake-context` is AAD. The encrypted plaintext carries the
exact Service Target/Instance-generation binding, recipient publication-key
identifier, Isolation Context, and the same one-use join/endpoint context.
Only the Service-side recipient decrypts or validates that plaintext. The
Introduction role relays the envelope and never receives Target/Instance
material.

Rendezvous atomically consumes `join-handle` before it joins opaque streams;
duplicate, expired, mismatched, or failed HPKE envelopes consume no alternate
Route and produce no direct fallback. `attachment-id` and `join-handle` are
fresh endpoint-generated 32-byte values, never caller-selected command fields.

## Conformance and transition

M8 owns public synthetic vectors for the exact LegBinding bytes, every
SealedIntroduction visible prefix, fixed HPKE known-answer envelope, and each
rejection listed above. The corpus must be runnable by an independent Go
implementation without State roots or private credentials. State/publication
is the only source of supported Profile generations. v1 begins `required` for
its own local target with no predecessor reader: a peer advertising H3, an
unknown generation, a lower generation, or a Profile mismatch is rejected.
Future overlap/retirement uses ADR-0006/R-015 and requires a new record/ADR;
v1 code must not add a generic version fallback now.

## Options

| Option | Disposition |
|---|---|
| Closed binary v1 LegBinding plus SealedIntroduction records | Choose: binds State/role/one-use facts while keeping Node knowledge finite. |
| H3 `AS*` frames or nested TLS setup | Reject: C0-retired tracer bytes with no selected peer observer or HPKE binding. |
| JSON/CBOR/protobuf self-description or generic version negotiation | Reject: creates unknown-field and downgrade policy before a real future observer exists. |

## Recommendation

Choose H1 with medium confidence and record the wire in ADR-0026. The strongest
argument against it is that a first native format is hard to change. That is
why it is narrow, synthetic-vector-bound, unannounced, and has no legacy
reader; future public overlap remains governed by ADR-0006 rather than an
imagined compatibility surface.

## Disposition

**Accepted 2026-08-23 under the Product Owner's standing Stage 8 delegation.**
ADR-0026 records the format. R-015 is decided for the maintained v1 wire and
conformance strategy. M8 must implement the codec/vectors and replace H3
readers before peer-facing use; no experiment code is retained.
