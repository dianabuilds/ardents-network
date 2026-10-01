---
status: accepted
date: 2026-09-26
supersedes: 0095, 0096
---

# ADR-0106 — Retire the Entry Invite subsystem; the Invite-root writer stops at a before-effect refusal

(F-08. The supersession is partial: ADR-0095's attachment-machinery retirement
and ADR-0096's issuance-surface rejection stay in force; only ADR-0095's
journal-schema retention clause and ADR-0096's `validateInvite` retention rule
are superseded.)

## Context

F-08 asked how C0 must treat the existing Invite roots that the dispatchable
`entry import` and `entry recipient` commands could still open and write:

- `internal/entry` carried two disjoint populations behind one package: the
  Invite subsystem (recipient-bound Entry Invite v2 decoding and validation,
  the recipient TLS identity, admission-history compatibility evidence, the
  attempt/contact journal, a two-slot replacement/replay set, and the
  `Open`/`Import`/`Close` owner) and the closed Entry set store
  (`OpenClosedSets`) owned by the Linux text Endpoint. The Invite machinery's
  only production consumer was the two operator commands in `cmd/ardents`;
  the closed set's only consumers are the Linux endpoint runtime and the
  qualification fixture command.
- The Route-side consumers were already gone: ADR-0092/ADR-0093 removed
  `OpenEntryAttachment` and the v2 execution closure; ADR-0095 retired the
  then-uncalled attachment machinery but retained the attempt/contact journal
  schema as decodable; ADR-0096 rejected the closed-alpha
  issuance/verification surface but retained the private `validateInvite`
  classifier for its live Import/reopen callers.
- The duty conflict ledger (`ReadConflict`/`store.Conflict`) had one
  Windows-visible caller chain left - the Invite import path - beside its
  Linux caller in the endpoint's closed Entry selection.
- The operator-surface audit classified `entry import` as the one route still
  writing an older durable format, and the first operator-surface reduction
  (F-08) was to decide the Invite root's data obligation and stop the writer.

The Product Owner's blanket direction from the F-51 decision applies: legacy
is dying, and neither support nor backward compatibility is required
(«нет. никакой поддержки не нужно. Обратной совместимости тоже»). A retained reader, converter, or migration
tool would have created a new supported read path for a subsystem that is
being retired.

## Decision

1. **Typed incompatibility is the absence of any read path** - the ADR-0105
   disposition applied to the Invite roots. An existing Invite root (its
   marker, lock, recipient identity file, state generations, and
   current/watermark pointers) stays on disk byte-for-byte. No code reads,
   converts, deletes, or migrates it.
2. **The Invite subsystem is deleted.** Eighteen files leave
   `internal/entry`: twelve Invite production files (`contract.go`,
   `open.go`, `import.go`, `invite.go`, `validation.go`, `verification.go`,
   `revalidate.go`, `state.go`, `admission_history.go`, `result_json.go`,
   `recipient.go`, `persistence.go`), three Invite tests (`entry_test.go`,
   `import_recipient_test.go`, `recipient_test.go`), and the three Windows
   platform twins whose Unix counterparts become Linux-only files. The
   ADR-0095 journal-schema retention clause and the ADR-0096 `validateInvite`
   retention rule are superseded: the schema and the classifier are deleted
   with the machinery that owned them.
3. **Both commands refuse before effects.** `entry import` and
   `entry recipient` return one sentinel - "entry Invite command is retired;
   the closed Entry set has no operator import surface" - at dispatch, before
   remaining arguments are interpreted, any plan file is read, or any root is
   created, following the ADR-0090 refusal pattern in the name route.
   Unknown or incomplete `entry` forms keep their usage error. The retirement
   oracle proves zero effects for a fully shaped legacy plan and for an
   absent plan: sentinel error, empty output, unaltered plan bytes, and no
   root, local-role, or network-state directory created.
