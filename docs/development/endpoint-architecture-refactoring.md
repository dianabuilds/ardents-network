# Endpoint architecture refactoring design

Status: **draft design for Product Owner review**. This is a target ownership map,
not an accepted runtime contract or a second delivery ledger.
Baseline: `origin/dev` at `c10d59dc` (2026-09-25).

## Objective

Make the owner of an Endpoint behavior easy to locate while preserving the
accepted text-Service contract, exact ownership checks, cleanup order, and
existing wire and persisted identities. Package boundaries should remove
coupling; directory depth and filename prefixes are not goals by themselves.

## Ownership that must remain distinct

| Owner | Responsibility | Target relationship |
| --- | --- | --- |
| `internal/endpoint` | Compose local authority, context cancellation, Route and Service owners, and Application Interfaces. | Remains the composition root. |
| `internal/network/source` | Direct-Origin Source transport for Network State. | Remains under Network; it is not an Endpoint Route prefix. |
| `internal/service/{instance,publication,connection,reachability,targetlink}` | Service identities, durable publication, logical connection, Descriptor, and Target Link contracts. | Remain Service modules with consumers beyond one text participant. |
| `internal/application/...` | Local versioned Application Interfaces and selected text-document workload. | Remains internal implementation of an externally usable local protocol. |
| `internal/route` | Carrier and protected Route mechanics. | Endpoint consumes Route; Route must not import Endpoint. |

The current `textResolutionSource` is implemented by Endpoint's exact Route
prefix acquisition in `text_source_lifecycle.go`. It is not implemented by
`internal/network/source`. Names and package moves must preserve this
distinction. The separate [Route boundary map](route-refactoring-boundary.md)
records why the closed file cluster cannot be moved by filename alone.

## Endpoint interior

`textContext` is the local admission, revocation, and join root. The current
implementation has private owners rather than one undifferentiated state bag:

| Responsibility | Current owner | Key coupling to remove or retain |
| --- | --- | --- |
| Source prefix | `textSourceLifecycle` | Exact handle identity and opening retirement remain atomic with Context admission. |
| Publisher prefixes | `textIntroductionPrefixLifecycle`, `textResponderPrefixLifecycle` | Separate Route handles and opening lifetimes; borrowed Source is not closed by either. |
| Publication | `textPublicationPairLifecycle`, refresh lifecycle, Context coordinators | Instance and Publication ownership spans Context and Endpoint locks. |
| Permission and issuance | `textPermission`, `textIssuanceOperation` | `textPermission` owns holder request creation, signed approval acceptance, currentness and remaining-quota checks, exact retry matching and batch quota reservation, candidate-stock inspection, pending-batch cancellation, issued-token deposit, then burns and verifies the exact challenge under the Context admission lock. Context asks the permission owner whether approval, an exact request, or a pending batch exists instead of reading those fields. Context performs the durable token-attempt mark and surviving-owner check before presentation. Pending batch and exact Source reservation share that lock. |
| Token attempt storage | `tokenjournal.Journal` | Own mutex, replay/time floors, and durable attempts; consumes the shared `durableroot` access, lease, and sync API. |
| Permission file handover | `permissionfile` | Own canonical owner-private request/response paths, exact retry, and request durability; Context retains currentness and offline approval authority. |
| Transit Grant acquisition | `transit.Acquisition` in `internal/endpoint/transit` | Own pending, ready, presenting, and terminal journal transitions, including stale-completion rejection; Endpoint supplies only the issuer exchange and exact TLS enrollment. Its durable root uses `durableroot`. |
| Descriptor history | `textDescriptorHistory` | Own per-Target verified publication/revision floors, conflict memory, capacity and context-retirement erasure; Context checks live authority before acceptance and before using a retained proof. |
| Resolution and JOIN | Context flights and narrow acquisitions | Exact current prefix must be checked again after network effects. |
| Job and worker | `textJobIdentity`, `textWorkerLifetime` | Context retains the job reservation; worker owns process and cgroup cleanup. |

Context-owned registration, withdrawal, and exchange flights are not separate
modules merely because they have distinct filenames. The shutdown dependency
order is defined by `text_context_retirement.go` and the maintained technical
contract in `docs/technical/endpoint-service-runtime.md`.

## Target code shape

1. Keep `internal/endpoint` as the local composition root. Its package-level
   interface should expose participant runtime operations, not the state of
   Source, token stock, worker, or publication internals.
2. Deepen existing private owners before extracting them. A transition should
   be performed by its owner; the Context coordinates admission, cancellation,
   and cross-owner ordering. This reduces direct field access without adding
   package imports or speculative interfaces.
3. Extract a package only when it has one cohesive resource or lifecycle, a
   small caller-facing contract, a non-test caller, and an import graph with no
   cycle. A new package gets `doc.go`, behavior tests, and a package-map entry
   in the same change.
4. Keep the extracted `internal/endpoint/tokenjournal` as the durable
   attempt owner. Endpoint supplies only the selected token and its binding;
   the journal owns replay/time floors and persisted receipts through
   `durableroot`. Its integration tests read durable bytes independently.
5. Keep the text workload name on code that really depends on the selected
   text Application. Use responsibility names for mechanisms only after their
   ownership is clear. A bulk `text_` to `participant_` rename is not the
   architecture change.

## Tests and diagnostics

Tests beside an owner may use a temporary disk root or real loopback network
when persistence or transport is the invariant under test.
Endpoint integration tests inspect token-attempt receipts through an independent
persisted-file oracle; journal implementation tests retain access to private
state to verify poisoning, pruning, and crash boundaries. Shared fixtures
must not replace independent canonical-vector builders. A fixture move must
remove actual duplication or setup cost, rather than gather unrelated helpers
in one file.

`internal/diagnostics/timeline` normalizes time, owner, event class, and safe
reason from bounded Node, Source, and Endpoint runtime schemas. The command
adapter only supplies input and output.
Owner-specific event fields and privacy limits stay typed. Background delivery failures terminate the participant and return to its caller;
a private observation owner serializes output and retains the first such failure.

## Integration rule for this worktree

Take accepted network changes from the integration branch at bounded
checkpoints while refactoring, resolve overlap in the owning module, and run
the affected checks after each intake. Keep this branch's commits coherent
and preserve the other agent's uncommitted work. Before integration, validate
the exact Ubuntu candidate and both selected Carriers with the required gate.
Closing the previous development goal follows acceptance and integration,
including explicit disposition of any remaining network issues.
