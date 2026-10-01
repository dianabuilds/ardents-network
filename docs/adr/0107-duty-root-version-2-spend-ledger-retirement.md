---
status: accepted
date: 2026-09-26
supersedes: 0093
---

# ADR-0107 — The duty root schema becomes version 2; legacy generations convert in place and the Transit Grant spend ledger retires

(F-53. The supersession is partial: ADR-0093's Route v2 execution closure and
its `SpendTransitGrant` retirement stay in force; only its point-3 clause
retaining "the persisted v1 receiving one-use Transit Grant spend-ledger
schema, its validators, and `Replace` filtering" pending a separate data
disposition is superseded by this decision.)

## Context

F-53 was the last Route v2-era data disposition. ADR-0093 deleted the spend
operation and its Grant verifier but kept the persisted ledger schema because
the duty root itself is live production data, not a dead historical store:

- `internal/node/local_roles.go` and `internal/network/state/local_roles.go`
  open the root with `Create: true`; the Endpoint qualification preflight
  opens it; `internal/endpoint/text_source_state.go` reads conflict truth
  through `duty.ReadConflict`. Conflict duties drive the current closed Entry
  selection, so the root's bytes are a current admission input.
- The spend side of the schema, by contrast, has no reader at all: since
  ADR-0093 no non-test production caller touches `SpendTransitGrant` or the
  `transit_grant_spends` records; `Replace` only carried them forward.

This is the opposite of the F-51/F-08 situation. The Namespace and Invite
roots became dead data — absence of any read path was the whole typed
incompatibility — but abandoning or refusing the duty root would destroy live
conflict truth on every upgraded node. The reconstruction findings permit
exactly this shape: "a bounded old-root conversion or explicit typed refusal"
that preserves "duty conflict records and generation continuity". The PO's
blanket direction ("no support of any kind, no backward compatibility")
covers the spend records themselves: data with zero readers is dropped, not
carried.

## Decision

1. The one current schema is version 2: `durableState` carries `version`,
   `generation`, `previous`, and `duties` only. No `transit_grant_spends`
   field exists in the written grammar. Every writer — including the first
   generation committed by a `Create: true` open and every `Replace` —
   commits version 2.
2. Bounded in-place conversion replaces the retention clause: `loadGeneration`
   strictly decodes a version-1 generation (unknown fields and trailing bytes
   refused) under the full historical validation, spend-record rules
   included, then converts it in memory to version 2, keeping every duty,
   the generation number, and the predecessor name and dropping every spend
   record. No writer emits version 1, so the conversion is one-way and the
   first `Replace` after such a load commits a version-2 successor.
3. Recovery guarantees survive the conversion: conflict duties stay
   authoritative, generation continuity and the content-addressed chain are
   untouched, and watermark recovery may legitimately land on a version-1
   generation; the recovered generation keeps its number and name.
4. Refusal replaces tolerance for bad bytes: an invalid version-1 generation
   (including spend-rule violations such as a duplicate GrantID) and any
   unknown version refuse at open, leaving the root unavailable — the same
   failure shape the v1 decoder always had.
5. `Replace` no longer carries spends forward and no longer enforces the
   spend bound; `liveTransitGrantSpends` is deleted. `transitGrantSpend` and
   `validTransitGrantSpends` survive only inside the legacy decode section of
   `persistence.go`, cited by this ADR.
6. ADR-0062's Grant v1 wire grammar is Route-side and was retired with
   ADR-0093; it is out of scope here. The closed Entry set store, the State
   root, and every other persisted schema are unchanged.

## Consequences

- Existing roots keep working across the upgrade: their v1 generations load,
  their conflict truth is intact, and their bytes become version 2 on the
  next write. No operator migration step exists.
- Spend history is unrecoverable after conversion. It had no reader and no
  admission effect since ADR-0093, so nothing observable is lost.
- Rolling the binary back to a pre-ADR-0107 build meets the v1 strict
  decoder's version check and refuses the v2 root. Under the no-support
  direction this typed refusal is the accepted outcome, not a compatibility
  defect.
- The ADR-0093 retirement guard flips: `contract.go` must no longer contain
  `TransitGrantSpends` or `transitGrantSpend`, and `persistence.go` must
  contain `legacyDurableStateVersion` and the `convert` method. The guard
  records both halves of the disposition.

## Verification

- `internal/network/duty/legacy_conversion_test.go`: a legacy generation 3
  with two duties and one spend opens, its conflict is visible through
  `Conflict`, and the next `Replace` commits bytes containing `"version":2`
  and no `transit_grant_spends`; watermark recovery re-points `current` at a
  version-1 generation and keeps its duties readable; a duplicate-GrantID
  spend and version 3 both refuse at open.
- `internal/architecture/route_v2_closure_retirement_test.go` asserts the
  flipped grammar guards.
- Gates: Windows and `GOOS=linux` build/vet, the full Windows test suite,
  `go test -c` for all five commands and the duty package, staticcheck,
  exact deadcode allowlist match (no new dead symbols), and `make
  quick-check` all pass on this commit.
