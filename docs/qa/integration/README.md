# Integration Scenarios

## Назначение

Каталог содержит scenario docs для integration tests.

Каждый документ в этом каталоге обязан описывать один самостоятельный
integration scenario и ссылаться на соответствующие automated tests.

## Обязательная форма

Каждый scenario doc обязан содержать:

- `Scenario ID`
- `Layer: integration`
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
- scenario doc описывает intent и expected behavior, а test metadata и report
  фиксируют runnable binding;
- integration test без scenario binding или scenario doc без runnable binding
  считаются coverage drift и подлежат remediation;
- новые integration tests должны быть step-oriented: preconditions, steps,
  expected assertions и degraded/failure assertions должны быть различимы в коде.

## Текущие сценарии

- [discovery-control-surfaces.md](./discovery-control-surfaces.md)
- [data-substrate-retention-and-fetch.md](./data-substrate-retention-and-fetch.md)
- [data-replica-reservation-placement.md](./data-replica-reservation-placement.md)
- [data-chunked-transfer.md](./data-chunked-transfer.md)
- [data-availability-observation-repair.md](./data-availability-observation-repair.md)
- [diagnostics-domain-and-degraded-paths.md](./diagnostics-domain-and-degraded-paths.md)
- [local-control-surface-domain-and-connect-rpc.md](./local-control-surface-domain-and-connect-rpc.md)
- [local-control-surface-transport-truth.md](./local-control-surface-transport-truth.md)
- [discovery-domain-and-degraded-paths.md](./discovery-domain-and-degraded-paths.md)
- [node-runtime-startup-and-recovery.md](./node-runtime-startup-and-recovery.md)
- [network-foundation-constrained-transport-mode.md](./network-foundation-constrained-transport-mode.md)
- [network-foundation-abuse-controls.md](./network-foundation-abuse-controls.md)
- [network-foundation-relay-and-bootstrap.md](./network-foundation-relay-and-bootstrap.md)
- [policy-domain-and-diagnostics.md](./policy-domain-and-diagnostics.md)
- [workload-service-publication-sync.md](./workload-service-publication-sync.md)
- [workload-runtime-recovery.md](./workload-runtime-recovery.md)
- [workload-control-surfaces.md](./workload-control-surfaces.md)
