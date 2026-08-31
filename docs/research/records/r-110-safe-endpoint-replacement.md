---
id: R-110
title: Safe unprivileged Endpoint replacement
status: decided
owner: Product Owner and Codex
started: 2026-08-25
reviewed: 2026-08-25
---

# R-110 — Safe unprivileged Endpoint replacement

## Decision this unlocks

Select the H4-1B Ubuntu-first participant replacement owner, filesystem
boundary, interruption recovery, and qualification matrix. It must not select
a background downloader, Windows profile, package manager, or public updater.

## Current contract

The selected Ubuntu foreground replacement requires authorized staging,
drain/stop, atomic activation, self-test, interruption recovery, and rollback
protection. Program bytes remain outside config, Vault, floors, cache, and
runtime roots. Authority Custody is never a replacement input.

`internal/release` issues opaque authorization only after authenticating a
decision. `internal/endpoint/replacement` owns the Endpoint-program binding,
journal, retained predecessor, activation, self-test, recovery, and rollback
semantics used by the product command. The former generic `internal/update`
transaction had no production caller and was retired from the C0 candidate;
its last source is immutable at
[`fbb42034757513ac009114a00b933aefa76d8ddf`](https://github.com/dianabuilds/ardents-network/tree/fbb42034757513ac009114a00b933aefa76d8ddf/internal/update).

## Hypotheses

- **H1:** A foreground unprivileged Ubuntu operation can compose Release
  authorization, Update evidence, unit stop/drain, sibling staging, atomic
  activation, candidate self-test, recovery, and explicit restart without
  receiving Authority material.
- **H2:** The Endpoint executable can own its own replacement after the unit
  stops, avoiding a distinct helper component.
- **H0:** Neither path can prove recovery without new bootstrap or platform
  authority; H4-1B must remain unselected.

## Evaluation criteria

- The Owner supplies and confirms one candidate; no polling, download, or
  unattended activation exists.
- Release authenticates candidate bytes before activation; an older build or
  public-looking Decision cannot authorize replacement.
- A live unit is drained/stopped before program-byte replacement.
- The journal is owner-private and separate from Vault/floors/cache/runtime;
  Update receives no password, Recovery Bundle, or Custody writer.
- Each crash boundary returns keep-current, resume-self-test, authorized
  rollback, or repair-required—never an inferred success.
- Candidate self-test runs the exact activated build before new network work.
- No elevation, linger, system service, proxy/DNS/route/VPN change, Windows
  support, or public-release claim is added.

## Evidence plan

Inspect the maintained Release authorization, Update transaction/recovery,
Portable roots, and user-unit boundary. On a clean Ubuntu user account run v1/v2
exact-artifact cases: active/corrupt/rollback refusal, interruption before and
after activation, self-test failure, recovery, and successful restart. Capture
program and Vault/floor digests, unit state, Update result, and residue.

## Findings

- **Source fact:** `update.Apply` accepts opaque `release.Authorization`, has
  no program path or systemd operation, and its public interface cannot select
  the participant replacement semantics.
- **Source fact:** `cmd/ardents` exposes no Update command or candidate source;
  H4-1A's unit has `Restart=no`.
- **Source fact:** the H4-1A unit starts `ardents endpoint enroll` on every
  restart. Its current first-enrollment verifier requires the running
  executable to be byte-identical to the artifact in the independently pinned
  first bundle. Replacing that executable without a successor-run record makes
  the next unit start correctly reject the candidate as non-enrolled.
- **Inference:** Directly exposing the tracer would leave program activation
  and self-test semantics in an unselected caller, contradicting H4-1B.
- **Inference:** H4-1B cannot be only an artifact-store transaction. It needs
  one Endpoint-owned, owner-private record binding the selected program bytes
  to Release authorization. The ordinary unit start must require either the
  first-bundle proof (before any replacement) or that selected-successor
  record; it must not silently treat a stored Release floor as authorization
  for arbitrary program bytes.
- **Measurement:** the disposable foreground-helper prototype on Ubuntu 24.04
  Docker printed five complete states. Active replacement retained v1; an
  interruption after staging retained v1 plus `staged`; an interruption after
  rename retained v2 plus `self-test-required`; self-test failure retained v2
  plus `rollback-authorization-required`; only passing self-test produced
  `committed-restart-permitted`. The synthetic Vault and floor bytes remained
  unchanged in every state. This supports the state ownership shape only, not
  authenticating candidates, systemd control, or crash durability.
- **Implementation evidence:** the maintained Ubuntu-only replacement owner
  now accepts only a bounded local bundle, requires opaque Release
  authorization, establishes a first-enrollment `current` record, retains a
  predecessor, stages the successor in the program directory, atomically
  renames it after the fixed user unit stops, invokes the exact new binary's
  no-network prepared-record self-test, and starts the fixed unit only after
  commit. Its behavior tests cover committed activation, substitution refusal,
  self-test failure, retained predecessor, and recovery classification. Native
  interruption-injection qualification runs under Linux Docker. The public
  command process test drives an initial enrollment, a newer signed local
  target, fixed-unit stop/start calls, a real atomic executable replacement,
  and the candidate's child self-test; its `systemctl` is deliberately a narrow
  recording fixture. A second Linux process case makes the activated v2 a
  valid-but-failing executable: it proves that the normal program path can be
  unusable, that only the `0700` retained predecessor at the journal-bound
  recovery path can invoke `endpoint rollback`, and that a newer authenticated
  TUF/Release authorization for exact v1 bytes restores v1 and triggers only
  the fixed stop/start pair. It also rejects the initial-root form of the
  rollback bundle after durable root v2, requiring the bundle to anchor the
  current root. On 2026-08-25, a disposable non-lingering Ubuntu VPS user ran
  the same participant unit against a real `systemd --user` manager. The
  successful v1-to-v2 path stopped, self-tested, and explicitly restarted the
  fixed unit while retaining Vault and floors. A second run authenticated an
  executable v2 whose self-test deliberately failed; its unit remained
  inactive, and only the owner-only journal-bound predecessor, supplied with a
  fresh v3 authorization for exact v1 bytes, restored and restarted v1. This
  is an Ubuntu qualification of those two paths, not a claim about unattended
  updates, package repair, Windows, or public release.
- **Implementation evidence:** the current owner-level behavior test creates an
  encrypted Authority Vault and a persisted Release-floor root outside the
  replacement state root, then checks their complete file trees byte-for-byte
  after successful replacement, stop refusal, and explicit rollback. This
  proves non-mutation at those exercised boundaries only; it does not prove
  crash, power-loss, permissions, or platform qualification.
- **Implementation evidence:** one completed immediate predecessor does not
  indefinitely block the next Release-authorized successor. The replacement
  owner first proves that the current bytes and committed journal agree, then
  retires only that completed reserve; a failed candidate remains a
  `current-mismatch` plus rollback-required state and cannot be bypassed by a
  new replacement invocation.

## Options

1. Foreground helper beside Endpoint: a purpose-owned unprivileged component
   controls only exact user-unit and sibling replacement, while never opening
   Custody.
2. Endpoint self-replacement: fewer bytes but an unclear post-activation
   self-test owner and self-overwrite lifecycle.
3. Package-manager updater: defer to H4-1D because it adds package, repair,
   uninstall, and possible privilege authority.

## Recommendation and disposition

Select option 1: an explicit foreground `endpoint replace` operation in the
already-authorized Endpoint binary. It accepts one local, bounded replacement
bundle and performs no fetch, polling, or unattended activation. Its durable
Endpoint replacement root is beneath the Portable state home but is distinct
from Release floors, cache, runtime, config, and Authority Vault.

The maintained contract is deliberately small:

1. Before stopping the unit, the foreground command proves that its own bytes
   are either the independently pinned first artifact or the currently
   committed successor. It then evaluates the supplied candidate with the
   existing Release floors; a public `Decision` is not sufficient.
2. It records a private transaction that names the current and candidate byte
   digests, retains a predecessor copy, and stages the candidate beside the
   program path. The final program replacement is an atomic same-directory
   rename. A candidate never shares a root with the Vault or Release floors.
3. After activation, the helper runs the exact replacement artifact in a
   bounded no-network self-test. That artifact verifies that its own bytes
   match the prepared successor record; this makes the unit's later ordinary
   start reject a disk rollback or substitution.
4. Only a successful self-test commits the successor record and permits the
   explicit `systemctl --user start` action. A failed or interrupted test
   remains visible as rollback-pending or repair-required. A broken current
   path cannot be asked to repair itself: the retained owner-only predecessor
   program is the bounded recovery copy. It accepts `endpoint rollback` only
   at that exact journal-bound path and only after a later Release
   authorization for the retained predecessor, never by inference from local
   bytes. Rollback re-runs the same no-network self-test before its explicit
   start.

This is not a second generic updater: `internal/endpoint/replacement` owns the
Endpoint-program binding and interruption state, while `internal/release`
continues to own accepted Release authorization. The generic transaction tracer
used during the decision is historical Git provenance and is not a maintained
package or participant command.

The selected Ubuntu-only foreground contract is now implemented and tested
against the stated interruption matrix. It is rejected if a later change needs
elevation, implicit background ownership, Vault access, a non-atomic
replacement, or a unit start that accepts an unbound successor. This decision
does not create a Windows, package-repair, unattended-update, or public-release
claim.
