## Scenario ID

`LCI-002`

## Layer

`integration`

## Domain

`Local Control Surface / Transport Truth`

## Category

Connect RPC projection of transport profile truth, transport mode truth,
bootstrap truth and degraded failure.

## Goal

Подтвердить, что operator-facing local control surface и diagnostics surface
показывают реальную transport truth для bootstrapped node: `ready` в нормальном
режиме и explainable `degraded` после потери bootstrap peer.

## Preconditions

- есть удалённый `transport.Service`, доступный как реальный bootstrap peer;
- node запускается через canonical `Waku` foundation c boot sources удалённого
  peer;
- connect RPC client читает `GetNodeStatus` и `GetDiagnostics` через
  authenticated path.

## Steps

1. Запустить удалённый `transport.Service`.
2. Запустить `node.Node` с `Boot.Sources`, равным endpoint set удалённого peer.
3. Через connect RPC дождаться `snapshot.boot.state = ready`,
   `snapshot.trans.state = ready` и `snapshot.boot.joined = true`.
4. Подтвердить, что `snapshot.boot.source` совпадает с активным bootstrap source
   set.
5. Остановить удалённый bootstrap peer.
6. Через connect RPC дождаться `snapshot.boot.state = degraded`,
   `snapshot.trans.state = degraded`.
7. Через diagnostics surface подтвердить primary reason по домену `boot` или
   `transport`.

## Expected Result

- local control surface показывает реальный transport profile, transport mode и
  bootstrap truth;
- bootstrap source не теряется между `ready` и `degraded` состояниями;
- diagnostics surface объясняет degraded path через `boot` или `transport`
  reason, а не через unrelated subsystem.

## Failure/Degraded Variant

- если connect surface остаётся `ready` после потери bootstrap peer, сценарий
  должен падать;
- если `boot.source` не совпадает с реальным bootstrap set, сценарий должен
  падать;
- если diagnostics не дают explainable primary reason по `boot` или `transport`,
  сценарий должен падать.

## Related Tests

- `tests/integration/local-control-surface/domain_test.go::TestConnectRPCProjectsTransportModeAndPeerLossTruth`
- `tests/integration/local-control-surface/domain_test.go::TestConnectRPCProjectsTCPWSSProfileTruth`

## False Positive Risk

- проверка только package-local snapshot без connect surface может скрыть drift
  между runtime truth и operator-visible projection.

## False Negative Risk

- peer-loss degradation асинхронна; тест должен ждать bounded convergence, а не
  читать snapshot один раз сразу после stop.

## Notes

- сценарий закрывает operator-visible truth obligation из
  `docs/network-transport-variants-requirements.md` для `ready` и degraded
  transport состояний.
