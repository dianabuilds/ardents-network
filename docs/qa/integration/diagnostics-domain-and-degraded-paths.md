# Diagnostics Domain And Degraded Paths

- `Scenario ID`: `DII-001`
- `Layer`: `integration`
- `Domain`: `Diagnostics`
- `Category`: `health / pending operations / redaction / degraded recovery`

## Preconditions

- есть node-managed diagnostics ledger в `Data.Dir`;
- diagnostics domain владеет health summaries, recent events, pending operations
  и redaction-aware explainability;
- проверки идут через public node surfaces, а не через package-private helpers.

## Steps

1. Подготовить persisted diagnostics ledger с failed/degraded health reason,
   recent event и open operation, затем запустить `Node`.
2. Считать `DiagnosticsSnapshot()` и `PendingOperations()`.
3. Проверить, что active `Health` после успешного restart возвращается к `ready`,
   а persisted event и recovering operation остаются видны через operator-facing surface.
4. Подготовить malformed persisted operation и снова запустить `Node`.
5. Проверить, что malformed entry не исчезает, а становится `recovering` с
   explainable reason через diagnostics surface.
6. Запустить новый `Node` без подготовленного ledger и проверить, что live
   diagnostics surface публикует recent events через public node API.

## Expected Result

- persisted or imported discovery records with trust problems stay operator-visible:
  invalid/expired entries remain visible through `trust` subsystem health, while
  valid-but-untrusted entries remain visible through diagnostics events and
  discovery/peer trust surfaces;
- observed discovery trust truth must also re-project on ordinary runtime reads:
  if a previously healthy record expires after startup, operator-facing diagnostics
  must degrade without requiring a restart or a new import event;
- discovery status summaries must count expired remote records as both `stale`
  and `rejected`, so catalog summaries do not under-report unusable routing data;

- diagnostics truth после успешного restart возвращает active `Health` к `ready`,
  не подмешивая retained reason в живой runtime snapshot;
- pending operations остаются operator-visible и переходят в `recovering`;
- malformed persisted entry не скрывается, а остаётся explainable;
- diagnostics payload, health and operation projections доступны через
  canonical node surface.

## Failure/Degraded Variant

- trust degradation in the discovery catalog must remain visible after import and restart,
  but valid untrusted peers must not degrade whole-node health by themselves;

- corrupt или malformed diagnostics ledger не должен тихо терять state;
- degraded path обязан показывать operator-visible reason и сохранить pending
  operation в explainable форме;
- если persisted diagnostics truth не восстанавливается в node surface, это
  считается blocker для diagnostics domain close.

## Related Tests

- `tests/integration/diagnostics/domain_test.go::TestDiagnosticsProjectsUntrustedDiscoveryRecord`
- `tests/integration/diagnostics/domain_test.go::TestDiagnosticsProjectsPersistedInvalidDiscoveryRecordOnRestart`
- `tests/integration/diagnostics/domain_test.go::TestDiagnosticsObservedTruthProjectsExpiredDiscoveryRecordWithoutRestart`

- `tests/integration/diagnostics/domain_test.go::TestDiagnosticsRestartKeepsRetainedExplainabilityWithoutMaskingActiveHealth`
- `tests/integration/diagnostics/domain_test.go::TestDiagnosticsRestartKeepsMalformedPendingOperationVisible`
- `tests/integration/diagnostics/domain_test.go::TestDiagnosticsSurfaceIncludesRecentEvents`

## False Positive Risk

- проверка только package-local `Recorder` дала бы ложный зелёный статус без
  подтверждения public node surface;
- проверка только count-based assertions пропустила бы потерю reason code или
  recovering state.

## False Negative Risk

- излишняя привязка к точному порядку unrelated events дала бы шумный провал;
- чтение snapshot без restart path не поймало бы drift между persisted ledger и
  node-managed diagnostics surface.
