# Testing

Ardents tests behavior through three explicit surfaces. None of them is a
delivery stage, and no surface is a prerequisite receipt for another.

## Unit

Unit and single-Module integration tests live beside their implementation as
`*_test.go`. They use public Module Interfaces, replace real time or external
I/O at the owning seam, and must not start Docker. `make unit` is the normal
feedback loop and should remain fast enough to run after each meaningful edit.
Exact-Target Service Connection transfer, backpressure, cancellation, and
same-connection recovery are tested here through `internal/serviceconn`; those
behaviors do not need a second private verifier around the same Module.

## End to end

Cross-process tests live under `tests/e2e/<behavior>/`. They build real product
commands, start separate operating-system processes, and assert only command
output and other public behavior. Each test creates fresh fixtures in its own
temporary directory and cleans them through `testing.T` ownership. Run all
process tests with `make e2e`.

The current process suites cover authenticated Network Source refresh, Node
lifecycle and pressure, Route selection, transfer, reference capacity, and
fourfold functional scale-up, plus Service command readiness,
bounded failure, cleanup, and same-connection recovery when the first
replacement Route Attachment fails. Recovery is exercised through real
Service and Application commands; only the public Route socket is replaced by
the scenario-owned fixture.

## Live

Real-network tests live under `tests/live/` and use the `live` build tag. They
build current images, start separate containers on an internal network, assert
public process results, and remove their containers, networks, volumes, images,
keys, and state before returning. Run them explicitly with `make live` on a
host with Docker.

Live scenarios are named for behavior such as authenticated Route transfer,
recovery, impairment, role capacity, or pressure. They never use numerical
stage/profile selectors. Each scenario is directly runnable and owns all prerequisites.
The current network suite checks successful authenticated transfer,
fail-closed behavior with a missing Route position, declared concurrent role
capacity, checked Route pressure/lifecycle under the declared cgroup profile,
and sustained Service Connections in both data directions through the
complete Route under real `tc/netem` delay, jitter, loss, and a finite link cap.
Each impaired direction uses 60-second direct baselines before and after its
batch, rejects baseline drift above 10%, samples endpoint CPU, RSS, carrier
traffic, and per-direction bitrate, and asserts Application-visible identity
and byte-stream continuity. It requires a locally built locked Carrier
tooling image, selected by its identity label or `ARDENTS_LIVE_TOOL_IMAGE`; this
is a live-host tool dependency, not a receipt from another test.

## Commands

- `make unit`: deterministic Module tests; no Docker or long campaigns.
- `make e2e`: real product commands in local processes.
- `make lab-test`: closed historical laboratory Modules, outside product testing and `make check`.
- `make live`: real product commands in containers and a real Docker network.
- `make quick-check`: architecture, vet, unit, build, and module tidiness.
- `make check`: unit, e2e, race, build, formatting, Staticcheck, and vulnerability checks launched concurrently.

`make check` launches its independent checks concurrently. Live tests require an explicit
Docker-capable job; lack of Docker is a failed live environment, not a skipped
passing test.
