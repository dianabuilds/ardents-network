---
status: accepted
date: 2026-09-27
---

# ADR-0109 — The Reachability Store refuses stored legacy records with a typed error; the retained v1/v2 Descriptor decode grammar is deleted

(F-32. This closes the last open cite-or-die card of the ADR-0091 sweep. The
finding allowed either an authenticated floor-adoption operation or a
documented typed refusal with a new-Target policy; under the product owner's
standing decisions that legacy dies and that no support or backward
compatibility is owed, this ADR selects the typed refusal.)

## Context

The generation-2 Service Reachability Descriptor (wire versions 1 and 2,
signed over a State-bound Introduction live slot) lost its writers long
before this decision. ADR-0091 retired the unwired OHTTP Relay/Gateway/
Client adapter; ADR-0105 deleted the uncalled generation-2 `Issue`,
`Store.Publish`, and `Store.Lookup` writer/reader APIs together with the
Namespace closure. After ADR-0105 no working-tree path could create or
return a generation-2 record.

What survived was a reader-side residue inside `internal/service/reachability`:

- `descriptor.go`: the complete v1/v2 decode grammar (`Verify`, `decode`,
  `validIntroduction`, `validSubmission`, `descriptorPrefix`,
  `cloneIntroduction`, `cloneDescriptor`) plus the version constants and
  transcript prefixes.
- `contract.go`: the legacy contract surface (`MaximumDescriptorSize`,
  `maximumAuthorization`, `SubmissionMode` with its two grant constants, the
  `Introduction` struct, and the `Descriptor.Introduction` member).
- `store_files.go`: the legacy branch of `decodeStored` (stored-record
  envelope byte 1), which authenticated a retained v1/v2 record and restored
  its signed Credential floor, and the legacy write branch of `encodeStored`
  that no publisher could reach.
- `store.go`: the zero-profile `Verify` branch of `lookup` (unreachable in
  production because `LookupPrivate` refuses a zero profile) and the legacy
  branches of `compareStored` (format-change refusal, epoch check, digest
  and live-slot ordering) that could fire only for a restored legacy record.

An audit of every production consumer confirmed the residue is reachable
exclusively through `restore()` of pre-v3 on-disk records: Node Resolution
(`closed_resolution_listener.go`, `closed_resolution_operation.go`) calls
only `OpenStore`, `PublishPrivate`, and `LookupPrivate` with a non-zero
profile, and the Endpoint composition uses only the private v3 issuance and
verification path. No test constructs a persisted legacy record. The whole
surface therefore stays alive solely to decode disk bytes that no supported
operation can produce anymore, while silently sharing the 128-Target root
with private v3 records and blocking same-Target v3 publication through a
format-change refusal.

## Decision

1. **Typed refusal at the stored-record envelope.** `reachability` exports a
   new sentinel `ErrLegacyRecord`. `decodeStored` recognizes the retired
   generation-2 envelope (stored-record version byte 1) only to return this
   sentinel; `restore()` propagates it wrapped with the record name instead
   of collapsing it into the generic invalid-record error, so `OpenStore`
   fails with an `errors.Is`-matchable cause. The `storeRecordVersion`
   constant survives solely as the recognized retired-envelope marker.
2. **Whole-root refusal, bytes preserved.** A root holding even one legacy
   record refuses to open as a whole. The Store never rewrites, converts,
   adopts, or deletes the historical bytes; they stay on disk untouched and
   unread. Removing them is an explicit operator decision, after which the
   root reopens normally with its valid private floors intact.
3. **New-Target policy.** A deployment that still holds a legacy root and
   needs private v3 reachability provisions a fresh root. There is no
   adoption or migration operation, consistent with the one-supported-
   runtime-version rule and the no-backward-compatibility decision.
4. **The v1/v2 decode grammar is deleted.** `descriptor.go` is removed
   entirely. `contract.go` loses `MaximumDescriptorSize`,
   `maximumAuthorization`, `SubmissionMode` and both grant constants, the
   `Introduction` type, and the `Descriptor.Introduction` member; the
   private v3 recipient is the only signed live-slot fact the package can
   compose or verify.
5. **Store internals contract to the private grammar.** `encodeStored`
   writes only the private envelope and refuses any non-v3 descriptor.
   `decodeStored` authenticates only private records (floor time derived
   from the signed private recipient expiry). `lookup` requires a non-zero
   profile and always re-verifies with `VerifyPrivate`; its dead zero-profile
   branch is gone. `compareStored` keeps the retained generation, overlap,
   conflicting-expiry, and publication-digest floors and then delegates to
   `comparePrivateRevision`; its format-change, epoch, digest-identity, and
   live-slot branches die with the records that could reach them.
6. **The redundant issuance clone dies.** `cloneDescriptor` existed for the
   legacy `Introduction.SubmissionAuthorization` slice; the private issuance
   path already builds every variable-length field as a fresh copy, so
   `IssuePrivate` returns its descriptor directly.

## Consequences

- A Node Resolution duty whose root predates the private v3 store now fails
  startup with the typed `ErrLegacyRecord` cause instead of silently
  carrying unreadable floors. Under the accepted no-support policy this is
  the intended outcome; the operator answer is a new root.
- The credential generation, publication digest, expiry, and conflict floors
  inside legacy records are unrecoverable through this codebase by design.
  Nothing else in the working tree can read them; the bytes remain for
  out-of-band forensic inspection only.
- The `LookupPrivate`/`compareStored` floor semantics for private records
  are unchanged; the retained-floor guarantee for the supported generation
  is intact across restart.
- The `naming_resolution_retirement` architecture guard flips from reading
  `descriptor.go` to asserting its absence, and additionally pins that the
  legacy contract surface (`SubmissionMode`, `Introduction`) and the legacy
  verifier/decoder stay gone.

## Verification

- `legacy_record_refusal_test.go` (Linux suite): a root seeded with a
  version-1 envelope record refuses `OpenStore` with an `errors.Is(err,
  ErrLegacyRecord)` match and the record bytes are byte-identical afterward;
  a mixed root holding a valid published private record plus one legacy
  record also refuses whole, and removing only the legacy bytes reopens the
  root with the private floor still `StoreAlreadyCurrent`.
- The former negative assertion that the legacy verifier rejects a private
  descriptor is deleted with the verifier it called.
- Gates: `gofmt`, Windows `go build`/`go vet`/`go test` (45 packages ok),
  Linux `go vet` plus test compilation of all five commands and every
  touched package, `staticcheck`, exact `make deadcode` (no allowlist
  change; the deleted symbols simply vanished), and `make quick-check`.
