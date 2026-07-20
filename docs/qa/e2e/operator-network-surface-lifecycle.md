# Operator Network Surface Lifecycle

- `Scenario ID`: `E2E-NET-SURFACE-001`
- `Layer`: `e2e`
- `Domain`: `Node Runtime + Network + Discovery + Diagnostics`
- `Category`: `functional`
- `Goal`: Проверить operator-facing lifecycle от старта узла до наблюдения
  network/discovery truth через публичную surface.
- `Preconditions`:
  доступен продуктовый runtime bootstrap;
  local/connectrpc control surface доступна оператору.
- `Steps`:
  1. Запустить узел.
  2. Получить `GetNodeStatus`, `GetNetworkStatus`, `GetDiscoveryStatus`.
  3. Подписаться на `StreamNodeEvents`.
  4. Выполнить route/discovery query.
  5. Остановить узел.
- `Expected Result`:
  оператор видит network readiness, degraded reasons, events и shutdown truth.
- `Failure/Degraded Variant`:
  при ограниченном transport profile или отсутствии usable route surface не
  выдает ложный ready-state.
- `Related Tests`:
  `tests/e2e/local-control-surface/public_surface_test.go::TestOperatorNetworkSurfaceLifecycle`
  `tests/e2e/network-operator-terminal/terminal_operator_test.go::TestTerminalNetworkSurfaceLifecycle`
- `False Positive Risk`:
  e2e проверяет только start/stop без network-facing assertions.
- `False Negative Risk`:
  сценарий зависит от нестабильного внешнего peer environment.
- `Notes`:
  сценарий должен оставаться explainable без просмотра внутренних пакетов.
