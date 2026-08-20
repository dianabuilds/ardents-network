---
id: R-047
title: Which narrow cryptographic profile authenticates and hides S6.2 naming exchanges?
status: decided
owner: Product Owner
started: 2026-08-20
reviewed: 2026-08-20
---

# R-047 — Stage 6 authenticated query hiding

## Decision this unlocks

Decide whether S6.2 may use Go's standard-library Ed25519 for Name Authority
and Name Record authentication and promote the exact RFC 9458 Adapter measured
by R-026 from laboratory-only ownership to a maintained private naming
exchange. This separates the narrow S6.2 decision from the still-open R-044
Recovery Policy cryptography decision.

## Current contract

R-003 and R-039 require authenticated Name Records, multi-role knowledge separation, no stable
cross-Isolation-Context identifier, bounded retries, and no direct/DNS/HTTP or
other less-private fallback. ADR-0005 restricts Destination Resolution to
Rendezvous-domain identities and requires same-destination/context family
exclusion. R-046 supplies the proposed exact field matrix.

R-026 selected `github.com/openpcc/ohttp v0.0.80` only for Gate C and measured
the exact raised dependency closure on Go 1.26.5. The official Ubuntu campaign
passed role-view, modification, stale, replay, nonce, binding, offline-supply,
and combined-role probes. It did not select a production Gateway operator,
key-distribution mechanism, public wire protocol, or privacy claim.

## Hypotheses

- **H1:** standard-library Ed25519 plus the exact R-026 OHTTP implementation can
  authenticate and hide S6.2 exchanges when canonical transcripts, key
  configuration, replay, padding, role proof, and failure remain Ardents-owned.
- **H2:** another maintained OHTTP implementation has materially better product
  fit and supply risk.
- **H0:** no measured candidate meets the contract; S6.2 stops without a
  plaintext fallback or first-party cryptography.

## Evaluation criteria

1. Relay learns Endpoint transport origin, fixed Gateway, key identifier,
   timing, and fixed envelope size, but no exact name, Target, query type, or
   publicly testable name-derived value.
2. Gateway learns plaintext and Relay origin, but no Endpoint/User identity,
   Isolation Context, stable per-client identifier, or prior-request key.
3. Every request uses a new HPKE context and 32-byte nonce; application code
   binds network, operation, name, deadline, and response and rejects replay.
4. Plaintext request and response are exactly 4096 bytes with canonical zero
   padding; all errors fail closed without alternate resolution.
5. Gateway key configuration is authenticated, finite, common to an anonymity
   set, and pre-provisioned. Runtime discovery, per-client keys, trial fallback,
   and unique configurations are forbidden.
6. Relay and Gateway use different identities and known families, have valid
   assignments for the whole operation, and never share one process or durable
   root in the maintained tracer.
7. Exact dependencies, versions, licenses, advisories, offline supply, cgo,
   unsafe, maintenance, and removal cost remain acceptable.
8. An Authority is one canonical lowercase hexadecimal Ed25519 public key (32
   decoded bytes). A Name Record signature is exactly 64 bytes over the
   domain-separated canonical transcript defined below.
9. Wrong key, network, domain, record byte, generation, revision, Target, or
   signature fails before Target exposure. Private key material is forbidden
   from Resolver, Relay, Gateway, logs, errors, and evidence.

### Canonical S6.2 signature transcript

S6.2 selects ordinary Ed25519 from Go's `crypto/ed25519`, without prehashing or
Ed25519ctx. Domain separation is part of the message bytes so all callers use
the same ordinary API:

`u16("ardents-name-record-v1") || network_id[32] ||
u64(record_length) || canonical_name_record`

`u16` and `u64` are unsigned big-endian byte lengths; the literal follows its
length and the Network ID is the exact non-zero 32-byte value. The record is the
exact S6.1 canonical encoding. The signed-record container is `schema_version = 1`,
the length-prefixed record, then the 64-byte signature, with no trailing bytes.

The common Gateway configuration is authenticated by the Gateway Node's
Ed25519 identity already committed by Network State. Its signature transcript
is:

`u16("ardents-naming-gateway-v1") || network_id[32] || gateway_node_id[32] ||
u64(assignment_not_after_unix_nanos) || u32(key_config_length) ||
ohttp_key_config`

`u32` is an unsigned big-endian byte length. Selection verifies this signature
against the exact Gateway candidate public key from the recovered authenticated
Network State; a digest copied beside an unsigned configuration is not proof.
The local execution plan remains entirely inside the naming Module after that
verification and is never exposed as a serializable authority source.

Later control-operation codecs use a distinct fixed literal
`ardents-name-control-v1` followed by network ID, operation discriminator, and
their complete canonical bytes. R-042/R-044/R-045 must freeze those bytes before
the operations are implemented; no generic map or caller-authored digest is a
signature transcript.

## Evidence plan

### Primary sources

Accessed 2026-08-20:

