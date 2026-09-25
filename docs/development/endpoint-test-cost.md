# Linux Endpoint test cost

Status: one measured development baseline, not a qualification verdict or a
change to the checked test profiles.

## Measurement

On 2026-09-25, branch `codex/architecture-refactor` at `7bd68da2` ran
`go test -json ./internal/endpoint -short -shuffle=on -count=1 -timeout=15m`
inside the `golang:1.26.8` Linux image
(`sha256:9d2f36f06329b2a141b9db99ffa32765cf695ee57b813ca29e245e8670bcbfff`).
The host was Windows with Docker Desktop. The raw JSON output is retained
outside the repository at
`C:\Users\vitek\AppData\Local\Temp\ardents-endpoint-timing-2026-09-25.jsonl`.
Its SHA-256 is
`b8c0f663dca7ace65b5e799ae32b38611af9738fbe8041544d8f6921401d2b01`.
This is one host and one shuffled run; it cannot establish a stable performance
regression or predict the Ubuntu gate duration.

The package passed in 548 seconds. Its 230 top-level tests account for 534
seconds of reported test time; the remaining time includes package/test runner
work. No test reported `fail` or `skip`. The 174 `TestText*` cases account for
526 seconds of top-level test time; 56 other cases account for 8 seconds. The
ten slowest tests account for 313 seconds of package time; the twenty slowest
account for 415 seconds. These are wall-clock test durations, not CPU profiles.

| Test | Seconds | Maintained reason to keep its workload |
| --- | ---: | --- |
| `TestTextPublisherBuildsRetainedQualificationSetAcrossFourReaders` | 121 | Actual 4 x 64 retained Connection setup and producer path. |
| `TestTextPublicationIsolatedRoleObservations` | 42 | Separate role processes and durable observations for both Carriers. |
| `TestTextIntroductionDeliversFourConcurrentReaders` | 33 | Four real Reader deliveries; 25 seconds of deliberate bootstrap spacing. |
| `TestTextPublicationLossBeforeAcknowledgementRetiresRecipients` | 23 | Multiple pre-ACK failure and retirement cases. |
| `TestTextInitialPublicationLossBeforeAcknowledgement` | 22 | Six carrier/failure combinations around the Store ACK barrier. |

The 256-Connection test also includes 25 seconds of explicit bootstrap
spacing. That spacing and the four-reader Introduction test model the installed
runner's offline permission exchange and duty-wide refill pacing; deleting
their sleeps solely to improve runtime would change the tested scenario.
`waitTextNetworkFixtureStart` can additionally wait until the next hour when
less than two minutes remain in the current Permission hour. That guard avoids
expiry during a bounded network episode, but makes runtime depend on start
time. This run did not establish the guard's contribution to the 548 seconds.

## Process consequence

Keep the full Linux deterministic and race profiles as acceptance gates.
`make unit` runs packages serially, and the 256-Connection test deliberately
shares a host-wide provider ledger across sixteen local Nodes. Parallelizing
packages or tests without proving resource isolation could change the
workload and make the result unreliable. Moving files between packages alone
will not remove the measured network work from the full gate.

For development feedback, run the affected owner tests on Linux first, then
run the full checked profile on the exact integration candidate. One candidate
improvement is to make the Permission-hour fixture independent of a wall-clock
wait while preserving the same signed-hour and expiry checks. Separately,
owner extraction can make focused tests compile and run without linking the
entire Endpoint package. Neither improvement is proven by this timing run.
