---
id: R-123
title: Which alpha transition facts must remain separate and inspectable?
status: decided
owner: Product Owner and Codex
started: 2026-08-29
reviewed: 2026-08-29
---

# R-123 — Which alpha transition facts must remain separate and inspectable?

## Decision this unlocks

H4-6B can freeze an inspectable Functional Alpha transition contract without
turning the first project's keys, catalog, VPS, or reader into one permanent
authority.

## Current contract

ADR-0038 fixes an enrollment-pinned ACA1 disclosure catalog and independently
verified Release, Network, and Compatibility components. Release has retained
root/floor and emergency mechanics; Network State owns Epoch successors; and
Compatibility binds their selected tuple. The canonical Namespace still has no
selected authenticated global close. Target Links remain the complete alpha
destination path.

## Hypotheses

- **H1:** A fixed, non-authorizing declaration over the existing four boundaries
  makes transition authority and failure visible without introducing a new
  control mechanism.
- **H2:** Reusing the catalog or one alpha key as authority for all four areas is
  sufficient.
- **H0:** Existing retained mechanisms cannot state explicit outcomes for all
  required failures.

## Evaluation criteria

The result must state, for each area, authority root, predecessor, freshness,
rotation, revocation, rollback floor, emergency action, participant-visible
failure, and retained evidence. It must reject forged, stale, replayed,
revoked, conflicting, withheld, and unavailable inputs without fallback; it
must leave current Namespace absent unless a real authenticated close is
selected; and two fresh Endpoint observations must agree.

## Evidence plan

### Primary sources

- ADR-0004, ADR-0006, ADR-0038, ADR-0043, and ADR-0053, accessed 2026-08-29.
- Maintained `internal/release`, `internal/network/state`,
  `internal/alphacontrol`, and `internal/naming/namespace`, inspected
  2026-08-29.

### Experiment

Run the Linux process seam
`TestAlphaControlTransitionsTwoFreshEnrolledEndpointsAgree` from separate fresh
Endpoint and reader roots. It exercises exact enrolled bytes and the separate
`inspect-transitions` reader projection without network access. Reproduce on a
Linux host with `go test ./tests/e2e/endpoint -run
'^TestAlphaControlTransitionsTwoFreshEnrolledEndpointsAgree$' -count=1`.
The test owns and removes both Endpoint roots, both reader roots, artifacts,
and processes; no generated evidence is retained in the repository. The
completed 2026-08-29 development run used a network-disabled Docker container
limited to 1 vCPU, 1 GiB, and 128 PIDs and passed. This is a deterministic
process cell, not a published-candidate qualification receipt.

### Failure scenarios

Forged, stale, replayed, revoked, conflicting, withheld, and unavailable
inputs must lead to a domain-local failure. Withholding cannot be distinguished
from an unavailable distributor by an Endpoint, so both render `unavailable`.

## Findings

- **Sourced fact:** the catalog is an index and not a component authority
  (ADR-0038).
- **Measurement:** two fresh Endpoint processes and two separate fresh reader
  roots produced byte-identical H4-6B reports on 2026-08-29.
- **Inference:** a fixed report can reveal the separation without adding a
  generic governance/signing interface.
- **Decision:** Namespace materialization is explicitly absent in the current
  Functional Alpha. No project statement may close, release, reclaim, or
  materialize a canonical Name.

## Options

1. Choose real authenticated close/release/reclaim now — rejected: there is no
   selected global-close producer, authority operation, or independent evidence.
2. Let alpha control act as a temporary Namespace registrar — rejected: it
   creates hidden discretionary authority.
3. Keep Namespace absent and disclose the four boundaries — selected.

## Recommendation

Choose option 3 with high confidence. The strongest limitation is that it
does not make human-readable canonical Names usable; Target Links and the
separate bounded alpha corpus remain the only current paths.

## Disposition

Decided and promoted through ADR-0054, the alpha transition technical contract,
H4-6 product owner, command reference, and maintained tests. H4-6C threshold
custody and public control remain separate future gates.
