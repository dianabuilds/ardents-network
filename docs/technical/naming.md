# Naming and private resolution

Status: **Stage 8 M5 technical contract in progress.** This document describes
the maintained `internal/naming/namespace` and `internal/nameresolution`
technical tracer. It is not a supported public Namespace, a public resolver,
or a claim of independent Network Epoch operation.

## Ownership and trust boundary

`naming/namespace` owns canonical Name parsing use, Authority/lifecycle
transitions, Recovery verification, claim evidence verification, admission-owned
naming facts, pending successor persistence, current Namespace materialization,
and local current-proof verification. Its persistence and commitments are
Namespace-owned under R-060; it does not import Network State as a foundation.

`naming/resolution` owns one fixed private OHTTP exchange, its role-local
selection, nonce/replay state, and observer-safe counters. It transports opaque
`namespace.Submission` control bytes and current proofs. Resolution receives a
verified immutable `namespace.Binding`; it does not receive or assemble a
lifecycle `Record` in its production Gateway or Resolver path.

The current flow is:

```text
Custody signs the exact Namespace-derived successor
  -> private Gateway accepts only submitted/denied
  -> Namespace persists the signed successor as pending
  -> authenticated Epoch installation selects a pending prefix and/or ClaimWinner
  -> threshold-attested Store commit publishes current state
  -> private Resolution transports a proof
  -> local VerifyBinding returns an immutable Binding
```

`submitted` is not current state. A current Binding exists only after the
threshold-attested materialization verifies at the local decision time.

## Claim and Epoch boundary

R-042/ADR-0017 require commitment in Epoch `E`, reveal in `E+1`, and the
lowest authenticated input ordinal for a single Name. A `ClaimWinner` is a
process-local opaque result of one authenticated close verification. It derives
the root or reclaim transition from the winner, verified predecessor,
materialization time, and Namespace policy; it does not accept a later raw
proof, arbitrary ordinal, Name, Authority, or lease deadline.

`EpochInstallation` starts from the verified Store snapshot, can select only
the next durable pending prefix, accepts a `ClaimWinner` only through a signer
that returns the exact derived Record, and uses the existing threshold
materialization statement for publication. `AdmitClaimCommitment` consumes one
local R-045 `root-claim` proof and yields an opaque `EpochClaimInput`: its
canonical 64 bytes are the commitment followed by the admitted challenge
digest. The input leaf also binds the Epoch-assigned ordinal. Network/Epoch
code can order and commit only those opaque bytes; it receives no Name,
Authority, secret, or local proof state. A threshold-signed close must still
prove that the revealed claim opens that exact input leaf before
`EpochClaimInput.VerifyClose` can yield a materializable fact for that local
submission. `OpenClaimWinner` remains the proof-only verifier for other
observers. This boundary does not
select a Network log, transport, or shared persistence foundation. An
incomplete or forked close must not mutate a Lease.

An installation captures its current-generation identity. It may publish a
selected pending prefix together with verified claim materialization, but it
fails closed if another current generation has appeared before commit; the
legacy raw `Store.CommitLegacy` is not the typed installation authority.

## Retained technical limits

| Boundary | Enforced current limit | Status / owner |
|---|---:|---|
| Canonical Name V1 | lowercase ASCII labels 1–63 bytes; total ≤253 bytes; depth ≤127 | retained R-041 profile |
| Claim reveals for one Name | ≤32 | retained R-042 measured tracer rule; not total Epoch capacity |
| Claim proof wire | ≤2,048 bytes | retained internal proof envelope |
| Current Namespace proof | ≤4,096 bytes | retained Resolution envelope |
| Current Namespace corpus | ≤127 signed Records | R-066 one-writer technical tracer; not product scale |
| Concurrent exact local readers | 8 in R-066 measurement | measured tracer condition, not a concurrency promise |
| Pending journal | ≤127 entries; each submission/successor ≤64 KiB | Namespace-local restart bridge |
| Static control input | ≤16 KiB | C0 internal control representation pending F031 cutover |
| Signed Record | payload ≤1,846 bytes; container ≤1,920 bytes | R-073 retains 76 bytes below the measured worst-case proof fit |
| Private Resolution request/response | exactly 4,096 bytes | retained R-067 OHTTP envelope |
| Relay OHTTP envelope | ≤8 KiB | role-local transport bound |
| Admission profiles | resolution `16/4096/64`; renewal-update `16/2048/32`; policy-recovery `17/1024/16`; root-claim `18/1024/8` (`work bits/spent/in-flight`) | retained R-045 local amplification guard; not Sybil resistance |

