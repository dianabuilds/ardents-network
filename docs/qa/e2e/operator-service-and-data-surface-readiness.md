# Operator Service And Data Surface Readiness

- `Scenario ID`: `E2E-SVC-DATA-SURFACE-001`
- `Layer`: `e2e`
- `Domain`: `Workload + Hosted Services + Data + Diagnostics`
- `Category`: `functional`
- `Goal`: Проверить, что оператор через публичную surface видит service
  publication truth и data transfer truth как части одного runtime.
- `Preconditions`:
  доступен workload-capable runtime;
  data exchange path доступен.
- `Steps`:
  1. Зарегистрировать workload с hosted service.
  2. Запустить workload и проверить publication status.
  3. Выполнить data publish/fetch flow.
  4. Проверить service/data/diagnostics status.
  5. Остановить workload и убедиться, что publication truth изменилась.
- `Expected Result`:
  service publication и data transfer surface привязаны к runtime truth;
  оператор может понять текущее состояние без обращения к внутренним деталям.
- `Failure/Degraded Variant`:
  при потере runtime backing или data source surface показывает explainable
  unavailable/failed reason.
- `Related Tests`:
  `tests/e2e/local-control-surface/public_surface_test.go::TestOperatorServiceAndDataSurfaceReadiness`
  `tests/e2e/network-operator-terminal/terminal_operator_test.go::TestTerminalServiceAndDataSurfaceReadiness`
- `False Positive Risk`:
  publication status не зависит от реального workload state.
- `False Negative Risk`:
  сценарий падает из-за среды, а не из-за product behavior.
- `Notes`:
  сценарий должен включать хотя бы один operator-visible degraded path.
