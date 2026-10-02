# Offline durable token issuance

This owner composes already selected offline Admission with immutable issuer
material. It grants no live State, time authority, network duty or admitted lane.
The selected implementation remains one local dev slice; no legacy migration.

## Completed interface design

Admission retains Debit and adds DebitVerified, returning Outcome and an opaque
DebitConfirmation. Both use one locked transaction. Only successful durable debit
or currently revalidated exact retry mints a confirmation. Snapshot returns
defensive copies of raw batch, exact ledger binding, kind and checked-at time;
zero confirmation refuses. No public constructor or storage decoder mints one.
Issuance admits only this type, validates exact key inventory and ledger binding,
and validates explicit now within permission/duty, not before checked-at/floor.
Admission stays stdlib-only; Issuance's exact additional imports are successor
Admission and CIRCL blindsign/blindrsa v1.6.5. Hosting stays independent.

InitializeResults(ctx,path,Store,LedgerBinding) creates an exclusive fresh root.
OpenResults(ctx,path,Store,LedgerBinding) returns a shared-lifecycle ResultStore;
Issue(ctx,DebitConfirmation,now) returns defensive response bytes and category;
Close retains the shared result. Store must remain open until ResultStore closes.
Admission -> key Store -> ResultStore is the fixed command lease order, reverse
cleanup with retained errors. The real operation owner is
`internal/successor/tokenissuance`: `Initialize(ctx, Plan)` and
`Issue(ctx, Plan, raw, Facts, Kind)` open those owners before debit and close in
reverse order. Its only non-standard imports are successor Admission and
Issuance. The command handles strict configuration, response export and OTel;
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

## Command and evidence

issuance initialize-results config fields: admission_root, admission_binding,
key_root, key_binding, result_root. issue additionally: batch_file, facts, kind,
response_file. All three roots are canonical absolute and distinct/non-nested;
strict JSON16KiB, batch16KiB, lowercase hex, whole-second UTC. Initialization
opens existing Admission and keys, creates only results. Issue debits only after
all three owners open. Export uses exclusive/idempotent owner-only file adapter;
export failure retains result. Finite diagnostics and bounded OTel contain only
operation/phase/outcome and duration; no secret or binding metadata.

| Rule | Current dev source | New independent evidence |
|---|---|---|
| Durable debit/checked retry | successor/admission ledger and batch | confirmation only after commit, copies, expired retry |
| Blind signer | route/credential/closed_token_issuer.go | real client blind/finalize, stdlib VerifyPSS, all classes |
| Exact result bytes | route/credential/closed_token_outcome.go | grammar/status/count/zero-tail independent assertions |
| Immutable key binding | successor/issuance Store | mismatched cohort/ledger/inventory refuses |
| Shared lifecycle/durability | successor Admission/Issuance filesystem | faults, pending/partial, links/substitution, lease/crash |
| Actual operation | ardents-next composition | compiled issue/retry/export/conflict/collector OTLP |

Before commit: quick/full checks and pinned Go1.27.1 Docker Linux race/process
profile, with real files and independent crypto client. Preserve every attempt
and source identity outside Git; scoped dev commit preserves unrelated files.
