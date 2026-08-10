---
id: R-026
title: Can an existing OHTTP implementation support Gate C Private Resolution?
status: active
owner: product research
started: 2026-08-10
reviewed: 2026-08-10
---

# R-026 — Gate C Private Resolution Adapter

## Decision this unlocks

Decide whether Gate C may use an existing RFC 9458 Oblivious HTTP
implementation for its bounded two-role Private Resolution Adapter. A passing
Adapter must hide the exact Name, Target, and tested lookup marker from the
Relay, hide the querying endpoint origin from the Gateway, and keep Gateway
identity distinct from the Service Target, Name Authority, and C-5/C2
Rendezvous.

This record selects an experiment dependency, not a production resolver,
Namespace implementation, public wire protocol, or privacy claim. If no
candidate satisfies the pre-registered criteria, the terminal result is
`stop`; Gate C does not receive a locally implemented OHTTP/PIR substitute or a
less-private fallback.

## Current contract

- [R-003](r-003-service-name-contract.md) fixes Private Resolution as a
  multi-role knowledge separation and forbids a less-private fallback.
- [R-017](r-017-named-private-site-anonymous-mailbox.md) selects the bounded
  Named Unlisted Site tracer without a public Namespace or application runtime.
- [R-013](r-013-carrier-lab-technology-candidates.md) records `advance` for the
  current native C-5/C2 Route shape and permits the Gate C tracer.
