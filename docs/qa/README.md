# QA Docs

## Назначение

Каталог `docs/qa/` хранит тестовую модель проекта и сценарные документы для repository-level
`integration` и `e2e` suites, а также методики active testing для controlled
exploratory weakness discovery.

Этот каталог нужен, чтобы:

- фиксировать test-layer split;
- держать сценарии рядом с реальными suite layers;
- поддерживать связность между test metadata, scenario IDs и automated tests;
- не допускать drift между test intent и текущим кодом.

## Актуальная структура

```text
docs/qa/
  README.md
  test-model.md
  unit-tests.md
  active-testing/
    README.md
    mutation-testing.md
    chaos-testing.md
    fault-injection-and-parameter-perturbation.md
  integration/
    README.md
    data-substrate-retention-and-fetch.md
    diagnostics-domain-and-degraded-paths.md
    discovery-control-surfaces.md
    discovery-domain-and-degraded-paths.md
    local-control-surface-domain-and-connect-rpc.md
    local-control-surface-transport-truth.md
    network-foundation-constrained-transport-mode.md
    network-foundation-relay-and-bootstrap.md
    network-foundation-store-and-subscriptions.md
    node-runtime-startup-and-recovery.md
    policy-domain-and-diagnostics.md
    workload-control-surfaces.md
    workload-runtime-recovery.md
    workload-service-publication-sync.md
  e2e/
    README.md
    data-substrate-fetch-and-unavailability.md
    discovery-bootstrap-and-withdrawal.md
    network-foundation-node-participation.md
    node-runtime-lifecycle-and-recovery.md
    workload-hosted-service-lifecycle.md
```

## Правила

- `unit` coverage фиксируется в [Unit Tests](./unit-tests.md).
- Active testing methods фиксируются в [Active Testing](./active-testing/README.md).
- Каждый repository-level `integration` test обязан иметь scenario document в `docs/qa/integration/`.
- Каждый repository-level `e2e` test обязан иметь scenario document в `docs/qa/e2e/`.
- Canonical metadata для repository-level tests живет рядом с кодом через `tests/testkit`.
- Scenario docs остаются source of truth для intent, preconditions, steps и expected outcomes.
- Active testing docs не создают новый test layer и не подменяют canonical regression model.
- Tagged suites остаются opt-in через build tags `integration` и `e2e`.
- Общий cross-package harness должен жить в `tests/testkit/`, а не копироваться между test packages.

## Связанные документы

- [Test Model](./test-model.md)
- [Requirements Coverage](./requirements-coverage.md)
- [Unit Tests](./unit-tests.md)
- [Active Testing](./active-testing/README.md)
- [Integration Scenarios](./integration/README.md)
- [E2E Scenarios](./e2e/README.md)
- [Process Template Kit](../process/process-template/README.md)
