# Independent offline Admission accounting

This completed slice design owns offline verification and durable issuance-right
accounting in `internal/successor/admission`, consumed by `ardents-next`. It does
not authorize network work, sign tokens, authenticate live State/time, provision
permissions or migrate existing issuer roots. Hosting remains independent.

## Transfer and behavioral oracle

| Rule | Current dev provenance | New verification |
| --- | --- | --- |
| Permission signature, hour, maxima and binding | successor/admission inspection; private-admission contract | shared inspection and independently signed batch fixtures |
| ARDIBR01, holder transcript, 1..32 elements | route/credential closed_token_batch.go | canonical framing, tampering, unsigned-time overflow |
| Exact 346-byte PSS SPKI and element range | network/closedprofile token_spki.go; credential closed_token_key.go | canonical re-encoding and independent DER fixtures |
| Shared 65536/hour, permission/class maxima, two bootstrap batches | credential closed_token_issuer_ledger.go | boundary, mixed classes/kinds/permissions and concurrency |
| Request ID + SHA256(full wire) + kind | credential issuer issue/reserve | changed-body refusal and exact retry after reopen |
| Debit before signing; never refund | private-admission; credential issuer issue | cancellation, crash and ambiguous storage |

## Interface and authority

Initialize(root, Binding), Open(root, Binding), Ledger.Debit(ctx, wire, Facts,
Kind), Ledger.Close. Binding pins Network, issuer, duty generation, authority,
profile digest, aligned one-to-six-hour interval and sorted unique class/hour
SPKI inventory (all three classes per hour). Inputs are explicit offline
assertions. Facts select holder, now, class and count independently of wire;
all duty facts must match the immutable binding. No callback supplies authority.
Inspection remains unchanged. Admission imports only standard library.
The exported binding type is `LedgerBinding`, preserving the existing `Binding`
inspection outcome name. Initialization returns `initialized-offline`; it does
not report an issuance debit.

## Persistence and refusal

Fresh canonical absolute owner-only Linux root: marker `ardents-admission-ledger-v1`,
`admission.pin` (canonical binding), `admission.lock`, `admission.journal`,
`admission.floor`, and temporary `admission.pending`. Initialization refuses
existing directories. Open requires exact inventory, pin and file identities;
old issuer roots refuse before writes. Linux nonblocking exclusive flock stays
held until Close. Other platforms refuse before file effects.

The journal has a binding digest header and fixed hash-chained debit records,
at most 6*65536. Each record carries request ID/digest, permission ID/class/hour,
kind/count, signed permission commitment/maxima and observation time. Data is
flushed before its final commit marker; the marker is flushed before success.
Any incomplete/corrupt tail refuses without truncation. Reopen rebuilds and
validates quotas and permission commitments rather than trusting totals.
Before returning an opened owner, flush the verified files, root and parent
directory again: a marker visible after a prior failed sync cannot be
acknowledged by an exact retry until that durability barrier succeeds.
No pruning, rebinding, reset or legacy conversion exists.

The separate checked floor is atomically replaced via exclusive pending file,
flush, rename, directory flush and readback before a valid request's quota/retry
decision. Pending or journal/floor inconsistency refuses. Whole-root rollback
and malicious root access are outside the surviving-storage claim. Syscalls
cannot be hard time-bounded by context under stalled storage.

Before mutation cancellation leaves bytes unchanged. Once mutation starts,
failure terminalizes the open owner and returns storage-uncertain; no automatic
retry/refund. Success may be reported after cancellation if commit completed.
Exact retries still require currently valid permission/facts. Changed body/kind
refuses. A copied Ledger handle shares one private state and Close identity.

## Command and evidence

`admission initialize --config PATH [collector]` takes root and binding.
`admission debit --config PATH [collector]` takes root, binding, batch_file,
facts and kind (bootstrap/admitted). Configuration and batch each <=16KiB;
exact required JSON fields, duplicate/alias/null/trailing refusal. IDs and SPKI
are lowercase hex, times UTC RFC3339 seconds, duty/class/count canonical decimal
strings. CLI exits 0 success/retry, 2 input errors, 130 pre-mutation cancellation,
1 other refusals. Local JSON contains operation/phase/outcome only. Admitted is
an accounting category, never evidence of authenticated lane authority.

OTel has finite operation/phase/outcome and duration only, no IDs, paths,
permissions, keys, counters or file contents; collector refusal cannot change
accounting. Command owns decode/signals/output/OTel; domain owns all decisions
and storage. Add no generic application or persistence framework.

Acceptance uses independent synthetic signed fixtures, real temporary files,
process leases/crash/reopen and compiled CLI; bounded local OTLP capture checks
actual protobuf. Run quick/full repository gates and pinned Go1.27.1 Linux
behavior/race profile. Preserve failed evidence/source identities in an external
checkpoint. No complete-network or C0 qualification claim follows.

## JSON configuration

Initialize requires exactly `root` and `binding`. Binding contains exactly
`network`, `issuer`, `authority`, `profile`, `duty`, `start`, `end`, `keys`;
each key contains `window`, `class`, `spki`. Identifiers are 64 hex characters,
SPKI is 692 hex characters, and keys are ordered by hour then class 1,2,3.
Every cohort uses a different exact SPKI. Debit additionally requires
`batch_file`, `kind` and `facts`. Facts retain the ten inspection fields:
`network`, `issuer`, `authority`, `holder`, `duty`, `duty_not_before`,
`duty_not_after`, `now`, `class`, `count`, each encoded as a string.
The batch path is an explicit canonical absolute regular-file input.
Signature/class/count failures are finite refusals (exit 1); malformed CLI
configuration uses exit 2. A committed operation remains successful after
SIGINT during bounded telemetry shutdown; ambiguous cleanup returns exit 1.

The old issuer root opener has a narrowly scoped pre-lease refusal for
`admission.pin`, preventing creation of its lock inside the new owner's root.
This does not connect the old issuer to the new ledger.
