# Дублирование и legacy

## Вывод

В production-коде не обнаружен второй параллельный runtime, старый HTTP control server, vendor-копия библиотек или отдельная «v1» доменная реализация. Поиск `TODO/FIXME/HACK`, deprecated declarations, неиспользуемых каталогов и альтернативных entry points не дал подтверждённых кандидатов, которые можно безопасно удалить без дополнительного product decision.

Основной legacy-долг находится в трёх местах: документация старой модели доступа, разошедшиеся клиентские реализации одного session protocol и устаревшие имена/acceptance-описания в CI/deployment.

## Подтверждённые параллельные представления

### Два клиента реализуют один Principal session handshake

`internal/cli/client/session.go` и `sdk/go/internal/adapter/session.go` независимо реализуют Begin/Complete session protocol, проверку challenge timestamps, подпись и проверку результата. Это не просто похожий boilerplate: реализации уже разошлись.

- CLI получает `now` до Begin RPC и использует это значение до конца обмена (`internal/cli/client/session.go:103-109,137,159-209`).
- SDK повторно получает время после сетевых вызовов (`sdk/go/internal/adapter/session.go:186,234`).
- В результате CLI иногда отвергает валидный challenge при переходе секундной границы, а SDK — нет (CLI-001).

Направление консолидации: вынести protocol-state machine и timestamp validation в общий внутренний client component либо сделать CLI клиентом публичного SDK. Transport, signer и persistence должны остаться передаваемыми зависимостями; копировать security-sensitive handshake третий раз нельзя.

### Policy resource и persistence key описывают разную identity

Для `content-object`/`content-manifest` authorization-модель использует `(Node, Owner, Kind, ID)`, тогда как content catalog индексирует только по `ID`. Это не буквальное копирование кода, но два источника истины для одной object identity. Расхождение уже приводит к cross-owner read/overwrite (SEC-001).

Направление консолидации: выбрать один canonical identity contract, применить его одновременно к API target, domain aggregate, store key и миграции данных. Локальная проверка Owner в handler без изменения ключа была бы неполной компенсацией.

### Два replay ledger могут разделить один физический store

Discovery и data messaging создают независимые `DurableReplayLedger` с одинаковыми bucket/key и отдельными in-memory snapshots. Конфигурация требует разные строки путей, но не разные canonical files. При filesystem aliases обе «независимые» реализации начинают поочерёдно перезаписывать один snapshot (SEC-005).

Направление консолидации: либо один process-owned ledger с namespaces, либо строго подтверждённые разные canonical file identities. Выбор требует ADR, потому что меняет persistence ownership.

## Legacy-документация

Старая модель — token-authenticated loopback control API:

- `README.md:22,36,102`;
- `docs/protocols/communication-contracts.md:45-56`;
- `docs/product/distribution-model.md:42,46`;
- `docs/operations/operator-runbook.md:22`.

Текущая модель — Principal-authenticated Operator/Application Unix sockets без permanent shared operator token:

- `docs/adr/0001-separate-application-interface.md`;
- `docs/adr/0002-principal-centered-identity-and-access.md`;
- `docs/product/principal-identity-and-access.md:703,728,873-886`;
- `docs/operations/operator-access-contract.md:22,37,43`;
- фактические binders/interceptors в `internal/localapi` и `internal/applicationapi`.

Старый runtime-код не найден, поэтому это кандидат не на удаление кода, а на удаление/переписывание устаревших нормативных инструкций (DOC-001).

## Legacy-имена и устаревшие ссылки

| Остаток | Доказательство | Практическое следствие | Связанное замечание |
|---|---|---|---|
| `cluster-data.ps1` | `scripts/deploy/rollout.ps1:107-110`; существует только `scripts/deploy/data.ps1` | Официальный Compose upgrade всегда падает до backup | OPS-001 |
| `tests/cmd/testcatalog -mode validate` | `.github/workflows/ci.yml:69-71`; актуальный tool — `tests/tooling/testcatalog`, flag `-mode` отсутствует | Static job не может пройти | CI-001 |
| «восемь generated bounded services» | `docs/engineering/codebase-architecture.md:1014`; proto определяет девять service | Acceptance architecture больше не отражает API | ARCH-001 |
| Запрет tracked `.agents` | `docs/engineering/codebase-architecture.md:346`; в commit 11 файлов `.agents` | Target-tree gate и фактический tree расходятся | ARCH-001 |
| Package budget ≤12 | `docs/engineering/codebase-architecture.md:722,1009,1049-1050`; восемь пакетов выше budget | Формальный acceptance не воспроизводится | ARCH-001 |

## Generated и vendor

- 28 Go-файлов определены как generated protobuf/Connect code; они инвентаризированы и проверены generation gate, но не оценивались как handwritten design.
- `vendor/` отсутствует.
- Tracked `build/`, `dist/` и бинарных артефактов нет.
- `internal/messaging/private.proto` и `private.pb.go` — единственное заметное смешение generated contract с handwritten domain package. Это не мёртвый код; перенос требует сохранить protobuf compatibility.

## Кандидаты на удаление

Подтверждённых production-кандидатов на немедленное удаление нет. Следующие действия допустимы только после фиксации нового источника истины:

1. удалить token/loopback инструкции после переписывания entry documentation;
2. убрать одну client session implementation после миграции всех CLI/SDK callers и parity tests;
3. заменить устаревшие CI/deploy references на актуальные имена и затем добавить self-validation;
4. решить судьбу repository-local `.agents` в architecture target tree;
5. при переносе `private.proto` удалить старое generated расположение только после совместимого regeneration.

## Этапы консолидации

1. **Сначала восстановить gates.** Исправить CI/deploy references, чтобы дальнейшие изменения имели проверяемый путь.
2. **Зафиксировать identity и persistence ADR.** Canonical Object/Manifest identity и replay-store ownership нельзя исправлять независимыми локальными патчами.
3. **Свести session clients.** Добавить общий timing/failure contract и differential tests CLI против SDK.
4. **Очистить narrative.** README, protocols, product и runbook должны ссылаться на один Principal access contract.
5. **Обновить architecture acceptance.** Service count, file budget, package docs и `.agents` должны либо снова выполняться, либо быть явно пересмотрены.
