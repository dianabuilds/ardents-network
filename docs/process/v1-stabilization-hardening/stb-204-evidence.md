# STB-204 Evidence — Private Discovery And Publication

Date: 2026-07-19

## Accepted Capability

Discovery and Publication now use signed discovery records inside
`ardents-private/1` envelopes over real Waku Relay/Store. Network Foundation is
only the opaque carrier. Identity resolves publish, subscribe, and Store-fetch
permissions on every operation; no capability material is cached by the
channel.

## Delivered Product Path

- `internal/publication/private_network.go` serializes local signed records and
  seals each record as a discovery-class private envelope.
- `internal/discovery/private_network.go` authorizes Store fetch, opens retained
  envelopes, rejects replay/tamper/revoked senders, and selects the freshest
  record per ID before normal Discovery intake validates signature/trust.
- `internal/network/privacy/channel.go` separately requires
  `CapabilityPublish`, `CapabilitySubscribe`, and `CapabilityStoreFetch`.
- runtime assembly injects one private channel into Publication and lifecycle;
  missing/revoked capability is fail-closed and visible as discovery
  degradation rather than plaintext fallback.
- persisted discovery records remain available across restart when retained
  Store envelopes are rejected as replay by the durable ledger.
- the readable `ardents/1/discovery-record` constant, legacy encoder/decoder,
  and `PublishDiscoveryEntries` / `FetchDiscoveryEntries` API are absent from
  production and tests.

## Failure And Recovery Truth

- missing or revoked capability: no network publication/import, structured
  `privacy.capability.*` discovery degradation, transport remains truthfully
  reported separately;
- malformed/unauthorized envelope: rejected before domain delivery;
- invalid signed discovery record: decrypted but rejected by normal Discovery
  intake and reported as discovery bootstrap degradation;
- partial publication: existing compensation behavior remains active;
- shutdown skips network withdrawal only when publication was attempted but
  never succeeded; loss of capability after a successful publication remains a
  terminal withdrawal error because stale presence may still exist;
- testkit provisions isolated replay/persistence state with interoperable test
  grants so unrelated integration scenarios do not silently run without the
  mandatory private foundation. Production defaults remain fail-closed.

## Scenarios

- `NPI-002`: ciphertext-only real Waku Relay capture, durable replay, tamper;
- `NPI-003`: retained Store envelope rejected after sender revocation;
- `NFI-001` / `NFI-003`: opaque Relay/Store carrier, private discovery fetch,
  withdrawal freshness, persistent Store;
- `DKI-002`: invalid signed private record and explainable degraded restart;
- `DKI-003`: signed multi-node private discovery import and receiver restart
  recovery;
- existing Discovery integration coverage continues to prove stale and
  untrusted record behavior after domain intake.

## Acceptance Commands

All commands used Go 1.26.5 with `GOFLAGS=-buildvcs=false` and the repository
workspace cache.

- `powershell -NoProfile -File tests/run.ps1 fast` — passed.
- `go test -tags=integration ./tests/integration/... -count=1 -timeout 5m`
  — 8/8 packages passed.
- `powershell -NoProfile -File tests/run.ps1 integration -ReportDir tests/.artifacts/reports/stb-204-integration`
  — 103/103 tests passed, 0 failed.
- canonical artifacts:
  `tests/.artifacts/reports/stb-204-integration/summary.json` and
  `tests/.artifacts/reports/stb-204-integration/junit.xml`.
- focused race suite across Network Privacy, Transport, Discovery, Publication,
  Node Lifecycle, and Runtime — passed.
- `go vet ./...` — passed.
- code-size guard across all touched production owners — passed with no soft or
  hard breach.
- `go run ./tests/cmd/testcatalog -mode validate ./tests/...` — 116 tests,
  30 scenarios, 116 formal bindings, `issue_count: 0`.
- production/test scan for `ardents/1/discovery-record`,
  `PublishDiscoveryEntries`, and `FetchDiscoveryEntries` — no matches.

The first canonical integration attempt was stopped by the external command
timeout and was not accepted as evidence. Subsequent failures exposed missing
test capability provisioning and one missing runtime cleanup; both were fixed,
then the direct full suite and canonical runner passed completely.

## Acceptance Decision

Accepted. The owning Discovery/Publication path is real Waku plus the canonical
privacy foundation, covers happy, degraded, revocation, invalid-record,
withdrawal, Store, and restart behavior, and contains no deferred critical
plaintext compatibility path.

