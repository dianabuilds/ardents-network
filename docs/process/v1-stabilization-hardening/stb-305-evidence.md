# STB-305 Evidence — Real Filter And Lightpush Product Paths

Date: 2026-07-19

## Capability Validated

Owning domain: `Network Foundation / Messaging`, consuming Identity-owned
capability resolution and the existing private-envelope boundary.

`constrained_light_client` is now a selectable `tcp_only`, `outbound_only`
product profile. It does not start Waku Relay, a Store provider/database, or a
Filter server. It discovers connected peers' actual libp2p protocol support and
becomes joined/ready only when Filter, Lightpush, and Store providers exist.

The light client can:

- subscribe through Waku Filter using a capability-derived opaque selector;
- submit an encrypted private envelope through Waku Lightpush;
- treat Lightpush success only as provider acceptance;
- prove end delivery separately through Filter;
- retrieve missed opaque envelopes separately through Waku Store;
- expose only observed client capabilities through runtime snapshots, Connect
  RPC, CLI, and canonical network status.

## Architecture And Role Fit

- no new domain, facade, protocol, or network foundation was introduced;
- `internal/network/messaging` owns Waku carrier operations;
- `internal/network/transport` owns role mounting, provider checks, and runtime
  truth;
- `internal/network/privacy` continues to own selector derivation, encryption,
  authentication, and replay admission;
- service/local-development nodes mount Relay, Store, Filter server, and
  Lightpush service, but not the Filter client product operation;
- constrained nodes mount the Filter/Lightpush client path and omit Relay,
  Store provider state, and Filter server.

The automatic `restricted_defense` exposure controller is not treated as a
synonym for the constrained client profile. go-waku does not expose a safe
public reattachment API for stopped Filter/Store handlers; one-way shutdown
would break required recovery. Abuse/exposure enforcement for the automatic
defense controller remains owned by STB-306 rather than being falsely claimed
as reversible here.

## Dependency Decision

The complete assessment is retained in `stb-305-dependency-review.md`.

- affected domain: Network Foundation / Messaging;
- dependency role: existing go-waku v0.10.3 Filter v2, Lightpush v2, and Store
  implementations;
- security posture: accepted with opaque selectors, encrypted envelopes,
  role-specific mounting, observed provider protocols, and upstream request
  limits;
- recommendation: retain these Waku protocols as the only carrier;
- mitigation: do not add a custom light protocol, do not enable Relay on
  constrained clients, and do not equate Lightpush acknowledgement with
  network propagation;
- no new module was added.

## Runtime Security Review

Sensitive assets: capability-derived content topics, capability secrets,
private plaintext payloads, sender identity meaning, and light-client peer
affinity.

Security invariants:

- Filter and Lightpush receive only opaque content topics;
- private payloads enter both paths only as `SealedEnvelope` ciphertext;
- raw selectors, secrets, keys, plaintext, and grants never enter status,
  diagnostics, or CLI output;
- the upstream Waku logger remains suppressed, while Ardents exposes only
  role names, health state, and redaction-safe reasons;
- connected peer presence alone never authorizes a capability claim.

Assessment: passed. The real carrier capture proves that plaintext is absent,
the authorized channel alone opens the envelope, and public status exposes
only names such as `filter_client`, never selector material.

## Real-Network And Degraded Evidence

Formal scenario `NFI-005` contains three real-runtime tests:

- `TestConstrainedClientFilterLightpushAndOfflineRecovery` proves encrypted
  Lightpush submission, Filter delivery/decryption, no local Relay, and Store
  recovery;
- `TestConstrainedClientRejectsPeerWithoutRequiredProviderProtocols` proves
  that a dialled peer lacking Filter server and Store support remains degraded,
  unjoined, and absent from active capability claims;
- `TestConstrainedClientCapabilitiesReachCanonicalStatus` proves that observed
  Filter/Lightpush/Store client capability reaches the process runtime,
  transport snapshot, canonical network status, protobuf mapping, and CLI
  projection without an inbound reachability claim.

Focused tagged transport evidence independently exercises the same real Waku
flow. Unit tests prove that constrained clients reject Relay operations, do not
create a Store-provider database, and full nodes reject the Filter-client
operation.

## Acceptance Gates

- focused package and formal NFI-005 suites — passed;
- formatting, `go vet ./...`, canonical fast runner, and import boundary —
  passed;
- race suite across messaging, transport, readiness, process/orchestration,
  control projections, Connect RPC, and daemon configuration — passed;
- handwritten production code-size guard — passed with no soft or hard breach
  in the checked Network Foundation/runtime/control paths;
- full integration report at
  `tests/.artifacts/reports/stb-305-integration`: 109/109 passed, 0 failed;
- full E2E report at `tests/.artifacts/reports/stb-305-e2e`: 14/14 passed,
  0 failed;
- test catalog: 123 tests, 34 scenarios, 123 formal bindings, 0 issues;
- `govulncheck` reconciliation: unchanged one symbol-reachable
  `GO-2026-4479` in Pion DTLS v2 with no fixed v2 release, zero
  imported-package findings, and one module-only `GO-2026-5932`; this exactly
  matches `docs/security-exceptions.md`.

## Acceptance Decision

Accepted. The slice changes actual go-waku role mounting and message flow,
covers encrypted success, unavailable-provider degradation, client/server role
separation, capability truth, and offline fallback, and leaves no mandatory
Filter/Lightpush product property as a fake or config-only path.
