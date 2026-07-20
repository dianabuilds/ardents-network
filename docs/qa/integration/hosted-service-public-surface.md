# Hosted Service Public Surface

- `Scenario ID`: `INT-SVC-SURFACE-001`
- `Layer`: `integration`
- `Domain`: `Workload Control + Hosted Services + Local Control Surface`
- `Category`: `functional`
- `Goal`: Подтвердить, что hosted service surface отражает publication truth,
  derived from workload runtime truth.
- `Preconditions`:
  node runtime доступен;
  зарегистрирован workload с published service capability.
- `Steps`:
  1. Зарегистрировать и запустить workload.
  2. Запросить `GetWorkload`, `ListHostedServices`,
     `GetServicePublicationStatus`.
  3. Остановить или disable workload.
  4. Повторить service-status queries.
- `Expected Result`:
  service published только при наличии runtime backing;
  status и publication reason меняются после остановки workload;
  local/connectrpc responses согласованы.
- `Failure/Degraded Variant`:
  policy denial или потеря runtime backing снимает publication и объясняется
  через reason/operator_action_required.
- `Related Tests`:
  `tests/integration/local-control-surface/public_surface_test.go::TestConnectRPCHostedServiceSurfaceMatchesLocalTruth`
  `tests/integration/local-control-surface/cli_public_surface_test.go::TestCLIHostedServiceSurfaceTracksWorkloadPublicationTruth`
- `False Positive Risk`:
  surface возвращает declarative registry state вместо runtime truth.
- `False Negative Risk`:
  тест не учитывает асинхронность reconcile/publication loop.
- `Notes`:
  publication truth нельзя проверять только по workload desired state.
