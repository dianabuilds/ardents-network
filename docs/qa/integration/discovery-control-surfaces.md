## Scenario ID

`DKI-001`

## Layer

`integration`

## Domain

`Discovery`

## Category

Control surfaces, import/reject flows, stopped-node route visibility.

## Goal

Подтвердить, что discovery control surfaces импортируют валидные записи,
отклоняют stale import и не выдают usable candidates после остановки узла.

## Preconditions

- есть локальный узел с активной operator-facing control surface;
- есть удалённый узел, публикующий discovery records;
- операторский вызов авторизован.

## Steps

1. Импортировать удалённую discovery record через `ImportRecord`.
2. Разрешить импортированную запись через `ResolveRecord` и проверить observable
   trust/result fields.
3. Повторно отправить stale-вариант той же записи и подтвердить `rejected`.
4. Импортировать удалённые node/service records в локальный узел, затем
   остановить локальный узел.
5. Проверить через `ResolveService` и `ListRouteCandidates`, что usable routes
   после stop больше не выдаются.

## Expected Result

- валидная импортированная запись видна как `found`;
- stale import возвращает `rejected` и не подменяет уже принятую truth;
- после `node stop` discovery results могут оставаться explainable, но route
  outcome становится `not_found`, а список usable candidates пуст.

## Failure/Degraded Variant

- неавторизованный вызов не должен модифицировать discovery state;
- остановленный узел не должен продолжать выдавать usable transport targets из
  устаревшего runtime state.

## Related Tests

- `tests/integration/discovery/control_surfaces_test.go::TestLocalDiscoveryResolveImportedRecord`
- `tests/integration/discovery/control_surfaces_test.go::TestLocalDiscoveryRejectsStaleImport`
- `tests/integration/discovery/control_surfaces_test.go::TestLocalDiscoveryCandidatesAreNotUsableAfterNodeStop`

## False Positive Risk

- тест пройдёт только по отсутствию ошибок, но не проверит `Outcome`, `Trust` и
  пустые candidates после stop.

## False Negative Risk

- тест станет флакать, если опираться на неявные тайминги вместо явной
  последовательности import -> resolve -> stop -> resolve.

## Notes

- сценарий замещает discovery-specific mixed tests из удалённого legacy
  local-surface слоя.
