# E2E Scenarios

## Назначение

Каталог содержит scenario docs для e2e tests.

Каждый документ в этом каталоге обязан описывать один полный пользовательский,
операторский или системный поток от preconditions до observable result.

## Обязательная форма

Каждый scenario doc обязан содержать:

- `Scenario ID`
- `Layer: e2e`
- `Domain`
- `Category`
- `Goal`
- `Preconditions`
- `Steps`
- `Expected Result`
- `Failure/Degraded Variant`
- `Related Tests`
- `False Positive Risk`
- `False Negative Risk`
- `Notes`

## Traceability Contract

- каждый related test обязан использовать тот же `Scenario ID` в formal
  metadata рядом с test code;
- e2e scenario doc фиксирует полный observable flow, а runner/reporting path
  обязан уметь отразить его scenario-aware result;
- e2e test без scenario binding или scenario doc без runnable binding считаются
  coverage drift и подлежат remediation;
- новые e2e tests должны сохранять явную структуру preconditions, steps,
  expected assertions и failure/degraded assertions.

## Текущие сценарии

- [discovery-bootstrap-and-withdrawal.md](./discovery-bootstrap-and-withdrawal.md)
- [network-foundation-bootstrap-recovery.md](./network-foundation-bootstrap-recovery.md)
- [network-foundation-node-participation.md](./network-foundation-node-participation.md)
- [workload-hosted-service-lifecycle.md](./workload-hosted-service-lifecycle.md)
- [data-availability-peer-loss-recovery.md](./data-availability-peer-loss-recovery.md)
- [data-availability-terminal-failure.md](./data-availability-terminal-failure.md)
