---
status: accepted
date: 2026-09-27
---

# ADR-0111 — AREP v3 becomes the sole closed-Epoch intake schema; retired envelopes are refused with a typed error before any durable effect

(F-50. The finding left the accepted closed-profile rule and the retained-root
disposition as an analysis boundary; under the product owner's standing
decisions that legacy dies and that no support or backward compatibility is
owed, this ADR selects the sole schema, the intake gates, and the classified
old-root outcome.)

## Context

`state.parseEpoch` decodes AREP envelope schema 1, 2, or 3, and the shared
`verifyEpoch` matches the `ardents-route-v3` profile without restricting the
envelope version. A correctly signed, otherwise valid AREP v1/v2 envelope
therefore passed the closed candidate path: offline `Accept` could commit it,
and `candidate.go:verifySourceBundle` could count it as a valid Source result
— including through its exact-byte current/pending reuse branches, which run
before the general candidate verifier. On restart, control-floor recovery
could restore an old-schema generation as current, and pending recovery could
later feed Source-wave activation, so a retained historical parser was not
inert. `storage.go:loadGeneration` must keep authenticating historical
predecessor generations regardless. Neither the State nor the Route technical
owner stated the AREP byte; the closed-profile `version u16=3` belongs to a
different signed record.

## Decision

1. **AREP v3 is the sole new closed-Epoch intake schema.** The rule is scoped
   to the closed Route profile (`ardents-route-v3`); other accepted profiles
   keep their existing envelope handling. `epoch_intake.go` owns the constant
   and both helpers.
2. **One gate at the new-candidate choke points, before any durable effect.**
   `requireClosedIntakeSchema` runs after decision verification in offline
   `Accept` and at the tail of `verifySourceBundle`, the single point both
   Source result forms pass through. A retired envelope fails with the typed
   sentinel `ErrLegacyEpochIntake` before any commit, staging, activation, or
   floor repair. The reuse branches need no separate gate: retained decisions
   are classified at Open (rule 3), and the epoch digest binds the version
   byte, so an exact-byte reuse of the current or pending decision cannot
   smuggle a retired schema past Open.
3. **Retained old current/recovered-active/pending generations are classified
   at Open, floors preserved.** `classifyRetainedClosedSchema` returns a
   `*RecoveryRequiredError` naming the generation role when `loadCurrent`,
   `recoverDistributionActive` (before its `persistDecision` floor repair), or
   `recoverPendingState` finds an AREP v1/v2 generation. The store never
   rewrites, converts, clears pointers, or selects a lower generation:
   `generations/`, `current`, and `distribution/` floors stay byte-intact for
   out-of-band inspection. Under the no-support policy the operator answer is
   a fresh root.
4. **Historical predecessor authentication is unchanged.** `loadGeneration`
   keeps verifying predecessor chain members with the existing decision
   verifier, so a v3 current atop authenticated v1/v2 predecessors opens and
   accepts successors normally. The old decoders are removed only when the
   retained population they serve is closed.
5. **The technical owners record the sole schema.** The State transition
   admissibility section of the network/Route/Node owner and the
   authenticated closed profile section of the protected-route protocol owner
   now state the AREP v3 intake rule and the typed refusals.

## Consequences

- A closed-State root whose current, recovered-active, or pending generation
  predates AREP v3 now fails Open with a typed recovery outcome instead of
  silently running old-schema State or feeding it to Source-wave activation.
  This is the intended outcome under the accepted no-support policy.
- A signed v1/v2 candidate presented offline or by a Source wave is refused
  with `ErrLegacyEpochIntake` before any durable effect; the refusal is
  `errors.Is`-matchable by commands and tests.
- Predecessor chain evidence and all durable floors of an old root survive
  the refusal untouched; nothing in the working tree can promote them, and
  their bytes remain for out-of-band forensic inspection only.
- Non-closed profiles (`h3-role-probe-v1`, `ardp-interactive-route-v2`) are
  unaffected: the gate and the classification both test
  `config.acceptedProfile == closedRouteProfile` first.

## Verification

- `internal/network/state/epoch_intake_test.go` (external, on the package's
  own canonical vector builders, as its package-map row requires): v1/v2 offline candidates refuse with
  `errors.Is(err, ErrLegacyEpochIntake)` and a reopen shows
  `ErrNoCurrentGeneration` (no durable effect), with a v3 sanity acceptance;
  seam-committed v1/v2 activated currents classify on reopen as
  `*RecoveryRequiredError` with `generations/` and `current` floors
  preserved; a v1 pending under a v3 current classifies with
  `distribution/current` and pending floors preserved; a v1→v2→v3
  predecessor chain opens with the v3 current and accepts a v3 successor;
  and the Source choke point covers both result forms — exact-current reuse
  passes, v1/v2 candidates refuse, a v3 candidate passes.
- `epoch_intake_export_test.go` supplies the durable-root reconstruction
  seams (`CommitRetainedGenerationForTest`, `CommitRetainedControlForTest`,
  `VerifySourceCandidateForTest`) following the existing export-test
  precedent; they simulate historical root populations honestly through the
  real commit paths.
- Gates: `gofmt`, Windows `go build`/`go vet`/full `go test`, `staticcheck`,
  exact `make deadcode`, Docker Linux `go test -race` for the touched
  packages, `./internal/architecture/` (all new files under the 500-line
  limit), and `make quick-check` via the pre-commit hook.
