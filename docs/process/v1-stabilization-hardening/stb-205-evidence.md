# STB-205 Evidence — Private Data Request/Response Messaging

Date: 2026-07-19

## Accepted Capability

Data Substrate remote blob fetch now sends signed request and response contracts
inside `ardents-private/1` envelopes on a capability-derived data-exchange
selector. Requester, request ID, blob ID, response status, source identity, and
payload no longer enter Waku as readable routing metadata or plaintext.

## Delivered Product Path

- `internal/data/transfer/private_exchange.go` owns one authenticated live
  subscription per runtime start, decrypts before dispatch, routes responses by
  the now-private request ID, and preserves replay admission centrally so
  concurrent fetch waiters cannot race separate decryptors.
- Data Substrate retains its existing signed inner requester/responder
  contracts, trusted-source checks, content-ID binding, encrypted-blob-only
  re-serve rule, and signed terminal denial responses.
- response acceptance rechecks request ID, requester, blob ID, source discovery,
  trust, responder signature, and canonical blob ID/CID before persistence.
- `CapabilityDataExchange` is distinct from the discovery realm. Publish and
  subscribe permissions are resolved on every operation; receive-time sender
  revocation prevents delivery to the Data handler.
- missing/revoked data capability is fail-closed, reported as a structured
  `data` subsystem degradation, disables only remote exchange, and clears as
  runtime-only health on node stop without clearing unrelated persistent data
  failures.
- testkit gives each node isolated capability/replay persistence with
  interoperable test material. Production defaults remain fail-closed.

## Removed Legacy Surface

The following production path was removed rather than retained as fallback:

- `ardents/1/blob-request`;
- `ardents/1/blob-response/{requester}/{request_id}`;
- `SubscribeBlobRequests`, `SubscribeBlobResponses`, `PublishBlobRequest`, and
  `PublishBlobResponse` on Network Foundation;
- requester/request ID topic construction in the Waku adapter.

## Security And Failure Scenarios

- successful encrypted fetch from a trusted peer over real Waku;
- forged/unsigned source rejection;
- signed response with mismatched content identity rejection;
- untrusted source rejection;
- plaintext blob re-serve rejection returned as a signed encrypted terminal
  denial;
- undiscovered/incomplete requester rejection;
- raw Waku capture contains none of the tested blob ID, requester, or request ID;
- exact captured data request is rejected as replay;
- revoked requester capability is rejected before Data Substrate dispatch;
- missing data capability is visible as `privacy.capability.missing` degradation.

## Acceptance Commands

- canonical fast runner — passed.
- focused Data Substrate and Network Foundation integration — passed.
- canonical integration runner at
  `tests/.artifacts/reports/stb-205-integration`: 105/105 passed, 0 failed;
  `summary.json` and `junit.xml` retained.
- canonical E2E runner at `tests/.artifacts/reports/stb-205-e2e`: 13/13 passed,
  0 failed; `summary.json` and `junit.xml` retained.
- focused race suite across Data Transfer, Network Privacy, Transport, Runtime
  Authority, and Runtime Process — passed.
- `go vet ./...` — passed.
- touched-file code-size guard — passed without soft or hard breach. The broad
  directory scan reported only the pre-existing untouched soft-limit in
  `internal/node/readiness/transport_health.go`.
- test catalog: 118 tests, 31 scenarios, 118 formal bindings,
  `issue_count: 0`.
- legacy scan under `internal/network` and `internal/data`: zero old blob topic
  or transport-method matches.

## Acceptance Decision

Accepted. The active data request/response path is capability-safe, encrypted,
authenticated, replay-protected, observable on failure, and uses real Waku
without a readable compatibility path or fake transport foundation.

