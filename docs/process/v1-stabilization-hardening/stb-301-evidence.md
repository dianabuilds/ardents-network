# STB-301 Evidence — Supported Node And Connectivity Profiles

Date: 2026-07-19

## Accepted Capability

Ardents now models node role, transport family, and dynamic runtime mode as
three separate configuration/status dimensions. The supported combinations
are defined in `docs/network-participation-profiles.md`, enforced before
lifecycle startup, and exposed through the canonical network status surface.

## Profile Contract

- `service_node` is selected explicitly by `ardd` and accepts `tcp_only` or a
  fully materialized `tcp_wss` runtime configuration.
- `local_development` is the safe embedded/runtime default, accepts only
  `tcp_only`, and requires a loopback bind address. Default construction sets
  the explicit bind to `127.0.0.1`.
- `constrained_light_client` is a required target but is rejected as not
  implemented until STB-305 supplies real private Filter/Lightpush paths.
- `restricted_defense` is an automatic runtime mode and cannot be selected as
  a startup node profile. Its current status-only behavior is documented and
  is not claimed as enforced exposure reduction.
- `tcp_quic`, unknown values, invalid node/transport pairs, and incomplete WSS
  material fail closed with no transport fallback.

The Waku role matrix distinguishes actual Relay/Store product paths from
Filter/Lightpush objects merely mounted by go-waku. Static profile definitions
do not claim protocol readiness.

## Preflight And Runtime Truth

- the runtime validates node profile, transport profile, WSS material
  presence, and local-development loopback binding before calling the
  lifecycle command;
- invalid runtime configuration leaves lifecycle and transport states
  unchanged;
- the transport validates its own profile before resolving listeners or
  creating the Waku Store and transport-key files;
- direct `tcp_quic` startup proves neither persistence file is created;
- `ARDENTS_NODE_PROFILE` is parsed and validated by `ardd`; its default is the
  explicit `service_node` role;
- `node_profile` is projected through local API, ConnectRPC, JSON, CLI, and
  protobuf separately from transport `active_profile` and runtime
  `active_mode`;
- a locally bound listener no longer makes `reachable=true`: an active family,
  endpoint, and observed peer path are all required. Public/NAT ingress remains
  explicitly owned by STB-304.

## Document Alignment

- added `docs/network-participation-profiles.md` with role, transport, Waku
  role, bootstrap/reconnect, address, degradation, and remaining-gate matrices;
- updated communication contracts and the docs map;
- corrected stale transport requirements/architecture text to reflect
  explicit `tcp_only`/`tcp_wss` runtime modeling and fail-closed QUIC;
- preserved DEC-STB-003 and the active DTLS containment exception.

## Acceptance Gates

- formatting gate — passed;
- `go vet ./...` — passed;
- canonical fast runner and import boundary — passed;
- touched production code-size guard — passed after extracting network-env
  parsing and Waku-node preparation; no soft or hard breach remains;
- focused real integration for local control, network foundation, and node
  startup/recovery — passed;
- focused E2E operator/network recovery — passed;
- canonical integration runner at
  `tests/.artifacts/reports/stb-301-integration`: 105/105 passed, 0 failed;
  raw reports, `summary.json`, and `junit.xml` retained;
- canonical E2E runner at `tests/.artifacts/reports/stb-301-e2e`: 14/14 passed,
  0 failed; raw reports, `summary.json`, and `junit.xml` retained;
- race suite across profile/readiness, transport, control projection, runtime,
  ConnectRPC, and daemon config — passed;
- test catalog: 119 tests, 32 scenarios, 119 formal bindings, 0 issues.

## Acceptance Decision

Accepted. Every selectable node/transport combination has implemented runtime
support, unsupported combinations fail before partial startup, profile truth is
operator-visible, and the matrix makes no unverified Filter/Lightpush,
restricted-defense, QUIC, or public-reachability claim.