The Record/container ceiling and several C0 construction limits are deliberately
listed together because they are not a compatible product capacity contract.
F031 must replace them with one owned compatibility table and reject a value
before signing when it cannot persist, prove, or traverse the private exchange.
F032 needs a separately measured scale/index decision before the 127-record
tracer can grow.

## Invariants

- A Gateway cannot manufacture a durable/current Record: ordinary control
  carries an Authority-signed exact successor, which Namespace recomputes and
  verifies before journaling.
- Current materialization may advance only from the durable pending prefix
  and/or verified `ClaimWinner` on the new installation path; restart
  reconstructs verified current plus unapplied pending state but never promotes
  pending on its own.
- A `ClaimWinner` materializes only into an installation for the same Network
  and Epoch that authenticated its close; a valid foreign close is not a local
  Namespace authority.
- Durable control rejects late root `claim` operations before consuming a
  Gateway admission proof. Claim admission belongs to Epoch input ingestion.
- Target resolution is valid only while its signed Record validity, own Lease,
  and complete parent lineage remain valid; a V3 target Record is decode-only
  and not resolvable until replaced by V4.
- A verified Binding is not an identity, a Person, an Endpoint location, or a
  privacy guarantee. Private Resolution has the conditions and limitations in
  the threat model; encrypted payloads do not imply anonymity.

## Remaining M5 work

The public `Record`, `Op`, `ApplyLegacy`/`ApplyAtLegacy`, raw
`Store.CommitLegacy`, historical Stage 6
fixtures, and interim Record signing callback remain compatibility surface.
They must be replaced by sealed Namespace constructors and one final
installation Interface. The remaining global-close owner must accept the
opaque admitted input, commit its ordinal/root, and issue the complete
threshold-signed close before it yields a `ClaimWinner`. Scale, index/cache,
product capacity, and supported-platform claims remain outside this tracer.

## Verification

The maintained local gate is `make quick-check`; `make check` is required
before integration. Focused Namespace/Resolution behavior is covered by
`go test ./internal/naming/namespace ./internal/nameresolution -count=1`.

## Decision records

- [R-041](../research/records/r-041-canonical-name-limits.md)
- [R-042](../research/records/r-042-claim-ordering.md) and
  [ADR-0017](../adr/0017-authenticated-name-claim-ordering.md)
- [R-045](../research/records/r-045-anonymous-cost.md)
- [R-057](../research/records/r-057-current-namespace-materialization.md) and
  [ADR-0020](../adr/0020-authenticate-current-namespace-materialization.md)
- [R-065](../research/records/r-065-naming-decision-time.md)
- [R-066](../research/records/r-066-namespace-tracer-envelope.md)
- [R-067](../research/records/r-067-naming-profile-retention.md)
- [R-068](../research/records/r-068-name-record-validity-migration.md)
- [R-069](../research/records/r-069-namespace-submission-result.md)
- [R-070](../research/records/r-070-namespace-pending-successor-record.md) and
  [ADR-0023](../adr/0023-pending-signed-namespace-successors.md)
- [R-071](../research/records/r-071-typed-epoch-claim-winner.md)
- [R-072](../research/records/r-072-namespace-epoch-installation.md)
- [R-073](../research/records/r-073-record-proof-envelope.md)
