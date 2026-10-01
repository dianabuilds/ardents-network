// Package qualification owns shared resource measurement, worker lifetime
// accounting, completion barriers, Introduction admission pacing, and the
// retained-run scenario orchestration for an installed multi-participant
// qualification run. On Linux it drives one measured workload through the
// bounded, authorized participant operations that Endpoint supplies as a
// Session; every authorization check stays in Endpoint. It does not select
// participants, launch workers, own Service authority, or define the measured
// workload.
package qualification
