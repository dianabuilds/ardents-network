# Independent Hosting budget

This bounded successor slice reuses the maintained Hosting arithmetic and
durable transaction semantics described in network-route-node.md and
private-admission.md. It grants no admission, token or network authority.

The standard-library-only internal/successor/hosting owner provides Initialize,
Open, Observe, Sample, Reserve, Close and Reservation.Release. Policy, Traffic and
Observation retain finite provider-period bounds and separate Tx/Rx counts.
Sample shares confirmed persisted observations up to one second old; every read
still checks the journal under its exclusive lease. There is no global in-memory
cache or process/cgroup placement. Reservations always measure fresh counters.

Fresh roots use ardents-hosting-budget-v1 and budget.pin, budget.lock,
budget.json and budget.pending. Old period roots refuse without effects;
legacy readers cannot consume these roots. Initialization never replaces a
directory. All processes must share one root for one allowance. Lost handles
retain their reservations; no reset, refund recovery or automatic migration is
provided. Explicitly separate test roots are not multiple production allowances.

The finite command exposes hosting initialize/observe/hold --config PATH,
optionally followed by a numeric loopback collector URL. Config files are
bounded regular files of at most 16 KiB, with exact fields and no duplicates.
Initialize consumes root and policy, observe consumes root, hold consumes root,
work, termination and hold_ms (1..60000). Roots are canonical absolute paths.
Hold reserves its interval plus five seconds for cleanup, observes every second
and releases only after its local wait finishes. Cancellation uses independent
five-second cleanup. Cleanup failure supersedes cancellation and remains visible.
The original hold deadline is fixed before reservation; lease/commit delay
does not extend it. Cleanup contexts bound cancellation and lease waits, not
the duration of an OS syscall against stalled storage. Local filesystem
behavior checks do not establish a hard real-time filesystem guarantee.
An unsupported platform refuses before reading config or touching storage.

Observe returns local used/reserved/remaining byte counts and protect/drain.
The command uses Sample and includes observed_at and valid_until. A snapshot
is diagnostic evidence, never permission to start work.
Other results and diagnostics disclose only operation, phase and finite outcome.
OTel exports fixed operation/phase/outcome and duration, never root, provider,
interfaces, boot identity or counters. Existing local-only collector limits and
bounded export/shutdown remain binding; collector failure changes no budget
result. Observation counts whole named interfaces, not attributed useful bytes.

Acceptance requires arithmetic and continuity cases, shared-process concurrency,
release ambiguity, actual filesystem failures, crash retention, real Linux
counters and CLI lifecycle, refusal of both foreign formats and OTLP privacy.
Run Windows quick/full gates and Linux Hosting behavior/race in the pinned
Go 1.27.1 image. Missing Linux prerequisites fail; no installed Endpoint, Node
switch-over or C0 qualification is claimed.

## Command growth policy

cmd/ardents-next owns arguments, bounded input decoding, rendering, signal
handling, finite command composition and concrete OTel setup. Budget rules,
durable state and reservation correctness stay in hosting; none may depend on
command syntax, output or the collector. The current hold wait is one command
scenario, not a general workload runner.

Extract a responsibility-named operation owner when a second non-test consumer
needs the same lifecycle, or when a scenario coordinates multiple state owners,
recovery or independent child work. Move the whole lifecycle with its acceptance
tests, leave a thin command adapter and register exact imports and a real caller
in the same change. Do not create speculative application layers or packages
called common/util. File length and number of verbs are navigation signals,
not numeric splitting rules. Each added verb must review this responsibility
boundary before implementation; configuration and rendering remain in cmd.

## Operator inputs and results

The initialize object has exactly root and policy. Policy fields are provider,
start, end, unit, quantity, directions, interfaces, initial_used_bytes and
low_watermark_bytes. Dates are RFC3339 timestamps at whole-second precision;
the period must contain the initial observation. Units are B, MB, MiB, GB, GiB,
TB or TiB, with checked conversion. Directions are tx, rx or tx+rx; interfaces
are a distinct list of one to sixteen actual names. Initial use is a retained
floor, never an allowance added on reopen. Low watermark is positive and less
than the total allowance. Provider text is local operator metadata only.

Observe's config is just {"root":"/absolute/private/budget"}. A hold config is
{"root":"/absolute/private/budget","work":{"tx":1000,"rx":1000},
"termination":{"tx":100,"rx":100},"hold_ms":1000}.
For example, run ardents-next hosting hold --config /private/hold.json;
append http://127.0.0.1:4318 only when explicitly selecting that collector.
The hold command waits locally; it does not generate a traffic workload or
demonstrate that a future workload can stay inside its envelope.

Stdout uses operation, phase, outcome and, for observe only, observation
(UsedBytes, ReservedBytes, RemainingBytes, Protect, Drain), observed_at and
valid_until. Stderr emits a fixed
reserved/hold event and terminal operation/phase/outcome/telemetry; raw errors
are never rendered. Outcomes are completed (exit 0), canceled (130),
invalid-input/output failure (2), budget-exhausted, budget-unavailable,
unsupported-platform or storage-uncertain (1). An expired period can still be
observed as Drain, but admits no reservation. Ambiguous state never produces a
usable reservation or an automatic retry of a potentially committed refund.


## Domain model and ownership

Hosting is a supporting domain with one aggregate: the provider-period budget.
Policy is immutable period input; Traffic and Snapshot are value facts.
Reservation is an opaque lifecycle handle into that aggregate, not another
aggregate or an independently renewable allowance. Copies share closure and
release state. Independently opened owners share the filesystem transaction.

