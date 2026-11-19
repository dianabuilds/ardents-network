---
status: accepted
date: 2026-11-19
supersedes: ADR-0081 (platform-selection clause only)
---

# ADR-0126 — Admit the platform by capability floor, not exact version pins

Status: **accepted** by the Product Owner on 2026-11-19 in the active
conversation («да принимаю ADR-0126»). This is the durable form of the PO
direction recorded in #643. Acceptance selects the contract; the runtime code
migration to the floor remains a separately gated implementation slice under
the C0 work-in-progress rules (#643) and is not itself performed by this ADR.

## Context

The Product Owner stated on 2026-11-19 that the requirements carry no binding to
specific OS versions and that the network should be maximally universal:
«не должно быть привязки под версию системы (ну может кроме старых)… как минимум
все 22+ должны работать», restated as «В требованиях нет привязки к конкретным
версиям. Более того, сеть должна быть максимально универсальна».

The product contract already declares this direction: `docs/product/scope.md`
selects an «Ubuntu LTS x86-64» artifact with no version pin and names Linux,
macOS and Windows as product directions; ADR-0125 repeats that the closed Ubuntu
profile is distinct from the broader product direction.

The exact-version pins in code implement ADR-0081's bounded closed-alpha
platform clause («Select Ubuntu 24.04 LTS x86-64, systemd 255 and cgroup v2»),
not a product requirement. The tree is internally inconsistent: the Endpoint
platform check has admitted Ubuntu 22.04/systemd 249 since f48fcdd0 without an
ADR or owner amendment, while the join/forwarding/descriptor/infrastructure
role runtimes still refuse everything but 24.04. PR #642 (#641) made the
Endpoint worker support both admitted pairs and tolerate the Ubuntu 22.04
hybrid `name=systemd` cgroup layout.

Ubuntu 22.04's systemd always creates the redundant v1 `name=systemd` named
hierarchy even on a unified boot; this is a permanent platform fact, not an
edge case. A frozen version whitelist also refuses every future release until
code edits add it, which contradicts the universality direction and the
one-human maintenance capacity.

## Decision

Amend ADR-0081's platform-selection clause only. All other ADR-0081 protection
and acceptance obligations remain in force.

1. **Admission floor.** Admit Ubuntu `ID=ubuntu` with `VERSION_ID >= 22.04` and
   a systemd system manager major `>= 249`, on the amd64 architecture. Older
   versions remain excluded.
2. **Capability grammar, not version equality.** Replace exact-version
   equality with fail-closed capability/grammar checks. A platform is admitted
   only where it demonstrably supplies the required confinement mechanisms
   (distribution-maintained systemd system manager, cgroup v2 placement,
   kernel namespace/seccomp, root-owned installed units, DynamicUser worker
   isolation). Per-version property differences are tolerated explicitly as
   secure defaults (for example systemd 249 omits the encrypted/imported
   credential properties and `ExitType`/`RestartMode`); an unknown or unsafe
   layout refuses before effects. Admission is never version-label trust.
3. **cgroup layout.** Admit unified cgroup2, and the Ubuntu hybrid layout where
   the redundant v1 `name=systemd` tracking line carries no controllers and the
   unified `0::` placement remains exactly verified (the #641 guarantee). A
   missing, repeated, or altered unified line refuses.
4. **One floor across roles.** The join/forwarding/descriptor/infrastructure
   role runtimes move to the same floor as the Endpoint platform check; no role
   keeps a narrower exact-version pin.
5. **Qualification lanes.** Qualification evidence lanes remain the LTS pairs
   (22.04/249 and 24.04/255). The admission floor accepts 22+, but intermediate
   non-LTS releases (9-month EOL) are not separately qualified unless the PO
   selects them; this matches the one-human capacity and is not a support claim.

## Consequences

- Future Ubuntu/systemd releases are admitted by demonstrated capability rather
  than by editing a whitelist; the product evolves across releases without a
  code change per release.
- The confinement owner (`docs/technical/application-confinement.md`) becomes a
  per-version capability grammar instead of a single-version narrative; its
  systemd 255 source references remain the 255 reference behavior with explicit
  249 tolerances.
- The infrastructure (Source/Forwarding) code that still pins 24.04 moves to the
  floor as a separately gated implementation slice; this ADR does not itself
  change runtime code.
- Broader Linux/macOS/Windows universality remains the product direction via the
  portable-core workstream (#602/#606/#609); this ADR qualifies no native runtime
  on those platforms and makes no public platform-support claim.
- The fail-closed security invariant is preserved: a platform that cannot
  demonstrate the confinement guarantees refuses; universality widens admission,
  never weakens it.

## Compliance

The contract is accepted; the implementation that realizes it still owes the
platform-owner contract update and causal admission/refusal tests per lane
(positive admission and root-writer/unsafe-layout refusal), with real Ubuntu
22.04 evidence. Prerequisite: #641 / PR #642 merged and proven on a real 22.04
host. Implementation selection follows the C0 work-in-progress rules and the
#643 ledger; this ADR starts no slice.