4. **The surviving package serves only the closed Entry set.** Every
   remaining `internal/entry` source carries the linux build tag; one untagged
   `doc.go` keeps the package compiling (empty) on every platform, matching
   the existing Linux-only package precedent. `root.go` and `atomic_files.go`
   contract to the durable primitives the closed-set store actually calls
   (`inspectRoot`, `validateRootPermissions`, `writeExclusive`,
   `writeGeneration`, `replaceCurrent`, `replaceOwnedFile`,
   `cleanupGenerations`, `readBounded`, `sha256Hex`, `stateName`,
   `maximumStateBytes`). The closed-set root keeps its own marker and
   allowed-name inspection, so a legacy Invite root can never be claimed.
5. **The duty conflict ledger keeps its Linux caller.**
   `ReadConflict`/`store.Conflict` stay in production through
   `internal/endpoint/text_source_state.go`. With their Windows-visible
   Invite caller gone, both symbols join the Windows deadcode allowlist
   group "selected Linux-only protected text runtime closure"; the
   GAP-6 store-side regression tests stay in `internal/network/duty`.
6. **`cmd/ardents` contracts accordingly.** `entry_import.go`,
   `entry_import_plan.go`, and `entry_import_test.go` are deleted;
   `freshOperatorRegularFile` dies with its only caller; the entry network
   fixture file reduces to the shared command-wire helpers under the neutral
   name `command_wire_fixture_test.go`. `cmd/ardents` no longer imports
   `internal/entry` or `internal/network/duty`.

## Consequences

- F-08 closes: no Invite writer remains in any supported runtime, and the
  Invite roots' data obligation is resolved as refusal, not migration.
- `internal/entry` shrinks from 24 production + 5 test files to 9 + 2 with
  standard-library-only dependencies (the Windows `golang.org/x/sys/windows`
  dependency is gone); `cmd/ardents` from 24 to 23 production files.
- The package map drops `cmd/ardents -> internal/entry` and
  `cmd/ardents -> internal/network/duty`; the package-graph CSV also drops
  the stale `internal/route -> internal/entry` claim; the file map loses 22
  rows and gains 3 (`entry_retirement.go`, `entry_retirement_test.go`,
  `command_wire_fixture_test.go`) plus three unix-to-linux renames.
- The ADR-0095/ADR-0096 architecture guards are rewritten as absence
  inventories; a new ADR-0106 guard asserts the refusal surface, the
  Linux-only entry package structure, and that no `cmd/ardents` file
  regains an entry import.
- Docs updated: command reference rows, the command-surface audit (now eight
  immediate retirement refusals), the network-route-node module row and
  Attachment paragraphs, the private-admission refusal note, the
  behavior-map row and its rewritten source trace, and the c0 component
  reconstruction rows.
- The Windows deadcode allowlist gains the 2 duty-ledger symbols; no Invite
  symbol remains anywhere in the registry. F-53 (the duty spend-ledger
  schema) is untouched: the duty root decoder stays its own concern.

## Verification

- Windows: gofmt clean; `go build ./...`; `go vet ./...`; full
  `go test ./...` green.
- Linux (GOOS=linux, GOARCH=amd64): `go vet ./...` green; `go test -c`
  succeeds for `cmd/ardents`, `cmd/ardents-control`, `cmd/ardents-node`,
  `cmd/ardents-qualification`, `cmd/ardents-custody`, and `internal/entry`.
- staticcheck clean; `make deadcode` exact in both the production and the
  test analysis; `make quick-check` green.
- Guards: `TestEntryInviteWriterIsRetired`,
  `TestClosedEntrySetPackageIsLinuxOnly`,
  `TestEntryInviteCommandsRetireBeforeEffects`,
  `TestEntryRouteRefusesUnknownAndIncompleteForms`,
  `TestEntryImportRouteReturnsRetirementRefusal`, and the rewritten
  ADR-0095/ADR-0096 absence inventories.
