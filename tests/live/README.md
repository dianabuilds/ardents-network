# Live network tests

`make live` builds the current product image, starts the Client, four Route
positions, and Publisher in separate containers on an internal Docker network,
and verifies one authenticated end-to-end canary through their public process
output. The test owns setup, assertions, teardown, and image cleanup.

The maintained scenarios prove an authenticated four-position transfer,
fail-closed behavior when one required position is absent, four concurrent
Attachments per role process, checked `NORMAL/PROTECT/DRAIN/EXIT` behavior in
the constrained Route image, and sustained Service Connections in both data
directions under real bidirectional netem impairment. Each impaired direction
uses 60-second direct workloads before and after the batch, checks drift,
minute goodput, delivery gaps, queue high-water, CPU/RSS/carrier traffic, and
per-direction bitrate, and requires the same authenticated Target and exact
Application bytes without reconnect. Cleanup is part of the
assertion: project containers, networks, volumes, and the built image must all
be absent before a scenario passes. Keys and state live under Go test-owned
temporary directories.

The Stage 5 blocked-entry fixture is also owned here. It runs the ordinary C0
control, blocked C1/C2 success, C3/C4 terminal faults, C5/C6 external probes,
and the recovery-parent cell with distinct role, Application, policy, probe,
and observer namespaces. Its exact workload and pinned-binary instructions are
recorded in `docs/development/stage-5-blocked-entry-test.md`.

Live tests do not consume receipts from earlier runs, retain qualification
bundles, or expose stage/profile selectors. A failed scenario can be rerun
directly with `go test -tags=live ./tests/live/network -count=1`.

Impairment uses the locked Carrier tooling image containing `tc`. Set
`ARDENTS_LIVE_TOOL_IMAGE` explicitly or keep one local image labelled
`io.ardents.carrier-lab.target=tooling`. The live test validates that label and
fails when the host lacks the tool; it never treats a missing dependency as a
passing skip.
