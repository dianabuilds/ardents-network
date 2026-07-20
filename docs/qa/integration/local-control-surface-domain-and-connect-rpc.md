# Scenario ID

`LCI-001`

## Layer

`integration`

## Domain

`Local Control Surface`

## Category

Local control surface and Connect thin binding validation.

## Goal

Подтвердить, что operator-facing local control surface и Connect binding
остаются thin adapter path над runtime-backed domain contracts: node truth,
diagnostics truth, data publication, canonical error mapping и mandatory
exact-action enforcement должны наблюдаться без отдельного API-мира.

## Preconditions

- runtime-backed `node.Node` can be started with writable local data dir;
- operator-facing surface is exposed directly from runtime/domain contracts;
- connect handler is served through `internal/transport/connectrpc.NewHandler`
  with valid auth config.

## Steps

1. Запустить runtime-backed node и transport binding.
2. Через Connect RPC запросить `GetNodeStatus` и `GetDiagnostics`.
3. Через Connect RPC выполнить blob/manifest publication и запросить
   `GetDataInventory`.
4. Через operator-facing query запросить missing object и проверить error
   mapping.
5. Через Connect RPC отправить запрос без `Authorization` header.
6. Через Connect RPC отправить read query с token, у которого нет
   exact target action capability.
7. Через Connect RPC отправить запрос со сломанным auth header: token без схемы
   `Bearer`.

## Expected Result

- connect RPC mutate paths preserve canonical `not_found` and `already_exists`
  semantics instead of collapsing to generic internal failures;
- node and diagnostics snapshots возвращают operator-visible truth;
- data publication и inventory проходят через thin binding без потери state;
- `not_found`, `unauthenticated` и `permission_denied` выражаются через
  canonical error model;
- `internal/transport/connectrpc` остаётся thin adapter над runtime/domain APIs.

## Failure/Degraded Variant

- abandoned node event subscribers must not leave a stuck stream bridge;
- missing mutate targets stay `not_found`, and duplicate workload registration
  stays `already_exists`;
- missing local object маппится в `not_found` без ложного `internal_failure`;
- отсутствующий auth header даёт `unauthenticated`;
- authenticated token without the exact target action gives `permission_denied`;
- token без схемы `Bearer` даёт `unauthenticated`;
- retain operation для announced-only blob остаётся explainable failure path.

## Related Tests

- `tests/integration/local-control-surface/domain_test.go::TestConnectRPCMutationsPreserveConflictAndNotFoundCodes`
- `tests/integration/local-control-surface/domain_test.go::TestConnectRPCExposesNodeAndDiagnostics`
- `tests/integration/local-control-surface/domain_test.go::TestConnectRPCProjectsDegradedHostedServiceAndDiagnostics`
- `tests/integration/local-control-surface/domain_test.go::TestConnectRPCReadRequiresExactAction`
- `tests/integration/local-control-surface/domain_test.go::TestConnectRPCDataRoundTripAndErrors`

## False Positive Risk

Если тест использует fake runtime или обходит
`internal/transport/connectrpc.NewHandler`, он перестаёт проверять настоящий
thin binding path.

## False Negative Risk

Если тест не проверяет structured error codes/categories и только факт ошибки,
можно пропустить drift в canonical local error model и authz contract.

## Notes

Сценарий фиксирует canonical thin-binding contract между runtime/domain APIs и
`internal/transport/connectrpc` для operator-facing flows.
