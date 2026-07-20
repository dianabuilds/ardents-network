## Scenario ID

`DKE-001`

## Layer

`e2e`

## Domain

`Discovery`

## Category

Multi-node bootstrap, network-fed discovery, remote withdrawal.

## Goal

Подтвердить, что discovery живёт как часть реальной сети:
новый узел bootstrap-ится от peer transport, получает remote records и перестаёт видеть withdrawn service после remote stop.

## Preconditions

- есть удалённый узел с transport participation;
- удалённый workload публикует network-backed service;
- локальные клиенты стартуют только от bootstrap endpoints удалённого peer.

## Steps

1. Запустить удалённый узел с workload-backed service.
2. Считать его bootstrap endpoints из опубликованной node record.
3. Запустить новый клиент, используя эти endpoints как единственный bootstrap source.
4. Проверить, что клиент получает remote service и usable node route из bootstrap-fed discovery.
5. Остановить удалённый workload и запустить нового клиента с теми же bootstrap endpoints.
6. Дождаться, пока withdrawn service исчезнет из observable discovery results.

## Expected Result

- bootstrap discovery доставляет remote records в новый узел без ручного import/export;
- remote node record разрешается в usable route;
- после withdrawal новый клиент больше не видит удалённый service match.

## Failure/Degraded Variant

- если bootstrap peer недоступен, сервис не должен появиться как ложноположительный match;
- withdrawn service не должен оставаться видимым для нового клиента за счёт stale publication.

## Related Tests

- `tests/e2e/discovery/bootstrap_test.go`

## False Positive Risk

- тест не должен ограничиваться проверкой `len(matches) > 0`; нужен assert на bootstrap-derived route outcome и исчезновение withdrawn service.

## False Negative Risk

- withdrawal propagation требует bounded wait; тест не должен падать из-за мгновенного single-shot assertion.

## Notes

- сценарий закрывает multi-node discovery coverage, ранее оставшуюся в `internal/node/node_discovery_test.go`.
