# Independent Hosting budget

This bounded successor slice reuses the maintained Hosting arithmetic and
durable transaction semantics described in network-route-node.md and
private-admission.md. It grants no admission, token or network authority.

The standard-library-only internal/successor/hosting owner provides Initialize,
Open, Observe, Reserve, Close and Reservation.Release. Policy, Traffic and
Observation retain finite provider-period bounds and separate Tx/Rx counts.
No observation cache or process/cgroup placement is included.

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
(UsedBytes, ReservedBytes, RemainingBytes, Protect, Drain). Stderr emits a fixed
reserved/hold event and terminal operation/phase/outcome/telemetry; raw errors
are never rendered. Outcomes are completed (exit 0), canceled (130),
invalid-input/output failure (2), budget-exhausted, budget-unavailable,
unsupported-platform or storage-uncertain (1). An expired period can still be
observed as Drain, but admits no reservation. Ambiguous state never produces a
usable reservation or an automatic retry of a potentially committed refund.
