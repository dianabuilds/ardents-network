# Stage 6 maintained journey trace

Status: **implemented at maintained command and Module seams. The bounded S6E1
campaign received independent `pass`; Product Owner disposition remains the
final Stage 6 gate.**

This trace links the cross-horizon J02, J03, and J05 journeys to the exact Stage
6 naming state, maintained command boundary, behavioral evidence, and honest
limitation. It does not claim that Stage 6 implements the whole Route,
distribution, platform, or qualification portions of those journeys.

## J02 — Open an Unlisted Service

1. `ardents-name resolve` reads authenticated Network State, selects one
   Initiator-domain Relay and one distinct Rendezvous-domain naming Gateway,
   performs one admitted fixed-size OHTTP lookup, and independently verifies the
   threshold-signed current Namespace plus compact Record membership proof.
2. Its `ardents-name-resolution-result-v1` terminal receipt carries the exact
   Name, generation, revision, authority, Target, parent generation, Record
   digest, and Destination Binding commitment. It carries no Route topology or
   endpoint location.
3. `serviceconn` binds that immutable provenance to a Name-origin Service
   Connection. Same-Target monotonic updates continue it; Recovery Pending,
   Release, stale/forked state, or a different Target closes it without
   retargeting. A direct-Target connection remains separate.
4. S6E1 cells A0, A2-A5, C1-C3, D0-D1, and D4-D6 retain the naming,
   connection-continuity, role-view, and no-fallback terminal evidence.

Maintained evidence: `TestResolveCommandRunsPrivateResolution`,
`TestDeepestLegalNameResolvesThroughSeparateRoles`,
`TestNameOriginConnectionClosesWhenTargetBindingChanges`, and the S6E1 verifier
predicates for the cells above. D0 additionally retains all `127` signed
Records and independently recomputes the exact `1667`-byte maximum-depth
compact proof.

## J03 — Publish a local Service

1. `ardents-name control` accepts only the eight frozen claim, renew, record,
   release, transfer, delegate, policy, and recovery operation families. Policy
   add/replace/disable and recovery initiate/cancel/complete/resume are explicit
   variants. Each invocation receives a
   fresh Isolation Context-bound Anonymous Cost challenge and crosses the same
   fixed-size OHTTP Relay/Gateway separation as resolution. The challenge binds
   the digest of the complete canonical static operation; changing any control
   field invalidates admission rather than authorizing a different operation.
2. `ardents-name-control-result-v1` binds the operation kind and exact Name to
   the accepted generation/revision and authority-produced canonical public
   state. It does not expose a private Name Authority key to the Gateway or
   publication runtime.
3. `ardents-publish-app publish` and `ardents-service run` remain the maintained
   Service Administration and Endpoint entrypoints. Routine Instance migration
   preserves Target and Name state; a catastrophe replacement publishes a fresh
   Name Record instead of silently moving a live connection.
4. D2 executes twelve transitions covering every family and lifecycle variant against the maintained Name Authority
   lifecycle, retains each predecessor and canonical result state, and lets the
   independent verifier recompute admission, signatures, revisions, and exact
   effects. S6E1 cells A2-A5, B0-B5, C0-C2, C4-C7, and D2 retain the lifecycle,
   authority, recovery, ordering, pressure, continuity, and private-control
   terminal evidence.

Maintained evidence: `TestControlCommandExecutesEveryPrivateControlShape`,
`TestServiceProcessesKeepConnectionWhenReplacementFails`, the authority and
recovery behavior suites, and the S6E1 verifier predicates for the cells above.

## J05 — Use the Named Unlisted Site tracer

`named-site-lab run` remains the maintained conditional Gate C entrypoint and
owns its nonce-bound HTTP exchange, offline failure, migration episodes,
terminal bundle, and cleanup. Stage 6 does not rewrite or relabel that historical
Route experiment. The current naming side of the trace is supplied by the J02
resolution receipt and J03 same-Target migration/control receipt above; C0, D0,
and D4 independently retain the matching migration, private-resolution, and
offline/no-fallback predicates.

Maintained evidence: `TestReferenceTopologyCarriesOneAuthenticatedWorkload` and
`TestReferenceTopologyRejectsSupersededPublicationDuringMigration` when the
declared immutable Gate C images are supplied, plus the ordinary unit/process
tests named above. This is a layered maintained trace, not a claim that the old
Gate C fixture is itself the current public Namespace or that its conditional
Docker run is part of S6E1.

## Completion interpretation

The trace is conjunctive: a command smoke alone, a passing S6E1 cell alone, or a
historical Gate C bundle alone is insufficient. Stage 6 can close only after the
complete immutable S6E1 campaign receives an independent `pass`, repository
gates pass, and the Product Owner records the final disposition. The 2026-08-20
campaign and repository gates satisfied the first two conditions.