Reserve takes Work, Termination, WorkUntil and HoldUntil. WorkUntil must still
be in the future when the committed reservation is handed out. HoldUntil must
cover WorkUntil and fit the provider period. Measurement or commit delay cannot
renew either deadline. A late or canceled handoff attempts one independent
cleanup; an uncertain refund remains unavailable. Expiry does not automatically
release admitted work. Its consumer must stop and join children before Release.
Used traffic remains spent after release, cancel, reopen or exhausted periods.

### Additional work under original termination coverage

Additional physical reservation operations are Linux-only, matching the current
Hosting measurement/storage platform and actual Route consumers. Shared envelope
arithmetic remains portable and has independent directional tests.

`ReserveAdditionalJoint` reserves positive additional
work through the same Budget and exact original Reservation. It retains its
WorkUntil/HoldUntil and termination dependency; it adds no new period, deadline
or cleanup allowance. Foreign, released, sealed or chained handles refuse.
Consumers join physical work and return additions before the original. Early
original Release seals further additions and returns ErrReservationInUse without
returning capacity. Copies share the same lifecycle. Uncertain addition/refund
retains the original dependency and terminal failure rather than retrying a
possibly committed return.

`JointTraffic` declares maximum Tx, Rx and their shared Total: both component
maxima fit Total and together cover it. Provider `tx` costs Tx, `rx` costs Rx,
and `tx+rx` costs Total. This accounts a shared bidirectional workload without
charging both directional maxima as independent traffic. `CoversJoint` checks
that its provider-counted capacity fits the original Work reservation, excluding
Termination; it neither certifies directional workload behavior nor attributes
interface measurements or Carrier overhead. Role envelope construction remains
with the caller, not Hosting.

For a deterministic tx+rx example, suppose total capacity is 100 MiB, measured
use is 10 MiB, watermark is 1 MiB and original Work/Termination reserve is
32/1 MiB. With unchanged counters, remaining provider capacity is 57 MiB.
If a caller's old shared work allowance leaves 1 MiB after its control debit,
replacing it with 32 MiB requires a 31 MiB addition. Reserved capacity becomes
64 MiB, provider remaining becomes 26 MiB and measured use stays 10 MiB.
Termination remains reserved once. The same request refuses if its measured
remaining capacity cannot cover 31 MiB while retaining the 1 MiB watermark.
Returning the addition restores the original reservation only; measured use
is never refunded. Wire debit and token spend remain outside Hosting.

| Existing source | Final owner and disposition | Behavioral evidence |
| --- | --- | --- |
| internal/hosting policy/ledger | Hosting Policy, allowance and durable Budget; checked directions, overflow, floors and watermark | TestDirectionalCostAndOverflow; TestReserveUsesProviderDirections; TestIndependentOwnersCannotOversubscribe |
| internal/hosting sampler/shared observation | Budget.Sample; shared persisted measurement under lease, no copied global cache | TestSharedSnapshotFreshnessAndReservationVisibility; TestSampleDoesNotRenewExpiredPeriod |
| internal/hosting storage and counters | Hosting root/transaction/counter implementation; fail closed on unknown continuity, no history reset | TestHostingLostCounterContinuityRefuses; TestForeignAndPendingStateRefusedWithoutMutation; TestHostingCorruptStateIsNeverInitializedAgain |
| Existing work/termination reservation lifecycle | Hosting owns atomic capacity and one refund; consumer owns stop/join | TestCompletionCoverageDoesNotRenewWork; TestCopiedReservationCannotRefundAnotherHandle; TestHostingReleaseAfterPersistenceFailureStaysUnresolved |
| internal/node/hosting control/join/admission/lifetime | Remain role consumers: role envelope construction and child lifetime do not belong in budget | Test-only composition demonstrates reservation ownership without role rules in Hosting |
| Admission token/quota/spending | Remain Admission; budget grants no token authority | TestAdmissionStandaloneCommandsIssuePresentReceiveAndReopen: actual issuance, durable spending, local transfer, release and replay refusal |
| Network assignment/currentness | Remain Network; supplied facts are not authenticated by Hosting | Standalone command explicitly supplies operator assertions; no Network qualification |

Fresh-root storage identity and encoding remain unchanged. Old runtime roots
and consumers stay independent; this completion does not migrate their history.
No repository/service/event-bus layers, resource placement or token policy are
part of this domain.

## Integration regression tests

Cross-domain regression composes Admission and Hosting in Linux tests under
cmd/ardents-next. The genuine Route command also composes these owners; no
additional regression package or workload command is exposed.
The non-test consumers include hosting initialize/observe/hold and the genuine
Linux Route holder/receiver. Route composition constructs role envelopes and
keeps physical borrowers joined; Hosting owns provider accounting and the exact
original reservation dependency. Neither owner acquires the other's policy.

The test fixture accepts a genuine token, reserves work and termination capacity,
transfers bounded local data, joins its socket reader and releases the reserve.
TestCanceledSocketChildRetainsReservationUntilJoined holds that reader before
join and proves that cancellation cannot release capacity early; after join,
capacity is released and the spent token still refuses replay. The command cycle
supplies tokens issued through the maintained issuer. These tests establish no
Network authenticity or Carrier qualification.

The additional `network_admission_hosting_linux_test.go` composition uses
signed intake into the opened new Network owner with genuine Admission tokens
and separate Hosting/spend roots. It checks conflict/clock refusal before
reservation effects, pre-spend loss, retained burn after a signed successor,
refill's original deadline and dual reservation ownership, reopen and release
after joined socket work. Network and Hosting have no calls into one another;
the application owns the authority/reservation ordering and work termination.
See the [domain map](../development/domain-map.md) for exact source/evidence
identity. This socket scenario does not qualify TCP/TLS or QUIC Route behavior.

make hosting-check runs Hosting and the command package with Linux race checks.
It includes measurement/commit expiry, cancellation, clock rollback, concurrency,
shared observations, ambiguous storage, restart and exact release regression.
