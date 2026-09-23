---
id: R-163
title: Exact closure of the retired Publisher Administration and Transit chain
status: proposed
owner: Product Owner
started: 2026-09-23
reviewed: 2026-09-23
---

# R-163 — What remains of the old Publisher Administration and Transit chain?

## Decision this unlocks

Issue [#233](https://github.com/dianabuilds/ardents-network/issues/233) asks for one exact keep/remove map after v1 participant startup retirement. This record prepares owner-separated removal work; it **does not authorize deletion** or revise accepted ADRs. The current protected text Administration and its publication, Instance, State, floor and local Interface obligations must survive.

## Current authority and competing hypotheses

The [C0 scope](../../product/scope.md) admits only the v2 protected text participant startup; v1 startup is refused without side effects, while separate Administration and retained root/floor evidence remain. [ADR-0089](../../adr/0089-retire-old-node-starts-preserve-owned-shutdown.md) retires the old Transit-issuance Node start, preserving old roots as evidence. The [Network/Node owner](../../technical/network-route-node.md#old-start-retirement) says its old signer/listener/root mutation and later User Route `Open/Attach` are absent, while the Endpoint acquisition client remains without a receiving Node caller.

There is a **consequential current-owner conflict**: the [Transit Grant acquisition owner](../../technical/transit-grant-acquisition.md) still labels signer, role-scoped Endpoint journals and Application runtime composition implemented/current, and promises exact v1 journal migration on reopen. The [Endpoint owner](../../technical/endpoint-service-runtime.md) still says the maintained participant runtime obtains Introduction/Responder Transit credentials through those journals. [ADR-0070](../../adr/0070-own-volatile-user-route-orchestration.md) retains an Endpoint durable Grant journal for its then-selected User Route. Those statements cannot be silently treated as deleted contracts simply because a call search finds no current caller. An accepted retirement must reconcile them in the owning documents and, if necessary, a superseding ADR before a dependent removal card is Ready.

- **H1, retire the uncalled old composition:** the current closed text path has no non-test dependency on `OpenServiceAdministration`, old Publisher slot/session, or old Transit Grant acquisition. Exact persisted bytes are retained; shared helpers and current owners remain.
- **H2, retain the old composition as a supported current path:** it has a named non-test caller, a selected receiving Node/Route and an operator-visible acceptance journey consistent with C0.
- **H0, defer deletion:** current authority or persisted compatibility obligations cannot yet be reconciled with H1, and H2 has no executable consumer.

Falsify H1 by finding one current non-test call path from `cmd/ardents`/the v2 participant into old `StartPublisher`, `configurePublisher`, `acquireTransitCredentialLifecycle` or its journal, or a shared symbol whose removal breaks the current text journal/publication. Falsify H2 by proving only test fixtures reach those operations and the old receiving Node/Route path is retired. A test-local constructor does not rescue H2.

## Source evidence and method

Primary local source inspected 2026-09-23 at dev@`7e381928dd4bd906a1d623ca78a2dedf3738829c`. Reproduce with `rg -n 'OpenServiceAdministration|configurePublisher|StartPublisher|AcceptPublisher|openTransitAcquisitionSet|acquireTransitCredentialLifecycle|PublisherAttachment' internal cmd -g '*.go' -g '!**/*_test.go'`, then inspect each direct caller and the v2 command/participant setup.

**Sourced facts:**

1. `OpenServiceAdministration` in `internal/endpoint/service_administration.go` has no non-test caller. The only observed direct calls are `publisher_start_test.go`. `configurePublisher` in `publisher_attachment_acquisition.go` likewise has only a test caller. The production functions they lead to (`StartPublisher`, `AcceptPublisher`, old `Withdraw`, `publisherPrepare`, old Introduction session and Transit credential issuance) form a closed call island; a function's presence in production source is not itself a product entrypoint.
2. `cmd/ardents/endpoint_text_plan.go` rejects nonempty `TransitAcquisitionRoot` for v2. `internal/endpoint/text_participant_linux.go` constructs `newEndpoint` without a Transit acquisition root, attaches the current Instance binding, and opens separate text Contexts. It never calls `OpenServiceAdministration`, `configurePublisher` or `StartPublisher`. `internal/endpoint/text_administration_linux.go` separately implements `administration.Interface` and `SnapshotPublisher` using a fresh Administration capability and `startTextPublisher`.
3. `internal/endpoint/service_runtime.go` opens the old `transitAcquisitionSet` only if the internal `setup.TransitAcquisitionRoot` is supplied. No current non-test setup supplies it. `transit_credential_acquisition.go` is reached only from old `publisher_attachment_acquisition.go`; its fixed-Grant branch is not a current v2 participant path. `transit_client_certificate.go` serves old Publisher slot/session TLS enrollment only.
4. `internal/endpoint/transit_acquisition_compatibility.go` reads the accepted persisted Introduction-only v1 record and maps it to v2. `transit_acquisition.go` and `transit_acquisition_files.go` retain the v2 state machine and root reader. Neither the v2 command nor current text participant reopens those old roots. Their bytes, keys and floors are user evidence and must not be deleted, reset or reinterpreted as closed-text authority by code cleanup.
5. `internal/endpoint/text_token_journal_store.go` **does** call `secureTransitAcquisitionRoot`, `acquireTransitAcquisitionLease` and `endpointSyncDirectory`. Thus `transit_acquisition_access_*.go`, `transit_acquisition_lock_*.go` and `transit_acquisition_sync_*.go` contain live shared platform code despite their historical filenames. Keep their behavior and tests; any purpose-correct rename belongs with the affected current text-journal owner.
6. `publisherBinding`, `publication.Open`, `PublicationRoot`, Authority/Introduction public keys and `endpoint.publications` in `service_runtime.go` are used by `text_participant_linux.go` and `text_descriptor_publication.go`. `service_publication.go` is mixed: old `unpublish` serves the uncalled old Administration path, while `decodePublication` is used by `service_connection.go`. Deleting that whole file would break a retained caller. The `internal/application/interfacev1/administration` server/client/grammar is current and used by text Administration.
7. `state.ResolutionView.PublisherAttachment`, `PublisherTransitPeer` and `CredentialIssuer` are called in production only from the old Endpoint acquisition island; the rest of `ResolutionView` and State candidate/authority/floor handling is not this closure. Removal of those projections would be a separate State-owner result, not an Endpoint file sweep.

**Inference:** H1 describes the observed executable call graph and the product's v2 start path. **Limitation:** static call search does not decide whether accepted documentation intentionally retains a dormant compatibility operation; that is a Product Owner/owner-contract choice. This record does not inspect every historical root on disk, measure how many installations hold them, or qualify a deletion.

## Proposed keep/remove boundary

| Owner and exact area | Proposed disposition after accepted H1 | Evidence/condition |
|---|---|---|
| Endpoint old local Administration: `service_administration.go`, old `StartPublisher`/`AcceptPublisher`/`Withdraw` entrypoints and their old request/result fields in `service_runtime.go` | Remove as one Endpoint adapter/session slice, together with only its exclusive tests. | No non-test entrypoint; current `textAdministration` and `administration.Interface` stay. Preserve the shared Endpoint admission and Publication/Instance close ordering. |
| Endpoint old Publisher profile/session: `publisher_start.go`, `publisher_introduction.go`, `publisher_attachment_acquisition.go`, `transit_credential_acquisition.go`, `transit_client_certificate.go` and exclusive fields/callers | Remove in the same bounded Publisher slice only after its complete call graph and owner documentation are reconciled. | Retain `publisherBinding`, `publications`, `PublicationRoot`, current text publication/JOIN/withdraw paths and their tests. Do not revive old Node roles to keep fixtures green. |
| Mixed `service_publication.go` | Remove only old `unpublish`; move/retain `decodePublication` with its actual Service Connection caller before deleting or renaming the file. | `service_connection.go` calls `decodePublication`; no test-only replacement caller. |
| Endpoint old role-scoped acquisition: `transit_acquisition.go`, `transit_acquisition_set.go`, `transit_acquisition_files.go`, `transit_acquisition_compatibility.go`, and old setup/open/close fields | Separate conditional storage/compatibility slice. Delete accepting runtime only after the accepted owner specifies what happens to existing v1/v2 journal roots; never delete or mutate user bytes as a side effect. | Accepted v1 migration promise and ADR-0070 wording currently conflict with immediate removal. Current v2 does not open these roots. Keep the historical format facts in an owner or immutable provenance if reader retirement is selected. |
| Endpoint shared platform functions currently in `transit_acquisition_access_*.go`, `transit_acquisition_lock_*.go`, `transit_acquisition_sync_*.go` | Keep; rename by responsibility only with the current text token journal change, preserving locking, path security, durability and Windows/Unix behavior. | `text_token_journal_store.go` is a real non-test caller. No new generic package or test-only owner. |
| State `PublisherAttachment`/`CredentialIssuer` projections | Separate conditional State-owner audit/removal after Endpoint island retires. | Do not delete other `ResolutionView` projections, candidate history, State authority or floors. |
| Publication/Instance, current text Administration, AAI3, interfacev1 Administration, closed Route/token journal | Keep. | Actual v2 participant and Service Connection callers; accepted C0 behavior and historical floors. |

The proposed work has no new wire, authority, dependency or package boundary. No old root becomes a current token stock or new Service credential. A pure file deletion must not remove shared platform functions, change the current publication drain, or hide an incompatible persisted root.

## Conditional removal cards and verification oracles

1. **[#267](https://github.com/dianabuilds/ardents-network/issues/267) — Endpoint old Publisher/Administration island** (one Endpoint owner): exact non-test call search remains empty for old entrypoints; current `textAdministration.PublishSnapshot/Withdraw` and Instance publication/withdrawal still work; a v1 plan refuses before effects. Preserve current `decodePublication` consumer and original cleanup causes. Include old profile/session/configuration, `transit_credential_acquisition.go` and their exclusive tests in the same buildable change, without altering current text callers. The credential file depends on `applicationEntry` declared in the old Publisher acquisition file; splitting those two files across cards would leave a broken intermediate package.
2. **[#268](https://github.com/dianabuilds/ardents-network/issues/268) — Endpoint old Transit acquisition journal** (separate Endpoint storage result): first accept root compatibility/evidence disposition; then remove only the old accepting root and its remaining storage owner. Existing roots are not opened, erased, rewritten or repurposed by the current v2 path. Current text token journal retains exact lock/path/sync behavior and restart floor; both platforms compile.
3. **[#269](https://github.com/dianabuilds/ardents-network/issues/269) — State old Publisher/issuer projection** (separate State owner): after its last Endpoint consumer is removed, remove only exclusive projection declarations/methods and tests. Current State/closed-profile/authority projections and floor restart checks continue to pass.

Each card needs a real non-test caller oracle and `make quick-check` while coding, `make check` before integration on its selected profile. No test-only reachability counts as Done. These are conditional GitHub cards, not implementation selections. Their issue bodies mark dependency on #233's accepted contract; none is Ready before that decision.

## Recommendation and disposition

Recommend H1 **as a decision direction**, with explicit amendment of the contradictory current owners and accepted-ADR implications before code removal. Confidence is high in the present call graph and shared-helper distinction, moderate in the historical-root compatibility policy. The strongest objection is that removing the v1/v2 journal reader could strand real retained roots despite its current lack of a v2 caller. If the Product Owner requires an accepting compatibility reader, name its actual operator task, input/effect boundary and tests instead of keeping a dormant runtime constructor.

Until that choice, H0 applies: no deletion from this record. The draft changes the research queue and evidence only. Promote the accepted result to the [Endpoint owner](../../technical/endpoint-service-runtime.md), [Transit Grant owner](../../technical/transit-grant-acquisition.md), package map and repository-layout owner as needed; keep GitHub as the execution ledger.
