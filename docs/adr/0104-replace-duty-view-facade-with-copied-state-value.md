---
status: accepted
date: 2026-09-26
supersedes: 0101
---

# ADR-0104 — Replace the 38-getter duty facade with one copied State value

## Context

F-07 recorded the State-to-Node duty handoff as a data-transfer Interface:

- `node.DutyView` declared 38 `Duty*` getters in `internal/node/contract.go`;
- `state.NodeDutyView` implemented them from a fully copied broad `Snapshot`
  in `internal/network/state/node_duty_view.go`;
- `node.currentFacts` immediately re-copied every getter into a private
  `dutyFacts` value plus candidate and authority arrays, then rejected the
  copy with four validation strings;
- `DutyAuthorityCount/ID/PublicKey` were copied into `dutyFacts` but had no
  production read after that copy, and three `DutyTransitIssuance*` methods
  had no non-test callers at all;
- ADR-0101 had already moved the reverse `dutyFacts` getter projection into
  test scope, leaving two production getter layers around one copied value.

The compatibility audit required before retiring the authority projection
is settled: Transit Grant verification retired with the Route v2 closure
(ADR-0093), and the disposition of the persisted spend ledger is F-53 in
`internal/network/duty` — an obligation of that owner, not of this handoff.
`cmd/ardents-node` is the only production caller of `CurrentNodeDuty`.

## Decision

1. **One copied value type.** `internal/network/state/node_duty.go` defines
   `NodeDuty`: Epoch identity and freshness (generation, Network, Epoch,
   digest, validity window, profile, fresh/conflicting flags), the local
   signed record and its assignment, and bounded candidates
   (`CandidateCount uint8`, `Candidates [64]NodeDutyCandidate`). It holds no
   nested Snapshot, no State handle, and no Source, retry, or pending
   metadata. `ProjectNodeDuty(snapshot Snapshot) NodeDuty` is a pure
   projection; `(*networkState).CurrentNodeDuty` composes it with the
   authenticated `Current()` check. `node_duty_view.go` and all 38 getters
   are deleted.
2. **Node seam is the value.** `node.Config.Current` becomes
   `func() (state.NodeDuty, error)`. The `DutyView` Interface, `dutyFacts`,
   `dutyCandidate`, and `dutyAuthority` types are deleted. Node revalidates
   the received value's bounds (candidate count within its array) and
   retains its per-poll copy exactly as before; the receipt-time validation
   strings change, and no test asserted the old ones.
3. **Unused projections retire.** The authority fields and the three
   Transit-issuance getters are not carried into `NodeDuty`. Closed duties
   continue to consume candidate values; the role join stays as fixed by
   ADR-0103.
4. **ADR-0101's test-scope projection retires with the seam.**
   `internal/node/duty_facts_projection_test.go` is deleted; the
   architecture guard now asserts the value seam: the copied-value
   `Config.Current` callback, the State projection and `CurrentNodeDuty`
   signatures, the receipt-time bound validation, the composed command
   wiring `runtime.node.Current = store.CurrentNodeDuty`, and the absence
   of the retired getter files.
5. **Behavior-test input schema unchanged.** The Endpoint role-process
   child `input.json` keeps its `Snapshot` field — the qualification
   heapdump report depends on `input.json:Snapshot.NetworkID` markers — so
   fixtures thread the snapshot explicitly and the child projects it with
   `state.ProjectNodeDuty`. The 170-line all-getter
   `textNetworkDutyFixture` is deleted.

## Consequences

- The seam now carries one State-created immutable value, copied by
  assignment and bounded again by Node, in the existing packages; no new
  package or reverse import was added. Adding a duty fact is now one struct
  field in one place instead of two getters in three files.
- Node can no longer mirror State's field list through an Interface; the
  deadcode ledger loses the whole 38-getter tracer class permanently.
- Node test fixtures construct `state.NodeDuty` values directly; roughly
  forty test files were mechanically migrated with no behavior change to
  any asserted outcome.
- F-53 remains the owner of the persisted duty ledger disposition; nothing
  in this handoff reads or writes that root.
