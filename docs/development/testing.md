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
lifecycle, Route selection and transfer, plus Service command readiness,
bounded failure, and cleanup. They do not claim a cross-process recovery path
until a maintained Route Attachment command owns that boundary.

## Live

Real-network tests live under `tests/live/` and use the `live` build tag. They
build current images, start separate containers on an internal network, assert
public process results, and remove their containers, networks, volumes, images,
keys, and state before returning. Run them explicitly with `make live` on a
host with Docker.

Live scenarios are named for behavior such as authenticated Route transfer,
recovery, impairment, or pressure. They never use numerical stage/profile
selectors. Each scenario is directly runnable and owns all prerequisites.
The current network suite checks successful authenticated transfer and
fail-closed behavior with a missing Route position; additional behavior is
added here only when its production command seam exists.

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
