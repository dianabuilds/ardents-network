# Offline durable token issuance

This owner composes already selected offline Admission with immutable issuer
material. It grants no live State, time authority, network duty or admitted lane.
The selected implementation remains one local dev slice; no legacy migration.

## Completed interface design

Admission quota retains Debit and adds DebitVerified, returning Outcome and an opaque
DebitConfirmation. Both use one locked transaction. Only successful durable debit
or currently revalidated exact retry mints a confirmation. Snapshot returns
defensive copies of raw batch, exact ledger binding, kind and checked-at time;
zero confirmation refuses. No public constructor or storage decoder mints one.
Issuance admits only this type, validates exact key inventory and ledger binding,
and validates explicit now within permission/duty, not before checked-at/floor.
Admission's public cohort/SPKI contract lives in `admission/issuerprofile` and
uses only the standard library. The ledger imports that contract. The
`admission/issuance` owner imports the ledger, public profile contract, purpose-
bound issuer-profile signing and reviewed CIRCL blindsign/blindrsa v1.6.5. Hosting stays
independent; directory nesting creates no new import permission.

InitializeResults(ctx,path,Store,LedgerBinding) creates an exclusive fresh root.
OpenResults(ctx,path,Store,LedgerBinding) returns a shared-lifecycle ResultStore;
Issue(ctx,DebitConfirmation,now) returns defensive response bytes and category;
Close retains the shared result. Store must remain open until ResultStore closes.
Admission -> key Store -> ResultStore is the fixed command lease order, reverse
cleanup with retained errors. The real operation owner is
`internal/successor/admission/issuer`: `Initialize(ctx, Plan)` and
`Issue(ctx, Plan, raw, Facts, Kind)` open those owners before debit and close in
reverse order. Its project imports are Admission contracts, quota, issuance and issuer-profile signing. The current-authority operation IssueCurrent reuses the same debit/sign/result path with fresh authority and permission-time checks before and after durable work. The command handles strict configuration, response export and OTel;
quota and signing rules remain within their respective domains. No arbitrary
signing or private-key API is added.

## Durable format and transitions

New root marker ardents-issuer-results-v1 binds complete Admission binding and
inventory digest. Retained files: results.pin/results.lock/results.journal/
results.floor; floor replacement uses exclusive results.pending. Linux files
0600, directory0700, current owner, single hardlink, no symlinks or substitution.
Foreign/partial/pending roots refuse without repair; unsupported platforms have
no effects. Initialize never replaces a directory. Open holds nonblocking lease
and validates all data then flushes files, root and parent before acknowledgement.

Each logical committed result has two physical checksum-chain frames: request
(kind, checked time, exact original batch), then exact ARDIOR01 response. Each
physical frame is u32 length, body, SHA256(previous checksum || body), <=16KiB.
The legacy successful ARDIOR01 payload is exactly 16347 bytes including its
grammar-owned zero tail, so its frame is 16383 bytes. No outer network nonce,
encryption or terminal-frame padding is added. This preserves the actual dev
grammar while keeping physical records bounded. An incomplete pair refuses.
Max logical results and total signatures are 6*65536, total journal256MiB.
Storage capacity refusal does not refund Admission. Record identities use ID,
SHA256(full raw batch), kind. Reopen verifies original batch against retained
offline binding/time and verifies every signature by reapplying the reviewed
CIRCL signer to its retained blinded input and comparing exact output; no
first-party RSA arithmetic is introduced. No result is exposed during that check.

Before mutation cancellation has no effect. After mutation finish durability or
terminalize uncertain. Failure between Admission commit and response commit
retains quota; exact Admission retry issues another confirmation. A complete
visible pair after ambiguous sync may reopen only after verified durability;
partial tails never truncate. Exact retries revalidate time before returning
saved bytes. Failure/expiry never refunds. Full rollback and power loss remain
outside demonstrated guarantees.

Cancellation observed during batch validation or retained-result verification
remains cancellation, including a deadline error. It is not translated into an
expired permission or corrupt storage result. A canceled reopen releases its
lease; a later explicit reopen can verify and reuse the unchanged result.

The operation `Result` retains its primary `Phase`/`Outcome` and a bounded
three-slot `Cleanup`: results/profile, keys, admission/identity in retirement
order. Only acquired owners produce completions. `Status()` preserves the
existing command projection: a failed close produces `storage-uncertain`, with
the last failing close phase taking precedence. Any close failure clears
`Response` before export, without erasing primary or earlier cleanup results.
The composition regression cancels after all three real leases are held and
removes two pins; it checks retained outcomes and released leases under race.

