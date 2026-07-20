# STB-503 Evidence

Date: 2026-07-19
Status: completed

## Outcome

Ardents now stores and transfers large encrypted payloads as deterministic
64 KiB chunks under a bounded two-level manifest tree. Chunks are encrypted and
content-addressed independently, so interrupted transfers resume from verified
local ciphertext without re-fetching completed work. The existing private Waku
data-exchange carrier remains authoritative; STB-503 does not add a parallel
transport or treat Waku relay/store retention as durable payload storage.

## Implemented Contract

- Streaming storage publishes no manifest until every chunk has been encrypted,
  persisted, and validated. Temporary chunks use an internal `staging` state
  that is neither advertised nor served.
- Cancellation, read failure, capacity refusal, filesystem write failure, and
  incomplete startup state synchronously remove or reconcile temporary files
  and metadata.
- Deterministic `512 × 512` leaf/root manifests bind chunk order, range, total chunk count,
  plaintext size, media type, cipher, and opaque key identifier. Unknown or
  inconsistent metadata fails closed.
- Chunk fetch uses a bounded worker pool: default concurrency four, hard cap
  eight, per-chunk timeout, parent cancellation, optional bandwidth limiting,
  ciphertext CID verification, and progress checkpoints.
- Manifest exchange reuses the signed `BLOB_FETCH_REQUEST` and
  `BLOB_FETCH_RESPONSE` classes with a signed optional `resource_kind` field;
  missing means legacy Blob behavior and `manifest` selects a manifest.
- Small encrypted Blobs retain their existing behavior and authoritative model.
  Canonical encrypted 64 KiB chunks, including the AES-GCM tag, remain valid
  replica-placement payloads.

## Acceptance Checks

- Canonical Linux-container fast suite passed after the final production
  changes; the longest package was `internal/runtime/process` at 14.365 seconds.
- Full Data Substrate integration passed 12/12 scenarios in 46.281 seconds.
  DAI-003 fetched and reconstructed a chunked payload over real private Waku,
  then proved resume by fetching zero and reusing all verified chunks.
- A final document-drift audit corrected the manifest fan-out from `4096 × 64`
  to the normative `512 × 512`; focused chunking/data tests and DAI-003 passed
  again, with the Waku scenario completing in 3.33 seconds.
- Focused unit tests cover deterministic manifests, maximum shape, ordering,
  tamper rejection, independent encryption, opaque key IDs, interruption/resume,
  corrupt ciphertext, cancellation, slow-peer timeout, parent timeout, bounded
  and simultaneous transfers, bandwidth limits, staging isolation, restart
  cleanup/finalization, quota pressure, and filesystem payload-write failure.
- Linux race validation passed for `./internal/data/...`; `go vet ./...` passed.
- `go mod verify` reported `all modules verified`.
- Test catalog validation reported 141 tests, 141 formal bindings, 40 scenarios,
  zero missing bindings/documents/scenarios, and zero issues.
- Production code-size validation found no hard breach. It reported the
  pre-existing 301-line soft warning in
  `internal/control/projection/snapshots.go`; STB-503 did not change that file.

## Security And Architecture Review

- Plaintext and content keys never enter Waku messages, transfer records, or
  diagnostics. Only ciphertext, authenticated manifests, and opaque key IDs are
  persisted or exchanged.
- The response signature binds the complete accepted manifest body, request,
  requester, source, resource kind, and error outcome. Tampered metadata fails
  signature or canonical manifest validation.
- Staging content is excluded from local availability, public payload reads,
  replica announcement, and private fetch responses.
- Transfer diagnostics expose bounded counts, bytes, completion/failure state,
  and stable reasons without payloads, keys, capabilities, selectors, or raw
  routes.
- Waku remains the canonical network foundation and the existing capability
  channel remains the privacy boundary.

## Resource And Orchestration Truth

All tests ran inside Linux Docker containers. The final host snapshot is
`tests/.artifacts/resources/stb-503-final.json`: `vmmemWSL` used approximately
3.61 GiB and drive C retained 216.69 GiB free. No memory, CPU, or disk
exhaustion was observed during the final integration run.

Earlier 12-20 minute UI waits were orchestration stalls, not running tests. A
named container completed successfully while the UI still displayed its shell
command. Final validation therefore used detached Compose containers, Linux
`timeout`, log redirection, and separate sub-second state/process/resource
polls. A red/green environment probe also found and fixed the temporary runner's
login-shell PATH reset (`go: not found`) by using `/bin/sh -c` with an explicit
`/usr/local/go/bin` path.

## Evidence Surface

- `docs/data-availability-replication-semantics.md`
- `docs/network-privacy-protocol.md`
- `docs/qa/integration/data-chunked-transfer.md`
- `internal/data/chunking/*`
- `internal/data/chunked_payload.go`
- `internal/data/transfer/chunked_fetch.go`
- `internal/data/transfer/chunk_workers.go`
- `internal/data/transfer/manifest_fetch.go`
- `internal/data/transfer/manifest_response.go`
- `tests/integration/data-substrate/chunked_transfer_test.go`
- `tests/.artifacts/reports/stb-503-data-integration-final/summary.json`
- `tests/.artifacts/reports/stb-503-data-integration-final/junit.xml`
- `tests/.artifacts/reports/stb-503-fanout-final/summary.json`
- `tests/.artifacts/resources/stb-503-final.json`