- [Gate C](../../development/entry-gates.md#gate-c--start-the-named-unlisted-site-reference-application)
  limits the slice to one pre-provisioned exact Name, private reachability,
  Target authentication, explicit failure, and ordinary migration.
- [ADR-0005](../../adr/0005-route-domains-and-bounded-entry-exposure.md)
  restricts Destination Resolution to a non-adjacent role and excludes that
  identity/family from the same destination's Rendezvous selection.

The fixed Gate C Application input is `ardents://site.reference`. Name and
Reachability are separate logical query types carried through the same Adapter.
Every plaintext request and response is exactly 4096 bytes before OHTTP
encapsulation and binds a fresh 32-byte nonce, run identity, network identity,
and deadline. The lab fixture signs the returned Name Record or Descriptor; the
OHTTP Gateway is not trusted to create product truth.

## Hypotheses

- **H1 — openpcc Adapter:** an exact reviewed revision of `openpcc/ohttp` can
  provide the client/gateway RFC 9458 encapsulation while a deliberately small
  Relay forwards opaque envelopes only.
- **H2 — reference Adapter:** if H1 fails because of a bounded implementation
  defect, an exact reviewed revision of `chris-wood/ohttp-go` can satisfy the
  same Interface and evidence contract.
- **H0 — stop:** the OHTTP shape or both maintained implementations fail the
  Gate C knowledge, authentication, supply, platform, or maintenance contract.

## Pre-registered evaluation criteria

A candidate is acceptable only when every item passes on Go 1.26.5:

1. Relay observations contain the Gateway endpoint and one encrypted,
   fixed-shape envelope, but no exact Name, Target, plaintext query type, or
   seeded lookup marker.
2. Gateway observations contain the plaintext lookup and only the Relay as
   transport origin. Gateway identity is not a Service Target, Name Authority,
   Service Authority, Instance identity, or C-5/C2 Rendezvous.
3. The client-side fixture rejects modified and stale responses, replay, nonce
   mismatch, wrong run/network identity, wrong Name/Target/Descriptor binding,
   and expired or superseded Instance Credential.
4. The implementation passes its upstream tests and RFC 9458 vectors. Gate C
   adds independent round-trip and adversarial tests at the Adapter Interface.
5. The exact module closure passes `go mod verify`, builds and tests with the
   network disabled after explicit preparation, uses no cgo, and introduces no
   first-party or dependency `unsafe` that lacks a separately reviewed reason.
6. Every runtime module and license is recorded. No unacceptable license and no
   reachable high/critical vulnerability may enter the root module.
7. Maintenance signals, security-reporting path, API stability, and removal
   cost are acceptable for a maintained experiment owned by one Product Owner
   and Codex.

An implementation defect may select H2. Failure caused by the OHTTP role shape,
or failure of both implementations, selects H0. The criteria are conjunctive;
performance or convenience cannot compensate for a knowledge or authentication
failure.

## Evidence plan

### Primary sources

Accessed 2026-08-10:

- [RFC 9458](https://www.rfc-editor.org/rfc/rfc9458.html), including the
  Client/Relay/Gateway views, HTTPS requirements, key-configuration caveat, and
  application-owned replay defense;
- [`openpcc/ohttp`](https://github.com/openpcc/ohttp) source, tests, Apache-2.0
  license, security contact, and module graph;
- [`chris-wood/ohttp-go`](https://github.com/chris-wood/ohttp-go) source, tests,
  MIT license, module graph, and experimental security disclaimer;
- Go module proxy checksum records and the Go vulnerability database for exact
  selected revisions and their complete runtime closure.

### External prototype

Create an owned system-temporary Go module outside this repository. For each
candidate, record the exact module version and commit, retrieve the module once,
run `go mod verify`, upstream tests, RFC vectors, cgo/unsafe and license scans,
`go list -m -json all`, `go mod graph`, `govulncheck`, and a minimal
client→Relay→Gateway round trip. Then repeat build and tests from the prepared
module cache with network access disabled.

The prototype is disposable and cannot alter root `go.mod` or `go.sum`. Its
bounded report may be copied into this record; module caches, source checkout,
binaries, coverage, keys, and raw messages are removed with the owned temporary
directory.

### Failure scenarios

- envelope modification, truncation, wrong key ID, malformed encapsulation, and
  response substitution;
- duplicate request, duplicate response, nonce mismatch, stale deadline, stale
  Name revision, wrong network/run identity, and wrong Target binding;
- Relay forwarding identifying headers or plaintext; Gateway receiving a
  direct endpoint connection; one process observing both origin and query;
- dependency unavailability, checksum mismatch, offline build failure,
  unsupported Go 1.26.5, cgo/unsafe surprise, unacceptable license, reachable
  high/critical vulnerability, or abandoned/unstable API.

## Initial sourced findings

- **Sourced fact:** RFC 9458 defines encrypted HTTP messages forwarded through
  a Relay to a Gateway. The Relay knows origin and Gateway but not plaintext;
  the Gateway knows plaintext and Relay origin but not client origin. Relay and
  Gateway must not be operated as one privacy role.
- **Sourced fact:** RFC 9458 does not define authenticated Gateway key
  acquisition and assigns replay safety to the using application. Gate C must
  therefore pre-provision the Gateway key and add its own nonce/deadline/replay
  and signed-record checks.
- **Sourced fact:** `openpcc/ohttp` supplies client `http.RoundTripper` and
  Gateway middleware but deliberately no Relay. Its upstream README identifies
  Apache-2.0 licensing, a security contact, injectable HPKE components, and RFC
  9458 support.
- **Sourced fact:** `chris-wood/ohttp-go` supplies client and Gateway library
  behavior under MIT, has no published release, depends directly on CIRCL HPKE,
  and warns that the API and implementation remain experimental.
- **Inference:** a purpose-limited Gate C Relay is acceptable glue because RFC
  9458 defines it as opaque forwarding behavior; implementing OHTTP
  encapsulation, HPKE, or a general resolver would not be acceptable glue.

## Options

### O1 — `openpcc/ohttp`

Primary candidate because its Interface already follows ordinary Go HTTP
client/server boundaries, supports injected cryptographic implementations, and
documents an operational security contact. Its small project history and
transitive cryptographic/encoding closure require exact review.

### O2 — `chris-wood/ohttp-go`

Reference candidate because it is authored by an RFC 9458 author and exposes a
small direct library. Its explicit experimental disclaimer, no releases, and
hard CIRCL dependency increase maintenance and supply risk.

### O0 — choose none

Record `stop` for Gate C. Do not implement local-table resolution, direct
resolution, DNS/HTTP fallback, alternate Namespace, cached-success fallback,
first-party OHTTP, or first-party cryptography.

## Recommendation before experiment

Evaluate O1 first. Evaluate O2 only if O1 fails for a bounded implementation
reason rather than because the OHTTP role split itself cannot meet the Gate C
contract. Confidence is medium: RFC 9458 directly matches the required
two-party knowledge split, but neither candidate's exact current dependency and
security state has yet been measured.

The strongest argument against proceeding is that OHTTP protects transport
origin from the Gateway, not the authenticity, freshness, padding, replay
safety, or privacy of application contents by itself. Gate C advances only if
its own signed fixture and knowledge tests close those gaps without weakening
the product contract.

## Disposition

- State: `active`; criteria frozen before prototype execution.
- No root dependency selected and no root module change authorized yet.
- Next action: run the external candidate evaluation and append exact
  measurements, recommendation, and dependency disposition here.
