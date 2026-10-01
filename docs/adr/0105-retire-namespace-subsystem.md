---
status: accepted
date: 2026-09-26
supersedes: 0090
---

# ADR-0105 — Retire the Namespace subsystem; absence of any reader is the incompatibility

## Context

F-51 asked how C0 must treat Namespace roots that ADR-0090 had retained on
disk while forbidding every network command that could create or use them:

- the whole `internal/naming/namespace` tree (claim, epoch, record, authority,
  admission, Store) had no production caller outside its own packages; the
  only composed entry point, the resolution gateway, was already deleted by
  ADR-0100;
- `internal/custody` carried three Name-signing operations
  (`sign-namespace-transition`, `prepare-namespace-submission`,
  `activate-recovered-authority`) that no maintained command ever exposed,
  plus their Operation fields and Receipt `Proof`/`Submission` payloads;
- `internal/service/reachability` still exported the generation-2 writers
  `Issue`, `Store.Publish`, and `Store.Lookup` with `IssueInput` and the
  private helpers `encodeBody`/`verifyStored`, all uncalled since the private
  v3 path became the only issued format;
- the deadcode registry carried 218 allowlisted symbols waiting on this
  decision (seven namespace tracer groups, the retained uncomposed private
  Resolution closure, and the reachability tracer group).

The Product Owner settled the blocking fact: no Namespace root was ever
created for a user promised data support, and neither support nor backward
compatibility is required («нет. никакой поддержки не нужно.
Обратной совместимости тоже»). Export or migration tooling would have created
a new supported read path for a subsystem that is being retired.

## Decision

1. **Typed incompatibility is the absence of any read path.** The whole
   `internal/naming/namespace` tree is deleted. A pre-existing Namespace root
   is not read as any current format, not converted, and not deleted: its
   bytes stay on disk untouched and no working-tree code can open them. The
   incompatibility error is the missing reader itself; no detector, sentinel,
   or refusal branch is added. This revises the retention clause of ADR-0090,
   which is superseded in that part; ADR-0090's command retirements and
   refusal behavior stand.
2. **Custody loses the never-exposed Name-signing operations.** The three
   operations, their Operation grammar fields, and the Receipt
   `Proof`/`Submission` payloads are deleted
   (`vault_namespace_preparation.go`, `vault_reconciliation.go`,
   `vault_namespace_signing_test.go`). The `AuthorityName` record kind stays
   in the vault's generic grammar without any naming import: existing vault
   records remain inspectable, verifiable, exportable, and purgeable, and a
   restored quarantine record stays terminal and export-only.
3. **The uncalled reachability generation-2 writers retire with it.** Per the
   registry groups' own superseding-service-disposition rule, `Issue`,
   `Store.Publish`, `Store.Lookup`, `IssueInput`, `encodeBody`, and
   `verifyStored` are deleted; `verifiedCurrent` moves into the Linux-only
   private issuance file that is its sole remaining caller, and the
   zero-caller `Verified.Authority` getter is removed. The retained v1/v2
   decode grammar and `compareStored` floor comparison are untouched and
   remain governed by F-32; the private v3 issue/lookup path is unchanged.
4. **`internal/naming` and the refusal surface are unchanged.** `name encode`,
   canonical Name parsing, and every retired-command refusal stay exactly as
   ADR-0090 fixed them. The `IsDescendant` helper is deleted: the Namespace
   packages were its only production consumer.
5. **Evidence becomes synthetic.** The retirement oracle
   (`TestNameNetworkCommandsRetireBeforeEffects`) keeps its original
   assertions; its fixture now writes one synthetic root shaped like the
   retired Namespace store (`retiredNameSyntheticNamespaceRoot`) and proves
   the tree stays byte-for-byte unchanged across every retired command. The
   architecture guard (`naming_resolution_retirement_test.go`) is rewritten
   to assert absence: both naming transport directories gone, the custody
   files and Operation constants gone, the generation-2 writer signatures
   gone, the F-32 reader path retained, and the fixture free of every
   removed import.

## Consequences

- The deadcode registry contracts from 253 to 35 common symbols: 218
  allowlisted symbols retire (191 across the seven namespace tracer groups,
  22 in the private Resolution closure, 5 in the reachability tracer group),
  and the Windows production reachability allowance shrinks to nothing.
- `tests/profiles/deterministic-packages.txt` drops the seven namespace rows
  (55 to 48); `docs/development/package-map.md` drops the seven registry
  entries and updates the `cmd/ardents`, `internal/custody`, and
  `internal/service/reachability` rows; `repository-package-graph.csv`
  records custody 19/5 (no naming imports), naming incoming 2, reachability
  test 6; `repository-file-map.csv` loses 89 rows and gains the Linux-only
  `fixture_linux_test.go` row.
- `docs/technical/naming.md` is rewritten around the retirement;
  `docs/reference/commands.md`, `docs/technical/release-update-custody.md`,
  `docs/technical/private-reachability.md`,
  `docs/technical/alpha-control-transition.md`, and the behavior map record
  the absent-reader evidence and the terminal export-only quarantine
  restoration.
- Old Descriptor roots already stored on disk remain a live question: the
  decoder and floor comparison stay, and their disposition remains F-32.
  F-51 is closed.

## Verification

- `go test ./cmd/ardents/ -run TestNameNetworkCommandsRetireBeforeEffects
  -count=1` (the original oracle against the synthetic root).
- `go test ./internal/architecture/ -run
  "TestPrivateResolutionTransportPackageIsAbsent|TestNamespaceRetirementRemovesUnexposedWriters|TestResolutionRetirementPreservesRefusalAndBoundedFixture"
  -count=1`.
- Full gates at acceptance: gofmt, Windows build/vet/full test (46 packages
  ok), GOOS=linux vet and `go test -c` for all five commands, staticcheck,
  `make deadcode` (exact allowlist match), `make quick-check`.
