# Network Foundation Performance Safety Baseline

## Scenario ID

`NFI-004`

## Layer

`integration`

## Domain

`Network Foundation / Messaging`

## Category

Non-functional Waku relay/store performance and bounded-resource safety.

## Goal

Prove that the representative two-node service profile starts, connects,
delivers a bounded encrypted message batch, queries Store, and shuts down within
the release thresholds declared by STB-704.

## Preconditions

- both peers use the canonical Waku transport implementation;
- the peers bind only to the container loopback interface;
- the local peer bootstraps from endpoints observed from the remote peer;
- relay messages are sealed through the real privacy channel.

## Steps

1. Start a remote service node and create a relay subscription.
2. Publish a retained encrypted sentinel to the remote Store.
3. Start a local service node, bootstrap it, and wait for relay readiness.
4. Assert bounded peer and relay connection counts.
5. Publish 16 distinct encrypted messages and receive the exact 16 ciphertexts.
6. Calculate batch throughput and p95 end-to-end delivery latency.
7. Fetch the retained sentinel from the remote Store.
8. Stop both nodes explicitly and measure shutdown.

## Expected Result

Every functional result is observed and every PERF-01 threshold passes. The
test logs measured startup, connection, delivery, Store, and shutdown values.

## Failure/Degraded Variant

Missing or duplicate delivery, an empty Store result, an unbounded peer count,
or any timing threshold breach fails the gate. Context expiration is reported as
a test failure rather than an indefinite wait.

## Related Tests

- `tests/integration/network-foundation/performance_test.go::TestTransportPerformanceSafetyBaseline`
- `docs/process/v1-stabilization-hardening/stb-704-performance-profile.md`

## False Positive Risk

Checking only elapsed time could pass a broken transport. The scenario matches
every received ciphertext to a message actually published and requires the
retained Store sentinel to be present.

## False Negative Risk

Host scheduling jitter can affect wall time. Thresholds are intentionally much
wider than expected local measurements, the batch is small, and all waits are
bounded by the test context.

## Notes

This is a safety baseline rather than a capacity benchmark. Internet and
multi-host scale claims require separate evidence.