- [RFC 9458](https://www.rfc-editor.org/rfc/rfc9458.html) — role views,
  authenticated key configuration, fresh HPKE contexts, HTTPS, padding,
  traffic-analysis, configuration-linkability, rotation, forward-secrecy, and
  collusion limitations;
- [RFC 8032](https://www.rfc-editor.org/rfc/rfc8032.html) and Go 1.26.5
  [`crypto/ed25519`](https://pkg.go.dev/crypto/ed25519) — Ed25519 behavior,
  deterministic verification, key/signature sizes, and standard-library
  constant-time private-key operations;
- [`openpcc/ohttp`](https://github.com/openpcc/ohttp) source and package docs —
  Client transport, Gateway middleware, injected key lookup, no production
  Relay, Apache-2.0 license, and security contact;
- R-026 — exact dependency/source identity, raised advisory closure,
  reproducible supply measurements, tests, and official Ubuntu result;
- `docs/development/dependencies.md` — exact lab ownership and removal gate.

### Experiment

Do not repeat the passing generic OHTTP experiment. The S6.2 tracer must instead
falsify promotion-specific seams:

1. use two Isolation Contexts with one common authenticated Gateway
   configuration and distinct HPKE contexts/nonces;
2. capture actual local-resolver, Relay, Gateway, and observer views;
3. reject every R-046 forbidden-field mutation and stable cross-context handle;
4. reject stale/missing assignment, same-family roles, Gateway reuse as the
   connection Rendezvous, key mismatch, replay, modified padding, oversized
   input, response substitution, and alternate/fallback contact;
5. verify fixed request/response sizes and dependency identity offline;
6. run current reachable-vulnerability and repository gates.
7. run RFC 8032 vectors and mutate Authority key, signature, network, domain,
   record length, every lifecycle field, and trailing bytes independently.

### Failure scenarios

- unique or attacker-selected key configuration partitions one client;
- Relay forwards `Forwarded`, `Via`, cookies, authorization, or identifying
  headers;
- Gateway sees a direct Endpoint connection or both roles share a family;
- response or error is accepted without OHTTP and application binding;
- retry reuses HPKE context, nonce, connection ID, or crosses a context;
- key compromise decrypts retained traffic or stale keys remain accepted;
- dependency, maintenance, license, offline, advisory, or role-view gate fails.

## Findings

- **Sourced fact:** RFC 9458 protects plaintext from Relay and transport origin
  from Gateway only with separate roles and HTTPS. It leaves authenticated key
  acquisition to the application.
- **Sourced fact:** RFC 9458 requires a fresh HPKE context per request and warns
  that unique configurations, content, differential treatment, and traffic
  analysis can link requests or shrink anonymity sets.
- **Sourced fact:** the library supplies Client and Gateway behavior, but no
  production Relay or key-distribution system.
- **Sourced fact:** Go 1.26.5 `crypto/ed25519` implements RFC 8032 Ed25519 in the
  standard library; public keys are 32 bytes, signatures are 64 bytes, ordinary
  Ed25519 signs the message directly, and private-key operations use
  constant-time algorithms.
- **Inference:** an explicit fixed transcript prefix is preferable here to
  Ed25519ctx: it keeps one ordinary Ed25519 API and makes domain/network binding
  visible in the canonical bytes. It is not a caller-selected hash.
- **Measurement:** R-026 observed that the selected version hid a 4096-byte
  Name/Target marker from Relay, delivered exact plaintext from Relay origin,
  and rejected response modification.
- **Measurement:** official Gate C passed 17 failure probes; the raised closure
  passed offline build/test and reachable-vulnerability scanning.
- **Inference:** promoting the existing dependency is lower risk than selecting
  another cryptographic implementation, provided S6.2 proves deployment seams.

## Options

### O1 — standard-library Ed25519 plus exact R-026 OHTTP

Use Go 1.26.5 `crypto/ed25519` for the fixed transcripts above and
`github.com/openpcc/ohttp v0.0.80` at commit
`79bec89d804248df1a71a0f56c882b116579035d` with the registered raised closure.
A product naming Module owns signatures, fixed 4096-byte messages, nonce/deadline/replay
binding, authenticated common Gateway config, Role Domain checks, and
fail-closed results. A thin Adapter owns only OHTTP encapsulation and
decapsulation. No general HTTP proxy or public resolver Interface is exposed.

### O2 — repeat implementation selection

Reject unless O1 fails a current maintenance, advisory, or integration gate.
R-026 already found the reference alternative less mature; repeating the
generic comparison does not address production deployment seams.

### O0 — choose none

Stop S6.2. Do not implement plaintext lookup, local-table fallback, DNS/HTTP,
PIR, first-party OHTTP/HPKE, or another Namespace.

## Recommendation

Choose O1 under a narrow replacement ADR for S6.2 authentication and query
hiding only. Keep R-044 open for Recovery Policy threshold cryptography; S6.2
must not introduce threshold or recovery APIs. Confidence is medium-high because the
transport and supply evidence passed, while remaining uncertainty is the role,
key, and deployment composition that S6.2 is designed to test.

The strongest objection is that OHTTP does not defeat Relay/Gateway collusion
or traffic correlation and its Gateway key lacks forward secrecy for captured
requests during that key's lifetime. The product must state those limitations,
rotate finite common configurations, and avoid stronger anonymity claims.

## Disposition

- State: `decided`; the Product Owner accepted O1 on 2026-08-20.
- Selected option: O1 with standard-library Ed25519 and the exact R-026
  dependency and raised closure.
- Accepted ADR-0014 authorizes S6.2 Name Authority/Record signatures
  and query hiding, not Recovery Policy cryptography.
- Acceptance unblocks S6.2 TDD together with accepted R-046.
- Any dependency/version/suite change reopens source, advisory, conformance,
  offline-supply, and role-view review rather than upgrading in place.
