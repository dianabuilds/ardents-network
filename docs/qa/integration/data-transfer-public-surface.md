# Data Transfer Public Surface

- `Scenario ID`: `INT-DATA-SURFACE-001`
- `Layer`: `integration`
- `Domain`: `Data Substrate + Network + Diagnostics`
- `Category`: `functional`
- `Goal`: Confirm that data-source and transfer public surfaces expose real
  fetch/re-serve runtime truth, including stale-source visibility.
- `Preconditions`:
  node runtime is available;
  a blob is published or a remote source is modeled;
  the data exchange path is enabled.
- `Steps`:
  1. Request `ListBlobSources` for a known blob.
  2. Run `FetchBlob`.
  3. Request `GetTransfer` and `ListTransfers`.
  4. Verify inventory and blob status after transfer completion.
  5. Surface a remote source with stale `LastSeenAt` and request
     `ListBlobSources` again through local, ConnectRPC, and CLI surfaces.
- `Expected Result`:
  source lists and transfer snapshots reflect runtime truth;
  progress, state, and reason update in an explainable way;
  successful fetch changes observable data state;
  a stale remote source stops looking usable and carries an explicit stale
  reason.
- `Failure/Degraded Variant`:
  when no usable source exists or a transfer fails, the public surface shows
  `unavailable`, `failed`, or `stale` truth without a fake success path.
- `Related Tests`:
  `tests/integration/local-control-surface/public_surface_test.go::TestConnectRPCDataTransferSurfaceMatchesLocalTruth`
  `tests/integration/local-control-surface/public_surface_test.go::TestConnectRPCDataSurfaceMarksStaleRemoteSourceUnusable`
  `tests/integration/local-control-surface/cli_public_surface_test.go::TestCLIDataTransferSurfaceReflectsFetchRuntimeTruth`
  `tests/integration/local-control-surface/cli_public_surface_test.go::TestCLIDataSurfaceShowsStaleRemoteSourceAsUnusable`
- `False Positive Risk`:
  transfer status appears without real fetch runtime, or stale sources still
  look usable because the surface reads inert persisted metadata.
- `False Negative Risk`:
  the test depends on unstable timing for transfer progress updates or stale
  classification.
- `Notes`:
  Canonical owner for source and transfer truth is `Data Substrate`; this
  scenario must fail if the public surface is reconstructed from diagnostics
  without reading the persisted domain ledgers.
  The scenario must verify both completion truth and degraded stale-source
  explainability.
