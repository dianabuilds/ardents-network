# Live network tests

`make live` builds the current product image, starts the Client, four Route
positions, and Publisher in separate containers on an internal Docker network,
and verifies one authenticated end-to-end canary through their public process
output. The test owns setup, assertions, teardown, and image cleanup.

The maintained scenarios prove an authenticated four-position transfer and
fail-closed behavior when one required position is absent. Cleanup is part of
the assertion: project containers, networks, volumes, and the built image must
all be absent before a scenario passes. Keys and state live under Go test-owned
temporary directories.

Live tests do not consume receipts from earlier runs, retain qualification
bundles, or expose stage/profile selectors. A failed scenario can be rerun
directly with `go test -tags=live ./tests/live/network -count=1`.
