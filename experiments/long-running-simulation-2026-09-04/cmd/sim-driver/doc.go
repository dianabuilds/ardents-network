//go:build ignore

// Command sim-driver is the R-138 long-running simulation's Go orchestrator
// for the S3.1 smoke slice. It owns two subcommands split across four
// responsibility files (timekeeper.go, observer.go, tripwires.go,
// fixtures.go):
//
//   - self-test — builds a synthetic tick event, exercises every trip-wire
//     with one input that SHOULD trip it and one that should not, and
//     prints PASS per trip-wire. Runs entirely in-process; no Docker
//     required. The slice 1 prebake machinery is NOT called; the
//     self-test only validates the trip-wire catalog and the observer
//     verdict structure.
//
//   - tick-loop — the 100-tick smoke run. Owns the tick loop, the
//     Timekeeper goroutine, and the Observer. Shells out to the
//     maintained `ardents refresh-sources` once per tick and writes
//     one tick.json per tick under evidence/ticks/. The prebake step
//     is delegated to a separate one-shot service in docker-compose.yml
//     that runs `test-driver-linux-amd64 prebake` from the slice 2
//     pilot; EnsureSourceState in fixtures.go is a defensive check
//     that the prebake has produced the expected fixtures before the
//     tick loop starts, NOT a substitute for the docker-compose
//     prebake service.
//
// S3.1 trip-wires (catalog in tripwires.go, all four exercised in
// self-test):
//
//   - generation_drift — the consumer-reported observed_digest must be
//     constant across the run (no adversary, no drift means the
//     digest is stable).
//   - source_exit      — ardents refresh-sources must exit 0; non-zero
//     means the source container is no longer responsive.
//   - consumer_parse_error — the consumer stdout/stderr must contain
//     exactly one ardents-source-event-v1 source-wave-accepted event;
//     missing or malformed events trip the wire.
//   - tick_budget      — a single tick must complete within the
//     configured wall-clock budget; the whole run must complete
//     within the run-level wall-clock budget.
//
// The binary is disposable and lives entirely under
// experiments/long-running-simulation-2026-09-04/. It is not a
// maintained command and is not registered in
// docs/development/package-map.md.
//
// S3.1 status: implemented, not accepted.
package main
