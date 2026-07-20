## Scenario ID

`NFE-001`

## Layer

`e2e`

## Domain

`Network Foundation / Messaging`

## Category

Node-managed transport participation, operator-visible degradation.

## Goal

Подтвердить полный node-level flow поверх canonical Waku foundation: узел
bootstrap-ится от удалённого peer, становится `ready`, а после потери peer
показывает operator-visible degraded transport/boot state.

## Preconditions

- удалённый bootstrap peer стартует на реальном `transport.Service`;
- локальный узел использует bootstrap endpoints удалённого peer как единственный
  network source;
- diagnostics и snapshot surface доступны через canonical node API.

## Steps

1. Запустить удалённый bootstrap peer.
2. Запустить локальный `node.Node` с bootstrap endpoints удалённого peer.
3. Дождаться `Boot.Joined = true`, `Boot.State = ready`, `Trans.State = ready`.
4. Остановить удалённый bootstrap peer.
5. Дождаться degraded transport/boot state на локальном узле.
6. Проверить structured primary reason по домену `boot` или `transport`.

## Expected Result

- node проходит реальный bootstrap на Waku-backed transport;
- node snapshot показывает `ready` transport participation до peer loss;
- после потери peer snapshot и diagnostics явно показывают degraded состояние.

## Failure/Degraded Variant

- узел не должен оставаться в ложном `ready`, если bootstrap peer потерян;
- degraded path должен быть explainable через structured primary reason.

## Related Tests

- `tests/e2e/network-foundation/participation_test.go::TestNodeTransportParticipationAndPeerLossVisibility`

## False Positive Risk

- тест должен проверять не только факт старта узла, но и конкретные boot/transport
  snapshot fields и наличие structured degraded reason.

## False Negative Risk

- деградация после peer loss асинхронна; тест не должен падать из-за single-shot
  assertion без bounded wait.

## Notes

- сценарий дополняет transport-level integration checks node-level operator flow.
