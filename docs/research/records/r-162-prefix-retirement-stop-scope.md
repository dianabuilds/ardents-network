---
id: R-162
title: Stop scope after idle text-prefix retirement
status: proposed
owner: Product Owner
started: 2026-09-23
reviewed: 2026-09-23
---

# R-162 — Which idle prefix-retirement outcomes stop a Context or the Endpoint?

## Decision this unlocks

Issue [#90](https://github.com/dianabuilds/ardents-network/issues/90) needs one outcome-to-stop-boundary table before conditional [#91](https://github.com/dianabuilds/ardents-network/issues/91) can change `retireTextPrefixLocked`. This record is a source-backed **proposal**, not a new accepted cleanup contract. It addresses only idle Source, Publisher Introduction and Publisher Responder prefix retirement. Worker/Job cleanup, other `failTextContexts` callers, Node parents and CLI error mapping remain separate.

## Current contract and hypotheses

The [C0 product scope](../../product/scope.md) requires finite budgets, joined cleanup, explicit unavailability and authenticated current authority. The [threat model](../../security/threat-model.md) assumes malicious peers and resource pressure; a failed or ambiguous cleanup cannot become an accepting weaker path. The [Endpoint owner](../../technical/endpoint-service-runtime.md) says each prefix has an exact lifecycle owner, Context shutdown revokes all children before ordered join, and a Job's admission reservation survives until joined cleanup. The selected issue requires an independent Context to survive only on proof of safety, while shared authority loss still closes all.

- **H1, selective stop on proof:** A network terminal failure can retire its exact idle prefix and preserve a sibling Context when the Route owner proves complete join and physical resource release. A different Context cannot inherit its handle, permission, token, Job or reserve.
- **H2, any non-nil idle `Close` error is safe after `Done`:** `Done` alone proves that all physical reservations were returned, so a non-nil `Close` may be confined to the affected Context.
- **H0, retain Endpoint-wide refusal for unproved release:** A non-nil idle `Close` lacks a release witness; keep the current fail-closed Endpoint boundary until Route provides one.

Falsify H1 with a legal sibling operation that borrows the retired prefix or with a claimed joined-clean case whose host/Route reservation remains owned. Falsify H2 by showing that `Done` can close while the transport `Close` reports a physical-retirement error. The current code does that. Falsify H0's necessity with a specific Route error class carrying an independently verified physical-release/refund witness; a generic `ErrClosedSourceCleanup` label is insufficient.

## Evidence and reproducibility

Primary local source inspected 2026-09-23 at dev@`7e381928dd4bd906a1d623ca78a2dedf3738829c`:

- `internal/endpoint/text_prefix_retirement.go`: `retireTextPrefixLocked` calls the three `retireIdleLocked` owners, joins their errors, then sets `owner.closed` and calls `endpoint.failTextContexts` on any non-nil result. It does not observe the original network terminal cause.
- `internal/endpoint/text_source_lifecycle.go`, `text_introduction_prefix_lifecycle.go`, `text_responder_prefix_lifecycle.go`: each idle owner acts only after `prefix.Done()`, invalidates its exact handle, cancels its context and returns `prefix.Close()`. A non-idle prefix and an opening in progress are not retired by this path.
- `internal/route/closed_source_lifetime.go` and `closed_source_prefix.go`: `Done` closes after `finishAfterChannels` waits for channel workers and `finish`. `Close` joins `done`, but its returned `failure` is built from `channels.Close` or physical retirement and `child.Close`, then wrapped with `ErrClosedSourceCleanup`. It does not return the recorded network terminal cause as such.
- `internal/route/closed_source_channels.go` and `closed_bootstrap_exchange.go`: channel `Close` joins its workers, clears its own queued/lane buffers and returns `retire()`; `closedRoleRetirement.close` stores the underlying transport `Close` error. Thus a completed join does not make an unsuccessful physical close a proved refund.
- `internal/endpoint/text_context_shutdown.go`, `text_context.go` and `text_context_retirement.go`: `failTextContexts` synchronously latches `textClosed`/first error and revokes all retained leases. Each Context's shutdown watcher then performs ordered stop/join and reports its result. This is an Endpoint-wide admission barrier, not an immediate proof that every child has already joined.
- `internal/endpoint/text_permission.go`: the next text operation rechecks current State/profile and local permission after idle retirement. A prior prefix result cannot authorize work under changed common State.
- Existing focused tests `TestTextSourceHandleRejectsUseAfterIdleRetirement`, `TestTextResolutionOldAcquisitionCannotCommitAfterSourceReplacement`, `TestClosedSourceQueuedCloseJoinsAlreadyFailedParent`, `TestClosedSourceParentTerminalBeforeCloseEmission`, and `TestTextContextCleanupFailureClosesExistingAndFutureJobAdmission` prove individual identity, joined-child or fail-closed facets. None is a two-real-Context positive oracle for a non-nil idle `Close` result with a proved host refund.

**Sourced fact:** `Done` proves the Route prefix's local join, while `Close` can still report physical retirement failure. **Inference:** a generic non-nil `Close` cannot be classified as harmless to a shared reserve from this Endpoint return value. **No measurement:** the record does not estimate failure frequency or claim an installed-host resource verdict.

## Proposed outcome → stop boundary

| Outcome observed at this exact idle path | Reservation owner and required witness | Proposed stop boundary and cause |
|---|---|---|
| Prefix remains active, or an opening/child is unfinished | Exact Source/Introduction/Responder lifecycle and its outstanding flight own it; `Done` is absent. | No idle retirement or release. Keep the current owner and lifetime; do not infer a sibling-safe failure from this path. A later Context stop must cancel and join its flights in the existing order. |
| Network terminal, peer refusal, deadline or expiry; then `Done` closes and `Close()==nil` | Route prefix has joined its channel/child workers and its physical retirement returned success. Exact handle is invalidated. The operation owner retains its original terminal cause separately. | Only that prefix is retired. The authorized Context and unrelated Contexts may remain, subject to fresh authority and admission checks on any next operation. This is current behavior; retirement itself returns nil. |
| `Done` closed but `Close()!=nil`, including underlying transport-close failure | Route owns the physical transport. Its join is known, but release/refund is not proved by this result. `ErrClosedSourceCleanup` preserves class and original error, not a release certificate. | Latch original error and close text admission for the Endpoint; cancel all Context leases and join their owned children. Do not refund an unproved shared reserve or let another Context accept new work. This is current behavior. |
| Child/opening/Job work has not joined, or a local refund is unknown | Its actual child/Job/Route owner retains the reservation until a joined result. The idle path has no independent witness for it. | Do not classify here. Its own caller retains the error and must use its existing unsafe cleanup boundary. A proposed selective stop requires a separate owner-specific proof and issue. |
| Authenticated common State/permission authority is lost or conflicting | State/permission authority owner must establish the condition; no prefix `Close` value can restore it. | No Context may continue on the old authority. Endpoint-wide refusal remains required. This table neither adds a State watcher nor treats a per-prefix network failure as shared authority loss. |

Preserve the original cause under `errors.Join`; the generic cleanup class must not erase it. A successful local join is necessary but insufficient when physical close or refund remains unknown. No failure creates a new token, permission, bootstrap batch, Route fallback or worker reservation.

## Options and acceptance oracles

**H1 (preferred availability property where proof exists):** Keep a sibling live after a network terminal followed by exact `Done` and nil physical cleanup. This property is already present at this caller. If a future non-nil `Close` class carries an explicit physical-release witness, research that Route result and its consumer as a separate acceptance slice before changing #91. A bare error string, `Done` or test-only injected result is not the witness.

**H2 (reject):** Treat all joined non-nil prefix errors as Context-only. The current `Close` return aggregates physical transport-close errors, so this can release admission without proving physical cleanup.

**H0 (recommend for present #90 decision):** Accept the table above and retain the Endpoint-wide stop for non-nil idle `Close`. This is conservative about physical retirement and makes #91's requested non-nil “safe joined failure” positive case unavailable in the current Interface. Do not claim #91 implemented by a test-only invented error.

For #91, the **positive real-path oracle** is two independently admitted Contexts: force one prefix's network terminal, wait for its actual `Done` and nil `Close`, show its exact old handle unusable and the sibling's independent operation admitted under current authority. The **negative oracle** injects a physical retirement failure *at the Route transport owner* after `Done`; show the original cause reaches repeated Context/Endpoint Close, no fresh or formerly idle Context admits work, and all retained owners are joined. Also test an opening/child stalled at stop and a changed common State; neither may be converted into a sibling-safe idle outcome. Current tests cover facets, not this whole oracle.

The strongest argument against H0 is lost availability if a transport reports an error after it nevertheless released every resource. That requires a Route/host release witness, not interpretation of the existing error at Endpoint. An accepted decision to pursue it should create a separately bounded proof/result card; #91 then depends on that accepted output. No public protocol or wire change is implied.

## Recommendation and disposition

Ask the Product Owner to accept H0 as the present stop scope, or explicitly select additional Route release-proof research before a selective non-nil-error implementation. Confidence is high about the current `Done`/`Close` distinction and moderate about complete host refund semantics, which were not measured here. On H0, #91 should be revised or closed as no applicable code delta at this caller; it must not be declared Done from existing tests alone. This draft changes only the research queue and this evidence record. Promote an accepted stop table to the [Endpoint technical owner](../../technical/endpoint-service-runtime.md) and update #90/#91 in GitHub; the research record is not the maintained contract.
