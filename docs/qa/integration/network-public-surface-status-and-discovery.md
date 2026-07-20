# Network Public Surface Status And Discovery

- `Scenario ID`: `INT-NET-SURFACE-001`
- `Layer`: `integration`
- `Domain`: `Network + Discovery + Local Control Surface`
- `Category`: `functional`
- `Goal`: Подтвердить, что network status, discovery status и peer/discovery
  queries через local/connectrpc surfaces отражают одну и ту же runtime truth.
- `Preconditions`:
  node runtime запущен;
  transport/discovery stack инициализирован;
  есть локальный presence и хотя бы один remote discovery input или его явное
  отсутствие.
- `Steps`:
  1. Запросить `GetNodeStatus`, `GetNetworkStatus` и `GetDiscoveryStatus`.
  2. Запросить `ListPeers` и `ListRecords`.
  3. Выполнить `ResolveRecord` и `ResolveService`.
  4. Сравнить ответы local и connectrpc path.
- `Expected Result`:
  status queries согласованы между собой;
  discovery counts, reasons и route usability explainable;
  local/connectrpc surfaces не расходятся по domain truth.
- `Failure/Degraded Variant`:
  при отсутствии usable route или peer continuity status и resolve responses
  показывают degraded/unavailable reason, а не пустой happy-path.
- `Related Tests`:
  `tests/integration/local-control-surface/public_surface_test.go::TestConnectRPCNetworkPublicSurfaceMatchesLocalTruth`
  `tests/integration/local-control-surface/cli_public_surface_test.go::TestCLINetworkPublicSurfaceReflectsLocalTruth`
- `False Positive Risk`:
  тест проверяет только transport success и не сверяет discovery/public status.
- `False Negative Risk`:
  тест излишне завязан на нестабильный peer set или timing refresh loop.
- `Notes`:
  нужно проверять reasons и timestamps, а не только state.
