# Diagnostics Public Surface Explainability

- `Scenario ID`: `INT-DIAG-SURFACE-001`
- `Layer`: `integration`
- `Domain`: `Diagnostics + Local Control Surface`
- `Category`: `non-functional`
- `Goal`: Подтвердить, что новые status-oriented methods имеют explainable
  diagnostics path и не скрывают degraded/failure reasons.
- `Preconditions`:
  подготовлен один degraded или failed path в network/service/data scope.
- `Steps`:
  1. Вызвать affected status method.
  2. Запросить `GetHealthSummary`, `GetDiagnostics`, `ExplainFailure`.
  3. Проверить recent events и pending operations, если они релевантны.
- `Expected Result`:
  operator видит state, reason, impact и recovery guidance;
  status method и diagnostics surface не противоречат друг другу.
- `Failure/Degraded Variant`:
  при отсутствии explanation method возвращает formal error, а не silent empty
  object.
- `Related Tests`:
  `tests/integration/local-control-surface/public_surface_test.go::TestConnectRPCDiagnosticsSurfaceMatchesLocalTruth`
  `tests/integration/local-control-surface/cli_public_surface_test.go::TestCLIDiagnosticsSurfaceExplainsDegradedTruth`
- `False Positive Risk`:
  тест ограничивается проверкой non-empty response без meaningful reason.
- `False Negative Risk`:
  тест считает нестабильный event ordering частью обязательного контракта.
- `Notes`:
  diagnostics не должны раскрывать секретный material.