## Original retained issuing lifetime

`issuer.OpenCurrent(ctx, Plan, observe)` opens prepared quota, immutable keys
and results in that order and publishes an `Owner` only after original authority
and actual custody rechecks. It initializes no root and generates no keys.
Bindings and public key slices are defensive copies. Its observer belongs to
the original external State/process/participation composition; it runs outside
lifecycle and transaction state mutexes. Native storage checks are followed by
another external observation before effects, so stalled I/O cannot retain an
earlier process/State approval.

`Owner.IssueCurrent` admits one finite issuing call; a concurrent call returns
busy before observation, debit or private crypto. It uses the same
`issueWithOwners` debit/sign/result path as offline issuance. Current authority
and permission time are rechecked before debit, after debit, after durable result
and before return. Route separately checks before physical output and retains
its work slot through output/half-close/join; Admission's call completion cannot
release that Route slot. Quotas and exact retry semantics stay with quota and
ResultStore, without a second ledger or signer. The finite `issuer.IssueCurrent`
consumer opens this same owner for its single call and joins cleanup before export.

Malformed/conflicting/exhausted/canceled requests do not retire a healthy
owner. Original caller/authority loss, key/root loss and definite journal
uncertainty deny its lifetime; restoration cannot revive it. `Owner.Err` retains
the original cause and all acquired-owner cleanup causes for supervision;
command diagnostics still project only finite categories. Quota's `Check`
observes its retained transaction failure without changing the existing separate
physical Close result or minting a debit confirmation.

`Deny` synchronously blocks new calls and cancels original issuing contexts.
The runtime joins Route users before calling `Close(ctx)`. Close additionally
joins issuing calls, observer/storage checks and request cancellation callbacks,
then closes results, keys and quota. A bounded Close wait returns
completion-unavailable while these roots remain held; repeated Close observes
the same original completion and first failure. A native crypto/storage call
may stall, so cancellation is not claimed as physical completion. Startup
rejection joins every acquired partial owner before returning.

The component causal controls use genuine signed Permissions, blind preparation,
RSA finalization and real journals with supplied external authority. They cover
healthy request refusals, exact retry/kind conflict, post-debit held observation
and root retention through Close, post-result authority loss, independent reopen,
partial startup and original observer join. Key-owner controls restore actual
root/lease/material paths after observed loss and require permanent refusal
of the original owner. These controls do not authenticate Network, accept a
Node duty or qualify either Carrier or native process protection.

## Command and evidence

issuance initialize-results config fields: admission_root, admission_binding,
key_root, key_binding, result_root. issue additionally: batch_file, facts, kind,
response_file. All three roots are canonical absolute and distinct/non-nested;
strict JSON16KiB, batch16KiB, lowercase hex, whole-second UTC. Initialization
opens existing Admission and keys, creates only results. Issue debits only after
all three owners open. Export uses exclusive/idempotent owner-only file adapter;
export failure retains result. Finite diagnostics and bounded OTel contain only
operation/phase/outcome and duration; no secret or binding metadata.

Response destinations cannot be any configured state root or its descendant.
Before opening owners or debiting, the command also refuses output directories
inside another recognized state root. The shared output adapter repeats this
check immediately before writing. Inventory and profile exports use the same
rule; initialization checks destinations before creating persistent state.

| Rule | Current dev source | New independent evidence |
|---|---|---|
| Durable debit/checked retry | successor/admission ledger and batch | confirmation only after commit, copies, expired retry |
| Blind signer | route/credential/closed_token_issuer.go | real client blind/finalize, stdlib VerifyPSS, all classes |
| Exact result bytes | route/credential/closed_token_outcome.go | grammar/status/count/zero-tail independent assertions |
| Immutable key binding | successor/admission/issuance Store | mismatched cohort/ledger/inventory refuses |
| Shared lifecycle/durability | successor Admission/Issuance filesystem | faults, pending/partial, links/substitution, lease/crash |
| Actual operation | ardents-next composition | compiled issue/retry/export/conflict/collector OTLP |

Before commit: quick/full checks and pinned Go1.27.2 Docker Linux race/process
profile, with real files and independent crypto client. Preserve every attempt
and source identity outside Git; scoped dev commit preserves unrelated files.
