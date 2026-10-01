# 0101 — Test-scope the dutyFacts DutyView projection; retire the unimplemented admission-deadline helpers

Status: accepted

## Context

The cite-or-die sweep (ADR-0091) reached the last two Node-side deadcode
registry groups that shared one factual claim: "no selected C0 command
starts a Node process." That claim is false. `cmd/ardents-node/main.go`
dispatches `ardents-node node --config PATH` to `runNode`, which opens the
State root and calls `node.Run`; `docs/technical/network-route-node.md`, the
current maintained fact owner, describes the Node command and its lifecycle
as implemented. The deadcode analysis itself runs over `./...`, so the whole
composed closure is production-reachable and only the exact listed symbols
are dead.

Two groups rode on the false claim:

1. **"unwired native-node tracer" (37 symbols).** All were `dutyFacts`
   `DutyXxx` accessors in `internal/node/contract.go`. `dutyFacts` itself is
   live production machinery: `currentFacts` copies all 38 `DutyView`
   getters into it and every closed listener, admission assessment, and
   bootstrap path consumes its fields. The accessors existed only so that
   `dutyFacts` satisfied `DutyView` — a conversion that happens exclusively
   in behavior tests, which return a snapshot from `Config.Current` without
   a Network State runtime. Production supplies `state.NodeDutyView`. The
   38th getter, `DutyRecordGeneration`, already lived in
   `lifecycle_test.go` under the comment "Test snapshots implement the
   external DutyView seam; production projects authenticated State", so the
   production copy of the other 37 was a split, dead-weight duplicate of a
   test seam.
2. **"retained Node admission-deadline helpers" (2 symbols).**
   `validAdmissionTimeout` and `boundedAdmissionDeadline` modeled a local
   admission-timeout configuration bounded by State's `NotAfter`. No such
   configuration exists; production deadlines read `receiver.NotAfter`
   directly and are State-bounded by construction. The helpers had no
   caller outside their own unit tests.

Neither group was a planned subsystem awaiting wiring, so neither needed a
product-scope decision; both were sweep-level hygiene hiding behind an
incorrect rationale.

## Decision

1. **Move the projection to test scope.** The 37 accessors move from
   `contract.go` to `internal/node/duty_facts_projection_test.go`, joined by
   `DutyRecordGeneration` from `lifecycle_test.go`, so the complete
   38-getter projection lives in one test-scope file. The `DutyView`
   interface, the `dutyFacts` struct, and `currentFacts` are unchanged;
   behavior tests are unchanged; the production build loses 37 methods that
   were never dispatched in it. The `dutyFacts` doc comment now states the
   seam explicitly.
2. **Retire the admission-deadline helpers.** `admission_deadline.go` and
   its unit tests are deleted. The persisted-behavior evidence for State-
   bounded deadlines remains in the closed forwarding/resolution/issuer
   behavior tests, which exercise the production `receiver.NotAfter` path.
3. **Dissolve both registry groups.** The common list drops 39 symbols:
   common 292 → 253, windows-amd64 496 → 457, linux-amd64 293 → 254. The
   exact-count gate confirmed every prediction.
4. **Guard the outcome.**
   `internal/architecture/node_duty_projection_test.go` forbids any
   `dutyFacts` accessor in production `contract.go`, requires the complete
   38-getter test projection, keeps the retired helper files absent and the
   dissolved classifications out of the registry, and preserves the
   composed handoff: the `node` dispatch, `store.CurrentNodeDuty` wiring,
   `node.Run`, the lifecycle copy projection, and State's
   `CurrentNodeDuty` view.

## Consequences

- The remaining F-07 seam assessment in the reconstruction ledger is now
  honest about what is left: the duplicated getter facade is only the
  State-side `NodeDutyView` projection plus the `DutyView` interface. The
  bounded F-07 value-shape refactor (a copied `state.NodeDuty` value
  instead of a 38-getter interface) remains open and would also retire the
  test projection file; nothing in this decision blocks it.
- The false "no selected C0 command starts a Node process" sentence is
  corrected wherever the registry groups repeated it
  (`docs/development/testing.md`).
- The remaining Node-related registry entries are platform allowances and
  the reachability group (blocked on the Namespace disposition); no Node
  group survives in the common list.
- Registry totals after this ADR: common 253, windows 457, linux 254.
