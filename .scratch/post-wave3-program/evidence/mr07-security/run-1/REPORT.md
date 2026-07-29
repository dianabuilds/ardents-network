# MR-07 security audit — run 1

Target commit: `21460bc`

Scope: MR-07 logical implementation range `15c44da..21460bc`.

## Executive summary

No exploitable vulnerability remains at the target commit.

The audit confirmed and remediated four state-integrity/availability defects:
per-file locking allowed overlapping cross-process effects; reverse checkpoints
and terminal compensation were not fully resumable; lock acquisition repaired
an exposed Unix parent instead of failing closed; and an expired request could
permanently prevent manifest convergence after irreversible Authority
activation. Independent final Security, Spec and Standards verification passed.

## Findings at target commit

| Severity | Finding |
|---|---|
| — | No confirmed remaining findings |

`findings.json` is an empty, schema-valid list for the audited target commit.

## Remediated during audit

### High — independent coordinators could overlap target and fallback effects

Revision locks originally covered only individual journal operations. A second
process could load `mutation_pending` and begin compensation while the first
process still executed the forward host mutation. MR-07 now holds a separate
OS-visible operation lease from before Load through terminal Clear; the
revision lock remains a separate CAS boundary.

### Medium — reverse checkpoints and terminal compensation could wedge recovery

Recovery formerly rewound every non-restored Node to `compensating`, which was
illegal from later fallback checkpoints, and terminal `compensated` was not
handled after a failed Clear. Recovery is now phase-aware for every reverse
checkpoint and clears a durable compensated terminal without repeating effects.
Compensation failure stores `rollback_failed` and aggregate
`recovery_required` in separate revisions.

### Medium — lock acquisition silently repaired an exposed Unix parent

The lock helper used a mutating directory preparation path. It now validates
the existing direct parent without changing it, rejects symlink parents,
requires private mode and current effective-user ownership on Unix, and keeps
Windows protected-ACL validation.

### High — expired deadline could block convergence after Authority activation

After monotonic migration activation, the old generation cannot safely be
restored. Exact idempotent manifest commit now uses a bounded recovery context
even after the original request deadline. Compatible rollout commit remains
under the forward deadline and compensates only after authoritative
non-commit.

## Positive patterns

- Strict request, compatibility, fallback and preflight binding precede host
  mutation.
- One operation owner and one durable state boundary per revision are enforced.
- Lost responses are reconciled against exact Authority/commit status.
- Migration never compensates after confirmed activation.
- Reverse effects remain bounded and resume from exact durable checkpoints.
- Journal content is bounded, strict, private, redacted and binding-protected.
- Ordinary output exposes only closed outcomes, stable reasons and counts.

## Residual assumptions

- Future production adapters must authenticate observations, make effects
  idempotent and make Status authoritative/linearizable.
- Real Linux/Windows filesystem crash semantics, three-host interruption,
  backup/restore, mixed-generation and release-material evidence remain MR-08.
- The journal's higher path ancestry is provisioned under a trusted protected
  root; same-user/SYSTEM compromise is outside the ACL boundary.
- Additional audit runs may explore different paths; one run is not a
  completeness guarantee.

## Dynamic evidence

- Focused deployment/deploymentjournal/storage tests: PASS
- Focused race tests: PASS
- Full `go test ./...`: PASS
- Full `go vet ./...`: PASS
- Formatting, architecture, catalogue, document, API-generation and audit
  traceability gates: PASS
- Reachable vulnerability scan: 0
- Independent final Spec, Standards and Security reviews: PASS
