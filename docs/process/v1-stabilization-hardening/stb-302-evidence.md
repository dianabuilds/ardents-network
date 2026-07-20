# STB-302 Evidence — Secure WSS Operator Configuration

Date: 2026-07-19

## Accepted Capability

`service_node` can now select `tcp_wss` through the `ardd` operator boundary
without bypassing the canonical Waku transport owner. TCP-only remains the
explicit default and rejects dormant WSS settings.

## Operator Contract

`ardd` accepts and propagates:

- `ARDENTS_WSS_PORT`;
- `ARDENTS_WSS_CERT_PATH`;
- `ARDENTS_WSS_KEY_PATH`;
- optional `ARDENTS_WSS_CA_PATH` for private PKI, otherwise system roots;
- `ARDENTS_WSS_ADVERTISE_ADDRESS`.

The same transport validator runs at daemon/runtime preflight and at direct
transport startup. Validation occurs before Waku Store or node-key creation.
Missing/unreadable/non-regular material, insecure private-key state, invalid or
mismatched pairs, premature/expired leaves, self-signed leaves, hostname/SAN or
server-auth mismatch, malformed CA bundles, and untrusted chains fail closed.
Errors are bounded and do not include configured material paths or contents.

## Endpoint And Rotation Truth

- the WSS listener binds through the existing Waku secure-WebSocket option;
- the configured advertised host replaces only the WSS bind host in published
  multiaddrs; TCP endpoint truth remains unchanged;
- advertisement alone is not interpreted as external reachability;
- certificate, key, and CA files are reread and revalidated on every start;
- rotation is deployment-managed replacement followed by controlled restart;
- no product path generates or accepts a self-signed certificate.

Successful tests use a test-CA-issued server leaf. The self-signed generator is
retained only as negative validation material.

## Document Alignment

- updated `docs/network-participation-profiles.md` with the complete environment
  contract, trust policy, endpoint semantics, and restart rule;
- updated transport requirements/architecture to describe completed WSS
  provisioning rather than a future skeleton;
- updated retained-state security and NFI-002 QA coverage for trust and rotation.

## Acceptance Gates

- formatting, `go vet ./...`, canonical fast runner, and import boundary — passed;
- touched production code-size guard — passed with no soft or hard breach;
- focused unit tests cover valid CA-issued material, missing material, mismatch,
  expiry, hostname mismatch, self-signing, untrusted chains, TCP-only rejection,
  path redaction, and pre-persistence failure;
- final NFI-002 integration report at
  `tests/.artifacts/reports/stb-302-wss-trust-integration`: 2/2 passed;
- full integration report at
  `tests/.artifacts/reports/stb-302-integration`: 105/105 passed, 0 failed;
- full E2E report at `tests/.artifacts/reports/stb-302-e2e`: 14/14 passed,
  0 failed;
- race suite across transport, runtime process/orchestration, and daemon config
  — passed;
- test catalog: 119 tests, 32 scenarios, 119 formal bindings, 0 issues.

## Acceptance Decision

Accepted. WSS is an explicit, trust-validated operator path over canonical
Waku, invalid configuration cannot create partial network state, endpoint
publication remains bounded, and certificate rotation has a tested restart
contract.
