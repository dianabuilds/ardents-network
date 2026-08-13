# Horizon 3 Stage 2 implementation brief

Status: **authorized by accepted R-030 for development evidence**

Authoritative inputs: accepted ADRs 0004–0005, R-001, R-004, R-023, R-029,
[R-030](../research/records/r-030-h3-real-multi-node-route.md), the H3 technical
design, package map, and repository rules.

## Outcome and seam under test

One Client opens current authenticated Network State, independently selects
four distinct eligible Candidate records, and sends a fresh 32-byte canary
through separate Initiator, Introduction, Rendezvous, and Responder processes
to a Publisher. Exact bytes, length, and SHA-256 digest match. No direct,
shortened, repeated-identity, family/source/domain-conflicting, DNS, proxy, or
ambient-network fallback exists.

The accepted Epoch profile is exactly `h3-route-tracer-v1`, and eligible Node
records declare laboratory capability `2`. Stage 1 `h3-role-probe-v1` records
cannot be consumed as Route Nodes.

The public seams are:

1. Network State returns one immutable authenticated Route view;
2. Route selection either returns the exact four-position plan or a classified
   rejection;
3. a role process serves one bounded duty with only adjacent-role knowledge;
4. the Client executes the full plan and returns terminal byte/evidence facts;
5. the verifier maps complete evidence to `pass|fail|invalid`.

Tests observe only those seams or the black-box command. Private carrier
framing, goroutine arrangement, and TLS configuration are implementation
details.

## Domain language and trust boundaries

- **Candidate:** one Node-signed record in the current authenticated Candidate
  View; not a Service, Endpoint, Person, or independent operator.
- **Route Plan:** the Client-owned ordered selection for one attempt; never sent
  as one object to a Node, Publisher, source, or distributor.
- **Position:** exactly one of `initiator`, `introduction`, `rendezvous`, or
  `responder`; the Introduction position proves separation only and does not
  implement future publication or reachability semantics.
- **Carrier Adapter:** private TLS 1.3 over literal TCP with exact authenticated
  Node-key pins; replaceable and not a production transport decision.
- **Publisher fixture:** the canary receiver and final end-to-end TLS endpoint;
  not a Stage 3 Publisher capability or Service Instance.

Network State authenticates eligibility but does not select a Route. The Client
alone selects. Nodes authenticate their own duty and the permitted adjacent
identity; no Node selects or receives the complete plan. The Publisher sees the
Responder and canary but no earlier position. H may observe everything for
development evidence but never proxies data.

## Process and information matrix

| Process | Receives | Emits/learns | Must not receive |
|---|---|---|---|
| Client | authenticated Route view, exclusions, Publisher fixture pin | full plan, canary digest, terminal result | private Node keys |
| Initiator | own key/duty, previous Client connection, permitted Introduction candidates | selected Introduction identity and opaque inner bytes | Publisher identity, canary plaintext, later positions |
| Introduction | own key/duty, permitted Initiator and Rendezvous candidates | adjacent identities and opaque inner bytes | Client address, Publisher identity, canary plaintext, Responder |
| Rendezvous | own key/duty, permitted Introduction and Responder candidates | adjacent identities and opaque inner bytes | Client address, Publisher identity, canary plaintext, Initiator |
| Responder | own key/duty, permitted Rendezvous candidates, Publisher attachment | Rendezvous and Publisher attachment, opaque end-to-end bytes | Client address, canary plaintext, earlier positions |
| Publisher | own fixture key, permitted Responder pins | exact canary bytes/length/digest | complete plan or Client network address |
| Verifier | frozen manifest and all bounded observations | recomputed verdict/evidence digest | data-path authority |

## Implementation boundary

Add `internal/route` only with its real implementation, `doc.go`, behavior and
black-box process tests, exact package-map imports, and `cmd/ardents-route` as a
thin non-test caller. Network State may expose an immutable list derived from
its already verified current Decision; it must not import Route or weaken any
Stage 1 acceptance rule. Qualification-specific recomputation belongs in
`internal/qualification/route` only if it appears with a real command caller
and architecture tests in the same change.

No new runtime dependency is expected. Do not import `internal/lab`, copy the
H2 native circuit, generalize `internal/node/probe`, or create generic crypto,
transport, protocol, schema, types, interfaces, common, util, API, or SDK
packages.

## Resource and failure contract

- fixed four-position plan; maximum 64 authenticated Candidates;
- one attempt, one dial per leg, zero retries, literal IP endpoints only;
- 4 KiB control and 64 KiB data bounds; exact 32-byte canary;
- 5-second setup/handshake, 15-second attempt, 5-second shutdown;
- maximum eight accepted connections and two copy goroutines per process;
- owned listeners/connections/timers/goroutines/processes terminate on context
  cancellation; no live Route or credential file persists;
- malformed, oversized, partial, slow, wrong-peer, wrong-role, wrong-family,
  wrong-domain, reused-identity, direct, and short inputs fail closed.

Candidate violation with complete valid evidence is `fail`. Missing, truncated,
mutated, contradictory, or unbound harness evidence is `invalid`. Candidate
self-report cannot turn invalid evidence into pass.

## Evidence schema

The versioned terminal record binds: source/build identity; manifest digest;
Network ID, generation, Epoch and View digest; exclusions; four ordered
role/Node/family commitments; per-process PID, own role, observed previous/next
role commitments, TLS pin outcome, lifecycle and cleanup; canary length and
digest at both endpoints; exact deadline/cancellation result; evidence digest;
and terminal verdict/reason. Secret keys, raw credentials, reusable access, and
complete route history are excluded from retained public evidence.

## TDD slices and required tests

Implement one red→green vertical at a time:

1. authenticated Route view is immutable and unavailable for stale/conflicting
   state;
2. positive selection returns four distinct correct-domain/family identities;
3. wrong identity, duplicate family, source exclusion, and wrong/missing domain
   reject;
4. one real role process accepts the correct adjacent identity and rejects the
   wrong identity plus malformed/partial/slow/oversized frames;
5. real separate processes carry the unpredictable canary through all four
   positions with exact byte/length/digest verification;
6. direct and every shortened plan reject before data transfer;
7. cancellation during setup and transfer reaches bounded terminal cleanup;
8. evidence mutation, truncation, missing process, or digest mismatch is
   `invalid`, while a complete wrong-byte or forbidden-path result is `fail`;
9. post-run process/listener/socket/goroutine/temp-resource ownership is empty.

Keep all existing Stage 1 suites. Run `make quick-check` during work and
`make check` before integration with the repository-approved Go 1.26.5.

## Definition of Done

- accepted current Network State is the only Candidate source;
- four real child processes with distinct authenticated Node identities carry
  the route, and Client selection is independently evidenced;
- exact unpredictable bytes arrive through no direct/short fallback;
- role-local evidence matches the matrix and no Node sees plaintext canary or
  complete protected binding;
- negative, cancellation, cleanup, quiescence, and evidence-integrity tests pass;
- architecture/package-map/file-size/import gates and full `make check` pass;
- report states the same-host/project-control, laboratory carrier, no Stage 1
  Ubuntu qualification, and no anonymity/production claim limitations.

Stop and return to R-030 if the tracer needs a production transport decision,
custom cryptography, H2 imports, hidden direct forwarding, full-route Node
state, unbounded work, or weakening/removal of Stage 1 tests.
